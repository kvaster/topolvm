package k8s

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/kvaster/topols"
	topolsv1 "github.com/kvaster/topols/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ErrVolumeNotFound represents the specified volume is not found.
var ErrVolumeNotFound = errors.New("VolumeID is not found")

// LogicalVolumeService represents service for LogicalVolume.
type LogicalVolumeService struct {
	client.Client
	mu sync.Mutex
}

const (
	indexFieldVolumeID = "status.volumeID"
)

var (
	logger = ctrl.Log.WithName("LogicalVolume")
)

//+kubebuilder:rbac:groups=topols.kvaster.com,resources=logicalvolumes,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// NewLogicalVolumeService returns LogicalVolumeService.
func NewLogicalVolumeService(mgr manager.Manager) (*LogicalVolumeService, error) {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &topolsv1.LogicalVolume{}, indexFieldVolumeID,
		func(o client.Object) []string {
			return []string{o.(*topolsv1.LogicalVolume).Status.VolumeID}
		})
	if err != nil {
		return nil, err
	}

	return &LogicalVolumeService{Client: mgr.GetClient()}, nil
}

// CreateVolume creates volume
func (s *LogicalVolumeService) CreateVolume(ctx context.Context, node, dc, name string, requestBytes int64) (string, error) {
	logger.Info("k8s.CreateVolume called", "name", name, "node", node, "size", requestBytes)
	s.mu.Lock()
	defer s.mu.Unlock()

	lv := &topolsv1.LogicalVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LogicalVolume",
			APIVersion: "topols.kvaster.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: topolsv1.LogicalVolumeSpec{
			Name:        name,
			NodeName:    node,
			DeviceClass: dc,
			Size:        *resource.NewQuantity(requestBytes, resource.BinarySI),
		},
	}

	existingLV := new(topolsv1.LogicalVolume)
	err := s.Get(ctx, client.ObjectKey{Name: name}, existingLV)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		err := s.Create(ctx, lv)
		if err != nil {
			return "", err
		}
		logger.Info("created LogicalVolume CRD", "name", name)
	} else {
		// LV with same name was found; check compatibility
		// skip check of capabilities because (1) we allow both of two access types, and (2) we allow only one access mode
		// for ease of comparison, sizes are compared strictly, not by compatibility of ranges
		if !existingLV.IsCompatibleWith(lv) {
			return "", status.Error(codes.AlreadyExists, "Incompatible LogicalVolume already exists")
		}
		// compatible LV was found
	}

	for {
		logger.Info("waiting for setting 'status.volumeID'", "name", name)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		var newLV topolsv1.LogicalVolume
		err := s.Get(ctx, client.ObjectKey{Name: name}, &newLV)
		if err != nil {
			logger.Error(err, "failed to get LogicalVolume", "name", name)
			return "", err
		}
		if newLV.Status.VolumeID != "" {
			logger.Info("end k8s.LogicalVolume", "volume_id", newLV.Status.VolumeID)
			return newLV.Status.VolumeID, nil
		}
		if newLV.Status.Code != codes.OK {
			err := s.Delete(ctx, &newLV)
			if err != nil {
				// log this error but do not return this error, because newLV.Status.Message is more important
				logger.Error(err, "failed to delete LogicalVolume")
			}
			return "", status.Error(newLV.Status.Code, newLV.Status.Message)
		}
	}
}

// DeleteVolume deletes volume
func (s *LogicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	logger.Info("k8s.DeleteVolume called", "volumeID", volumeID)

	lv, err := s.GetVolume(ctx, volumeID)
	if err != nil {
		if err == ErrVolumeNotFound {
			logger.Info("volume is not found", "volume_id", volumeID)
			return nil
		}
		return err
	}

	err = s.Delete(ctx, lv)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// wait until delete the target volume
	for {
		logger.Info("waiting for delete LogicalVolume", "name", lv.Name)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		err := s.Get(ctx, client.ObjectKey{Name: lv.Name}, new(topolsv1.LogicalVolume))
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			logger.Error(err, "failed to get LogicalVolume", "name", lv.Name)
			return err
		}
	}
}

