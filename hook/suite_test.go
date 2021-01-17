package hook

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/kvaster/topols"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var testCtx = context.Background()
var stopCtx, stopCancel = context.WithCancel(testCtx)

const (
	topolsProvisionerStorageClassName          = "topols-provisioner"
	topolsProvisioner2StorageClassName         = "topols-provisioner2"
	topolsProvisioner3StorageClassName         = "topols-provisioner3"
	topolsProvisionerImmediateStorageClassName = "topols-provisioner-immediate"
	hostLocalStorageClassName                  = "host-local"
	missingStorageClassName                    = "missing-storageclass"
)

var (
	podMutatingWebhookPath = "/pod/mutate"
	pvcMutatingWebhookPath = "/pvc/mutate"
)

func strPtr(s string) *string { return &s }

func modePtr(m storagev1.VolumeBindingMode) *storagev1.VolumeBindingMode { return &m }

func setupCommonResources() {
	// StrageClass
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolsProvisionerStorageClassName,
		},
		Provisioner:       "topols.kvaster.com",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingWaitForFirstConsumer),
		Parameters: map[string]string{
			topols.DeviceClassKey: "ssd",
		},
	}
	err := k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolsProvisioner2StorageClassName,
		},
		Provisioner:       "topols.kvaster.com",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingWaitForFirstConsumer),
		Parameters: map[string]string{
			topols.DeviceClassKey: "hdd1",
		},
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolsProvisioner3StorageClassName,
		},
		Provisioner:       "topols.kvaster.com",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingWaitForFirstConsumer),
		Parameters: map[string]string{
			topols.DeviceClassKey: "hdd2",
		},
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: topolsProvisionerImmediateStorageClassName,
		},
		Provisioner:       "topols.kvaster.com",
		VolumeBindingMode: modePtr(storagev1.VolumeBindingImmediate),
		Parameters: map[string]string{
			topols.DeviceClassKey: "ssd",
		},
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())

	sc = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: hostLocalStorageClassName,
		},
		Provisioner: "kubernetes.io/no-provisioner",
	}
	err = k8sClient.Create(testCtx, sc)
	Expect(err).ShouldNot(HaveOccurred())
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	SetDefaultEventuallyTimeout(time.Minute)
	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))

	By("bootstrapping test environment")
	failPolicy := admissionregistrationv1beta1.Fail
	webhookInstallOptions := envtest.WebhookInstallOptions{
		MutatingWebhooks: []client.Object{
			&admissionregistrationv1beta1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "topols-hook",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "MutatingWebhookConfiguration",
					APIVersion: "admissionregistration.k8s.io/v1beta1",
				},
				Webhooks: []admissionregistrationv1beta1.MutatingWebhook{
					{
						Name:          "pod-hook.topols.kvaster.com",
						FailurePolicy: &failPolicy,
						ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
							Service: &admissionregistrationv1beta1.ServiceReference{
								Path: &podMutatingWebhookPath,
							},
						},
						Rules: []admissionregistrationv1beta1.RuleWithOperations{
							{
								Operations: []admissionregistrationv1beta1.OperationType{
									admissionregistrationv1beta1.Create,
								},
								Rule: admissionregistrationv1beta1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"pods"},
								},
							},
						},
					},
					{
						Name:          "pvc-hook.topols.kvaster.com",
						FailurePolicy: &failPolicy,
						ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
							Service: &admissionregistrationv1beta1.ServiceReference{
								Path: &pvcMutatingWebhookPath,
							},
						},
						Rules: []admissionregistrationv1beta1.RuleWithOperations{
							{
								Operations: []admissionregistrationv1beta1.OperationType{
									admissionregistrationv1beta1.Create,
								},
								Rule: admissionregistrationv1beta1.Rule{
									APIGroups:   []string{""},
									APIVersions: []string{"v1"},
									Resources:   []string{"persistentvolumeclaims"},
								},
							},
						},
					},
				},
			},
		},
	}

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		WebhookInstallOptions: webhookInstallOptions,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).ToNot(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	By("running webhook server")
	go run(stopCtx, cfg, scheme, &testEnv.WebhookInstallOptions)
	d := &net.Dialer{Timeout: time.Second}
	Eventually(func() error {
		serverURL := fmt.Sprintf("%s:%d", testEnv.WebhookInstallOptions.LocalServingHost, testEnv.WebhookInstallOptions.LocalServingPort)
		conn, err := tls.DialWithDialer(d, "tcp", serverURL, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())

	By("setting up resources")
	setupCommonResources()
	setupMutatePodResources()
	setupMutatePVCResources()
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	stopCancel()
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
