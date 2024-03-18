package k8s

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kvaster/topols"
	topolsv1 "github.com/kvaster/topols/api/v1"
	clientwrapper "github.com/kvaster/topols/client"
	"github.com/kvaster/topols/getter"
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
// This is not concurrent safe, must take lock on caller.
type LogicalVolumeService struct {
	writer interface {
		client.Writer
		client.StatusClient
	}
	getter       getter.Interface
	volumeGetter *volumeGetter
}

const (
	indexFieldVolumeID = "status.volumeID"
)

var (
	logger = ctrl.Log.WithName("LogicalVolume")
)

type retryMissingGetter struct {
	cacheReader client.Reader
	apiReader   client.Reader
	getter      getter.Interface
}

func newRetryMissingGetter(cacheReader client.Reader, apiReader client.Reader) *retryMissingGetter {
	return &retryMissingGetter{
		cacheReader: cacheReader,
		apiReader:   apiReader,
		getter:      getter.NewRetryMissingGetter(cacheReader, apiReader),
	}
}

func (r *retryMissingGetter) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	var lv *topolsv1.LogicalVolume
	var ok bool
	if lv, ok = obj.(*topolsv1.LogicalVolume); !ok {
		return r.getter.Get(ctx, key, obj)
	}

	err := r.cacheReader.Get(ctx, key, lv)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	return r.apiReader.Get(ctx, key, lv)
}

// This type is a safe guard to prohibit calling List from LogicalVolumeService directly.
type volumeGetter struct {
	cacheReader client.Reader
	apiReader   client.Reader
}

// Get returns LogicalVolume by volume ID.
// This ensures read-after-create consistency.
func (v *volumeGetter) Get(ctx context.Context, volumeID string) (*topolsv1.LogicalVolume, error) {
	lvList := new(topolsv1.LogicalVolumeList)
	err := v.cacheReader.List(ctx, lvList, client.MatchingFields{indexFieldVolumeID: volumeID})
	if err != nil {
		return nil, err
	}

	if len(lvList.Items) > 1 {
		return nil, fmt.Errorf("multiple LogicalVolume is found for VolumeID %s", volumeID)
	} else if len(lvList.Items) != 0 {
		return &lvList.Items[0], nil
	}

	// not found. try direct reader.
	err = v.apiReader.List(ctx, lvList)
	if err != nil {
		return nil, err
	}

	count := 0
	var foundLv *topolsv1.LogicalVolume
	for _, lv := range lvList.Items {
		if lv.Status.VolumeID == volumeID {
			count++
			foundLv = &lv
		}
	}
	if count > 1 {
		return nil, fmt.Errorf("multiple LogicalVolume is found for VolumeID %s", volumeID)
	}
	if foundLv == nil {
		return nil, ErrVolumeNotFound
	}
	return foundLv, nil
}

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

	reader := clientwrapper.NewWrappedClient(mgr.GetClient())
	apiReader := clientwrapper.NewWrappedReader(mgr.GetAPIReader(), mgr.GetClient().Scheme())
	return &LogicalVolumeService{
		writer:       reader,
		getter:       newRetryMissingGetter(reader, apiReader),
		volumeGetter: &volumeGetter{cacheReader: reader, apiReader: apiReader},
	}, nil
}

// CreateVolume creates volume
func (s *LogicalVolumeService) CreateVolume(ctx context.Context, node, dc string, noCow bool, name, sourceName string, requestBytes int64) (string, error) {
	logger.Info("k8s.CreateVolume called", "name", name, "node", node, "size", requestBytes, "sourceName", sourceName)

	var lv *topolsv1.LogicalVolume
	// if the create volume request has no source, proceed with regular lv creation.
	if sourceName == "" {
		lv = &topolsv1.LogicalVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: topolsv1.LogicalVolumeSpec{
				Name:        name,
				NodeName:    node,
				DeviceClass: dc,
				NoCow:       noCow,
				Size:        *resource.NewQuantity(requestBytes, resource.BinarySI),
			},
		}
	} else {
		lv = &topolsv1.LogicalVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: topolsv1.LogicalVolumeSpec{
				Name:        name,
				NodeName:    node,
				DeviceClass: dc,
				NoCow:       noCow,
				Size:        *resource.NewQuantity(requestBytes, resource.BinarySI),
				Source:      sourceName,
				AccessType:  "rw",
			},
		}
	}

	existingLV := new(topolsv1.LogicalVolume)
	err := s.getter.Get(ctx, client.ObjectKey{Name: name}, existingLV)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		err := s.writer.Create(ctx, lv)
		if err != nil {
			return "", err
		}
		logger.Info("created LogicalVolume CR", "name", name, "sourceID", lv.Spec.Source)
	} else {
		// LV with same name was found; check compatibility
		// skip check of capabilities because (1) we allow both of two access types, and (2) we allow only one access mode
		// for ease of comparison, sizes are compared strictly, not by compatibility of ranges
		if !existingLV.IsCompatibleWith(lv) {
			return "", status.Error(codes.AlreadyExists, "Incompatible LogicalVolume already exists")
		}
		// compatible LV was found
	}

	volumeID, err := s.waitForStatusUpdate(ctx, name)
	if err != nil {
		return "", err
	}

	return volumeID, nil
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

	err = s.writer.Delete(ctx, lv)
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

		err := s.getter.Get(ctx, client.ObjectKey{Name: lv.Name}, new(topolsv1.LogicalVolume))
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			logger.Error(err, "failed to get LogicalVolume", "name", lv.Name)
			return err
		}
	}
}

