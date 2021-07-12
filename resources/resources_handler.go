package resources

import (
	"path/filepath"
	"runtime"

	mf "github.com/manifestival/manifestival"
)

const resourcesPath = "keda.yaml"

func GetResourcesManifest() (mf.Manifest, error) {
	_, path, _, _ := runtime.Caller(0)
	fullPath := filepath.Join(filepath.Dir(path), resourcesPath)
	return mf.NewManifest(fullPath)
}
