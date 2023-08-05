package driver

import (
	"context"
	"os"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kvaster/topols"
	"github.com/kvaster/topols/driver/internal/k8s"
	"github.com/kvaster/topols/lsm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mountutil "k8s.io/mount-utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var nodeLogger = ctrl.Log.WithName("driver").WithName("node")

// NewNodeServer returns a new NodeServer.
func NewNodeServer(nodeName string, client lsm.Client, mgr manager.Manager) (csi.NodeServer, error) {
	lvService, err := k8s.NewLogicalVolumeService(mgr)
	if err != nil {
		return nil, err
	}

	return &nodeServer{
		server: &nodeServerNoLocked{
			nodeName:     nodeName,
			client:       client,
			k8sLVService: lvService,
			mounter:      mountutil.New(""),
		},
	}, nil
}

// This is a wrapper for nodeServerNoLocked to protect concurrent method calls.
type nodeServer struct {
	csi.UnimplementedNodeServer

	// This protects concurrent nodeServerNoLocked method calls.
	// We use a global lock because it assumes that each method does not take a long time,
	// and we scare about wired behaviors from concurrent device or filesystem operations.
	mu     sync.Mutex
	server *nodeServerNoLocked
}

func (s *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.server.NodePublishVolume(ctx, req)
}

func (s *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.server.NodeUnpublishVolume(ctx, req)
}

func (s *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.server.NodeGetVolumeStats(ctx, req)
}

func (s *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.server.NodeExpandVolume(ctx, req)
}

func (s *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	// This returns constant value only, it is unnecessary to take lock.
	return s.server.NodeGetCapabilities(ctx, req)
}

func (s *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	// This returns unmodified value only, it is unnecessary to take lock.
	return s.server.NodeGetInfo(ctx, req)
}

// NewNodeService returns a new NodeServer.
type nodeServerNoLocked struct {
	csi.UnimplementedNodeServer

	nodeName     string
	client       lsm.Client
	k8sLVService *k8s.LogicalVolumeService
	mounter      mountutil.Interface
}

func (s *nodeServerNoLocked) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
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
	// we only support SINGLE_NODE_WRITER
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	switch accessMode {
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
	default:
		modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s (%d)", modeName, accessMode)
	}

	var lv *lsm.LogicalVolume
	var err error

	lvr, err := s.k8sLVService.GetVolume(ctx, volumeID)
	if err != nil {
		return nil, err
	}
	lv, err = s.getLvFromContext(ctx, lvr.Spec.DeviceClass, volumeID)
	if err != nil {
		return nil, err
	}
	if lv == nil {
		return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", volumeID)
	}

	err = s.nodePublishFilesystemVolume(req, lv)

	if err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func makeMountOptions(readOnly bool, mountOption *csi.VolumeCapability_MountVolume) ([]string, error) {
	var mountOptions []string
	if readOnly {
		mountOptions = append(mountOptions, "ro")
	}

	for _, f := range mountOption.MountFlags {
		if f == "rw" && readOnly {
			return nil, status.Error(codes.InvalidArgument, "mount option \"rw\" is specified even though read only mode is specified")
		}
		mountOptions = append(mountOptions, f)
	}

	return mountOptions, nil
}

func (s *nodeServerNoLocked) nodePublishFilesystemVolume(req *csi.NodePublishVolumeRequest, lv *lsm.LogicalVolume) error {
	// Check request
	mountOption := req.GetVolumeCapability().GetMount()

	mountOptions, err := makeMountOptions(req.GetReadonly(), mountOption)
	if err != nil {
		return err
	}

	volumeId := req.GetVolumeId()
	sourcePath := s.client.GetPath(lv)
	targetPath := req.GetTargetPath()

	isMnt, err := s.mounter.IsMountPoint(targetPath)

	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(targetPath, 0755); err != nil {
				return status.Errorf(codes.Internal, "target path create failed: volume=%s, target=%s, error=%v", volumeId, targetPath, err)
			}

			nodeLogger.Info("NodePublishVolume(fs) target path created",
				"volume_id", volumeId,
				"target_path", targetPath,
			)

			isMnt = false
		} else {
			return status.Errorf(codes.Internal, "target path check failed: volume=%s, target=%s, error=%v", volumeId, targetPath, err)
		}
	}

	if isMnt {
		nodeLogger.Info("NodePublishVolume(fs) target path is already mounted",
			"volume_id", volumeId,
			"target_path", targetPath,
		)
	} else {
		if err := s.mounter.Mount(sourcePath, targetPath, "", mountOptions); err != nil {
			return status.Errorf(codes.Internal, "mount failed: volume=%s, target=%s, error=%v", volumeId, targetPath, err)
		}

		if err := os.Chmod(targetPath, 0777|os.ModeSetgid); err != nil {
			return status.Errorf(codes.Internal, "chmod 2777 failed: volume=%s, target=%s, error=%v", volumeId, targetPath, err)
		}
	}

	nodeLogger.Info("NodePublishVolume(fs) succeeded",
		"volume_id", volumeId,
		"target_path", targetPath,
		"fstype", mountOption.FsType)

	return nil
}

