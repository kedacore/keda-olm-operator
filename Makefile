##################################################
# Variables                                      #
##################################################
IMAGE_TAG      ?= master
IMAGE_REGISTRY ?= docker.io
IMAGE_REPO     ?= kedacore

IMAGE_CONTROLLER = $(IMAGE_REGISTRY)/$(IMAGE_REPO)/keda-olm-operator:$(IMAGE_TAG)


ARCH       ?=amd64
CGO        ?=0
TARGET_OS  ?=linux

GIT_VERSION = $(shell git describe --always --abbrev=7)
GIT_COMMIT  = $(shell git rev-list -1 HEAD)
DATE        = $(shell date -u +"%Y.%m.%d.%H.%M.%S")

##################################################
# All                                            #
##################################################
.PHONY: All
all: build


##################################################
# PUBLISH                                        #
##################################################
.PHONY: publish
publish: build
	docker push $(IMAGE_CONTROLLER)

.PHONY: dev
dev:
	$(GO_BUILD_VARS) operator-sdk build $(IMAGE_CONTROLLER) \
		--go-build-args "-ldflags -X=main.GitCommit=$(GIT_COMMIT) -o build/_output/bin/keda-olm-operator"
	docker push $(IMAGE_CONTROLLER)

##################################################
# Build                                          #
##################################################
GO_BUILD_VARS= GO111MODULE=on CGO_ENABLED=$(CGO) GOOS=$(TARGET_OS) GOARCH=$(ARCH)

.PHONY: build
build: generate-api 
	$(GO_BUILD_VARS) operator-sdk build $(IMAGE_CONTROLLER) \
		--go-build-args "-ldflags -X=main.GitCommit=$(GIT_COMMIT) -o build/_output/bin/keda-olm-operator"


.PHONY: generate-api
generate-api:
	$(GO_BUILD_VARS) operator-sdk generate k8s
	$(GO_BUILD_VARS) operator-sdk generate crds

##################################################
# Test                                           #
##################################################
.PHONY: test-unit
test-unit:
	$(GO_BUILD_VARS) go test ./...
