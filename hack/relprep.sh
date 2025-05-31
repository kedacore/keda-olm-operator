#!/bin/bash

ver=$1
if ! [[ "$ver" =~ ^[0-9]\.[0-9][0-9]*\.[0-9][0-9]*$ ]]; then
  echo "Usage: $0 <version>"
  echo "Example:  $0 2.11.2"
  exit 1
fi

set -o pipefail
set -e

# these components k8s.io/<item> are versioned for each k8s release and should match the version of k8s used in KEDA for a given release
kube_components="api apimachinery apiextensions-apiserver apiserver client-go component-base kube-aggregator kms"

match_keda_version_deps="sigs.k8s.io/controller-runtime sigs.k8s.io/controller-runtime/tools/setup-envtest sigs.k8s.io/controller-tools"

echo "Fetching sample CRs for KEDA v$ver"
curl -s "https://raw.githubusercontent.com/kedacore/keda/v${ver}/config/samples/kustomization.yaml" > config/samples/kustomization.yaml
for cr in $(sed -n '/^resources:$/,/^[^-]/ { s#[^0-9a-zA-Z_. -]##g; s#^- ##p}' config/samples/kustomization.yaml); do
  curl -s "https://raw.githubusercontent.com/kedacore/keda/v${ver}/config/samples/$cr" > "config/samples/$cr"
done

echo "Updating list of sample CRs to include KedaControllers"
# Since the above fetch of config/samples/kustomization.yaml reverts changes specific to this repo, re-add here
sed -i $'/^resources:$/ a\\\n- keda_v1alpha1_kedacontroller.yaml' config/samples/kustomization.yaml

echo "Fetching go.mod for KEDA v$ver"
keda_gomod="$(curl -s "https://raw.githubusercontent.com/kedacore/keda/v${ver}/go.mod")"

echo -n "Finding out which version of Go KEDA v$ver is using... "
gover=$(echo "$keda_gomod" | grep -Po '(?<=^go )[1-9][0-9]*\.[0-9][0-9]*(?=(\.[0-9]+)?$)')
echo $gover

echo -n "Finding out which K8s version KEDA v$ver is using... "
k8sver=$(echo "$keda_gomod" | grep -Po '(?<=^\tk8s\.io/api )v[0-9]*\.[0-9]*\.[0-9]*$')
echo $k8sver

k8sminorver=$(echo "$k8sver" | sed 's/v0\.\([0-9]*\)\.[0-9]*$/\1/')

echo "Making sure your go version is new enough to run 'go mod tidy' after this script does its updates"
fake_go_ver="go version go$gover"
if test "$( { echo "$fake_go_ver"; go version; } | sort --version-sort | head -1)" != "$fake_go_ver"; then
  echo "Your go version '$(go version)' is not new enough for this script to run 'go mod tidy' after it updates go.mod to 'go $gover'"
  exit 1
fi

echo "Updating go version in go.mod to '$gover'"
sed -i "s/^go  *[1-9][0-9]*\.[0-9][0-9]*$/go $gover/" go.mod
echo "Updatign go version in github workflows"
while read f; do
  echo " $f"
  sed -i "s/^\\(  *go-version: \\) *'[1-9][0-9]*\\.[0-9][0-9]*'\$/\\1'${gover}'/" "$f"
done < <(git grep -Pl "^  *go-version:  *'[1-9][0-9]*\\.[0-9][0-9]*'\$" .github/workflows/)

echo
echo 'Running go mod tidy (pass 1)'
go mod tidy

echo "Getting tag for keda-tools used to build KEDA version $ver"
bttag=$(curl -s "https://raw.githubusercontent.com/kedacore/keda/v${ver}/Dockerfile" | sed -n 's#^FROM.* ghcr.io/kedacore/keda-tools:\([0-9][0-9.]*\) AS builder$#\1#p;T;q')

