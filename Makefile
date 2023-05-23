## Dependency versions

CONTROLLER_RUNTIME_VERSION=$(shell awk '/sigs\.k8s\.io\/controller-runtime/ {print substr($$2, 2)}' go.mod)
CONTROLLER_TOOLS_VERSION=$(shell awk '/sigs\.k8s\.io\/controller-tools/ {print substr($$2, 2)}' go.mod)
PROTOC_VERSION=23.1
HELM_VERSION=3.12.0
HELM_DOCS_VERSION=1.11.0
CHART_TESTING_VERSION=3.8.0
YQ_VERSION=4.33.3
BUILDX_VERSION=0.10.4
CONTAINER_STRUCTURE_TEST_VERSION=1.15.0

SUDO := sudo
CURL := curl -sSLf
BINDIR := $(shell pwd)/bin
CONTROLLER_GEN := $(BINDIR)/controller-gen
STATICCHECK := $(BINDIR)/staticcheck
CONTAINER_STRUCTURE_TEST := $(BINDIR)/container-structure-test
PROTOC := PATH=$(BINDIR):$(PATH) $(BINDIR)/protoc -I=$(shell pwd)/include:.
PACKAGES := unzip patch
ENVTEST_ASSETS_DIR := $(shell pwd)/testbin

GO_FILES=$(shell find -name '*.go' -not -name '*_test.go')
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
GOFLAGS =
export GOFLAGS

BUILD_TARGET=hypertopols
TOPOLS_VERSION ?= devel
IMAGE_TAG ?= latest
STRUCTURE_TEST_TARGET ?= normal

ENVTEST_KUBERNETES_VERSION=1.26

PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' go.mod)
PROTOC_GEN_DOC_VERSION := $(shell awk '/github.com\/pseudomuto\/protoc-gen-doc/ {print substr($$2, 2)}' go.mod)
PROTOC_GEN_GO_GRPC_VERSION := $(shell awk '/google.golang.org\/grpc\/cmd\/protoc-gen-go-grpc/ {print substr($$2, 2)}' go.mod)

PUSH ?= false
BUILDX_PUSH_OPTIONS := "-o type=tar,dest=build/topols.tar"
ifeq ($(PUSH),true)
BUILDX_PUSH_OPTIONS := --push
endif

# Set the shell used to bash for better error handling.
SHELL = /bin/bash
.SHELLFLAGS = -e -o pipefail -c

CONTAINER_STRUCTURE_TEST_IMAGE=$(IMAGE_PREFIX)topols:devel
ifeq ($(STRUCTURE_TEST_TARGET),with-sidecar)
CONTAINER_STRUCTURE_TEST_IMAGE=$(IMAGE_PREFIX)topols-with-sidecar:devel
endif

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) \
		crd:crdVersions=v1 \
		rbac:roleName=topols-controller \
		webhook \
		paths="./api/...;./controllers;./hook;./driver/internal/k8s;./pkg/..." \
		output:crd:artifacts:config=config/crd/bases
	$(BINDIR)/yq eval 'del(.status)' config/crd/bases/topols.kvaster.com_logicalvolumes.yaml > charts/topols/templates/crds/topols.kvaster.com_logicalvolumes.yaml

.PHONY: generate-api ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
generate-api:
	$(CONTROLLER_GEN) object:headerFile="./hack/boilerplate.go.txt" paths="./api/..."

.PHONY: generate-helm-docs
generate-helm-docs:
	./bin/helm-docs -c charts/topols/

.PHONY: generate
generate: manifests generate-api generate-helm-docs

.PHONY: check-uncommitted
check-uncommitted: generate ## Check if latest generated artifacts are committed.
	git diff --exit-code --name-only

.PHONY: lint
lint: ## Run lint
	test -z "$$(gofmt -s -l . | grep -vE '^vendor|^api/v1/zz_generated.deepcopy.go' | tee /dev/stderr)"
	$(STATICCHECK) ./...
	go vet ./...
	test -z "$$(go vet ./... | grep -v '^vendor' | tee /dev/stderr)"

.PHONY: test
test: lint ## Run lint and unit tests.
	go install ./...

	mkdir -p $(ENVTEST_ASSETS_DIR)
	source <($(BINDIR)/setup-envtest use $(ENVTEST_KUBERNETES_VERSION) --bin-dir=$(ENVTEST_ASSETS_DIR) -p env); GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn go test -count=1 -race -v --timeout=120s ./...

