#!/bin/bash

# Script to generate the keda-http-addon.yaml manifest from the upstream HTTP Add-on Helm chart
# This script renders the Helm chart templates and outputs a single YAML file
# that can be embedded in the operator.
#
# Usage: ./hack/generate-http-addon-manifest.sh [VERSION]
#   VERSION: HTTP Add-on version (default: 0.9.0)
#
# Prerequisites:
#   - helm (https://helm.sh/docs/intro/install/)
#   - yq (https://github.com/mikefarah/yq)
#
# Note: The generated manifest may need manual adjustments:
#   - Update kube-rbac-proxy image to a maintained version (e.g., quay.io/brancz/kube-rbac-proxy:v0.20.2)
#   - Verify namespace references are set to "keda" (will be replaced at runtime)
#   - Review any deprecated API versions

set -euo pipefail

HTTP_ADDON_VERSION="${1:-0.9.0}"
OUTPUT_FILE="resources/keda-http-addon.yaml"
TEMP_DIR=$(mktemp -d)

cleanup() {
    rm -rf "${TEMP_DIR}"
}
trap cleanup EXIT

echo "Generating HTTP Add-on manifest for version ${HTTP_ADDON_VERSION}..."

# Add the KEDA Helm repository
helm repo add kedacore https://kedacore.github.io/charts 2>/dev/null || true
helm repo update kedacore

# Render the HTTP Add-on Helm chart with default values
# We use --include-crds to include the HTTPScaledObject CRD
helm template keda-http-add-on kedacore/keda-add-ons-http \
    --version "${HTTP_ADDON_VERSION}" \
    --namespace keda \
    --include-crds \
    > "${TEMP_DIR}/rendered.yaml"

# Post-process the manifest:
# 1. Remove Helm-specific labels and annotations (helm.sh/*, app.kubernetes.io/managed-by: Helm)
# 2. Sort resources by kind for consistency
# 3. Add separator between documents

cat "${TEMP_DIR}/rendered.yaml" | \
    yq eval 'del(.metadata.labels."helm.sh/chart") | del(.metadata.labels."app.kubernetes.io/managed-by") | del(.metadata.annotations."meta.helm.sh/release-name") | del(.metadata.annotations."meta.helm.sh/release-namespace")' - \
    > "${OUTPUT_FILE}"

echo "Generated ${OUTPUT_FILE}"
echo ""
echo "IMPORTANT: Please review the generated manifest and make the following adjustments if needed:"
echo "  1. Update kube-rbac-proxy image from gcr.io/kubebuilder/kube-rbac-proxy to quay.io/brancz/kube-rbac-proxy:v0.20.2"
echo "  2. Verify all namespace references are set to 'keda'"
echo "  3. Check for any deprecated API versions"
echo ""
echo "HTTP Add-on version: ${HTTP_ADDON_VERSION}"
