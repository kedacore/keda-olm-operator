package resources

import (
	"bytes"
	"embed"

	mf "github.com/manifestival/manifestival"
)

//go:embed keda.yaml keda-olm-operator.yaml keda-http-addon.yaml
var content embed.FS

const (
	resourcesPath          = "keda.yaml"
	olmResourcesPath       = "keda-olm-operator.yaml"
	httpAddonResourcesPath = "keda-http-addon.yaml"
	LastConfigID           = "olm-operator.keda.sh/last-applied-configuration"
)

func GetResourcesManifest() (mf.Manifest, error) {
	kedamf, err := manifestFromEmbed(resourcesPath)
	if err != nil {
		return kedamf, err
	}
	operatormf, err := manifestFromEmbed(olmResourcesPath)
	return kedamf.Append(operatormf), err
}

func GetHTTPAddonResourcesManifest() (mf.Manifest, error) {
	return manifestFromEmbed(httpAddonResourcesPath)
}

func manifestFromEmbed(name string) (mf.Manifest, error) {
	data, err := content.ReadFile(name)
	if err != nil {
		return mf.Manifest{}, err
	}
	return mf.ManifestFrom(mf.Reader(bytes.NewReader(data)), mf.UseLastAppliedConfigAnnotation(LastConfigID))
}