groupname-test: ## Run unit tests that depends on the groupname.
	go install ./...

	mkdir -p $(ENVTEST_ASSETS_DIR)
	source <($(BINDIR)/setup-envtest use $(ENVTEST_KUBERNETES_VERSION) --bin-dir=$(ENVTEST_ASSETS_DIR) -p env); GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn TEST_LEGACY=true go test -count=1 -race -v --timeout=60s ./client/*
	TEST_LEGACY=true go test -count=1 -race -v --timeout=60s ./constants*.go

.PHONY: clean
clean: ## Clean working directory.
	rm -rf build/
	rm -rf bin/
	rm -rf include/
	rm -rf testbin/
	rm -f $(HOME)/.docker/cli-plugins/docker-buildx
	docker run --privileged --rm tonistiigi/binfmt --uninstall linux/amd64,linux/arm64/v8,linux/ppc64le

##@ Build

.PHONY: build
build: build-topols csi-sidecars ## Build binaries.

.PHONY: build-topols
build-topols: build/hypertopols

build/hypertopols: $(GO_FILES)
	mkdir -p build
	GOARCH=$(GOARCH) go build -o $@ -ldflags "-w -s -X github.com/kvaster/topols.Version=$(TOPOLS_VERSION)" ./pkg/hypertopols

.PHONY: csi-sidecars
csi-sidecars: ## Build sidecar binaries.
	mkdir -p build
	make -f csi-sidecars.mk OUTPUT_DIR=build BUILD_PLATFORMS="linux $(GOARCH)"

.PHONY: image
image: image-normal image-with-sidecar ## Build topols images.

.PHONY: image-normal
image-normal:
	docker buildx build --no-cache --load -t $(IMAGE_PREFIX)topols:devel --build-arg TOPOLS_VERSION=$(TOPOLS_VERSION) --target topols .

.PHONY: image-with-sidecar
image-with-sidecar:
	docker buildx build --no-cache --load -t $(IMAGE_PREFIX)topols-with-sidecar:devel --build-arg IMAGE_PREFIX=$(IMAGE_PREFIX) --build-arg TOPOLS_VERSION=$(TOPOLS_VERSION) --target topols-with-sidecar .

.PHONY: container-structure-test
container-structure-test: ## Run container-structure-test.
	$(CONTAINER_STRUCTURE_TEST) test \
		--image $(CONTAINER_STRUCTURE_TEST_IMAGE) \
		--config container-structure-test.yaml
ifeq ($(STRUCTURE_TEST_TARGET),with-sidecar)
	$(CONTAINER_STRUCTURE_TEST) test \
		--image $(CONTAINER_STRUCTURE_TEST_IMAGE) \
		--config container-structure-test-with-sidecar.yaml
endif

.PHONY: create-docker-container
create-docker-container: ## Create docker-container.
	docker buildx create --use

BUILD_TAGS := -t $(IMAGE_PREFIX)topols:$(IMAGE_TAG)
BUILD_TAGS_SIDECAR := -t $(IMAGE_PREFIX)topols-with-sidecar:$(IMAGE_TAG)
ifeq ($(PUSH_LATEST),true)
BUILD_TAGS := $(BUILD_TAGS) -t $(IMAGE_PREFIX)topols:latest
BUILD_TAGS_SIDECAR := $(BUILD_TAGS_SIDECAR) -t $(IMAGE_PREFIX)topols-with-sidecar:latest
endif

.PHONY: multi-platform-images
multi-platform-images: multi-platform-image-normal multi-platform-image-with-sidecar ## Build or push multi-platform topols images.

.PHONY: multi-platform-image-normal
multi-platform-image-normal:
	mkdir -p build
	docker buildx build --no-cache $(BUILDX_PUSH_OPTIONS) \
		--platform linux/amd64,linux/arm64/v8,linux/ppc64le \
		$(BUILD_TAGS) \
		--build-arg TOPOLS_VERSION=$(TOPOLS_VERSION) \
		--target topols \
		.

.PHONY: multi-platform-image-with-sidecar
multi-platform-image-with-sidecar:
	mkdir -p build
	docker buildx build --no-cache $(BUILDX_PUSH_OPTIONS) \
		--platform linux/amd64,linux/arm64/v8,linux/ppc64le \
		$(BUILD_TAGS_SIDECAR) \
		--build-arg TOPOLS_VERSION=$(TOPOLS_VERSION) \
		--build-arg IMAGE_PREFIX=$(IMAGE_PREFIX) \
		--target topols-with-sidecar \
		.

.PHONY: tag
tag: ## Tag topols images.
	docker buildx imagetools create \
		--tag $(IMAGE_PREFIX)topols:devel \
		$(IMAGE_PREFIX)topols:$(IMAGE_TAG)
	docker buildx imagetools create \
		--tag $(IMAGE_PREFIX)topols-with-sidecar:devel \
		$(IMAGE_PREFIX)topols-with-sidecar:$(IMAGE_TAG)

.PHONY: push
push: ## Push topols images.
	docker push $(IMAGE_PREFIX)topols:$(IMAGE_TAG)
	docker push $(IMAGE_PREFIX)topols-with-sidecar:$(IMAGE_TAG)

##@ Chart Testing

.PHONY: ct-lint
ct-lint: ## Lint and validate a chart.
	docker run \
		--rm \
		--user $(shell id -u $(USER)) \
		--workdir /data \
		--volume $(shell pwd):/data \
		quay.io/helmpack/chart-testing:v$(CHART_TESTING_VERSION) \
		ct lint --config ct.yaml

##@ Setup

$(BINDIR):
	mkdir -p $@

.PHONY: install-container-structure-test
install-container-structure-test: | $(BINDIR)
	$(CURL) -o $(CONTAINER_STRUCTURE_TEST) \
		https://storage.googleapis.com/container-structure-test/v$(CONTAINER_STRUCTURE_TEST_VERSION)/container-structure-test-linux-amd64 \
    && chmod +x $(CONTAINER_STRUCTURE_TEST)

.PHONY: install-helm
install-helm: | $(BINDIR)
	$(CURL) https://get.helm.sh/helm-v$(HELM_VERSION)-linux-amd64.tar.gz \
		| tar xvz -C $(BINDIR) --strip-components 1 linux-amd64/helm

.PHONY: install-helm-docs
install-helm-docs: | $(BINDIR)
	GOBIN=$(BINDIR) go install github.com/norwoodj/helm-docs/cmd/helm-docs@v$(HELM_DOCS_VERSION)

.PHONY: tools
tools: install-container-structure-test install-helm install-helm-docs | $(BINDIR) ## Install development tools.
	GOBIN=$(BINDIR) go install honnef.co/go/tools/cmd/staticcheck@latest
	# Follow the official documentation to install the `latest` version, because explicitly specifying the version will get an error.
	GOBIN=$(BINDIR) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
	GOBIN=$(BINDIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_TOOLS_VERSION)

	$(CURL) -o protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-linux-x86_64.zip
	unzip -o protoc.zip bin/protoc 'include/*'
	rm -f protoc.zip
	GOBIN=$(BINDIR) go install google.golang.org/protobuf/cmd/protoc-gen-go@v$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(BINDIR) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v$(PROTOC_GEN_GO_GRPC_VERSION)
	GOBIN=$(BINDIR) go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@v$(PROTOC_GEN_DOC_VERSION)

	$(CURL) https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64 -o $(BINDIR)/yq \
		&& chmod +x $(BINDIR)/yq

.PHONY: setup
setup: ## Setup local environment.
	$(SUDO) apt-get update
	$(SUDO) apt-get -y install --no-install-recommends $(PACKAGES)
	$(MAKE) tools
	$(MAKE) setup-docker-buildx

.PHONY: setup-docker-buildx
setup-docker-buildx: ## Setup docker buildx environment.
	$(MAKE) $(HOME)/.docker/cli-plugins/docker-buildx
	# https://github.com/tonistiigi/binfmt
	docker run --privileged --rm tonistiigi/binfmt --install linux/amd64,linux/arm64/v8,linux/ppc64le

# https://docs.docker.com/build/buildx/install/
$(HOME)/.docker/cli-plugins/docker-buildx:
	mkdir -p $(HOME)/.docker/cli-plugins
	$(CURL) -o $@ \
		https://github.com/docker/buildx/releases/download/v$(BUILDX_VERSION)/buildx-v$(BUILDX_VERSION).$(GOOS)-$(GOARCH) \
		&& chmod +x $@