// CreateSnapshot creates a snapshot of existing volume.
func (s *LogicalVolumeService) CreateSnapshot(ctx context.Context, node, dc, sourceVol, sname, accessType string, snapSize resource.Quantity) (string, error) {
	logger.Info("CreateSnapshot called", "name", sname)
	snapshotLV := &topolsv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: sname,
		},
		Spec: topolsv1.LogicalVolumeSpec{
			Name:        sname,
			NodeName:    node,
			DeviceClass: dc,
			Size:        snapSize,
			Source:      sourceVol,
			AccessType:  accessType,
		},
	}

	existingSnapshot := new(topolsv1.LogicalVolume)
	err := s.getter.Get(ctx, client.ObjectKey{Name: sname}, existingSnapshot)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}
		err := s.writer.Create(ctx, snapshotLV)
		if err != nil {
			return "", err
		}
		logger.Info("created LogicalVolume CR", "name", sname, "source", snapshotLV.Spec.Source, "accessType", snapshotLV.Spec.AccessType)
	} else {
		if !existingSnapshot.IsCompatibleWith(snapshotLV) {
			return "", status.Error(codes.AlreadyExists, "Incompatible LogicalVolume already exists")
		}
	}

	volumeID, err := s.waitForStatusUpdate(ctx, sname)
	if err != nil {
		return "", err
	}

	return volumeID, nil
}

// ExpandVolume expands volume
func (s *LogicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, requestBytes int64) error {
	logger.Info("k8s.ExpandVolume called", "volumeID", volumeID, "requestBytes", requestBytes)

	lv, err := s.GetVolume(ctx, volumeID)
	if err != nil {
		return err
	}

	err = s.updateSpecSize(ctx, volumeID, resource.NewQuantity(requestBytes, resource.BinarySI))
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
		err := s.getter.Get(ctx, client.ObjectKey{Name: lv.Name}, &changedLV)
		if err != nil {
			logger.Error(err, "failed to get LogicalVolume", "name", lv.Name)
			return err
		}
		if changedLV.Status.Code != codes.OK {
			return status.Error(changedLV.Status.Code, changedLV.Status.Message)
		}
		if changedLV.Status.CurrentSize == nil {
			// WA: since Status.CurrentSize is added in v0.4.0. it may be missing.
			// if the expansion is completed, it is filled, so wait for that.
			continue
		}
		if changedLV.Status.CurrentSize.Value() != changedLV.Spec.Size.Value() {
			logger.Info("failed to match current size and requested size", "current", changedLV.Status.CurrentSize.Value(), "requested", changedLV.Spec.Size.Value())
			continue
		}

		return nil
	}
}

// GetVolume returns LogicalVolume by volume ID.
func (s *LogicalVolumeService) GetVolume(ctx context.Context, volumeID string) (*topolsv1.LogicalVolume, error) {
	return s.volumeGetter.Get(ctx, volumeID)
}

// updateSpecSize updates .Spec.Size of LogicalVolume.
func (s *LogicalVolumeService) updateSpecSize(ctx context.Context, volumeID string, size *resource.Quantity) error {
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

		if err := s.writer.Update(ctx, lv); err != nil {
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

// waitForStatusUpdate waits for logical volume creation/failure/timeout, whichever comes first.
func (s *LogicalVolumeService) waitForStatusUpdate(ctx context.Context, name string) (string, error) {
	for {
		logger.Info("waiting for setting 'status.volumeID'", "name", name)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		var newLV topolsv1.LogicalVolume
		err := s.getter.Get(ctx, client.ObjectKey{Name: name}, &newLV)
		if err != nil {
			logger.Error(err, "failed to get LogicalVolume", "name", name)
			return "", err
		}
		if newLV.Status.VolumeID != "" {
			logger.Info("end k8s.LogicalVolume", "volume_id", newLV.Status.VolumeID)
			return newLV.Status.VolumeID, nil
		}
		if newLV.Status.Code != codes.OK {
			err := s.writer.Delete(ctx, &newLV)
			if err != nil {
				// log this error but do not return this error, because newLV.Status.Message is more important
				logger.Error(err, "failed to delete LogicalVolume")
			}
			return "", status.Error(newLV.Status.Code, newLV.Status.Message)
		}
	}
}
