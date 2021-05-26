# Release Process

The release process of a new version of KEDA OLM Operator involves the following:

## 0. Prerequisites

Look at the last released KEDA version in the releases page: https://github.com/kedacore/keda/releases
For example: currently it is 2.1.0

## 1. KEDA release yaml file

Copy contents of released KEDA yaml file to the `resource/keda.yaml` file in this directory.
For example: https://github.com/kedacore/keda/releases/download/v2.1.0/keda-2.1.0.yaml

## 2. Create a new Bundle

In `keda` directory copy the directory of lastly released version (eg. `2.0.0`) and create a new one (eg. `2.1.0`)
```bash
cp -r keda/2.0.0 keda/2.1.0
```

## 3. Update KEDA CRDs
Update all KEDA CRDs in the newly (eg. `2.1.0`) created directory, get the up-to-date version from the release file mentioned in step 1.

## 4. Update CSV file
Update ClusterServiceVersion file in the newly (eg. `2.1.0`) created directory:
- rename the file to respect the version.
```bash
mv keda/2.1.0/manifests/keda.v2.0.0.clusterserviceversion keda/2.1.0/manifests/keda.v2.1.0.clusterserviceversion
```
- add new CRDs (if there are any new introduced in KEDA or KEDA OLM Operator)
- update the description and fields in the existing CRDs (if needed), make sure to update it on all places
- update the version reference (eg. locate all occurencies of previous version)
- update the `replaces` field to point to the previous version (eg. `2.0.0`)
- update other necessary fields

## 5. Validate and test the bundle
- validate the new bundle, eg:
```
operator-sdk bundle validate ./keda/2.1.0
```
- test that the bundle is deployable and functional on OpenShift instance.

## 6. Commit and push the changed code to GitHub
```bash
git checkout -b release210
git commit -s -a -m 'prepare release 2.1.0'
git push origin release210
```

## 7. Create KEDA release on GitHub

Creating a new release in the releases page (https://github.com/kedacore/keda/releases) will trigger a GitHub workflow which will create a new image with the latest code and tagged with the next version (in this example 2.1.0).

> Note: The Docker Hub repo with all the different images can be seen here: https://hub.docker.com/r/kedacore/keda-olm-operator/tags


## 8. Publish KEDA OLM Operator on OperatorHub.io
Create 2 pull requests on https://github.com/operator-framework/community-operators:
- copy the newly created bundle directory from keda-olm-repo (eg. `keda/2.1.0`) to `community-operators/keda` directory in [community operators repo](https://github.com/operator-framework/community-operators), you should see the previous version over there 
- send a pull request with this change
- repeat the same but now copy the new bundle to `upstream-community-operators/keda` directory in [community operators repo](https://github.com/operator-framework/community-operators)
- send this change in **a separate PR**
