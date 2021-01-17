package hook

import (
	"github.com/kvaster/topols"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

const (
	mutatePVCNamespace = "test-mutate-pvc"
	defaultPVCName     = "test-pvc"
)

func setupMutatePVCResources() {
	// Namespace and namespace resources
	ns := &corev1.Namespace{}
	ns.Name = mutatePVCNamespace
	err := k8sClient.Create(testCtx, ns)
	Expect(err).ShouldNot(HaveOccurred())
}

func createPVC(sc string, pvcName string) {
	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Namespace = mutatePVCNamespace
	pvc.Name = pvcName
	pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	if sc != "" {
		pvc.Spec.StorageClassName = strPtr(sc)
	}
	pvc.Spec.Resources.Requests = corev1.ResourceList{
		"storage": *resource.NewQuantity(10<<30, resource.DecimalSI),
	}
	err := k8sClient.Create(testCtx, pvc)
	Expect(err).ShouldNot(HaveOccurred())
}

func getPVC(pvcName string) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	name := types.NamespacedName{
		Namespace: mutatePVCNamespace,
		Name:      pvcName,
	}
	err := k8sClient.Get(testCtx, name, pvc)
	return pvc, err
}

func hasTopoLSFinalizer(pvc *corev1.PersistentVolumeClaim) bool {
	for _, fin := range pvc.Finalizers {
		if fin == topols.PVCFinalizer {
			return true
		}
	}
	return false
}

var _ = Describe("pvc mutation webhook", func() {
	It("should not have topols.kvaster.com/pvc finalizer when not specified storageclass", func() {
		pvcName := "empty-storageclass-pvc"
		createPVC("", pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLSFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(false), "finalizer should not be set for storageclass=%s", hostLocalStorageClassName)
	})

	It("should not have topols.kvaster.com/pvc finalizer when the specified StorageClass does not exist", func() {
		pvcName := "unexists-storageclass-pvc"
		createPVC(missingStorageClassName, pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLSFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(false), "finalizer should not be set for storageclass=%s", missingStorageClassName)
	})

	It("should not have topols.kvaster.com/pvc finalizer with storageclass host-local", func() {
		pvcName := "host-local-pvc"
		createPVC(hostLocalStorageClassName, pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLSFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(false), "finalizer should not be set for storageclass=%s", hostLocalStorageClassName)
	})

	It("should have topols.kvaster.com/pvc finalizer with storageclass topols-provisioner", func() {
		pvcName := "topols-provisioner-pvc"
		createPVC(topolsProvisionerStorageClassName, pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLSFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(true), "finalizer should be set for storageclass=%s", topolsProvisionerStorageClassName)
	})

	It("should have topols.kvaster.com/pvc finalizer with storageclass topols-provisioner-immediate", func() {
		pvcName := "topols-provisioner-immediate-pvc"
		createPVC(topolsProvisionerImmediateStorageClassName, pvcName)
		pvc, err := getPVC(pvcName)
		Expect(err).ShouldNot(HaveOccurred())
		hasFinalizer := hasTopoLSFinalizer(pvc)
		Expect(hasFinalizer).Should(Equal(true), "finalizer should be set for storageclass=%s", topolsProvisionerImmediateStorageClassName)
	})
})
