# Build the manager binary
FROM golang:1.16.7 as builder

ARG BUILD_VERSION=main
ARG GIT_COMMIT=HEAD
ARG GIT_VERSION=main

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY Makefile Makefile

# Copy the go source
COPY hack/ hack/
COPY version/ version/
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY resources/ resources/

# Build
RUN VERSION=${BUILD_VERSION} GIT_COMMIT=${GIT_COMMIT} GIT_VERSION=${GIT_VERSION} make build

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/resources/keda.yaml /workspace/resources/keda.yaml
COPY --from=builder /workspace/bin/manager .
USER nonroot:nonroot

ENTRYPOINT ["/manager"]
