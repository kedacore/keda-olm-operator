#!/usr/bin/env bash

# Copyright 2025 The KEDA Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Test script for KEDA HTTP Add-on installation on Kind cluster
# This script:
# 1. Creates a Kind cluster (optional)
# 2. Builds and deploys the KEDA OLM Operator
# 3. Installs KEDA with HTTP Add-on enabled
# 4. Verifies the installation
# 5. Optionally deploys a sample application with HTTPScaledObject

set -euo pipefail

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-keda-http-addon-test}"
KEDA_NAMESPACE="${KEDA_NAMESPACE:-keda}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-ghcr.io}"
IMAGE_REPO="${IMAGE_REPO:-kedacore}"
VERSION="${VERSION:-test}"
SKIP_CLUSTER_CREATE="${SKIP_CLUSTER_CREATE:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"
DEPLOY_SAMPLE_APP="${DEPLOY_SAMPLE_APP:-false}"
CLEANUP="${CLEANUP:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check required tools
check_prerequisites() {
    log_info "Checking prerequisites..."

    local missing_tools=()

    for tool in kind kubectl docker make; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        fi
    done

    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        exit 1
    fi

    log_success "All prerequisites are installed"
}

# Create Kind cluster
create_kind_cluster() {
    if [ "$SKIP_CLUSTER_CREATE" = "true" ]; then
        log_info "Skipping cluster creation (SKIP_CLUSTER_CREATE=true)"
        return
    fi

    # Check if cluster already exists
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_warning "Kind cluster '${CLUSTER_NAME}' already exists"
        read -p "Do you want to delete and recreate it? [y/N] " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Deleting existing cluster..."
            kind delete cluster --name "${CLUSTER_NAME}"
        else
            log_info "Using existing cluster"
            return
        fi
    fi

    log_info "Creating Kind cluster '${CLUSTER_NAME}'..."

    # Create kind cluster with extra port mappings for ingress
    cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
  - containerPort: 8080
    hostPort: 8080
    protocol: TCP
EOF

    log_success "Kind cluster '${CLUSTER_NAME}' created successfully"
}

# Build and load the operator image into Kind
build_and_load_image() {
    if [ "$SKIP_BUILD" = "true" ]; then
        log_info "Skipping image build (SKIP_BUILD=true)"
        return
    fi

    log_info "Building KEDA OLM Operator image..."

    local image="${IMAGE_REGISTRY}/${IMAGE_REPO}/keda-olm-operator:${VERSION}"

    # Build the image
    make docker-build IMAGE_REGISTRY="${IMAGE_REGISTRY}" IMAGE_REPO="${IMAGE_REPO}" VERSION="${VERSION}"

    # Load image directly into Kind cluster (avoids registry issues)
    log_info "Loading image into Kind cluster..."
    kind load docker-image "${image}" --name "${CLUSTER_NAME}"

    log_success "Image '${image}' built and loaded into Kind cluster successfully"
}

# Deploy the KEDA OLM Operator
deploy_operator() {
    log_info "Deploying KEDA OLM Operator..."

    # Create keda namespace
    kubectl create namespace "${KEDA_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

    # Delete existing deployment to avoid server-side apply conflicts
    kubectl delete deployment keda-olm-operator -n "${KEDA_NAMESPACE}" --ignore-not-found=true

    # Install CRDs
    log_info "Installing CRDs..."
    make install

    # Deploy the operator
    log_info "Deploying operator..."
    make deploy IMAGE_REGISTRY="${IMAGE_REGISTRY}" IMAGE_REPO="${IMAGE_REPO}" VERSION="${VERSION}"

    # Patch the deployment to use local image (imagePullPolicy: Never)
    # This is required because the image is loaded directly into Kind, not pulled from a registry
    log_info "Patching deployment to use local image..."
    kubectl patch deployment keda-olm-operator -n "${KEDA_NAMESPACE}" \
        --type='json' \
        -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/imagePullPolicy", "value": "Never"}]'

    # Wait for operator to be ready
    log_info "Waiting for KEDA OLM Operator to be ready..."
    kubectl wait --for=condition=available --timeout=120s deployment/keda-olm-operator -n "${KEDA_NAMESPACE}"

    log_success "KEDA OLM Operator deployed successfully"
}

