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

package keda

import (
	"bytes"
	"context"
	goerrors "errors"
	"fmt"

	"github.com/go-logr/logr"
	mf "github.com/manifestival/manifestival"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/internal/controller/keda/gcp"
	"github.com/kedacore/keda-olm-operator/internal/controller/keda/util"
)

// gcpWorkloadIdentityTransforms brings the GCP credential Secret in line with cfg
// and returns the transforms that wire it into the keda-operator Deployment. With
// cfg == nil Workload Identity is not configured and no transforms are returned,
// so the Deployment renders without any GCP wiring; a Secret left behind from an
// earlier configuration is removed by deleteGCPCredentialsSecret once that
// Deployment has been applied and nothing references the Secret anymore.
func (r *KedaControllerReconciler) gcpWorkloadIdentityTransforms(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController, cfg *gcp.Config) ([]mf.Transformer, error) {
	if cfg == nil {
		return nil, nil
	}

	logger.Info("Configuring GCP Workload Identity Federation for KEDA Controller", "serviceAccountEmail", cfg.ServiceAccountEmail)

	projectID, universeDomain, err := r.gcpPlatform(ctx, logger)
	if err != nil {
		return nil, err
	}
	if cfg.ProjectID != "" {
		projectID = cfg.ProjectID
	}
	if projectID == "" {
		logger.Info("GCP project ID could not be determined; keda-operator will rely on the GCE metadata server for the default project. Set " + gcp.EnvProjectID + " to configure it explicitly")
	}

	credentials, err := cfg.CredentialsJSON(universeDomain)
	if err != nil {
		return nil, err
	}
	if err := r.ensureGCPCredentialsSecret(ctx, logger, instance, credentials); err != nil {
		return nil, err
	}

	return gcp.DeploymentTransforms(cfg, projectID, gcp.CredentialsHash(credentials), r.Scheme), nil
}

// gcpPlatform reads the GCP project ID and universe domain from the OpenShift
// Infrastructure resource. Both are empty on non-OpenShift clusters and on
// clusters that don't run on GCP. A failed read on OpenShift is an error rather
// than a fallback to the public cloud defaults, so that a transient API problem
// can't overwrite a correct Secret with the wrong endpoints and roll the pods.
func (r *KedaControllerReconciler) gcpPlatform(ctx context.Context, logger logr.Logger) (projectID, universeDomain string, err error) {
	if !util.RunningOnOpenshift(ctx, logger, r.Client) {
		return "", "", nil
	}
	infra := &openshiftconfigv1.Infrastructure{}
	if err := r.Get(ctx, client.ObjectKey{Name: "cluster"}, infra); err != nil {
		return "", "", fmt.Errorf("reading the cluster Infrastructure for the GCP project ID and universe domain: %w", err)
	}
	projectID, universeDomain = gcp.PlatformFromInfrastructure(infra)
	return projectID, universeDomain, nil
}

// gcpCredentialsSecretKey is the name of the credential Secret in the namespace of the KedaController.
func gcpCredentialsSecretKey(instance *kedav1alpha1.KedaController) types.NamespacedName {
	return types.NamespacedName{Name: gcp.CredentialsSecretName, Namespace: instance.Namespace}
}

// ensureGCPCredentialsSecret creates or updates the credential Secret so that it
// carries exactly credentials and is controlled by the KedaController, which
// makes garbage collection remove it together with the KedaController.
func (r *KedaControllerReconciler) ensureGCPCredentialsSecret(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController, credentials []byte) error {
	key := gcpCredentialsSecretKey(instance)
	desiredLabels := gcp.CredentialsSecretLabels()

	secret := &corev1.Secret{}
	err := r.Get(ctx, key, secret)
	if errors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
				Labels:    desiredLabels,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{gcp.CredentialsSecretKey: credentials},
		}
		if err := controllerutil.SetControllerReference(instance, secret, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference for GCP credentials Secret")
			return err
		}
		logger.Info("Creating GCP credentials Secret", "Secret.Namespace", key.Namespace, "Secret.Name", key.Name)
		if err := r.Create(ctx, secret); err != nil {
			logger.Error(err, "Failed to create GCP credentials Secret")
			return err
		}
		return nil
	}
	if err != nil {
		logger.Error(err, "Failed to get GCP credentials Secret")
		return err
	}

	needsUpdate := false

	if !metav1.IsControlledBy(secret, instance) {
		// A Secret with our name that is controlled by something else is not ours to
		// overwrite. An unowned one (for example created by hand) is adopted, unless it
		// is of another type: the type is immutable, so it can't be turned into the
		// credential Secret in place, and it clearly wasn't created for this purpose.
		if secret.Type != "" && secret.Type != corev1.SecretTypeOpaque {
			return fmt.Errorf("secret %s has type %q and can't be used for GCP credentials; remove or rename it",
				key, secret.Type)
		}
		if err := controllerutil.SetControllerReference(instance, secret, r.Scheme); err != nil {
			var alreadyOwnedErr *controllerutil.AlreadyOwnedError
			if goerrors.As(err, &alreadyOwnedErr) {
				return fmt.Errorf("secret %s is controlled by %s %s and can't be used for GCP credentials",
					key, alreadyOwnedErr.Owner.Kind, alreadyOwnedErr.Owner.Name)
			}
			logger.Error(err, "Failed to set controller reference for GCP credentials Secret")
			return err
		}
		needsUpdate = true
	}

	if !bytes.Equal(secret.Data[gcp.CredentialsSecretKey], credentials) || len(secret.Data) != 1 {
		secret.Data = map[string][]byte{gcp.CredentialsSecretKey: credentials}
		secret.StringData = nil
		needsUpdate = true
	}

	for k, v := range desiredLabels {
		if secret.Labels[k] != v {
			if secret.Labels == nil {
				secret.Labels = map[string]string{}
			}
			secret.Labels[k] = v
			needsUpdate = true
		}
	}

	if !needsUpdate {
		return nil
	}
	logger.Info("Updating GCP credentials Secret", "Secret.Namespace", key.Namespace, "Secret.Name", key.Name)
	if err := r.Update(ctx, secret); err != nil {
		logger.Error(err, "Failed to update GCP credentials Secret")
		return err
	}
	return nil
}

// deleteGCPCredentialsSecret removes the credential Secret if it was created by
// this KedaController. Secrets of the same name that belong to someone else are
// left alone.
func (r *KedaControllerReconciler) deleteGCPCredentialsSecret(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	key := gcpCredentialsSecretKey(instance)
	secret := &corev1.Secret{}
	if err := r.Get(ctx, key, secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		logger.Error(err, "Failed to get GCP credentials Secret")
		return err
	}
	if !metav1.IsControlledBy(secret, instance) {
		logger.V(4).Info("Secret is not controlled by the KedaController, leaving it in place", "Secret.Namespace", key.Namespace, "Secret.Name", key.Name)
		return nil
	}
	logger.Info("GCP Workload Identity Federation is not configured, deleting GCP credentials Secret", "Secret.Namespace", key.Namespace, "Secret.Name", key.Name)
	if err := r.Delete(ctx, secret); err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Failed to delete GCP credentials Secret")
		return err
	}
	return nil
}
