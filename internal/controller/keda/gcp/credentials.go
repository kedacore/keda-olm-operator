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

package gcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	mf "github.com/manifestival/manifestival"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kedacore/keda-olm-operator/internal/controller/keda/transform"
)

const (
	// CredentialsSecretName is the Secret holding the external_account credential
	// configuration. The key matches the one the OpenShift Cloud Credential Operator
	// uses, so the content is interchangeable with a ccoctl-generated Secret.
	CredentialsSecretName = "keda-gcp-credentials"
	CredentialsSecretKey  = "service_account.json"

	credentialsVolumeName = "gcp-credentials"
	credentialsMountPath  = "/var/run/secrets/gcp"
	// CredentialsFilePath is where keda-operator reads the credential configuration from.
	CredentialsFilePath = credentialsMountPath + "/" + CredentialsSecretKey

	// The projected Kubernetes service account token follows the OpenShift convention
	// for token authentication, so that the layout is the same as for operators that
	// use the Cloud Credential Operator.
	boundSATokenVolumeName = "bound-sa-token"
	boundSATokenMountPath  = "/var/run/secrets/openshift/serviceaccount"
	boundSATokenFileName   = "token"
	// SubjectTokenFilePath is the token keda-operator exchanges at the GCP STS endpoint.
	SubjectTokenFilePath = boundSATokenMountPath + "/" + boundSATokenFileName

	// DefaultUniverseDomain is the domain of the public Google Cloud API endpoints.
	DefaultUniverseDomain = "googleapis.com"

	// CredentialsHashAnnotation is set on the keda-operator pod template so that a
	// change of the credential configuration restarts the pods. The Google auth
	// library loads the configuration once per client, so a pod would otherwise keep
	// impersonating the previous service account until it gets restarted for some
	// other reason.
	CredentialsHashAnnotation = "kedacontroller.keda.sh/gcp-credentials-hash"

	// envGoogleApplicationCredentials points Application Default Credentials at the
	// credential configuration file.
	envGoogleApplicationCredentials = "GOOGLE_APPLICATION_CREDENTIALS"
	// envCloudSDKCoreProject is the project keda-operator defaults to for scalers that
	// don't specify one. KEDA reads it before falling back to the GCE metadata server.
	envCloudSDKCoreProject = "CLOUDSDK_CORE_PROJECT"

	// kedaOperatorName is both the container to wire the credentials into and the
	// app the credential Secret is labelled as part of.
	kedaOperatorName = "keda-operator"
)

// CredentialsSecretLabels returns the labels the operator sets on the credential Secret.
func CredentialsSecretLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":    CredentialsSecretName,
		"app.kubernetes.io/part-of": kedaOperatorName,
	}
}

// externalAccountCredentials is the credential configuration consumed by the
// Google auth libraries for workload identity federation. The field order matches
// the file the Cloud Credential Operator generates.
type externalAccountCredentials struct {
	Type                           string           `json:"type"`
	Audience                       string           `json:"audience"`
	SubjectTokenType               string           `json:"subject_token_type"`
	TokenURL                       string           `json:"token_url"`
	ServiceAccountImpersonationURL string           `json:"service_account_impersonation_url"`
	CredentialSource               credentialSource `json:"credential_source"`
	UniverseDomain                 string           `json:"universe_domain,omitempty"`
}

type credentialSource struct {
	File   string           `json:"file"`
	Format credentialFormat `json:"format"`
}

type credentialFormat struct {
	Type string `json:"type"`
}

// CredentialsJSON renders the external_account credential configuration for the
// Google Cloud universe identified by universeDomain (DefaultUniverseDomain for
// the public cloud).
func (c *Config) CredentialsJSON(universeDomain string) ([]byte, error) {
	if universeDomain == "" {
		universeDomain = DefaultUniverseDomain
	}
	creds := externalAccountCredentials{
		Type:                           "external_account",
		Audience:                       c.Audience,
		SubjectTokenType:               "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:                       fmt.Sprintf("https://sts.%s/v1/token", universeDomain),
		ServiceAccountImpersonationURL: fmt.Sprintf("https://iamcredentials.%s/v1/projects/-/serviceAccounts/%s:generateAccessToken", universeDomain, c.ServiceAccountEmail),
		CredentialSource: credentialSource{
			File:   SubjectTokenFilePath,
			Format: credentialFormat{Type: "text"},
		},
	}
	if universeDomain != DefaultUniverseDomain {
		creds.UniverseDomain = universeDomain
	}
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling GCP credential configuration: %w", err)
	}
	return data, nil
}

// CredentialsHash returns the value for CredentialsHashAnnotation.
func CredentialsHash(credentials []byte) string {
	sum := sha256.Sum256(credentials)
	return hex.EncodeToString(sum[:])
}

// PlatformFromInfrastructure returns the GCP project ID and universe domain of the
// cluster, or empty strings when the cluster does not run on GCP.
func PlatformFromInfrastructure(infra *configv1.Infrastructure) (projectID, universeDomain string) {
	if infra == nil || infra.Status.PlatformStatus == nil || infra.Status.PlatformStatus.GCP == nil {
		return "", ""
	}
	return infra.Status.PlatformStatus.GCP.ProjectID, infra.Status.PlatformStatus.GCP.UniverseDomain
}

// DeploymentTransforms wires the credential Secret and the projected service
// account token into the keda-operator container and points Application Default
// Credentials at them. projectID may be empty, in which case keda-operator falls
// back to the GCE metadata server for the default project.
func DeploymentTransforms(cfg *Config, projectID, credentialsHash string, scheme *runtime.Scheme) []mf.Transformer {
	volumes := []corev1.Volume{
		{
			Name: boundSATokenVolumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Audience: cfg.SubjectTokenAudience,
							Path:     boundSATokenFileName,
						},
					}},
				},
			},
		},
		{
			Name: credentialsVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: CredentialsSecretName,
				},
			},
		},
	}
	mounts := []corev1.VolumeMount{
		{Name: boundSATokenVolumeName, MountPath: boundSATokenMountPath, ReadOnly: true},
		{Name: credentialsVolumeName, MountPath: credentialsMountPath, ReadOnly: true},
	}
	env := []corev1.EnvVar{
		{Name: envGoogleApplicationCredentials, Value: CredentialsFilePath},
	}
	if projectID != "" {
		env = append(env, corev1.EnvVar{Name: envCloudSDKCoreProject, Value: projectID})
	}

	return []mf.Transformer{
		transform.ReplaceDeploymentVolumes(volumes, scheme),
		transform.ReplaceContainerVolumeMounts(mounts, kedaOperatorName, scheme),
		transform.ReplaceContainerEnv(env, kedaOperatorName, scheme),
		transform.AddPodAnnotations(map[string]string{CredentialsHashAnnotation: credentialsHash}, scheme),
	}
}
