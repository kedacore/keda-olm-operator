package resources

import (
	"path/filepath"
	"runtime"

	mf "github.com/manifestival/manifestival"
)

const resourcesPath = "keda.yaml"
const olmResourcesPath = "keda-olm-operator.yaml"

func GetResourcesManifest() (mf.Manifest, error) {
	_, path, _, _ := runtime.Caller(0)
	fullPath := filepath.Join(filepath.Dir(path), resourcesPath)
	olmFullPath := filepath.Join(filepath.Dir(path), olmResourcesPath)
	kedamf, err := mf.NewManifest(fullPath)
	if err != nil {
		return kedamf, err
	}
	operatormf, err := mf.NewManifest(olmFullPath)
	return kedamf.Append(operatormf), err
}
