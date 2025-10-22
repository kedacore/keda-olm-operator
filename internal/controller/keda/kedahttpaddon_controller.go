/*
Copyright 2025 The KEDA Authors

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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-logr/logr"
	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimruntime "k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/internal/controller/keda/transform"
	"github.com/kedacore/keda-olm-operator/resources"
)

const (
	// Required namespace for KedaHTTPAddOn resources
	kedaHTTPAddOnResourceNamespace = "keda"
	defaultHTTPAddOnVersion        = "0.11.0"

	kedaHTTPAddOnWatchNamespaceEnvName = "KEDA_HTTP_OPERATOR_WATCH_NAMESPACE"
)

// KedaHTTPAddOnReconciler reconciles a KedaHTTPAddOn object
type KedaHTTPAddOnReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *apimruntime.Scheme
	resourceNamespace string
	resources         mf.Manifest
}

func (r *KedaHTTPAddOnReconciler) SetupWithManager(mgr ctrl.Manager, kedaHTTPAddOnResourceNamespace string, logger logr.Logger) error {
	r.resourceNamespace = kedaHTTPAddOnResourceNamespace

	// Load HTTP Add-on manifest
	resourcesManifest, err := getHTTPAddOnResourcesManifest()
	if err != nil {
		return err
	}

	manifestClient := mfc.NewClient(r.Client)
	r.resources, err = mf.ManifestFrom(mf.Slice(resourcesManifest.Resources()), mf.UseLastAppliedConfigAnnotation(resources.LastConfigID))
	if err != nil {
		return err
	}
	r.resources.Client = manifestClient

	return ctrl.NewControllerManagedBy(mgr).
		For(&kedav1alpha1.KedaHTTPAddOn{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=keda.sh,resources=kedahttpaddons;kedahttpaddons/finalizers;kedahttpaddons/status,verbs="*"
// +kubebuilder:rbac:groups="",resources=serviceaccounts;services;configmaps;secrets;pods;events,verbs="*"
// +kubebuilder:rbac:groups=apps,resources=deployments;replicasets;statefulsets,verbs="*"
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;rolebindings,verbs="*"
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=http.keda.sh,resources=httpscaledobjects;httpscaledobjects/status;httpscaledobjects/finalizers,verbs="*"

// Reconcile reads that state of the cluster for a KedaHTTPAddOn object and makes changes based on the state read
// and what is in the KedaHTTPAddOn.Spec
func (r *KedaHTTPAddOnReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("KedaHTTPAddOn", req.NamespacedName)

	logger.Info("Reconciling KedaHTTPAddOn")

	// Fetch the KedaHTTPAddOn instance
	instance := &kedav1alpha1.KedaHTTPAddOn{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if !isHTTPAddOnInteresting(req) {
		msg := fmt.Sprintf("The KedaHTTPAddOn resource needs to be created in namespace %s, otherwise it will be ignored", kedaHTTPAddOnResourceNamespace)
		logger.Info(msg)
		status := instance.Status
		status.MarkIgnored(msg)
		return ctrl.Result{}, updateKedaHTTPAddOnStatus(ctx, r.Client, instance, &status)
	}

	// Validate that no other KedaHTTPAddOn resource has the same WatchNamespace
	if conflict, err := r.checkWatchNamespaceConflict(ctx, instance); err != nil {
		return ctrl.Result{}, err
	} else if conflict != "" {
		msg := fmt.Sprintf("Another KedaHTTPAddOn resource '%s' is already configured with the same WatchNamespace '%s'", conflict, instance.Spec.WatchNamespace)
		logger.Info(msg)
		status := instance.Status
		status.MarkIgnored(msg)
		return ctrl.Result{}, updateKedaHTTPAddOnStatus(ctx, r.Client, instance, &status)
	}

	if instance.GetDeletionTimestamp() != nil {
		if contains(instance.GetFinalizers(), kedaHTTPAddOnFinalizer) {
			// Run finalization logic for kedaHTTPAddOnFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeKedaHTTPAddOn(logger); err != nil {
				return ctrl.Result{}, err
			}
			// Remove kedaHTTPAddOnFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			patch := client.MergeFrom(instance.DeepCopy())
			instance.SetFinalizers(remove(instance.GetFinalizers(), kedaHTTPAddOnFinalizer))
			err := r.Client.Patch(ctx, instance, patch)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), kedaHTTPAddOnFinalizer) {
		if err := r.addHTTPAddOnFinalizer(ctx, logger, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	status := instance.Status

	if err := r.installHTTPAddOn(ctx, logger, instance); err != nil {
		status.MarkInstallFailed("Not able to install KEDA HTTP Add-on")
		if statusErr := updateKedaHTTPAddOnStatus(ctx, r.Client, instance, &status); statusErr != nil {
			err = fmt.Errorf("got error: %s and then another: %s", err, statusErr)
		}
		return ctrl.Result{}, err
	}

	version := resolveHTTPAddOnVersion(instance)
	status.Version = version
	status.MarkInstallSucceeded(fmt.Sprintf("KEDA HTTP Add-on v%s is installed in namespace '%s'", version, r.resourceNamespace))
	if err := updateKedaHTTPAddOnStatus(ctx, r.Client, instance, &status); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KedaHTTPAddOnReconciler) installHTTPAddOn(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaHTTPAddOn) error {
	logger.Info("Reconciling KEDA HTTP Add-on deployment")

	transforms := []mf.Transformer{
		transform.InjectOwner(instance),
		transform.ReplaceAllNamespaces(instance.Namespace),
		transform.ReplaceEnv("keda-http-add-on-operator", kedaHTTPAddOnWatchNamespaceEnvName, instance.Spec.WatchNamespace, r.Scheme, logger),
	}

	// Use custom operator image if specified or from env var
	operatorImage := instance.Spec.OperatorImage
	if operatorImage == "" {
		if envImage := os.Getenv("KEDA_HTTP_ADDON_OPERATOR_IMAGE"); len(envImage) > 0 {
			operatorImage = envImage
		} else {
			// Use default image with version
			version := resolveHTTPAddOnVersion(instance)
			operatorImage = fmt.Sprintf("ghcr.io/kedacore/http-add-on-operator:%s", version)
		}
	}

	// Use custom scaler image if specified
	scalerImage := instance.Spec.ScalerImage
	if scalerImage == "" {
		version := resolveHTTPAddOnVersion(instance)
		scalerImage = fmt.Sprintf("ghcr.io/kedacore/http-add-on-scaler:%s", version)
	}

	// Use custom interceptor image if specified
	interceptorImage := instance.Spec.InterceptorImage
	if interceptorImage == "" {
		version := resolveHTTPAddOnVersion(instance)
		interceptorImage = fmt.Sprintf("ghcr.io/kedacore/http-add-on-interceptor:%s", version)
	}

	transforms = append(transforms, replaceHTTPAddOnOperatorImage(operatorImage, r.Scheme))
	transforms = append(transforms, replaceHTTPAddOnScalerImage(scalerImage, r.Scheme))
	transforms = append(transforms, replaceHTTPAddOnInterceptorImage(interceptorImage, r.Scheme))

	manifest, err := r.resources.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform KEDA HTTP Add-on manifest")
		return err
	}
	r.resources = manifest

	if err := r.resources.Apply(); err != nil {
		logger.Error(err, "Unable to install KEDA HTTP Add-on")
		return err
	}

	return nil
}

// checkWatchNamespaceConflict validates that no other KedaHTTPAddOn resource has the same WatchNamespace
// Returns the name of the conflicting resource if found, or empty string if no conflict exists
func (r *KedaHTTPAddOnReconciler) checkWatchNamespaceConflict(ctx context.Context, instance *kedav1alpha1.KedaHTTPAddOn) (string, error) {
	// List all KedaHTTPAddOn resources in the keda namespace
	httpAddOnList := &kedav1alpha1.KedaHTTPAddOnList{}
	err := r.Client.List(ctx, httpAddOnList, client.InNamespace(kedaHTTPAddOnResourceNamespace))
	if err != nil {
		return "", err
	}

	// Check for WatchNamespace conflicts
	for _, addon := range httpAddOnList.Items {
		// Skip the current instance
		if addon.Name == instance.Name && addon.Namespace == instance.Namespace {
			continue
		}

		// Check if WatchNamespace values match
		if addon.Spec.WatchNamespace == instance.Spec.WatchNamespace {
			return addon.Name, nil
		}
	}

	return "", nil
}

// KedaHTTPAddOn resources must be created in the keda namespace
// Multiple instances are allowed with different WatchNamespaces
func isHTTPAddOnInteresting(request reconcile.Request) bool {
	return request.Namespace == kedaHTTPAddOnResourceNamespace
}

func resolveHTTPAddOnVersion(instance *kedav1alpha1.KedaHTTPAddOn) string {
	if instance.Spec.Version != "" {
		return instance.Spec.Version
	}
	return defaultHTTPAddOnVersion
}

func updateKedaHTTPAddOnStatus(ctx context.Context, client client.Client, instance *kedav1alpha1.KedaHTTPAddOn, status *kedav1alpha1.KedaHTTPAddOnStatus) error {
	instance.Status = *status
	return client.Status().Update(ctx, instance)
}

func getHTTPAddOnResourcesManifest() (mf.Manifest, error) {
	_, path, _, _ := runtime.Caller(0)
	resourcesPath := filepath.Join(filepath.Dir(path), "..", "..", "..", "resources", "keda-http-add-on.yaml")
	return mf.NewManifest(resourcesPath, mf.UseLastAppliedConfigAnnotation(resources.LastConfigID))
}

func replaceHTTPAddOnOperatorImage(image string, scheme *apimruntime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == "keda-http-add-on-operator" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == "operator" {
					containers[i].Image = image
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func replaceHTTPAddOnScalerImage(image string, scheme *apimruntime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == "keda-add-ons-http-external-scaler" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == "external-scaler" {
					containers[i].Image = image
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func replaceHTTPAddOnInterceptorImage(image string, scheme *apimruntime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == "keda-add-ons-http-interceptor" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == "interceptor" {
					containers[i].Image = image
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}
