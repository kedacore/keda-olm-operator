package resources

import (
	"fmt"
	"path/filepath"
	"runtime"

	mf "github.com/manifestival/manifestival"
	"golang.org/x/mod/semver"
)

const resourcesPath = "keda.yaml"
const olmResourcesPath = "keda-olm-operator.yaml"
const LastConfigID = "olm-operator.keda.sh/last-applied-configuration"

func GetResourcesManifest(kedaRelease string) (mf.Manifest, error) {
	if err := validateSemanticVersion(kedaRelease); err != nil {
		return mf.Manifest{}, err
	}
	_, path, _, _ := runtime.Caller(0)
	fullPath := filepath.Join(filepath.Dir(path), kedaRelease, resourcesPath)
	olmFullPath := filepath.Join(filepath.Dir(path), kedaRelease, olmResourcesPath)
	kedamf, err := mf.NewManifest(fullPath, mf.UseLastAppliedConfigAnnotation(LastConfigID))
	if err != nil {
		return mf.Manifest{}, fmt.Errorf("error creating manifest from %s, KEDA release %q not supported: %v", fullPath, kedaRelease, err)
	}
	operatormf, err := mf.NewManifest(olmFullPath, mf.UseLastAppliedConfigAnnotation(LastConfigID))
	if err != nil {
		return mf.Manifest{}, fmt.Errorf("error creating manifest from %s, KEDA release %q not supported: %v", olmFullPath, kedaRelease, err)
	}
	return kedamf.Append(operatormf), nil
}

func validateSemanticVersion(v string) error {
	if v == "" {
		// we allow empty version for reverse compatibility
		return nil
	}
	if !semver.IsValid(v) {
		return fmt.Errorf("given KEDA release version %q is not a valid semantic version. Use a valid release, for example v2.12.1", v)
	}
	return nil
}
