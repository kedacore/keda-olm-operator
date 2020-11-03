/*
Copyright 2020 The KEDA Authors

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

package controllers

import (
	"context"
	goerrors "errors"
	"fmt"

	"github.com/go-logr/logr"
	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/v1alpha1"
	"github.com/kedacore/keda-olm-operator/controllers/transform"
	"github.com/kedacore/keda-olm-operator/controllers/util"
	"github.com/kedacore/keda-olm-operator/version"
)

const (

	// Allowed Name and Namespace of KedaController resource
	kedaControllerResourceName      = "keda"
	kedaControllerResourceNamespace = "keda"

	installationNamespace = "keda"

	moduleName = "keda-olm-operator"

	metricsServcerServiceName        = "keda-metrics-apiserver"
	metricsServerConfigMapName       = "keda-metrics-apiserver"
	injectCABundleAnnotation         = "service.beta.openshift.io/inject-cabundle"
	injectCABundleAnnotationValue    = "true"
	injectservingCertAnnotation      = "service.beta.openshift.io/serving-cert-secret-name"
	injectservingCertAnnotationValue = "keda-metrics-apiserver"
	roleBindingName                  = "keda-auth-reader"
	roleBindingNamespace             = "kube-system"
)

// KedaControllerReconciler reconciles a KedaController object
type KedaControllerReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	resourcesGeneral    mf.Manifest
	resourcesController mf.Manifest
	resourcesMetrics    mf.Manifest
}

func (r *KedaControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {

	manifestGeneral, manifestController, manifestMetrics, err := parseManifestsFromFile("config/resources/keda-2.0.0-rc.yaml", r.Client)
	if err != nil {
		return err
	}

	r.resourcesGeneral = manifestGeneral
	r.resourcesController = manifestController
	r.resourcesMetrics = manifestMetrics

	createKedaInstallationNamespace(r.Log, mgr.GetClient())

	return ctrl.NewControllerManagedBy(mgr).
		For(&kedav1alpha1.KedaController{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=keda.sh,resources=kedacontrollers;kedacontrollers/finalizers;kedacontrollers/status,verbs="*"
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts;pods;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets,verbs="*"
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs="*"
// +kubebuilder:rbac:groups=apps,resourceNames=keda-olm-operator,resources=deployments/finalizers,verbs="*"
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings;clusterroles;rolebindings,verbs="*"
// +kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices,verbs="*"
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=list

// Reconcile reads that state of the cluster for a KedaController object and makes changes based on the state read
// and what is in the KedaController.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *KedaControllerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log.WithValues("KedaController", req.NamespacedName)

	logger.Info("Reconciling KedaController")

	// Fetch the KedaController instance
	instance := &kedav1alpha1.KedaController{}
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

	if !isInteresting(req) {
		msg := fmt.Sprintf("The KedaController resource needs to be created in namespace %s with name %s, otherwise it will be ignored", kedaControllerResourceNamespace, kedaControllerResourceName)
		logger.Info(msg)
		status := instance.Status.DeepCopy()
		status.MarkIgnored(msg)
		err = util.UpdateKedaControllerStatus(r.Client, instance, status)
		return ctrl.Result{}, nil
	}

	if instance.GetDeletionTimestamp() != nil {
		if contains(instance.GetFinalizers(), kedaControllerFinalizer) {
			// Run finalization logic for kedaControllerFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeKedaController(logger, instance); err != nil {
				return ctrl.Result{}, err
			}
			// Remove kedaControllerFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			patch := client.MergeFrom(instance.DeepCopy())
			instance.SetFinalizers(remove(instance.GetFinalizers(), kedaControllerFinalizer))
			err := r.Client.Patch(ctx, instance, patch)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), kedaControllerFinalizer) {
		if err := r.addFinalizer(logger, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	status := instance.Status.DeepCopy()
	defer util.UpdateKedaControllerStatus(r.Client, instance, status)

	if err := r.installSA(logger, instance); err != nil {
		status.MarkInstallFailed("Not able to create ServiceAccount")
		return ctrl.Result{}, err
	}
	if err := r.installController(logger, instance); err != nil {
		status.MarkInstallFailed("Not able to install KEDA Controller")
		return ctrl.Result{}, err
	}
	if err := r.installMetricsServer(logger, instance); err != nil {
		status.MarkInstallFailed("Not able to install KEDA Metrics Server")
		return ctrl.Result{}, err
	}

	status.Version = version.Version
	status.MarkInstallSucceeded(fmt.Sprintf("KEDA v%s is installed in namespace '%s'", version.Version, installationNamespace))

	return ctrl.Result{}, nil
}

func parseManifestsFromFile(pathname string, c client.Client) (manifestGeneral, manifestController, manifestMetrics mf.Manifest, err error) {
	manifest, err := mf.NewManifest(pathname)
	if err != nil {
		return
	}

	var generalResources, controllerResources, metricsResources []unstructured.Unstructured

	for _, r := range manifest.Resources() {
		switch kind := r.GetKind(); kind {
		case "APIService", "RoleBinding", "Service":
			metricsResources = append(metricsResources, r)
		case "ClusterRole", "ClusterRoleBinding", "Deployment":
			if name := r.GetName(); name == "keda-operator" {
				controllerResources = append(controllerResources, r)
			} else {
				metricsResources = append(metricsResources, r)
			}
		case "Namespace", "ServiceAccount":
			generalResources = append(generalResources, r)
		}
	}

	manifestClient := mfc.NewClient(c)
	manifestGeneral, err = mf.ManifestFrom(mf.Slice(generalResources))
	if err != nil {
		return mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, err
	}
	manifestGeneral.Client = manifestClient

	manifestController, err = mf.ManifestFrom(mf.Slice(controllerResources))
	if err != nil {
		return mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, err
	}
	manifestController.Client = manifestClient

	manifestMetrics, err = mf.ManifestFrom(mf.Slice(sortMetricsResources(&metricsResources)))
	if err != nil {
		return mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, err
	}
	manifestMetrics.Client = manifestClient

	return
}

func sortMetricsResources(resources *[]unstructured.Unstructured) []unstructured.Unstructured {

	sortedResources := make([]unstructured.Unstructured, 7)

	for _, r := range *resources {
		switch kind := r.GetKind(); kind {
		case "ClusterRole":
			sortedResources[0] = r
		case "RoleBinding":
			sortedResources[1] = r
		case "ClusterRoleBinding":
			if name := r.GetName(); name == "keda-hpa-controller-external-metrics" {
				sortedResources[2] = r
			} else {
				sortedResources[3] = r
			}
		case "Service":
			sortedResources[4] = r
		case "Deployment":
			sortedResources[5] = r
		case "APIService":
			sortedResources[6] = r
		}
	}

	return sortedResources
}

func (r *KedaControllerReconciler) installSA(logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling Keda ServiceAccount")
	transforms := []mf.Transformer{mf.InjectOwner(instance)}
	manifest, err := r.resourcesGeneral.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform ServiceAccount manifest")
		return err
	}
	r.resourcesGeneral = manifest

	if err := r.resourcesGeneral.Apply(); err != nil {
		logger.Error(err, "Unable to install ServiceAccount")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) installController(logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling KEDA Controller deployment")
	transforms := []mf.Transformer{
		mf.InjectOwner(instance),
		transform.ReplaceWatchNamespace(instance.Spec.WatchNamespace, "keda-operator", r.Scheme, logger),
	}
	if len(instance.Spec.LogLevel) > 0 {
		transforms = append(transforms, transform.ReplaceKedaOperatorLogLevel(instance.Spec.LogLevel, r.Scheme, logger))
	}
	if len(instance.Spec.LogTimeFormat) > 0 {
		transforms = append(transforms, transform.ReplaceKedaOperatorLogTimeFormat(instance.Spec.LogTimeFormat, r.Scheme, logger))
	}

	manifest, err := r.resourcesController.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform KEDA Controller manifest")
		return err
	}
	r.resourcesController = manifest

	if err := r.resourcesController.Apply(); err != nil {
		logger.Error(err, "Unable to install KEDA Controller")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) installMetricsServer(logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling Metrics Server Deployment")

	transforms := []mf.Transformer{
		mf.InjectOwner(instance),
	}

	// certificates rotation works only on Openshift due to openshift/service-ca-operator
	if util.RunningOnOpenshift(logger, r.Client) {
		if err := r.ensureMetricsServerConfigMap(logger, instance); err != nil {
			logger.Error(err, "Unable to check Metrics Server ConfigMap is present")
			return err
		}

		argsPrefixes := []transform.Prefix{transform.ClientCAFile, transform.TLSCertFile, transform.TLSPrivateKeyFile}
		newArgs := []string{"/cabundle/service-ca.crt", "/certs/tls.crt", "/certs/tls.key"}

		transforms = append(transforms,
			transform.EnsureCertInjectionForAPIService(injectCABundleAnnotation, injectCABundleAnnotationValue, r.Scheme, logger),
			transform.EnsureCertInjectionForService(metricsServcerServiceName, injectservingCertAnnotation, injectservingCertAnnotationValue, r.Scheme, logger),
			transform.EnsureCertInjectionForDeployment(metricsServerConfigMapName, metricsServcerServiceName, r.Scheme, logger),
		)
		transforms = append(transforms, transform.EnsurePathsToCertsInDeployment(newArgs, argsPrefixes, r.Scheme, logger)...)
	} else {
		logger.Info("Not running on OpenShift -> using generated self-signed cert for KEDA Metrics Server")
	}

	if len(instance.Spec.LogLevelMetrics) > 0 {
		transforms = append(transforms, transform.ReplaceMetricsServerLogLevel(instance.Spec.LogLevelMetrics, r.Scheme, logger))
	}

	// replace namespace in RoleBinding from keda to kube-system
	transforms = append(transforms, transform.ReplaceNamespace(roleBindingName, roleBindingNamespace, r.Scheme, logger))

	manifest, err := r.resourcesMetrics.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform Metrics Server manifest")
		return err
	}
	r.resourcesMetrics = manifest

	if err := r.resourcesMetrics.Apply(); err != nil {
		logger.Error(err, "Unable to install Metrics Server")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) ensureMetricsServerConfigMap(logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Ensure ConfigMap for Metrics Server CA bundle exists")

	configMap := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: metricsServerConfigMapName, Namespace: instance.Namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			configMap.Name = metricsServerConfigMapName
			configMap.Namespace = instance.Namespace
			metav1.SetMetaDataAnnotation(&configMap.ObjectMeta, injectCABundleAnnotation, injectCABundleAnnotationValue)

			if err := controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
				logger.Error(err, "Failed to set Controller Reference for ConfigMap")
				return err
			}

			err = r.Client.Create(context.TODO(), configMap)
			if err != nil {
				logger.Error(err, "Failed to create new ConfigMap in cluster", "ConfigMap.Namespace", instance.Namespace, "ConfigMap.Name", metricsServerConfigMapName)
				return err
			}

			return nil
		}
		// Error reading the object
		logger.Error(err, "Failed to get ConfigMap from cluster")
		return err
	}

	configMapUpdate := false

	if !metav1.HasAnnotation(configMap.ObjectMeta, injectCABundleAnnotation) ||
		configMap.Annotations[injectCABundleAnnotation] != injectCABundleAnnotationValue {
		metav1.SetMetaDataAnnotation(&configMap.ObjectMeta, injectCABundleAnnotation, injectCABundleAnnotationValue)
		configMapUpdate = true
	}

	if err := controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
		if !goerrors.Is(err, &controllerutil.AlreadyOwnedError{}) {
			logger.Error(err, "Failed to check Controller Reference for ConfigMap")
			return err
		}
	} else {
		configMapUpdate = true
	}

	if configMapUpdate {
		err = r.Client.Update(context.TODO(), configMap)
		if err != nil {
			logger.Error(err, "Failed to update ConfigMap in cluster", "ConfigMap.Namespace", instance.Namespace, "ConfigMap.Name", metricsServerConfigMapName)
			return err
		}
	}

	return nil
}

// Because it's effectively cluster-scoped, we only care about a
// single, named resource: keda in the specified namespace
func isInteresting(request reconcile.Request) bool {
	return request.Name == kedaControllerResourceName && request.Namespace == kedaControllerResourceNamespace
}

// createKedaInstallationNamespace creates namespace specified in `installationNamespace`
// during the startup of the controller. Creation is skipped if the namespace already exists.
func createKedaInstallationNamespace(logger logr.Logger, cl client.Client) {
	namespace := installationNamespace
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
	unstr := &unstructured.Unstructured{}
	unstr.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": namespace,
		},
	}
	unstr.SetGroupVersionKind(gvk)

	if err := cl.Get(context.TODO(), client.ObjectKey{Name: namespace}, unstr); err != nil {
		if errors.IsNotFound(err) {
			if err = cl.Create(context.TODO(), unstr); err != nil {
				logger.Info("Wasn't able to create KEDA installation namespace", "namespace", namespace, "error", err)
			}
			logger.Info("Creating KEDA installation namespace", "namespace", namespace)
		}
	}
}