func (s *nodeServerNoLocked) findVolumeByID(volumes []*lsm.LogicalVolume, name string) *lsm.LogicalVolume {
	for _, v := range volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (s *nodeServerNoLocked) getLvFromContext(ctx context.Context, deviceClass, volumeID string) (*lsm.LogicalVolume, error) {
	listResp, err := s.client.GetLVList(deviceClass)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list LV: %v", err)
	}
	return s.findVolumeByID(listResp, volumeID), nil
}

func (s *nodeServerNoLocked) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	targetPath := req.GetTargetPath()
	nodeLogger.Info("NodeUnpublishVolume called",
		"volume_id", volumeId,
		"target_path", targetPath)

	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}

	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return &csi.NodeUnpublishVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", targetPath, err)
	}

	if !info.IsDir() {
		return nil, status.Errorf(codes.Internal, "target_path is not directory: %s", targetPath)
	}

	err = s.nodeUnpublishFilesystemVolume(req)
	if err != nil {
		return nil, err
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeServerNoLocked) nodeUnpublishFilesystemVolume(req *csi.NodeUnpublishVolumeRequest) error {
	targetPath := req.GetTargetPath()
	volumeId := req.GetVolumeId()

	if isMnt, err := s.mounter.IsMountPoint(targetPath); err != nil {
		if os.IsNotExist(err) {
			nodeLogger.Info("NodeUnpublishVolume(fs) target path does not exists",
				"volume_id", volumeId,
				"target_path", targetPath,
			)
		} else {
			return status.Errorf(codes.Internal, "target path check failed: volume=%s, target=%s, error=%v", volumeId, targetPath, err)
		}
	} else if isMnt {
		if err := s.mounter.Unmount(targetPath); err != nil {
			return status.Errorf(codes.Internal, "target path unmount failed: volume=%s, target=%s, error=%v", volumeId, targetPath, err)
		}

		nodeLogger.Info("NodeUnpublishVolume(fs) target path unmounted",
			"volume_id", volumeId,
			"target_path", targetPath,
		)

		if err := os.Remove(targetPath); err != nil {
			return status.Errorf(codes.Internal, "error removing target path: volume=%s, target=%s, error=%v", volumeId, targetPath, err)
		}
	}

	nodeLogger.Info("NodeUnpublishVolume(fs) is succeeded",
		"volume_id", volumeId,
		"target_path", targetPath)

	return nil
}

func (s *nodeServerNoLocked) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volumeId := req.GetVolumeId()
	volumePath := req.GetVolumePath()
	nodeLogger.Info("NodeGetVolumeStats is called", "volume_id", volumeId, "volume_path", volumePath)
	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(volumePath) == 0 {
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
		return nil, status.Errorf(codes.Internal, "stat on %s was failed: %v", volumePath, err)
	}

	usage := []*csi.VolumeUsage{{
		Unit:      csi.VolumeUsage_BYTES,
		Total:     int64(stats.TotalBytes),
		Used:      int64(stats.UsedBytes),
		Available: int64(stats.TotalBytes - stats.UsedBytes),
	}}

	return &csi.NodeGetVolumeStatsResponse{Usage: usage}, nil
}

func (s *nodeServerNoLocked) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	volumeId := req.GetVolumeId()
	volumePath := req.GetVolumePath()

	nodeLogger.Info("NodeExpandVolume is called",
		"volume_id", volumeId,
		"volume_path", volumePath,
		"required", req.GetCapacityRange().GetRequiredBytes(),
		"limit", req.GetCapacityRange().GetLimitBytes(),
	)

	if len(volumeId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(volumePath) == 0 {
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

func (s *nodeServerNoLocked) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
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

func (s *nodeServerNoLocked) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.nodeName,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				topols.TopologyNodeKey: s.nodeName,
			},
		},
	}, nil
}