# Create KedaController with HTTP Add-on enabled
create_keda_controller() {
    log_info "Creating KedaController with HTTP Add-on enabled..."

    cat <<EOF | kubectl apply -f -
apiVersion: keda.sh/v1alpha1
kind: KedaController
metadata:
  name: keda
  namespace: ${KEDA_NAMESPACE}
spec:
  watchNamespace: ""
  
  operator:
    logLevel: info
    logEncoder: console
  
  metricsServer:
    logLevel: "0"
  
  admissionWebhooks:
    logLevel: info
    logEncoder: console
  
  # Enable HTTP Add-on
  httpAddon:
    enabled: true

    operator:
      replicas: 1
      logging:
        level: info
        format: console

    scaler:
      replicas: 1
      logging:
        level: info
        format: console

    interceptor:
      replicas:
        min: 1
        max: 10
      logging:
        level: info
        format: console
      podDisruptionBudget:
        enabled: true
        maxUnavailable: 1
EOF

    log_success "KedaController resource created"
}

# Wait for all KEDA components to be ready
wait_for_keda() {
    log_info "Waiting for KEDA components to be ready..."

    # Wait for KEDA Operator
    log_info "Waiting for KEDA Operator..."
    kubectl wait --for=condition=available --timeout=180s deployment/keda-operator -n "${KEDA_NAMESPACE}" || {
        log_error "KEDA Operator did not become ready"
        kubectl describe deployment keda-operator -n "${KEDA_NAMESPACE}"
        return 1
    }

    # Wait for KEDA Metrics Server
    log_info "Waiting for KEDA Metrics Server..."
    kubectl wait --for=condition=available --timeout=180s deployment/keda-metrics-apiserver -n "${KEDA_NAMESPACE}" || {
        log_error "KEDA Metrics Server did not become ready"
        kubectl describe deployment keda-metrics-apiserver -n "${KEDA_NAMESPACE}"
        return 1
    }

    # Wait for KEDA Admission Webhooks
    log_info "Waiting for KEDA Admission Webhooks..."
    kubectl wait --for=condition=available --timeout=180s deployment/keda-admission -n "${KEDA_NAMESPACE}" || {
        log_error "KEDA Admission Webhooks did not become ready"
        kubectl describe deployment keda-admission -n "${KEDA_NAMESPACE}"
        return 1
    }

    log_success "All KEDA components are ready"
}