echo "Updating keda-tools tag to $bttag"
while read f; do
  echo " $f"
  sed -i "s#ghcr.io/kedacore/keda-tools:[0-9][0-9.]*#ghcr.io/kedacore/keda-tools:$bttag#g" "$f"
done < <(git grep -l "ghcr.io/kedacore/keda-tools:[0-9]")

echo "Updating resources from KEDA $ver release"
wget "https://github.com/kedacore/keda/releases/download/v${ver}/keda-${ver}.yaml" -O resources/keda.yaml

echo "Finding previous release version"
prev=$(ls keda/ | grep -v "^${ver//./\\.}$" | sort --version-sort | tail -1)

echo "Using a copy of previous release version $prev as a starting point for $ver"
# using tar for easy overwrite of existing files (for idempotency, in case the script gets runs more than once)
mkdir -p keda/$ver
tar cf - --directory keda/$prev . | tar xvf - --directory keda/$ver
echo "Updating version string in all manifests in keda/${ver}/manifests/"
sed -i 's/\(app.kubernetes.io\/version:\) [0-9.][0-9.]*/\1 '${ver}/ keda/${ver}/manifests/*

date="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo "Updating all version strings, 'replaces' and 'createdAt' fields in base CSV"
sed -i "s/${prev//./\\.}/${ver}/; s/^\\(  replaces: *keda.v\\)[0-9][0-9]*\\.[0-9][0-9]*\\.[0-9][0-9]*/\1${prev}/; s/\\(  *createdAt: *\\)\"?[0-9-]*T[0-9:.]*Z\"/\1\"${date}\"/" config/manifests/bases/keda.clusterserviceversion.yaml

rm -f keda/${ver}/manifests/keda.v${prev}.clusterserviceversion.yaml
echo "Creating release CSV keda.v${ver}.clusterserviceversion.yaml"
cp config/manifests/bases/keda.clusterserviceversion.yaml keda/${ver}/manifests/keda.v${ver}.clusterserviceversion.yaml
sed -i 's#\(image: ghcr.io/kedacore/keda-olm-operator\):main#\1:'"${ver}"'#' keda/${ver}/manifests/keda.v${ver}.clusterserviceversion.yaml

echo "Getting CRD list from resources/keda.yaml"
crds="$(awk 'BEGIN {RS="\n---\n";} /\nkind: CustomResourceDefinition\n/ {print "---";print;}' resources/keda.yaml | sed -n '/^metadata:/,/^[^ ]/ { s/  name: *//p; }')"

echo "Splitting out each CRD from resources/keda.yaml into individual files and copyng to keda/$ver/manifests"
for i in $crds; do
  pluralname="${i%%.*}"
  crdns="${i#*.}"
  echo " $i (for ${crdns}_${pluralname}.yaml)"
  awk 'BEGIN {RS="\n---\n";} /\nkind: CustomResourceDefinition\n.*\n  name: '$i'(\n.*)*\nspec:/ {print "---"; print;}' \
      resources/keda.yaml > config/crd/bases/${crdns}_${pluralname}.yaml
  cp config/crd/bases/${crdns}_${pluralname}.yaml keda/${ver}/manifests/
  if ! ( sed -n '/resources:/,/^[^ -]/ { s/ *- *//p; }' config/crd/kustomization.yaml | grep -qFl "bases/${crdns}_${pluralname}.yaml" ); then
    echo "Error: New CRD 'bases/${crdns}_${pluralname}.yaml' must be manually added to config/crd/kustomization.yaml resources list"
    exit 1
  fi
done

all_mods="$(go list -mod=readonly -m -f '{{ if and (not .Indirect) (not .Main)}}{{.Path}}{{end}}' all)"
declare -A updated_mods
to_update=()

# force versions of k8s components
for i in $kube_components; do
    to_update+=("k8s.io/$i@$k8sver")
    updated_mods["k8s.io/$i"]=1
done

