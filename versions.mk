# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.13.1
# https://github.com/cert-manager/cert-manager/releases
CERT_MANAGER_VERSION := v1.14.4
# https://github.com/helm/chart-testing/releases
CHART_TESTING_VERSION := 3.10.1
# https://github.com/containernetworking/plugins/releases
CNI_PLUGINS_VERSION := v1.4.1
# https://github.com/GoogleContainerTools/container-structure-test/releases
CONTAINER_STRUCTURE_TEST_VERSION := 1.16.0
# https://github.com/Mirantis/cri-dockerd/releases
CRI_DOCKERD_VERSION := v0.3.11
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.29.0
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v1.56.2
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.13.1
# https://github.com/helm/helm/releases
HELM_VERSION := 3.14.3
# It is set by CI using the environment variable, use conditional assignment.
KUBERNETES_VERSION ?= 1.29.3
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION := 26.0

ENVTEST_KUBERNETES_VERSION := $(shell echo $(KUBERNETES_VERSION) | cut -d "." -f 1-2)

# Tools versions which are defined in go.mod
SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
CONTROLLER_RUNTIME_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-runtime/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
CONTROLLER_TOOLS_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-tools/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
GINKGO_VERSION := $(shell awk '/github.com\/onsi\/ginkgo\/v2/ {print $$2}' $(SELF_DIR)/go.mod)
PROTOC_GEN_DOC_VERSION := $(shell awk '/github.com\/pseudomuto\/protoc-gen-doc/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
PROTOC_GEN_GO_GRPC_VERSION := $(shell awk '/google.golang.org\/grpc\/cmd\/protoc-gen-go-grpc/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)

# CSI sidecar versions
EXTERNAL_PROVISIONER_VERSION := 4.0.0
EXTERNAL_RESIZER_VERSION := 1.10.0
EXTERNAL_SNAPSHOTTER_VERSION := 7.0.1
LIVENESSPROBE_VERSION := 2.12.0
NODE_DRIVER_REGISTRAR_VERSION := 2.10.0
