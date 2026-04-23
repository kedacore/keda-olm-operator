#!/bin/bash

# Script to update HTTP Add-on manifests for a new release version.
# This script downloads the release manifest from GitHub, extracts CRDs into
# config/crd/bases/, strips CRDs and Namespace documents from the runtime
# manifest, and copies CRDs to the OLM bundle directory.
#
# Usage: ./hack/http-add-on-relprep.sh <version>
#   Example: ./hack/http-add-on-relprep.sh 0.13.0

ver=$1
if ! [[ "$ver" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Usage: $0 <version>"
  echo "Example:  $0 0.13.0"
  exit 1
fi

set -o pipefail
set -e

echo "Updating HTTP Add-on resources for version $ver"

# Download the main manifest from GitHub releases
manifest_url="https://github.com/kedacore/http-add-on/releases/download/v${ver}/keda-add-ons-http-${ver}.yaml"
echo "Downloading manifest from ${manifest_url}"
tmpfile=$(mktemp)
if ! curl -L --fail "${manifest_url}" | sed 's/\r//g' > "$tmpfile"; then
  echo "Error: Failed to download manifest. Please verify version ${ver} exists at:"
  echo "  https://github.com/kedacore/http-add-on/releases"
  rm -f "$tmpfile"
  exit 1
fi

# Post-process the manifest to fix labels
echo "Fixing labels in manifest..."
sed -i '/app\.kubernetes\.io\/managed-by: kustomize/d' "$tmpfile"
sed -i "s/app\.kubernetes\.io\/version: HEAD/app.kubernetes.io\/version: ${ver}/" "$tmpfile"

# Extract all CRDs from the manifest using a generic loop (same approach as relprep.sh)
echo ""
echo "Extracting CRDs from manifest..."
crds="$(awk 'BEGIN {RS="\n---\n";} /\nkind: CustomResourceDefinition\n/ {print "---";print;}' "$tmpfile" | sed -n '/^metadata:/,/^[^ ]/ { s/  name: *//p; }')"

if [ -z "$crds" ]; then
  echo "Warning: No CRDs found in manifest"
else
  for i in $crds; do
    pluralname="${i%%.*}"
    crdns="${i#*.}"
    crd_file="config/crd/bases/${crdns}_${pluralname}.yaml"
    echo "  Extracting $i -> ${crd_file}"
    awk 'BEGIN {RS="\n---\n";} /\nkind: CustomResourceDefinition\n.*\n  name: '$i'(\n.*)*\nspec:/ {print "---"; print;}' \
        "$tmpfile" > "${crd_file}"

    if [ ! -s "${crd_file}" ]; then
      echo "  Warning: Extraction failed for $i"
      rm -f "${crd_file}"
      continue
    fi

    # Verify CRD is listed in config/crd/kustomization.yaml
    if ! ( sed -n '/resources:/,/^[^ -]/ { s/ *- *//p; }' config/crd/kustomization.yaml | grep -qFl "bases/${crdns}_${pluralname}.yaml" ); then
      echo "  Error: New CRD 'bases/${crdns}_${pluralname}.yaml' must be manually added to config/crd/kustomization.yaml resources list"
      rm -f "$tmpfile"
      exit 1
    fi

    # Copy CRDs to OLM bundle directory if it exists
    keda_ver=$(ls keda/ 2>/dev/null | sort --version-sort | tail -1)
    if [ -n "$keda_ver" ] && [ -d "keda/${keda_ver}/manifests" ]; then
      echo "  Copying to keda/${keda_ver}/manifests/"
      cp "${crd_file}" "keda/${keda_ver}/manifests/"
    fi
  done
fi

# Strip CRDs and Namespace documents from the runtime manifest
echo ""
echo "Stripping CRDs and Namespace documents from runtime manifest..."
python3 -c "
import re, sys
with open(sys.argv[1]) as f:
    content = f.read()
docs = re.split(r'\n---\n', content)
kept = []
for doc in docs:
    stripped = doc.strip()
    if not stripped:
        continue
    if re.search(r'^kind:\s*CustomResourceDefinition', stripped, re.M):
        continue
    if re.search(r'^kind:\s*Namespace', stripped, re.M):
        continue
    kept.append(doc)
with open(sys.argv[2], 'w') as f:
    f.write('\n---\n'.join(kept))
" "$tmpfile" resources/keda-http-addon.yaml

rm -f "$tmpfile"

# Verify the manifest contains expected components
echo ""
echo "Verifying manifest contents..."
components=("interceptor" "operator" "scaler")
for component in "${components[@]}"; do
  if grep -q "http-add-on-${component}" resources/keda-http-addon.yaml; then
    img_ver=$(grep -oP "ghcr.io/kedacore/http-add-on-${component}:\K[0-9]+\.[0-9]+\.[0-9]+" resources/keda-http-addon.yaml | head -1)
    echo "  OK: ${component} component found (image version: ${img_ver})"
  else
    echo "  MISSING: ${component} component NOT found - manifest may be incomplete"
  fi
done

echo ""
echo "Done! HTTP Add-on manifest updated to version ${ver}"
echo ""
echo "Files updated:"
echo "  - resources/keda-http-addon.yaml (runtime manifest, CRDs/Namespace stripped)"
for i in $crds; do
  pluralname="${i%%.*}"
  crdns="${i#*.}"
  crd_file="config/crd/bases/${crdns}_${pluralname}.yaml"
  [ -s "${crd_file}" ] && echo "  - ${crd_file}"
done
echo ""
echo "Next steps:"
echo "  1. Review the changes: git diff resources/ config/crd/"
echo "  2. Run 'make manifests' to regenerate controller manifests"
echo "  3. Test the changes with a KedaController that has httpAddon.enabled: true"
echo "  4. Commit the changes when validated"