# Wait for HTTP Add-on components to be ready
wait_for_http_addon() {
    log_info "Waiting for HTTP Add-on components to be ready..."

    # Wait for HTTP Add-on Operator
    log_info "Waiting for HTTP Add-on Operator..."
    kubectl wait --for=condition=available --timeout=180s deployment/keda-http-add-on-operator -n "${KEDA_NAMESPACE}" || {
        log_error "HTTP Add-on Operator did not become ready"
        kubectl describe deployment keda-http-add-on-operator -n "${KEDA_NAMESPACE}"
        return 1
    }

    # Wait for HTTP Add-on Scaler
    log_info "Waiting for HTTP Add-on Scaler..."
    kubectl wait --for=condition=available --timeout=180s deployment/keda-http-add-on-external-scaler -n "${KEDA_NAMESPACE}" || {
        log_error "HTTP Add-on Scaler did not become ready"
        kubectl describe deployment keda-http-add-on-external-scaler -n "${KEDA_NAMESPACE}"
        return 1
    }

    # Wait for HTTP Add-on Interceptor (ScaledObject controlled)
    log_info "Waiting for HTTP Add-on Interceptor..."
    local timeout=180
    local elapsed=0
    while [ $elapsed -lt $timeout ]; do
        local ready=$(kubectl get deployment keda-http-add-on-interceptor -n "${KEDA_NAMESPACE}" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        if [ "$ready" != "" ] && [ "$ready" != "0" ]; then
            break
        fi
        sleep 5
        elapsed=$((elapsed + 5))
        log_info "Waiting for interceptor... (${elapsed}s/${timeout}s)"
    done

    if [ $elapsed -ge $timeout ]; then
        log_error "HTTP Add-on Interceptor did not become ready"
        kubectl describe deployment keda-http-add-on-interceptor -n "${KEDA_NAMESPACE}"
        return 1
    fi

    log_success "All HTTP Add-on components are ready"
}

# Deploy sample application with HTTPScaledObject
deploy_sample_app() {
    if [ "$DEPLOY_SAMPLE_APP" != "true" ]; then
        log_info "Skipping sample app deployment (DEPLOY_SAMPLE_APP=false)"
        return
    fi

    log_info "Deploying sample application with HTTPScaledObject..."

    local sample_namespace="http-sample"

    # Create namespace
    kubectl create namespace "${sample_namespace}" --dry-run=client -o yaml | kubectl apply -f -

    # Deploy sample nginx application
    cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-app
  namespace: ${sample_namespace}
spec:
  replicas: 0
  selector:
    matchLabels:
      app: sample-app
  template:
    metadata:
      labels:
        app: sample-app
    spec:
      containers:
      - name: nginx
        image: nginx:alpine
        ports:
        - containerPort: 80
        resources:
          limits:
            cpu: 100m
            memory: 128Mi
          requests:
            cpu: 50m
            memory: 64Mi
---
apiVersion: v1
kind: Service
metadata:
  name: sample-app
  namespace: ${sample_namespace}
spec:
  selector:
    app: sample-app
  ports:
  - port: 80
    targetPort: 80
EOF

    # Create HTTPScaledObject
    cat <<EOF | kubectl apply -f -
apiVersion: http.keda.sh/v1alpha1
kind: HTTPScaledObject
metadata:
  name: sample-app-httpso
  namespace: ${sample_namespace}
spec:
  hosts:
  - sample-app.local
  pathPrefixes:
  - /
  scaleTargetRef:
    name: sample-app
    service: sample-app
    port: 80
  replicas:
    min: 0
    max: 5
  scalingMetric:
    requestRate:
      targetValue: 10
      window: 1m
EOF

    log_success "Sample application deployed"
    log_info "HTTPScaledObject created in namespace '${sample_namespace}'"
    log_info "Send requests to the interceptor proxy to trigger scaling:"
    log_info "  kubectl port-forward svc/keda-http-add-on-interceptor-proxy -n ${KEDA_NAMESPACE} 8080:8080"
    log_info "  curl -H 'Host: sample-app.local' http://localhost:8080/"
}

# Verify the installation
verify_installation() {
    log_info "Verifying installation..."

    echo ""
    echo "=========================================="
    echo "KEDA Components Status"
    echo "=========================================="
    kubectl get deployments -n "${KEDA_NAMESPACE}"

    echo ""
    echo "=========================================="
    echo "KEDA Pods Status"
    echo "=========================================="
    kubectl get pods -n "${KEDA_NAMESPACE}"

    echo ""
    echo "=========================================="
    echo "KedaController Status"
    echo "=========================================="
    kubectl get kedacontroller keda -n "${KEDA_NAMESPACE}" -o yaml | grep -A 10 "status:"

    echo ""
    echo "=========================================="
    echo "HTTPScaledObject CRD"
    echo "=========================================="
    kubectl get crd httpscaledobjects.http.keda.sh 2>/dev/null && log_success "HTTPScaledObject CRD is installed" || log_error "HTTPScaledObject CRD is not installed"

    echo ""
    echo "=========================================="
    echo "Services"
    echo "=========================================="
    kubectl get services -n "${KEDA_NAMESPACE}"

    log_success "Installation verification complete"
}

# Cleanup function
cleanup() {
    if [ "$CLEANUP" != "true" ]; then
        log_info "Skipping cleanup (CLEANUP=false)"
        return
    fi

    log_info "Cleaning up..."

    # Delete KedaController
    kubectl delete kedacontroller keda -n "${KEDA_NAMESPACE}" --ignore-not-found

    # Wait for resources to be deleted
    sleep 10

    # Undeploy operator
    make undeploy || true

    # Delete Kind cluster
    if [ "$SKIP_CLUSTER_CREATE" != "true" ]; then
        kind delete cluster --name "${CLUSTER_NAME}"
    fi

    log_success "Cleanup complete"
}

# Print usage
usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Test KEDA HTTP Add-on installation on Kind cluster.

Options:
    -h, --help              Show this help message
    -c, --cluster-name      Kind cluster name (default: keda-http-addon-test)
    -n, --namespace         KEDA namespace (default: keda)
    -s, --skip-cluster      Skip cluster creation (use existing cluster)
    -b, --skip-build        Skip image build (use existing image)
    -a, --deploy-sample     Deploy sample application with HTTPScaledObject
    -x, --cleanup           Cleanup resources after test

Environment variables:
    CLUSTER_NAME            Kind cluster name
    KEDA_NAMESPACE          KEDA namespace
    IMAGE_REGISTRY          Image registry (default: ghcr.io)
    IMAGE_REPO              Image repository (default: kedacore)
    VERSION                 Image version (default: test)
    SKIP_CLUSTER_CREATE     Skip cluster creation (true/false)
    SKIP_BUILD              Skip image build (true/false)
    DEPLOY_SAMPLE_APP       Deploy sample app (true/false)
    CLEANUP                 Cleanup after test (true/false)

Examples:
    # Full test with new cluster
    $0

    # Use existing cluster, skip build
    $0 --skip-cluster --skip-build

    # Deploy with sample app
    $0 --deploy-sample

    # Cleanup after test
    $0 --cleanup
EOF
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                usage
                exit 0
                ;;
            -c|--cluster-name)
                CLUSTER_NAME="$2"
                shift 2
                ;;
            -n|--namespace)
                KEDA_NAMESPACE="$2"
                shift 2
                ;;
            -s|--skip-cluster)
                SKIP_CLUSTER_CREATE="true"
                shift
                ;;
            -b|--skip-build)
                SKIP_BUILD="true"
                shift
                ;;
            -a|--deploy-sample)
                DEPLOY_SAMPLE_APP="true"
                shift
                ;;
            -x|--cleanup)
                CLEANUP="true"
                shift
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

