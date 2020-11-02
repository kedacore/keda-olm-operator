##################################################
# Variables                                      #
##################################################
VERSION        ?= master
IMAGE_REGISTRY ?= docker.io
IMAGE_REPO     ?= kedacore

IMAGE_CONTROLLER = $(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator:$(VERSION)

ARCH       ?=amd64
CGO        ?=0
TARGET_OS  ?=linux

GIT_COMMIT  = $(shell git rev-list -1 HEAD)

GO_BUILD_VARS= GO111MODULE=on CGO_ENABLED=$(CGO) GOOS=$(TARGET_OS) GOARCH=$(ARCH)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

##################################################
# All                                            #
##################################################
# all: manager
all: build

##################################################
# PUBLISH                                        #
##################################################
publish: docker-build docker-push

.PHONY: set-version
set-version:
	@sed -i 's@Version =.*@Version = "$(VERSION)"@g' ./version/version.go;

# Push the docker image
docker-push:
	docker push ${IMAGE_CONTROLLER}

##################################################
# RUN / (UN)INSTALL / DEPLOY                     #
##################################################
# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate
	go run \
	-ldflags "-X=github.com/kedacore/keda-olm-operator/version.GitCommit=$(GIT_COMMIT) -X=github.com/kedacore/keda-olm-operator/version.Version=$(VERSION)" \
	./main.go $(ARGS)

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && \
	$(KUSTOMIZE) edit set image docker.io/kedacore/keda-olm-operator=${IMAGE_CONTROLLER}
	cd config/default && \
    $(KUSTOMIZE) edit add label -f app.kubernetes.io/version:${VERSION}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Undeploy controller
.PHONY: undeploy
undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -

##################################################
# Build                                          #
##################################################	
.PHONY: build
build: fmt vet manifests manager

# Build the docker image
docker-build:
	docker build . -t ${IMAGE_CONTROLLER}

# Build manager binary
manager: generate set-version
	${GO_BUILD_VARS} go build \
	-ldflags "-X=github.com/kedacore/keda-olm-operator/version.GitCommit=$(GIT_COMMIT) -X=github.com/kedacore/keda-olm-operator/version.Version=$(VERSION)" \
	-o bin/manager main.go

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) crd:crdVersions=v1 rbac:roleName=keda-olm-operator paths="./..." output:crd:artifacts:config=config/crd/bases

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

##################################################
# Test                                           #
##################################################
# Run tests
test-unit: generate fmt vet manifests
	go test ./... -coverprofile cover.out