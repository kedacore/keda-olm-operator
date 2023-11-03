# Release Process

The release process of a new version of KEDA OLM Operator involves the following:

## 0. Prerequisites

Look at the last released KEDA version in the releases page: https://github.com/kedacore/keda/releases
For example: currently it is 2.8.0

## 1. KEDA release yaml file

Copy contents of released KEDA yaml file to the `resource/keda.yaml` file in this directory.
For example: https://github.com/kedacore/keda/releases/download/v2.8.0/keda-2.8.0.yaml

## 2. Create a new Bundle

In `keda` directory copy the directory of lastly released version (eg. `2.8.0`) and create a new one (eg. `2.9.0`)
```bash
cp -r keda/2.8.0 keda/2.9.0
```

## 3. Update KEDA CRDs
Update all KEDA CRDs in the newly (eg. `2.9.0`) created directory, get the up-to-date version from the release file mentioned in [step 1](#1-keda-release-yaml-file).

To update the `keda.sh_kedacontrollers.yaml` CRD, perform the following steps:

```bash
make manifests
cp config/crd/bases/keda.sh_kedacontrollers.yaml keda/2.9.0/manifests/
```

## 4. Update CSV file
Update ClusterServiceVersion file in the newly (eg. `2.9.0`) created directory:
- rename the file to respect the version.
```bash
mv keda/2.9.0/manifests/keda.v2.8.0.clusterserviceversion.yaml keda/2.9.0/manifests/keda.v2.9.0.clusterserviceversion.yaml
```
- add new CRDs (if there are any new introduced in KEDA or KEDA OLM Operator)
- update the description and fields in the existing CRDs (if needed), make sure to update it on all places
- update the version reference (eg. locate all occurencies of previous version)
- update the `replaces` field to point to the previous version (eg. `2.8.0`)
- update `createdAt` field with a new date
- update other necessary fields

## 5. Update dependencies if necessary
- bump Go version in go.mod and Dockerfile
- update `go.mod` file versions to match `kedacore/keda` repository versions (subsequently update go.sum)
- update necessary dependent packages (based on previous bumps)

## 6. Validate and test the bundle
- validate the new bundle, eg:
```
operator-sdk bundle validate ./keda/2.9.0
```
- test that the bundle is deployable and functional on OpenShift instance and on a vanilla K8s cluster with OLM installed.

## 7. Commit and push the changed code to GitHub
```bash
git checkout -b release290
git commit -s -a -m 'prepare release 2.9.0'
git push origin release290
```

## 8. Create KEDA release on GitHub

Creating a new release in the releases page (https://github.com/kedacore/keda-olm-operator/releases) will trigger a GitHub workflow which will create a new image with the latest code and tagged with the next version (in this example 2.9.0).

> Note: The GitHub Container Registry repo with all the different images can be seen here: https://github.com/kedacore/keda-olm-operator/pkgs/container/keda-olm-operator


## 9. Publish KEDA OLM Operator on OperatorHub.io
Create pull request on https://github.com/k8s-operatorhub/community-operators:
- copy the newly created bundle directory from keda-olm-repo (eg. `keda/2.9.0`) to `operators/keda` directory in [https://github.com/k8s-operatorhub/community-operators](https://github.com/k8s-operatorhub/community-operators), you should see the previous version over there
- send a pull request with this change