for i in $match_keda_version_deps; do
    echo -n checking upstream version of $i .....
    if ! modver=$(echo "$keda_gomod" | grep -Po '(?<=^\t'"$i"' )v[0-9]*\.[0-9]*\.[0-9]*(-[0-9]*(-[0-9a-e]*)?)?$'); then
      echo "  Unable to find $i in https://raw.githubusercontent.com/kedacore/keda/v${ver}/go.mod .  Exiting!"
      exit 1
    fi
    echo "  got version $modver"
    to_update+=("$i@$modver")
    updated_mods["$i"]=1
done

# hack: force version of openshift API module based upon k8s->openshift version skew (e.g. 1.27 -> 4.14)
openshift_branch="release-4.$((k8sminorver-13))"
to_update+=("github.com/openshift/api@$openshift_branch")
updated_mods[github.com/openshift/api]=1

for m in $all_mods; do
    [ -n "${updated_mods[$m]+1}" ] && continue # already updated
    to_update+=("$m" )
    updated_mods["$m"]=1
done

echo
echo Updating all deps
echo go get "${to_update[@]}"
go get -u "${to_update[@]}"

echo
echo 'Running go mod tidy (pass 2)'
go mod tidy

# make sure we re-build the right version of the build tools
rm -f bin/controller-gen bin/kustomize

echo
echo Updating bundle files
make bundle

echo "Updating the kedacontrollers crd from code and copying it to $ver manifests"
make manifests
cp config/crd/bases/keda.sh_kedacontrollers.yaml keda/${ver}/manifests/
# revert any changes to the kustomization
git co config/manager/kustomization.yaml

echo "Verifying that bundle-generated CSV (for testing) is equivalent to shipping CSV"
ignorefields='createdAt|operators\.operatorframework\.io/builder|app\.kubernetes\.io/version'
bcsv=bundle/manifests/keda.clusterserviceversion.yaml
mcsv=config/manifests/bases/keda.clusterserviceversion.yaml
if ! diff -u <(grep -vE "$ignorefields" < $bcsv) <(grep -vE "$ignorefields" < $mcsv) | \
    sed 's#^--- /dev/fd/[0-9]*#--- '"$bcsv"'#;s#^+++ /dev/fd/[0-9]*#+++ '"$mcsv"'#'; then
  echo "Error: It appears that there are non-trivial differences between $bcsv and $mcsv."
  echo "As appropriate, make changes to $mcsv or the inputs to $bcsv, and re-run this script"
  exit 1
fi

echo "Updating K8s version for envtest components"
sed -i "s#ENVTEST_K8S_VERSION *= *[0-9.]*#ENVTEST_K8S_VERSION = 1.${k8sminorver}#" Makefile

echo Validating bundle
operator-sdk bundle validate ./keda/${ver}
echo
echo "Done! $0 successful"
echo
echo "To Do:"
echo " 1. Validate changes made by this script and 'git add' them"
echo " 2. Verify the code builds and works with vendor & toolchain update. Make any necessary changes to match changed APIs"
echo " 3. Test that the bundle is deployable and functional on an OpenShift cluster:"
echo "      make VERSION=${ver} IMAGE_REGISTRY=quay.io IMAGE_REPO=example_quay_user RESTRICTED=true deploy-olm-testing"
echo " 4. Test that the bundle is deployable and functional on a Kubernetes cluster:"
echo "   a. Install OLM"
echo "      OLM_VERSION=$(curl -s https://api.github.com/repos/operator-framework/operator-lifecycle-manager/releases/latest | jq -r .tag_name)"
echo "      bash <(curl -L https://github.com/operator-framework/operator-lifecycle-manager/releases/download/\$OLM_VERSION/install.sh) \$OLM_VERSION"
echo "   b. Install bundle"
echo "      make VERSION=${ver} IMAGE_REGISTRY=quay.io IMAGE_REPO=example_quay_user deploy-olm-testing"
echo " 5. Follow release process steps 7-10. See: https://github.com/kedacore/keda-olm-operator/blob/main/RELEASE-PROCESS.md#7-commit-and-push-the-changed-code-to-github"
