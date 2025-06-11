# ====================
# Base Settings
# ====================

SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

# Directory settings
WORKSPACE_DIR := $(CURDIR)
LOCALBIN := $(WORKSPACE_DIR)/bin

# Build settings
GOOS ?= linux
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0
COMMON_LDFLAGS := -s -w

# Version settings
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || git describe --tags --always)
VERSION_DATE := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
VERSION_PKG := github.com/hellices/openapi-aggregator-operator/pkg/version
OPERATOR_LDFLAGS := -X $(VERSION_PKG).version=$(VERSION) -X $(VERSION_PKG).buildDate=$(VERSION_DATE)

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# aggregator.io/golang-bundle:$VERSION and aggregator.io/golang-catalog:$VERSION.
IMAGE_TAG_BASE ?= aggregator.io/golang

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.39.2
# Image configuration
DOCKER_USER ?= hellices
IMG_PREFIX ?= ghcr.io/${DOCKER_USER}
IMG_REPO ?= openapi-aggregator-operator
IMG ?= ${IMG_PREFIX}/${IMG_REPO}:${VERSION}

# Testing configuration
ENVTEST_K8S_VERSION = 1.31.0
MIN_KUBERNETES_VERSION ?= 1.23.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

LOCALBIN ?= $(shell pwd)/bin

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test test-e2e test-coverage
test: manifests generate fmt vet envtest ## Run unit tests
	KUBEBUILDER_ASSETS="$$($(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" \
	go test $(shell go list ./... | grep -v /test/e2e) -v -race -coverprofile=cover.out -covermode=atomic

test-e2e: docker-build ## Run e2e tests
	go test ./test/e2e/... -v -ginkgo.v

test-coverage: test ## Generate test coverage report
	go tool cover -html=cover.out -o coverage.html

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build build-only build-all
build: manifests generate fmt vet ## Build manager binary for current architecture
	@$(MAKE) build-only

build-only: ## Build manager binary without preprocessing
	@mkdir -p $(LOCALBIN)
	@echo "Building manager for $(GOARCH)..."
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "$(COMMON_LDFLAGS) $(OPERATOR_LDFLAGS)" \
		-o $(LOCALBIN)/manager_$(GOARCH) cmd/main.go
	@cd $(LOCALBIN) && ln -sf manager_$(GOARCH) manager

BUILD_PLATFORMS := amd64 arm64
build-all: manifests generate fmt vet ## Build manager binaries for all architectures
	@mkdir -p $(LOCALBIN)
	@for arch in $(BUILD_PLATFORMS); do \
		echo "Building manager for $$arch..." ;\
		GOOS=$(GOOS) GOARCH=$$arch CGO_ENABLED=0 go build \
			-ldflags "$(COMMON_LDFLAGS) $(OPERATOR_LDFLAGS)" \
			-o $(LOCALBIN)/manager_$$arch cmd/main.go ;\
	done
	@cd $(LOCALBIN) && ln -sf manager_$$(go env GOARCH) manager

.PHONY: run
run: fmt vet manifests generate ## Run a controller from your host.
	DEV_MODE=true go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker build \
		--build-arg VERSION=${VERSION} \
		--build-arg BUILD_DATE=${VERSION_DATE} \
		--build-arg TARGETOS=$(GOOS) \
		--build-arg TARGETARCH=$(ARCH) \
		-t ${IMG} \
		.

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	docker run --privileged --rm tonistiigi/binfmt --install all
	docker buildx create --use --name multi-platform-builder || true
	# Build and push multi-platform images
	docker buildx build \
		--platform $(PLATFORMS) \
		--build-arg VERSION=$(shell git describe --tags --always) \
		--build-arg BUILD_DATE=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
		--build-arg BUILDKIT_CONTEXT_KEEP_GIT_DIR=1 \
		--tag ${IMG} \
		--push \
		.
	# Remove the builder instance
	docker buildx rm multi-platform-builder

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	cd config/manager && $(KUSTOMIZE) edit set image controller=ghcr.io/hellices/openapi-aggregator-operator:$(TAG)
	$(KUSTOMIZE) build config/default > install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

# Set image for manager deployment
.PHONY: set-image-controller
set-image-controller: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
.PHONY: deploy
deploy: manifests kustomize set-image-controller
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin

## Create bin directory if it doesn't exist
bin_dir: ## Create bin directory
	mkdir -p $(LOCALBIN)

