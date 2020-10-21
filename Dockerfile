# Build the manager binary
FROM golang:1.15 as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY Makefile Makefile
COPY .git/ .git/
COPY hack/ hack/

# Copy the go source
COPY version/ version/
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY config/ config/

# Build
RUN make manager

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
# WORKDIR /
# # COPY --from=builder /workspace/config/ config/
# # COPY --from=builder /workspace/manager .
# COPY --from=builder /workspace/ .
# USER nonroot:nonroot
# 
# ENTRYPOINT ["/manager"]
ENTRYPOINT ["/workspace/manager"]