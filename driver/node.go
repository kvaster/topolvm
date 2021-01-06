package driver

import (
	"context"
	"os"
	"strconv"
	"sync"

	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/csi"
	"github.com/topolvm/topolvm/driver/k8s"
	"github.com/topolvm/topolvm/lvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/utils/mount"
)

const (
	ephVolConKey     = "csi.storage.k8s.io/ephemeral"
)

var nodeLogger = ctrl.Log.WithName("driver").WithName("node")

// NewNodeService returns a new NodeServer.
func NewNodeService(nodeName string, client lvm.Client, service *k8s.LogicalVolumeService) csi.NodeServer {
	return &nodeService{
		nodeName:     nodeName,
		k8sLVService: service,
		client:       client,
	}
}

type nodeService struct {
	csi.UnimplementedNodeServer

	nodeName     string
	k8sLVService *k8s.LogicalVolumeService
	mu           sync.Mutex
	client           lvm.Client
}

func (s *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeContext := req.GetVolumeContext()
	volumeID := req.GetVolumeId()

	nodeLogger.Info("NodePublishVolume called",
		"volume_id", volumeID,
		"publish_context", req.GetPublishContext(),
		"target_path", req.GetTargetPath(),
		"volume_capability", req.GetVolumeCapability(),
		"read_only", req.GetReadonly(),
		"num_secrets", len(req.GetSecrets()),
		"volume_context", volumeContext)

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume_capability is provided")
	}
	isFsVol := req.GetVolumeCapability().GetMount() != nil
	if !isFsVol {
		return nil, status.Errorf(codes.InvalidArgument, "no supported volume capability: %v", req.GetVolumeCapability())
	}
	isInlineEphemeralVolumeReq := volumeContext[ephVolConKey] == "true"

	s.mu.Lock()
	defer s.mu.Unlock()

	var lv *lvm.LogicalVolume
	var err error
	if isInlineEphemeralVolumeReq {
		lv, err = s.getLvFromContext(ctx, topolvm.DefaultDeviceClassName, volumeID)
		if err != nil {
			return nil, err
		}
		// Need to check if the LV already exists so this block is idempotent.
		if lv == nil {
			var reqBytes uint64 = topolvm.DefaultSize
			if sizeStr, ok := volumeContext[topolvm.EphemeralVolumeSizeKey]; ok {
				var err error
				reqBytes, err = strconv.ParseUint(sizeStr, 10, 64)
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "Invalid size: %s", sizeStr)
				}
			}
			nodeLogger.Info("Processing ephemeral inline volume request", "reqBytes", reqBytes)
			_, err = s.client.CreateLV(volumeID, topolvm.DefaultDeviceClassName, reqBytes, []string{"ephemeral"})
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create LV %v", err)
			}
			lv, err = s.getLvFromContext(ctx, topolvm.DefaultDeviceClassName, volumeID)
			if err != nil {
				return nil, err
			}
		}
	} else {
		lvr, err := s.k8sLVService.GetVolume(ctx, volumeID)
		if err != nil {
			return nil, err
		}
		lv, err = s.getLvFromContext(ctx, lvr.Spec.DeviceClass, volumeID)
		if err != nil {
			return nil, err
		}
	}
	if lv == nil {
		return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", volumeID)
	}

	_, err = s.nodePublishFilesystemVolume(req, lv)

	if err != nil {
		if isInlineEphemeralVolumeReq {
			// In the case of an inline ephemeral volume, there is no
			// guarantee that NodePublishVolume will be called again, so if
			// anything fails after the volume is created we need to attempt to
			// clean up the LVM so we don't leak storage space.
			if err = s.client.RemoveLV(volumeID, topolvm.DefaultDeviceClassName); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to remove LV for %s: %v", volumeID, err)
			}
		}
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishFilesystemVolume(req *csi.NodePublishVolumeRequest, lv *lvm.LogicalVolume) (*csi.NodePublishVolumeResponse, error) {
	// Check request
	mountOption := req.GetVolumeCapability().GetMount()
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	if accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s", modeName)
	}

	sourcePath := s.client.GetPath(lv.Name, lv.DeviceClass)

	err := os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", req.GetTargetPath(), err)
	}
	mounter := mount.New("")
	if err := mounter.Mount(sourcePath, req.GetTargetPath(), "", mountOption.GetMountFlags()); err != nil {
		return nil, status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
	}
	if err := os.Chmod(req.GetTargetPath(), 0777|os.ModeSetgid); err != nil {
		return nil, status.Errorf(codes.Internal, "chmod 2777 failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	nodeLogger.Info("NodePublishVolume(fs) succeeded",
		"volume_id", req.GetVolumeId(),
		"target_path", req.GetTargetPath(),
		"fstype", mountOption.FsType)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) findVolumeByID(volumes []*lvm.LogicalVolume, name string) *lvm.LogicalVolume {
	for _, v := range volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (s *nodeService) getLvFromContext(ctx context.Context, deviceClass, volumeID string) (*lvm.LogicalVolume, error) {
	listResp, err := s.client.GetLVList(deviceClass)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list LV: %v", err)
	}
	return s.findVolumeByID(listResp, volumeID), nil
}

