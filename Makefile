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

GO_BUILD_VARS= GO111MODULE=on CGO_ENABLED=$(CGO) GOOS=$(TARGET_OS) GOARCH=$(ARCH)

COSIGN_FLAGS ?= -a GIT_HASH=${GIT_COMMIT} -a GIT_VERSION=${VERSION} -a BUILD_DATE=${DATE}

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.26

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

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd:crdVersions=v1 rbac:roleName=keda-olm-operator webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

test-audit: manifests generate fmt vet envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -v -ginkgo.v -coverprofile cover.out -test.type functionality -ginkgo.focus "Testing audit flags"

test-functionality: manifests generate fmt vet envtest ## Test functionality.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -v -ginkgo.v -coverprofile cover.out -test.type functionality -ginkgo.focus "Testing functionality"

test-deployment: manifests generate fmt vet envtest ## Test OLM deployment.
	kubectl create namespace olm --dry-run=client -o yaml | kubectl apply --server-side -f -
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -v -ginkgo.v -coverprofile cover.out -test.type deployment -ginkgo.focus "Deploying KedaController manifest"

test: manifests generate fmt vet envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -v -ginkgo.v -coverprofile cover.out -test.type unit

##@ Build

build: generate fmt vet ## Build manager binary.
	${GO_BUILD_VARS} go build \
	-ldflags "-X=github.com/kedacore/keda-olm-operator/version.GitCommit=$(GIT_COMMIT) -X=github.com/kedacore/keda-olm-operator/version.Version=$(VERSION)" \
	-o bin/manager main.go

run: manifests generate fmt vet ## Run a controller from your host.
	WATCH_NAMESPACE="keda" go run ./main.go

docker-build: ## Build docker image with the manager.
	docker build . -t ${IMAGE_CONTROLLER}  --build-arg BUILD_VERSION=${VERSION} --build-arg GIT_VERSION=${GIT_VERSION} --build-arg GIT_COMMIT=${GIT_COMMIT}

docker-push: ## Push docker image with the manager.
	docker push ${IMAGE_CONTROLLER}

publish: docker-build docker-push ## Build & push docker image with the manager.

sign-images: ## Sign KEDA images published on GitHub Container Registry
	COSIGN_EXPERIMENTAL=1 cosign sign ${COSIGN_FLAGS} $(IMAGE_CONTROLLER)

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply --server-side -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
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
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Install controller-gen from vendor dir if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Install kustomize from vendor dir if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) go install sigs.k8s.io/kustomize/kustomize/v5

.PHONY: envtest
envtest: $(ENVTEST) ## Install envtest-setup from vendor dir if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest

# Run golangci against code
.PHONY: golangci
golangci:	## Run golangci against code.
	golangci-lint run

##@ OLM Bundle

# Default bundle image tag
BUNDLE = $(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator-bundle:$(VERSION)
INDEX = $(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator-index:$(VERSION)
# Options for 'bundle-build'
DEFAULT_CHANNEL=alpha

ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize	## Generate bundle manifests and metadata, then validate generated files.
# edit image in config for current changes made to this Makefile so the deployed image is
# the one that is being built & pushed (in case its no ghcr.io/kedacore)
	cd config/manager && \
		$(KUSTOMIZE) edit set image ghcr.io/kedacore/keda-olm-operator=${IMAGE_CONTROLLER}
	cd config/default && \
  	$(KUSTOMIZE) edit add label -f app.kubernetes.io/version:${VERSION}
	operator-sdk generate kustomize manifests -q
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build	## Build the bundle image.
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE) .

.PHONY: bundle-push
bundle-push:
	docker push ${BUNDLE}
	operator-sdk bundle validate ${BUNDLE}

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
	operator-sdk run bundle ${BUNDLE} --namespace keda

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
