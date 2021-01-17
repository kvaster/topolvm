module github.com/kvaster/topols

go 1.15

replace launchpad.net/gocheck => github.com/go-check/check v0.0.0-20180628173108-788fd7840127

require (
	cloud.google.com/go v0.63.0 // indirect
	github.com/container-storage-interface/spec v1.1.0 // indirect
	github.com/cybozu-go/log v1.5.0 // indirect
	github.com/cybozu-go/well v1.10.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/go-logr/logr v0.3.0
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/kubernetes-csi/csi-test v2.2.0+incompatible // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/pseudomuto/protoc-gen-doc v1.3.2
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.1
	google.golang.org/grpc v1.32.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.0.0
	google.golang.org/protobuf v1.25.0
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v0.20.1
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.8.0
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/yaml v1.2.0
)