func (s *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volID := req.GetVolumeId()
	target := req.GetTargetPath()
	nodeLogger.Info("NodeUnpublishVolume called",
		"volume_id", volID,
		"target_path", target)

	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", target, err)
	}

	if !info.IsDir() {
		return nil, status.Errorf(codes.Internal, "target path is not directory: %s", target)
	}

	unpublishResp, err := s.nodeUnpublishFilesystemVolume(req)
	if err != nil {
		return unpublishResp, err
	}
	volume, err := s.getLvFromContext(ctx, topolvm.DefaultDeviceClassName, volID)
	if err != nil {
		return nil, err
	}
	if volume != nil && s.isEphemeralVolume(volume) {
		if err = s.client.RemoveLV(volID, topolvm.DefaultDeviceClassName); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to remove LV for %s: %v", volID, err)
		}
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) isEphemeralVolume(volume *lvm.LogicalVolume) bool {
	for _, tag := range volume.Tags {
		if tag == "ephemeral" {
			return true
		}
	}
	return false
}

func (s *nodeService) nodeUnpublishFilesystemVolume(req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	target := req.GetTargetPath()
	mounter := mount.New("")
	if err := mounter.Unmount(target); err != nil {
		return nil, status.Errorf(codes.Internal, "unmount failed for %s: error=%v", target, err)
	}
	if err := os.RemoveAll(target); err != nil {
		return nil, status.Errorf(codes.Internal, "remove dir failed for %s: error=%v", target, err)
	}

	nodeLogger.Info("NodeUnpublishVolume(fs) is succeeded",
		"volume_id", req.GetVolumeId(),
		"target_path", target)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volID := req.GetVolumeId()
	p := req.GetVolumePath()
	nodeLogger.Info("NodeGetVolumeStats is called", "volume_id", volID, "volume_path", p)
	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(p) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_path is provided")
	}

	lvr, err := s.k8sLVService.GetVolume(ctx, req.GetVolumeId())
	if err != nil {
		return nil, err
	}
	lv, err := s.getLvFromContext(ctx, lvr.Spec.DeviceClass, req.GetVolumeId())
	if err != nil {
		return nil, err
	}

	stats, err := s.client.VolumeStats(lv.Name, lv.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "stat on %s was failed: %v", p, err)
	}

	usage := []*csi.VolumeUsage{{
		Unit:      csi.VolumeUsage_BYTES,
		Total:     int64(stats.TotalBytes),
		Used:      int64(stats.UsedBytes),
		Available: int64(stats.TotalBytes - stats.UsedBytes),
	}}

	return &csi.NodeGetVolumeStatsResponse{Usage: usage}, nil
}

func (s *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	vid := req.GetVolumeId()
	vpath := req.GetVolumePath()

	nodeLogger.Info("NodeExpandVolume is called",
		"volume_id", vid,
		"volume_path", vpath,
		"required", req.GetCapacityRange().GetRequiredBytes(),
		"limit", req.GetCapacityRange().GetLimitBytes(),
	)

	if len(vid) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(vpath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_path is provided")
	}

	// We need to check the capacity range but don't use the converted value
	// because the filesystem can be resized without the requested size.
	_, err := convertRequestCapacity(req.GetCapacityRange().GetRequiredBytes(), req.GetCapacityRange().GetLimitBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Filesystem should be already expanded by qouta change in logicalvolume controller

	// `capacity_bytes` in NodeExpandVolumeResponse is defined as OPTIONAL.
	// If this field needs to be filled, the value should be equal to `.status.currentSize` of the corresponding
	// `LogicalVolume`, but currently the node plugin does not have an access to the resource.
	// In addtion to this, Kubernetes does not care if the field is blank or not, so leave it blank.
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (s *nodeService) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	capabilities := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
	}

	csiCaps := make([]*csi.NodeServiceCapability, len(capabilities))
	for i, capability := range capabilities {
		csiCaps[i] = &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: capability,
				},
			},
		}
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}, nil
}

func (s *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.nodeName,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				topolvm.TopologyNodeKey: s.nodeName,
			},
		},
	}, nil
}
