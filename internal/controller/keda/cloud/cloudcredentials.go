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
	"fmt"

	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	BoundSATokenPath   = "/var/run/secrets/openshift/serviceaccount"
	CloudTokenPath     = BoundSATokenPath + "/token"
	KedaOperatorSAName = "keda-operator"
)

// CloudCredentialProvider encapsulates the provider-specific parts of the
// workload identity workflow.
type CloudCredentialProvider interface {
	Name() string
	Enabled() bool

	// SecretName returns the name for the credential Secret the operator creates.
	SecretName() string

	// BuildCredentialSecret returns the Secret data (key → value) for the
	// cloud-specific credential file (e.g. GCP external_account JSON).
	BuildCredentialSecret() (map[string][]byte, error)

	// Transforms returns the manifestival transforms to wire the credential
	// Secret and env vars into the keda-operator Deployment.
	Transforms(ctx context.Context, reader client.Reader, scheme *runtime.Scheme) ([]mf.Transformer, error)
}

// Providers returns the registered cloud credential providers. The first one
// whose Enabled() returns true wins.
func Providers() []CloudCredentialProvider {
	return []CloudCredentialProvider{
		// TODO(maxcao13): add more here if we ever support AWS STS or Azure AD Workload Identity.
		&gcpProvider{},
	}
}

// EnsureCredentialSecret creates or updates the cloud credential Secret for
// the given provider. The Secret is owned by the KedaController CR so that
// garbage collection handles cleanup automatically.
func EnsureCredentialSecret(ctx context.Context, c client.Client, p CloudCredentialProvider, namespace string, owner metav1.Object, scheme *runtime.Scheme) (controllerutil.OperationResult, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.SecretName(),
			Namespace: namespace,
		},
	}
	result, err := controllerutil.CreateOrUpdate(ctx, c, secret, func() error {
		secret.Labels = map[string]string{
			"app.kubernetes.io/name": "keda-operator",
		}
		data, buildErr := p.BuildCredentialSecret()
		if buildErr != nil {
			return fmt.Errorf("building credential data for %s: %w", p.Name(), buildErr)
		}
		secret.Data = data
		return controllerutil.SetOwnerReference(owner, secret, scheme)
	})
	return result, err
}

// BoundSATokenVolume returns the projected volume that mounts the
// OpenShift-issued service account token, shared across all providers.
func BoundSATokenVolume() corev1.Volume {
	return corev1.Volume{
		Name: "bound-sa-token",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{
					{
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							// TODO: "openshift" matches the allowed-audiences that ccoctl
							// configures on the GCP WIF OIDC provider. Currently, GCP WIF
							// is only supported on OpenShift clusters.
							Audience: "openshift",
							Path:     "token",
						},
					},
				},
			},
		},
	}
}

// BoundSATokenVolumeMount returns the volume mount for the bound SA token.
func BoundSATokenVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      "bound-sa-token",
		MountPath: BoundSATokenPath,
		ReadOnly:  true,
	}
}

// CredentialSecretVolume returns a volume backed by the credential Secret.
func CredentialSecretVolume(volumeName, secretName string) corev1.Volume {
	return corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
}
