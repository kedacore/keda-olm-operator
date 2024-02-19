package resources

import (
	"fmt"
	"path/filepath"
	"runtime"

	mf "github.com/manifestival/manifestival"
)

const resourcesPath = "keda.yaml"
const olmResourcesPath = "keda-olm-operator.yaml"
const LastConfigID = "olm-operator.keda.sh/last-applied-configuration"

func GetResourcesManifest(kedaRelease string) (mf.Manifest, error) {
	_, path, _, _ := runtime.Caller(0)
	fullPath := filepath.Join(filepath.Dir(path), kedaRelease, resourcesPath)
	olmFullPath := filepath.Join(filepath.Dir(path), kedaRelease, olmResourcesPath)
	kedamf, err := mf.NewManifest(fullPath, mf.UseLastAppliedConfigAnnotation(LastConfigID))
	if err != nil {
		return kedamf, fmt.Errorf("error creating manifest from %s: %v", fullPath, err)
	}
	operatormf, err := mf.NewManifest(olmFullPath, mf.UseLastAppliedConfigAnnotation(LastConfigID))
	if err != nil {
		return mf.Manifest{}, fmt.Errorf("error creating manifest from %s: %v", olmFullPath, err)
	}
	return kedamf.Append(operatormf), nil
}
