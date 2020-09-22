/*


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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/v1alpha1"



	"context"
	goerrors "errors"
	"fmt"

	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	// kedav1alpha1 "github.com/kedacore/keda-olm-operator/pkg/apis/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/controllers/transform"
	"github.com/kedacore/keda-olm-operator/controllers/util"
	"github.com/kedacore/keda-olm-operator/version"

	// "github.com/operator-framework/operator-sdk/pkg/predicate"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (

	// Allowed Name and Namespace of KedaController resource
	kedaControllerResourceName      = "keda"
	kedaControllerResourceNamespace = "keda"

	installationNamespace = "keda"

	metricsServcerServiceName        = "keda-metrics-apiserver"
	metricsServerConfigMapName       = "keda-metrics-apiserver"
	injectCABundleAnnotation         = "service.beta.openshift.io/inject-cabundle"
	injectCABundleAnnotationValue    = "true"
	injectservingCertAnnotation      = "service.beta.openshift.io/serving-cert-secret-name"
	injectservingCertAnnotationValue = "keda-metrics-apiserver"

	resourcesPrefix = "deploy/resources/"

	resourceNamespace      = resourcesPrefix + "00-namespace.yaml"
	resourceServiceAccount = resourcesPrefix + "01-service_account.yaml"

	resourceClusterRole = resourcesPrefix + "10-cluster_role.yaml"
	resourceRoleBinding = resourcesPrefix + "11-role_binding.yaml"
	resourceOperator    = resourcesPrefix + "12-operator.yaml"

	resourceMetricsClusterRole = resourcesPrefix + "20-metrics-cluster_role.yaml"
	resourceMetricsRoleBinding = resourcesPrefix + "21-metrics-role_binding.yaml"
	resourceMetricsDeployment  = resourcesPrefix + "22-metrics-deployment.yaml"
	resourceMetricsService     = resourcesPrefix + "23-metrics-service.yaml"
	resourceMetricsAPIService  = resourcesPrefix + "24-metrics-api_service.yaml"
)

// KedaControllerReconciler reconciles a KedaController object
type KedaControllerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	resourcesGeneral    mf.Manifest
	resourcesController mf.Manifest
	resourcesMetrics    mf.Manifest
}

// InjectClient creates manifestival resources at start
func (r *KedaControllerReconciler) InjectClient(c client.Client) error {
	manifest, err := mfc.NewManifest(fmt.Sprintf("%s", resourceServiceAccount), c)
	if err != nil {
		return err
	}
	r.resourcesGeneral = manifest

	manifest, err = mfc.NewManifest(fmt.Sprintf("%s,%s,%s", resourceClusterRole, resourceRoleBinding, resourceOperator), c)
	if err != nil {
		return err
	}
	r.resourcesController = manifest

	manifest, err = mfc.NewManifest(fmt.Sprintf("%s,%s,%s,%s,%s", resourceMetricsClusterRole, resourceMetricsRoleBinding, resourceMetricsDeployment, resourceMetricsService, resourceMetricsAPIService), c)
	if err != nil {
		return err
	}
	r.resourcesMetrics = manifest
	return nil
}

// +kubebuilder:rbac:groups=keda.sh,resources=kedacontrollers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keda.sh,resources=kedacontrollers/status,verbs=get;update;patch

// Reconcile reads that state of the cluster for a KedaController object and makes changes based on the state read
// and what is in the KedaController.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *KedaControllerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	// _ = context.Background()
	// _ = r.Log.WithValues("kedacontroller", req.NamespacedName)

	// // your logic here

	// return ctrl.Result{}, nil

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KedaController")

	// Fetch the KedaController instance
	instance := &kedav1alpha1.KedaController{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if !isInteresting(request) {
		msg := fmt.Sprintf("The KedaController resource needs to be created in namespace %s with name %s, otherwise it will be ignored", kedaControllerResourceNamespace, kedaControllerResourceName)
		reqLogger.Info(msg)
		status := instance.Status.DeepCopy()
		status.MarkIgnored(msg)
		err = util.UpdateKedaControllerStatus(r.client, instance, status)
		return reconcile.Result{}, nil
	}

	if instance.GetDeletionTimestamp() != nil {
		if contains(instance.GetFinalizers(), kedaControllerFinalizer) {
			// Run finalization logic for kedaControllerFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeKedaController(reqLogger, instance); err != nil {
				return reconcile.Result{}, err
			}
			// Remove kedaControllerFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			patch := client.MergeFrom(instance.DeepCopy())
			instance.SetFinalizers(remove(instance.GetFinalizers(), kedaControllerFinalizer))
			err := r.client.Patch(context.TODO(), instance, patch)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), kedaControllerFinalizer) {
		if err := r.addFinalizer(reqLogger, instance); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{Requeue: true}, nil
	}

	status := instance.Status.DeepCopy()
	defer util.UpdateKedaControllerStatus(r.client, instance, status)

	// DO NOT manage creation of namespace at the moment (we expect that it is precreated manually)
	// if err := r.createNamespace(installationNamespace); err != nil {
	// 	status.MarkInstallFailed(fmt.Sprintf("Not able to create Namespace '%s'", installationNamespace))
	// 	return reconcile.Result{}, err
	// }

	if err := r.installSA(instance); err != nil {
		status.MarkInstallFailed("Not able to create ServiceAccount")
		return reconcile.Result{}, err
	}
	if err := r.installController(instance); err != nil {
		status.MarkInstallFailed("Not able to install KEDA Controller")
		return reconcile.Result{}, err
	}
	if err := r.installMetricsServer(instance); err != nil {
		status.MarkInstallFailed("Not able to install KEDA Metrics Server")
		return reconcile.Result{}, err
	}

	status.Version = version.Version
	status.MarkInstallSucceeded(fmt.Sprintf("KEDA v%s is installed in namespace '%s'", version.Version, installationNamespace))
	return reconcile.Result{}, nil
}

func (r *KedaControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kedav1alpha1.KedaController{}).
		Complete(r)
}

func (r *KedaControllerReconciler) createNamespace(namespace string) error {
	log.Info("Reconciling namespace", "namespace", namespace)
	ns := &corev1.Namespace{}
	if err := r.client.Get(context.TODO(), client.ObjectKey{Name: namespace}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			ns.Name = namespace
			if err = r.client.Create(context.TODO(), ns); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func (r *KedaControllerReconciler) removeNamespace(namespace string) error {
	log.Info("Removing namespace", "namespace", namespace)
	ns := &corev1.Namespace{}
	if err := r.client.Get(context.TODO(), client.ObjectKey{Name: namespace}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			// We can safely ignore this. There is nothing to do for us.
			return nil
		} else if err != nil {
			return err
		}
	}
	return r.client.Delete(context.TODO(), ns)
}

func (r *KedaControllerReconciler) installSA(instance *kedav1alpha1.KedaController) error {
	log.Info("Reconciling Keda ServiceAccount")
	transforms := []mf.Transformer{mf.InjectOwner(instance)}
	manifest, err := r.resourcesGeneral.Transform(transforms...)
	if err != nil {
		log.Error(err, "Unable to transform ServiceAccount manifest")
		return err
	}
	r.resourcesGeneral = manifest

	if err := r.resourcesGeneral.Apply(); err != nil {
		log.Error(err, "Unable to install ServiceAccount")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) installController(instance *kedav1alpha1.KedaController) error {
	log.Info("Reconciling KEDA Controller deployment")
	transforms := []mf.Transformer{
		mf.InjectOwner(instance),
		transform.ReplaceWatchNamespace(instance.Spec.WatchNamespace, "keda-operator", r.scheme, log),
	}
	if len(instance.Spec.LogLevel) > 0 {
		transforms = append(transforms, transform.ReplaceKedaOperatorLogLevel(instance.Spec.LogLevel, r.scheme, log))
	}
	if len(instance.Spec.LogTimeFormat) > 0 {
		transforms = append(transforms, transform.ReplaceKedaOperatorLogTimeFormat(instance.Spec.LogTimeFormat, r.scheme, log))
	}

	manifest, err := r.resourcesController.Transform(transforms...)
	if err != nil {
		log.Error(err, "Unable to transform KEDA Controller manifest")		
		return err
	}
	r.resourcesController = manifest

	if err := r.resourcesController.Apply(); err != nil {
		log.Error(err, "Unable to install KEDA Controller")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) installMetricsServer(instance *kedav1alpha1.KedaController) error {
	log.Info("Reconciling Metrics Server Deployment")

	transforms := []mf.Transformer{
		mf.InjectOwner(instance),
	}

	// certificates rotation works only on Openshift due to openshift/service-ca-operator
	if util.RunningOnOpenshift(log, r.client) {
		if err := r.ensureMetricsServerConfigMap(instance); err != nil {
			log.Error(err, "Unable to check Metrics Server ConfigMap is present")
			return err
		}

		argsPrefixes := []transform.Prefix{transform.ClientCAFile, transform.TLSCertFile, transform.TLSPrivateKeyFile}
		newArgs := []string{"/cabundle/service-ca.crt", "/certs/tls.crt", "/certs/tls.key"}

		transforms = append(transforms,
			transform.EnsureCertInjectionForAPIService(injectCABundleAnnotation, injectCABundleAnnotationValue, r.scheme, log),
			transform.EnsureCertInjectionForService(metricsServcerServiceName, injectservingCertAnnotation, injectservingCertAnnotationValue, r.scheme, log),
			transform.EnsureCertInjectionForDeployment(metricsServerConfigMapName, metricsServcerServiceName, r.scheme, log),
		)
		transforms = append(transforms, transform.EnsurePathsToCertsInDeployment(newArgs, argsPrefixes, r.scheme, log)...)
	}else{
		log.Info("Not running on OpenShift -> using generated self-signed cert for KEDA Metrics Server")
	}

	if len(instance.Spec.LogLevelMetrics) > 0 {
		transforms = append(transforms, transform.ReplaceMetricsServerLogLevel(instance.Spec.LogLevelMetrics, r.scheme, log))
	}
	manifest, err := r.resourcesMetrics.Transform(transforms...)
	if err != nil {
		log.Error(err, "Unable to transform Metrics Server manifest")
		return err
	}
	r.resourcesMetrics = manifest

	if err := r.resourcesMetrics.Apply(); err != nil {
		log.Error(err, "Unable to install Metrics Server")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) ensureMetricsServerConfigMap(instance *kedav1alpha1.KedaController) error {
	log.Info("Ensure ConfigMap for Metrics Server CA bundle exists")

	configMap := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: metricsServerConfigMapName, Namespace: instance.Namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			configMap.Name = metricsServerConfigMapName
			configMap.Namespace = instance.Namespace
			metav1.SetMetaDataAnnotation(&configMap.ObjectMeta, injectCABundleAnnotation, injectCABundleAnnotationValue)

			if err := controllerutil.SetControllerReference(instance, configMap, r.scheme); err != nil {
				log.Error(err, "Failed to set Controller Reference for ConfigMap")
				return err
			}

			err = r.client.Create(context.TODO(), configMap)
			if err != nil {
				log.Error(err, "Failed to create new ConfigMap in cluster", "ConfigMap.Namespace", instance.Namespace, "ConfigMap.Name", metricsServerConfigMapName)
				return err
			}

			return nil
		}
		// Error reading the object
		log.Error(err, "Failed to get ConfigMap from cluster")
		return err
	}

	configMapUpdate := false

	if !metav1.HasAnnotation(configMap.ObjectMeta, injectCABundleAnnotation) ||
		configMap.Annotations[injectCABundleAnnotation] != injectCABundleAnnotationValue {
		metav1.SetMetaDataAnnotation(&configMap.ObjectMeta, injectCABundleAnnotation, injectCABundleAnnotationValue)
		configMapUpdate = true
	}

	if err := controllerutil.SetControllerReference(instance, configMap, r.scheme); err != nil {
		if !goerrors.Is(err, &controllerutil.AlreadyOwnedError{}) {
			log.Error(err, "Failed to check Controller Reference for ConfigMap")
			return err
		}
	} else {
		configMapUpdate = true
	}

	if configMapUpdate {
		err = r.client.Update(context.TODO(), configMap)
		if err != nil {
			log.Error(err, "Failed to update ConfigMap in cluster", "ConfigMap.Namespace", instance.Namespace, "ConfigMap.Name", metricsServerConfigMapName)
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