// ExpandVolume expands volume
func (s *LogicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, requestBytes int64) error {
	logger.Info("k8s.ExpandVolume called", "volumeID", volumeID, "requestBytes", requestBytes)
	s.mu.Lock()
	defer s.mu.Unlock()

	lv, err := s.GetVolume(ctx, volumeID)
	if err != nil {
		return err
	}

	err = s.UpdateSpecSize(ctx, volumeID, resource.NewQuantity(requestBytes, resource.BinarySI))
	if err != nil {
		return err
	}

	// wait until topols-node expands the target volume
	for {
		logger.Info("waiting for update of 'status.currentSize'", "name", lv.Name)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		var changedLV topolsv1.LogicalVolume
		err := s.Get(ctx, client.ObjectKey{Name: lv.Name}, &changedLV)
		if err != nil {
			logger.Error(err, "failed to get LogicalVolume", "name", lv.Name)
			return err
		}
		if changedLV.Status.CurrentSize == nil {
			return errors.New("status.currentSize should not be nil")
		}
		if changedLV.Status.CurrentSize.Value() != changedLV.Spec.Size.Value() {
			logger.Info("failed to match current size and requested size", "current", changedLV.Status.CurrentSize.Value(), "requested", changedLV.Spec.Size.Value())
			continue
		}

		if changedLV.Status.Code != codes.OK {
			return status.Error(changedLV.Status.Code, changedLV.Status.Message)
		}

		return nil
	}
}

// GetVolume returns LogicalVolume by volume ID.
func (s *LogicalVolumeService) GetVolume(ctx context.Context, volumeID string) (*topolsv1.LogicalVolume, error) {
	lvList := new(topolsv1.LogicalVolumeList)
	err := s.List(ctx, lvList, client.MatchingFields{indexFieldVolumeID: volumeID})
	if err != nil {
		return nil, err
	}

	if len(lvList.Items) == 0 {
		return nil, ErrVolumeNotFound
	} else if len(lvList.Items) > 1 {
		return nil, fmt.Errorf("multiple LogicalVolume is found for VolumeID %s", volumeID)
	}
	return &lvList.Items[0], nil
}

// UpdateSpecSize updates .Spec.Size of LogicalVolume.
func (s *LogicalVolumeService) UpdateSpecSize(ctx context.Context, volumeID string, size *resource.Quantity) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		lv, err := s.GetVolume(ctx, volumeID)
		if err != nil {
			return err
		}

		lv.Spec.Size = *size
		if lv.Annotations == nil {
			lv.Annotations = make(map[string]string)
		}
		lv.Annotations[topols.ResizeRequestedAtKey] = time.Now().UTC().String()

		if err := s.Update(ctx, lv); err != nil {
			if apierrors.IsConflict(err) {
				logger.Info("detect conflict when LogicalVolume spec update", "name", lv.Name)
				continue
			}
			logger.Error(err, "failed to update LogicalVolume spec", "name", lv.Name)
			return err
		}

		return nil
	}
}

// UpdateCurrentSize updates .Status.CurrentSize of LogicalVolume.
func (s *LogicalVolumeService) UpdateCurrentSize(ctx context.Context, volumeID string, size *resource.Quantity) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		lv, err := s.GetVolume(ctx, volumeID)
		if err != nil {
			return err
		}

		lv.Status.CurrentSize = size

		if err := s.Status().Update(ctx, lv); err != nil {
			if apierrors.IsConflict(err) {
				logger.Info("detect conflict when LogicalVolume status update", "name", lv.Name)
				continue
			}
			logger.Error(err, "failed to update LogicalVolume status", "name", lv.Name)
			return err
		}

		return nil
	}
}