# Main function
main() {
    parse_args "$@"

    echo ""
    echo "=========================================="
    echo "KEDA HTTP Add-on Installation Test"
    echo "=========================================="
    echo "Cluster Name:    ${CLUSTER_NAME}"
    echo "Namespace:       ${KEDA_NAMESPACE}"
    echo "Image:           ${IMAGE_REGISTRY}/${IMAGE_REPO}/keda-olm-operator:${VERSION}"
    echo "Skip Cluster:    ${SKIP_CLUSTER_CREATE}"
    echo "Skip Build:      ${SKIP_BUILD}"
    echo "Deploy Sample:   ${DEPLOY_SAMPLE_APP}"
    echo "Cleanup:         ${CLEANUP}"
    echo "=========================================="
    echo ""

    check_prerequisites
    create_kind_cluster
    build_and_load_image
    deploy_operator
    create_keda_controller
    wait_for_keda
    wait_for_http_addon
    deploy_sample_app
    verify_installation
    cleanup

    echo ""
    log_success "=========================================="
    log_success "KEDA HTTP Add-on test completed successfully!"
    log_success "=========================================="
    echo ""

    if [ "$CLEANUP" != "true" ]; then
        log_info "To access the cluster:"
        log_info "  kubectl config use-context kind-${CLUSTER_NAME}"
        echo ""
        log_info "To cleanup:"
        log_info "  kind delete cluster --name ${CLUSTER_NAME}"
        
        if [ "$DEPLOY_SAMPLE_APP" = "true" ]; then
            echo ""
            log_info "=========================================="
            log_info "Sample App Usage Instructions"
            log_info "=========================================="
            echo ""
            log_info "To send requests to the sample app, first port-forward the Interceptor service:"
            log_info "  kubectl port-forward -n ${KEDA_NAMESPACE} svc/keda-http-add-on-interceptor-proxy 8888:8080"
            echo ""
            log_info "Then, in another terminal, you can send requests using curl:"
            log_info "  curl -H \"Host: sample-app.local\" localhost:8888/"
        fi
    fi
}

# Run main function
main "$@"

