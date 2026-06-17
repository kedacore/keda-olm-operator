##################################################
# Variables                                      #
##################################################
VERSION        ?= main
IMAGE_REGISTRY ?= ghcr.io
IMAGE_REPO     ?= kedacore

IMAGE_CONTROLLER = $(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator:$(VERSION)

ARCH       ?=amd64
CGO        ?=0
TARGET_OS  ?=linux

GIT_VERSION ?= $(shell git describe --always --abbrev=7)
GIT_COMMIT  ?= $(shell git rev-list -1 HEAD)
DATE        = $(shell date -u +"%Y.%m.%d.%H.%M.%S")

GO_BUILD_VARS= CGO_ENABLED=$(CGO) GOOS=$(TARGET_OS) GOARCH=$(ARCH)

COSIGN_FLAGS ?= -y -a GIT_HASH=${GIT_COMMIT} -a GIT_VERSION=${VERSION} -a BUILD_DATE=${DATE}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# if we're running on a platform where the bundle is going to be deploying into a restricted namespace,
# allow that to be specified so we can supply the proper args
RESTRICTED ?= false
ifeq ($(RESTRICTED),true)
BUNDLE_RUN_OPTS= --security-context-config restricted
endif

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.35

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##################################################
# All                                            #
##################################################
# all: manager
all: build

##@ Development

manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd:crdVersions=v1 rbac:roleName=keda-olm-operator webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run golangci-lint fmt against code.
	golangci-lint fmt

vet: ## Run go vet against code.
	go vet ./...

test-audit: manifests generate
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -v -ginkgo.v -coverprofile cover.out -test.type functionality -ginkgo.focus "Testing audit flags"

test-functionality: manifests generate ## Test functionality.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -v -ginkgo.v -coverprofile cover.out -test.type functionality -ginkgo.focus "Testing functionality"

test-deployment: manifests generate ## Test OLM deployment.
	kubectl create namespace olm --dry-run=client -o yaml | kubectl apply --server-side -f -
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -v -ginkgo.v -coverprofile cover.out -test.type deployment -ginkgo.focus "Deploying KedaController manifest"

test: manifests generate
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -v -ginkgo.v -coverprofile cover.out -test.type unit

.PHONY: e2e-test
e2e-test: ## Run e2e smoke tests against existing cluster.
	go test -tags=e2e -count=1 -timeout=10m -v ./test/e2e/...

.PHONY: e2e-test-ci
e2e-test-ci: ## Run e2e smoke tests (CI mode with GitHub Actions output).
	go tool gotestsum --rerun-fails=2 --format=github-actions --packages="./test/e2e/..." -- -tags=e2e -count=1 -timeout=10m

.PHONY: e2e-olm-setup
e2e-olm-setup: build bundle docker-build docker-push bundle-build bundle-push ## Deploy operator via OLM for e2e testing.
	kubectl create namespace keda --dry-run=client -o yaml | kubectl apply --server-side -f -
	kubectl annotate namespace keda keda-olm-operator/create-default-controller=skip --overwrite
	$(OPERATOR_SDK) run bundle $(BUNDLE) --namespace keda --use-http --timeout 5m $(BUNDLE_RUN_OPTS)
	kubectl rollout status deployment/keda-olm-operator -n keda --timeout=120s

.PHONY: e2e-olm-cleanup
e2e-olm-cleanup: operator-sdk ## Clean up OLM-deployed operator.
	- $(OPERATOR_SDK) cleanup keda --namespace keda

##@ Build

build: generate ## Build manager binary.
	${GO_BUILD_VARS} go build \
	-ldflags "-X=github.com/kedacore/keda-olm-operator/version.GitCommit=$(GIT_COMMIT) -X=github.com/kedacore/keda-olm-operator/version.Version=$(VERSION)" \
	-o bin/manager cmd/main.go

run: manifests generate ## Run a controller from your host.
	WATCH_NAMESPACE="keda" go run ./cmd/main.go

docker-build: ## Build docker image with the manager.
	docker build . -t ${IMAGE_CONTROLLER}  --build-arg BUILD_VERSION=${VERSION} --build-arg GIT_VERSION=${GIT_VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-push: ## Push docker image with the manager.
	docker push ${IMAGE_CONTROLLER}

publish: docker-build docker-push ## Build & push docker image with the manager.

sign-images: ## Sign KEDA images published on GitHub Container Registry
	cosign sign ${COSIGN_FLAGS} $(IMAGE_CONTROLLER)

##@ Deployment

install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply --server-side -f -

uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && \
	$(KUSTOMIZE) edit set image ghcr.io/kedacore/keda-olm-operator=${IMAGE_CONTROLLER}
	cd config/default && \
    $(KUSTOMIZE) edit add label -f app.kubernetes.io/version:${VERSION}
	$(KUSTOMIZE) build config/default | kubectl apply --server-side -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= go tool kustomize
CONTROLLER_GEN ?= go tool controller-gen
ENVTEST ?= go tool setup-envtest
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk

## Tool Versions
# renovate: datasource=github-releases depName=operator-framework/operator-sdk
OPERATOR_SDK_VERSION ?= v1.38.0

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK) ## Download operator-sdk locally if necessary.
$(OPERATOR_SDK): $(LOCALBIN)
	@if test -x $(LOCALBIN)/operator-sdk && ! $(LOCALBIN)/operator-sdk version | grep -q $(OPERATOR_SDK_VERSION); then \
	    echo "$(LOCALBIN)/operator-sdk version is not expected $(OPERATOR_SDK_VERSION). Removing it before downloading."; \
	    rm -rf $(LOCALBIN)/operator-sdk; \
	fi
	test -s $(LOCALBIN)/operator-sdk || \
	    { curl -sSLo $(LOCALBIN)/operator-sdk https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$$(go env GOOS)_$$(go env GOARCH) && \
	    chmod +x $(LOCALBIN)/operator-sdk; }

.PHONY: lint
lint: ## Run golangci-lint against code.
	golangci-lint run

.PHONY: lint-fix
lint-fix: ## Run golangci-lint against code and fix issues.
	golangci-lint run --fix

##@ OLM Bundle

# Default bundle image tag
BUNDLE = $(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator-bundle:$(VERSION)
INDEX = $(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator-index:$(VERSION)
# Options for 'bundle-build'
DEFAULT_CHANNEL?=stable
CHANNELS?=stable

ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests operator-sdk	## Generate bundle manifests and metadata, then validate generated files.
# edit image in config for current changes made to this Makefile so the deployed image is
# the one that is being built & pushed (in case its no ghcr.io/kedacore)
	cd config/manager && \
		$(KUSTOMIZE) edit set image ghcr.io/kedacore/keda-olm-operator=${IMAGE_CONTROLLER}
	cd config/default && \
  	$(KUSTOMIZE) edit add label -f app.kubernetes.io/version:${VERSION}
	$(OPERATOR_SDK) generate kustomize manifests -q
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build	## Build the bundle image.
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE) .

.PHONY: bundle-push
bundle-push:
	docker push ${BUNDLE}
	$(OPERATOR_SDK) bundle validate ${BUNDLE}

.PHONY: index-build
index-build:
	opm index add --bundles ${BUNDLE} --tag ${INDEX} -u docker --permissive

.PHONY: index-push
index-push:
	docker push ${INDEX}

## docker-build & docker-push bellow are added because in generated dir
## bundle/manifests csv.yaml file, it refers to docker-pushed image (aka without "bundle")
## so it needs to be updated as well.

.PHONY: deploy-olm	## Deploy bundle. -- build & bundle to update if changes were made to code
deploy-olm: build bundle docker-build docker-push bundle-build bundle-push index-build index-push
	kubectl create namespace keda --dry-run=client -o yaml | kubectl apply --server-side -f -
	$(OPERATOR_SDK) run bundle ${BUNDLE} --namespace keda $(BUNDLE_RUN_OPTS)

.PHONY: deploy-olm-testing
deploy-olm-testing:
	sed -i 's/keda/keda-test/' bundle/metadata/annotations.yaml
	sed -i 's/keda.v${VERSION}/keda-test.v${VERSION}/' bundle/manifests/keda.clusterserviceversion.yaml
	# disable 'replaces' field, as the testing bundle doesn't replace anything
	sed -i 's/replaces: /# replaces: /' bundle/manifests/keda.clusterserviceversion.yaml

	$(eval BUNDLE=$(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator-bundle-testing:$(VERSION))
	$(eval INDEX=$(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator-index-testing:$(VERSION))
	make deploy-olm

	sed -i 's/keda-test/keda/' bundle/metadata/annotations.yaml
	sed -i 's/keda-test.v${VERSION}/keda.v${VERSION}/' bundle/manifests/keda.clusterserviceversion.yaml


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
