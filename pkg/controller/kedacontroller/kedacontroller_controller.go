package kedacontroller

import (
	"context"
	goerrors "errors"
	"fmt"

	mf "github.com/jcrossley3/manifestival"
	kedav1alpha1 "github.com/kedacore/keda-olm-operator/pkg/apis/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/pkg/controller/kedacontroller/transform"
	"github.com/kedacore/keda-olm-operator/pkg/controller/util"
	"github.com/kedacore/keda-olm-operator/version"

	"github.com/operator-framework/operator-sdk/pkg/predicate"
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

var log = logf.Log.WithName("controller_kedacontroller")

// Add creates a new KedaController Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKedaController{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller.  All injections (e.g. InjectClient) are performed after this call to controller.New()
	c, err := controller.New("kedacontroller-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource KedaController
	err = c.Watch(&source.Kind{Type: &kedav1alpha1.KedaController{}}, &handler.EnqueueRequestForObject{}, predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployment and requeue the owner KedaController
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &kedav1alpha1.KedaController{},
	}, predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKedaController implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileKedaController{}

// ReconcileKedaController reconciles a KedaController object
type ReconcileKedaController struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client              client.Client
	scheme              *runtime.Scheme
	resourcesGeneral    mf.Manifest
	resourcesController mf.Manifest
	resourcesMetrics    mf.Manifest
}

// InjectClient creates manifestival resources at start
func (r *ReconcileKedaController) InjectClient(c client.Client) error {
	manifest, err := mf.NewManifest(fmt.Sprintf("%s", resourceServiceAccount), false, c)
	if err != nil {
		return err
	}
	r.resourcesGeneral = manifest

	manifest, err = mf.NewManifest(fmt.Sprintf("%s,%s,%s", resourceClusterRole, resourceRoleBinding, resourceOperator), false, c)
	if err != nil {
		return err
	}
	r.resourcesController = manifest

	manifest, err = mf.NewManifest(fmt.Sprintf("%s,%s,%s,%s,%s", resourceMetricsClusterRole, resourceMetricsRoleBinding, resourceMetricsDeployment, resourceMetricsService, resourceMetricsAPIService), false, c)
	if err != nil {
		return err
	}
	r.resourcesMetrics = manifest
	return nil
}

// Reconcile reads that state of the cluster for a KedaController object and makes changes based on the state read
// and what is in the KedaController.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKedaController) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

func (r *ReconcileKedaController) createNamespace(namespace string) error {
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

func (r *ReconcileKedaController) removeNamespace(namespace string) error {
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

func (r *ReconcileKedaController) installSA(instance *kedav1alpha1.KedaController) error {
	log.Info("Reconciling Keda ServiceAccount")
	transforms := []mf.Transformer{mf.InjectOwner(instance)}
	if err := r.resourcesGeneral.Transform(transforms...); err != nil {
		log.Error(err, "Unable to transform ServiceAccount manifest")
		return err
	}

	if err := r.resourcesGeneral.ApplyAll(); err != nil {
		log.Error(err, "Unable to install ServiceAccount")
		return err
	}
	return nil
}

func (r *ReconcileKedaController) installController(instance *kedav1alpha1.KedaController) error {
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

	if err := r.resourcesController.Transform(transforms...); err != nil {
		log.Error(err, "Unable to transform KEDA Controller manifest")
		return err
	}

	if err := r.resourcesController.ApplyAll(); err != nil {
		log.Error(err, "Unable to install KEDA Controller")
		return err
	}
	return nil
}

func (r *ReconcileKedaController) installMetricsServer(instance *kedav1alpha1.KedaController) error {
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
	if err := r.resourcesMetrics.Transform(transforms...); err != nil {
		log.Error(err, "Unable to transform Metrics Server manifest")
		return err
	}

	if err := r.resourcesMetrics.ApplyAll(); err != nil {
		log.Error(err, "Unable to install Metrics Server")
		return err
	}
	return nil
}

func (r *ReconcileKedaController) ensureMetricsServerConfigMap(instance *kedav1alpha1.KedaController) error {
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