## Tool Versions and Configurations
TOOLS := kustomize controller-gen setup-envtest golangci-lint operator-sdk
TOOL_VERSIONS := \
    KUSTOMIZE=v5.4.3 \
    CONTROLLER_GEN=v0.16.1 \
    ENVTEST=release-0.19 \
    GOLANGCI_LINT=v1.59.1 \
    OPERATOR_SDK=v1.39.2

# Build Architecture Settings
BUILD_ARCH ?= $(shell go env GOARCH)
ifeq ($(RUNNING_IN_CI),true)
    # Force AMD64 in CI environment
    BUILD_ARCH := amd64
endif

# Tool Paths and URLs
KUBECTL := kubectl
KUSTOMIZE := $(LOCALBIN)/kustomize
CONTROLLER_GEN := $(LOCALBIN)/controller-gen
ENVTEST := $(LOCALBIN)/setup-envtest
GOLANGCI_LINT := $(LOCALBIN)/golangci-lint
OPERATOR_SDK := $(LOCALBIN)/operator-sdk

# Tool URLs
KUSTOMIZE_PKG := sigs.k8s.io/kustomize/kustomize/v5
CONTROLLER_GEN_PKG := sigs.k8s.io/controller-tools/cmd/controller-gen
ENVTEST_PKG := sigs.k8s.io/controller-runtime/tools/setup-envtest
GOLANGCI_LINT_PKG := github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: tools tools-verify kustomize controller-gen envtest golangci-lint
tools: bin_dir ## Download and install all tools
	@echo "Installing tools for $(BUILD_ARCH)..."
	@$(MAKE) kustomize controller-gen envtest golangci-lint
	@echo "All tools installed successfully!"

tools-verify: ## Verify all required tools are installed
	@echo "Verifying tools..."
	@for tool in $(TOOLS); do \
		if [ ! -f "$(LOCALBIN)/$$tool" ]; then \
			echo "❌ Missing tool: $$tool" ;\
			exit 1 ;\
		fi ;\
	done
	@echo "✓ All tools are installed"

kustomize: bin_dir ## Install kustomize
	$(call go-install-tool,$(KUSTOMIZE),$(KUSTOMIZE_PKG),v5.4.3)

controller-gen: bin_dir ## Install controller-gen
	$(call go-install-tool,$(CONTROLLER_GEN),$(CONTROLLER_GEN_PKG),v0.16.1)

envtest: bin_dir ## Install envtest
	$(call go-install-tool,$(ENVTEST),$(ENVTEST_PKG),release-0.19)

golangci-lint: bin_dir ## Install golangci-lint
	$(call go-install-tool,$(GOLANGCI_LINT),$(GOLANGCI_LINT_PKG),v1.59.1)

# Install Go tools
# params: binary-path package-url version
define go-install-tool
@{ \
    if [ -f "$(1)" ]; then \
        echo "Tool already installed: $(1)" ;\
        exit 0 ;\
    fi ;\
    set -e ;\
    echo "Installing $(2)@$(3) for $(BUILD_ARCH)..." ;\
    TEMP_DIR=$$(mktemp -d) ;\
    cd $$TEMP_DIR ;\
    GO111MODULE=on go mod init tmp ;\
    GO111MODULE=on go get $(2)@$(3) ;\
    BASE_NAME=$$(basename $(1)) ;\
    BINARY_NAME="$$BASE_NAME-$(3)-$(BUILD_ARCH)" ;\
    echo "Building $$BINARY_NAME..." ;\
    CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(BUILD_ARCH) go build -o "$$BINARY_NAME" $(2) ;\
    mkdir -p $(LOCALBIN) ;\
    mv "$$BINARY_NAME" "$(LOCALBIN)/" ;\
    cd $(LOCALBIN) ;\
    rm -f "$$BASE_NAME" ;\
    ln -sf "$$BINARY_NAME" "$$BASE_NAME" ;\
    cd $(WORKSPACE_DIR) ;\
    rm -rf $$TEMP_DIR ;\
    echo "✓ Installed $$BASE_NAME for $(BUILD_ARCH)" ;\
}
endef

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = $(LOCALBIN)/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)

##@ Development tools

.PHONY: install-tools
install-tools: bin_dir ## Install all development tools
	$(MAKE) controller-gen
	$(MAKE) kustomize
	$(MAKE) envtest
	$(MAKE) golangci-lint
	$(MAKE) operator-sdk
	@echo "Installing envtest assets..."
	$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN)

# Stop the controller manager
stop:
	@echo "To stop the controller manager, press Ctrl+C in the terminal where it is running."
