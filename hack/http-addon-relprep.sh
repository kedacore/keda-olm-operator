#!/bin/bash

# Script to update HTTP Add-on manifests for a new release version.
# This script downloads the release manifest from GitHub and updates local files.
#
# Usage: ./hack/http-addon-relprep.sh <version>
#   Example: ./hack/http-addon-relprep.sh 0.12.0

ver=$1
if ! [[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Usage: $0 <version>"
  echo "Example:  $0 0.12.0"
  exit 1
fi

set -o pipefail
set -e

echo "Updating HTTP Add-on resources for version $ver"

# Download the main manifest from GitHub releases
manifest_url="https://github.com/kedacore/http-add-on/releases/download/v${ver}/keda-add-ons-http-${ver}.yaml"
echo "Downloading manifest from ${manifest_url}"
if ! curl -L --fail "${manifest_url}" | sed 's/\r//g' > resources/keda-http-addon.yaml; then
  echo "Error: Failed to download manifest. Please verify version ${ver} exists at:"
  echo "  https://github.com/kedacore/http-add-on/releases"
  exit 1
fi

# Post-process the manifest to fix labels
echo "Fixing labels in manifest..."
# Remove 'app.kubernetes.io/managed-by: kustomize' - resources are managed by our operator, not kustomize
sed -i '/app\.kubernetes\.io\/managed-by: kustomize/d' resources/keda-http-addon.yaml
# Replace 'app.kubernetes.io/version: HEAD' with the actual version
sed -i "s/app\.kubernetes\.io\/version: HEAD/app.kubernetes.io\/version: ${ver}/" resources/keda-http-addon.yaml

# Extract the HTTPScaledObject CRD from the manifest and save it separately
echo "Extracting HTTPScaledObject CRD to config/crd/bases/"
crd_file="config/crd/bases/http.keda.sh_httpscaledobjects.yaml"
awk 'BEGIN {RS="\n---\n";} /\nkind: CustomResourceDefinition\n.*\n  name: httpscaledobjects.http.keda.sh/ {print "---"; print;}' \
    resources/keda-http-addon.yaml > "${crd_file}"

if [ ! -s "${crd_file}" ]; then
  echo "Warning: HTTPScaledObject CRD not found in manifest or extraction failed"
  rm -f "${crd_file}"
else
  echo "CRD extracted to ${crd_file}"
fi

# Verify the manifest contains expected components
echo ""
echo "Verifying manifest contents..."
components=("interceptor" "operator" "scaler")
for component in "${components[@]}"; do
  if grep -q "http-add-on-${component}" resources/keda-http-addon.yaml; then
    # Extract version from the image tag
    img_ver=$(grep -oP "ghcr.io/kedacore/http-add-on-${component}:\K[0-9]+\.[0-9]+\.[0-9]+" resources/keda-http-addon.yaml | head -1)
    echo "  ✓ ${component} component found (image version: ${img_ver})"
  else
    echo "  ✗ ${component} component NOT found - manifest may be incomplete"
  fi
done

echo ""
echo "Done! HTTP Add-on manifest updated to version ${ver}"
echo ""
echo "Files updated:"
echo "  - resources/keda-http-addon.yaml"
[ -s "${crd_file}" ] && echo "  - ${crd_file}"
echo ""
echo "Next steps:"
echo "  1. Review the changes: git diff resources/keda-http-addon.yaml"
echo "  2. Update httpaddon_types.go if the CRD schema has changed"
echo "  3. Run 'make manifests' to regenerate controller manifests"
echo "  4. Test the changes with a KedaController that has httpAddon.enabled: true"
echo "  5. Commit the changes when validated"
