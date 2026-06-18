/*
Copyright 2026 The KEDA Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	mf "github.com/manifestival/manifestival"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kedacore/keda-olm-operator/internal/controller/keda/transform"
)

const (
	gcpSecretName     = "keda-gcp-credentials"
	gcpSecretKey      = "service_account.json"
	gcpCredVolumeName = "gcp-credentials"
	gcpCredMountPath  = "/var/run/secrets/gcp"

	gcpSTSTokenURL              = "https://sts.googleapis.com/v1/token"
	gcpIAMCredentialsURLPattern = "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken"
	gcpWIFAudiencePattern       = "//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s"

	// envGoogleAppCredentials is used by the Google SDK's Application Default
	// Credentials (ADC) chain as a fallback to locate a credential file.
	// We pass this through to the keda-operator Deployment.
	envGoogleAppCredentials = "GOOGLE_APPLICATION_CREDENTIALS"

	// envCloudSDKCoreProject is used by the Google SDK as a fallback for the
	// GCP project ID when the GCE metadata server is unavailable.
	// The operator reads this from its own env, falling back to the
	// OpenShift Infrastructure CR, and passes it through to the
	// keda-operator Deployment.
	envCloudSDKCoreProject = "CLOUDSDK_CORE_PROJECT"
)

type gcpProvider struct{}

func (g *gcpProvider) Name() string { return "GCP WIF" }

// Enabled returns true when the operator pod has either:
//   - AUDIENCE + SERVICE_ACCOUNT_EMAIL (manual/CLI Subscription patch), or
//   - PROJECT_NUMBER + POOL_ID + PROVIDER_ID + SERVICE_ACCOUNT_EMAIL (console install)
func (g *gcpProvider) Enabled() bool {
	if os.Getenv("SERVICE_ACCOUNT_EMAIL") == "" {
		return false
	}
	return os.Getenv("AUDIENCE") != "" ||
		(os.Getenv("PROJECT_NUMBER") != "" && os.Getenv("POOL_ID") != "" && os.Getenv("PROVIDER_ID") != "")
}

func (g *gcpProvider) SecretName() string { return gcpSecretName }

// audience returns the full audience string, constructing it from components
// if AUDIENCE is not set directly.
func (g *gcpProvider) audience() string {
	if aud := os.Getenv("AUDIENCE"); aud != "" {
		return aud
	}
	return fmt.Sprintf(gcpWIFAudiencePattern,
		os.Getenv("PROJECT_NUMBER"), os.Getenv("POOL_ID"), os.Getenv("PROVIDER_ID"),
	)
}

// gcpExternalAccountCredential is the JSON schema for the GCP external_account
// credential file that the GCP SDK reads for workload identity federation.
type gcpExternalAccountCredential struct {
	Type                           string              `json:"type"`
	Audience                       string              `json:"audience"`
	SubjectTokenType               string              `json:"subject_token_type"`
	TokenURL                       string              `json:"token_url"`
	CredentialSource               gcpCredentialSource `json:"credential_source"`
	ServiceAccountImpersonationURL string              `json:"service_account_impersonation_url"`
}

type gcpCredentialSource struct {
	File   string                  `json:"file"`
	Format gcpCredentialFileFormat `json:"format"`
}

type gcpCredentialFileFormat struct {
	Type string `json:"type"`
}

func (g *gcpProvider) BuildCredentialSecret() (map[string][]byte, error) {
	saEmail := os.Getenv("SERVICE_ACCOUNT_EMAIL")
	cred := gcpExternalAccountCredential{
		Type:             "external_account",
		Audience:         g.audience(),
		SubjectTokenType: "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:         gcpSTSTokenURL,
		CredentialSource: gcpCredentialSource{
			File:   CloudTokenPath,
			Format: gcpCredentialFileFormat{Type: "text"},
		},
		ServiceAccountImpersonationURL: fmt.Sprintf(gcpIAMCredentialsURLPattern, saEmail),
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return nil, fmt.Errorf("marshaling GCP credential JSON: %w", err)
	}
	return map[string][]byte{gcpSecretKey: raw}, nil
}

// resolveProjectID returns the GCP project ID, checking CLOUDSDK_CORE_PROJECT
// first and falling back to the Infrastructure CR if on OpenShift.
// Returns empty string if neither source provides a project ID.
func (g *gcpProvider) resolveProjectID(ctx context.Context, reader client.Reader) string {
	if id := os.Getenv(envCloudSDKCoreProject); id != "" {
		return id
	}
	infra := &configv1.Infrastructure{}
	if err := reader.Get(ctx, types.NamespacedName{Name: "cluster"}, infra); err != nil {
		return ""
	}
	if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.GCP != nil {
		return infra.Status.PlatformStatus.GCP.ProjectID
	}
	return ""
}

func (g *gcpProvider) Transforms(ctx context.Context, reader client.Reader, scheme *runtime.Scheme) ([]mf.Transformer, error) {
	projectID := g.resolveProjectID(ctx, reader)

	envVars := []corev1.EnvVar{
		{Name: envGoogleAppCredentials, Value: gcpCredMountPath + "/" + gcpSecretKey},
	}
	if projectID != "" {
		envVars = append(envVars, corev1.EnvVar{Name: envCloudSDKCoreProject, Value: projectID})
	}

	volumes := []corev1.Volume{
		CredentialSecretVolume(gcpCredVolumeName, gcpSecretName),
		BoundSATokenVolume(),
	}
	mounts := []corev1.VolumeMount{
		{Name: gcpCredVolumeName, MountPath: gcpCredMountPath, ReadOnly: true},
		BoundSATokenVolumeMount(),
	}
	return []mf.Transformer{
		transform.ReplaceDeploymentVolumes(volumes, scheme),
		transform.ReplaceDeploymentVolumeMounts(mounts, scheme),
		transform.ReplaceContainerEnv(envVars, "keda-operator", scheme),
	}, nil
}
