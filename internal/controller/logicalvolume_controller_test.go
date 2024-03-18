package controller

import (
	"context"
	"time"

	"github.com/kvaster/topols"
	topolsv1 "github.com/kvaster/topols/api/v1"
	"github.com/kvaster/topols/internal/lsm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storegev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var volumes = &[]*lsm.LogicalVolume{}

type MockLsmClient struct {
}

func (l MockLsmClient) Start(context.Context) error {
	return nil
}

func (l MockLsmClient) GetLVList(deviceClass string) ([]*lsm.LogicalVolume, error) {
	return *volumes, nil
}

func (l MockLsmClient) CreateLV(name, deviceClass string, noCow bool, size uint64) (*lsm.LogicalVolume, error) {
	lv := lsm.LogicalVolume{
		Name:        name,
		DeviceClass: deviceClass,
		Size:        size,
	}

	*volumes = append(*volumes, &lv)

	return &lv, nil
}

func (l MockLsmClient) RemoveLV(name, deviceClass string) error {
	panic("unimplemented")
}

func (l MockLsmClient) ResizeLV(name, deviceClass string, size uint64) error {
	panic("unimplemented")
}

func (l MockLsmClient) CreateLVSnapshot(name, deviceClass, sourceVolID string, size uint64, accessType string) (*lsm.LogicalVolume, error) {
	panic("unimplemented")
}

func (l MockLsmClient) GetPath(v *lsm.LogicalVolume) string {
	panic("unimplemented")
}

func (l MockLsmClient) VolumeStats(name, deviceClass string) (*lsm.VolumeStats, error) {
	panic("unimplemented")
}

func (l MockLsmClient) NodeStats() (*lsm.NodeStats, error) {
	panic("unimplemented")
}

func (l MockLsmClient) Watch() chan struct{} {
	panic("unimplemented")
}

var _ = Describe("LogicalVolume controller", func() {
	ctx := context.Background()
	var stopFunc func()
	errCh := make(chan error)
	var lsm lsm.Client

	startReconciler := func(suffix string) {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		lsm = MockLsmClient{}

		reconciler := NewLogicalVolumeReconciler(mgr.GetClient(), lsm, "node"+suffix)
		err = reconciler.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			errCh <- mgr.Start(ctx)
		}()
		time.Sleep(100 * time.Millisecond)
	}

	AfterEach(func() {
		stopFunc()
		Expect(<-errCh).NotTo(HaveOccurred())
	})

	setupResources := func(ctx context.Context, suffix string) topolsv1.LogicalVolume {
		node := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node" + suffix,
				Finalizers: []string{
					topols.NodeFinalizer,
				},
			},
		}
		err := k8sClient.Create(ctx, &node)
		Expect(err).NotTo(HaveOccurred())

		sc := storegev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sc" + suffix,
			},
			Provisioner: topols.PluginName,
		}
		err = k8sClient.Create(ctx, &sc)
		Expect(err).NotTo(HaveOccurred())

		ns := createNamespace()
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pvc" + suffix,
				Namespace: ns,
				Annotations: map[string]string{
					AnnSelectedNode: node.Name,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &sc.Name,
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *resource.NewQuantity(1, resource.BinarySI),
					},
				},
			},
		}
		err = k8sClient.Create(ctx, &pvc)
		Expect(err).NotTo(HaveOccurred())

		lv := topolsv1.LogicalVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "lv" + suffix,
			},
			Spec: topolsv1.LogicalVolumeSpec{
				NodeName: node.Name,
			},
		}
		err = k8sClient.Create(ctx, &lv)
		Expect(err).NotTo(HaveOccurred())

		return lv
	}

	It("should add finalizer to LogicalVolume", func() {
		startReconciler("-add-finalizer")

		ctx := context.Background()

		// Setup
		lv := setupResources(ctx, "-add-finalizer")

		// Verify
		// ensure LV has finalizer
		Eventually(func(g Gomega) bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)
			if err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&lv, topols.LogicalVolumeFinalizer)
		}).Should(BeTrue())
	})

	It("should not add finalizer to LogicalVolume when volume has pendingdeletion annotation", func() {
		startReconciler("-pendingdeletion")

		ctx := context.Background()

		// Setup
		lv := setupResources(ctx, "-pendingdeletion")

		// ensure LV gets finalizer
		Eventually(func(g Gomega) bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)
			if err != nil {
				return false
			}
			return controllerutil.ContainsFinalizer(&lv, topols.LogicalVolumeFinalizer)
		}).Should(BeTrue())

		// Add pending deletion key & remove finalizer
		lv2 := lv.DeepCopy()
		lv2.Annotations = map[string]string{
			topols.LVPendingDeletionKey: "true",
		}
		controllerutil.RemoveFinalizer(lv2, topols.LogicalVolumeFinalizer)

		patch := client.MergeFrom(&lv)
		err := k8sClient.Patch(ctx, lv2, patch)
		Expect(err).NotTo(HaveOccurred())

		// ensure LV finalizer is removed
		Consistently(func(g Gomega) bool {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&lv), &lv)
			if err != nil {
				return false
			}
			return !controllerutil.ContainsFinalizer(&lv, topols.LogicalVolumeFinalizer)
		}, "2s").Should(BeTrue())
	})
})
