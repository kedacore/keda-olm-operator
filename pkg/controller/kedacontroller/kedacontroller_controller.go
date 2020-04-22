package kedacontroller

import (
	"context"
	"fmt"

	mf "github.com/jcrossley3/manifestival"
	kedav1alpha1 "github.com/kedacore/keda-olm-operator/pkg/apis/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/pkg/controller/kedacontroller/transform"
	"github.com/kedacore/keda-olm-operator/version"

	"github.com/operator-framework/operator-sdk/pkg/predicate"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
	})
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
		instance.Status.MarkIgnored(msg)
		err = r.updateStatus(instance)
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
			instance.SetFinalizers(remove(instance.GetFinalizers(), kedaControllerFinalizer))
			err := r.client.Update(context.TODO(), instance)
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
		return reconcile.Result{}, nil
	}

	defer r.updateStatus(instance)

	// DO NOT manage creation of namespace at the moment (we expect that it is precreated manually)
	// if err := r.createNamespace(installationNamespace); err != nil {
	// 	instance.Status.MarkInstallFailed(fmt.Sprintf("Not able to create Namespace '%s'", installationNamespace))
	// 	return reconcile.Result{}, err
	// }

	if err := r.installSA(instance); err != nil {
		instance.Status.MarkInstallFailed("Not able to create ServiceAccount")
		return reconcile.Result{}, err
	}
	if err := r.installController(instance); err != nil {
		instance.Status.MarkInstallFailed("Not able to install KEDA Controller")
		return reconcile.Result{}, err
	}
	if err := r.installMetricsServer(instance); err != nil {
		instance.Status.MarkInstallFailed("Not able to install KEDA Metrics Server")
		return reconcile.Result{}, err
	}

	instance.Status.Version = version.Version
	instance.Status.MarkInstallSucceeded(fmt.Sprintf("KEDA v%s is installed in namespace '%s'", version.Version, installationNamespace))
	return reconcile.Result{}, nil
}

func (r *ReconcileKedaController) createNamespace(namespace string) error {
	log.Info("Reconciling namespace", "namespace", namespace)
	ns := &v1.Namespace{}
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
	ns := &v1.Namespace{}
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
	transforms := []mf.Transformer{mf.InjectOwner(instance)}
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

// Update the status subresource
func (r *ReconcileKedaController) updateStatus(instance *kedav1alpha1.KedaController) error {
	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		return err
	}
	return nil
}

// Because it's effectively cluster-scoped, we only care about a
// single, named resource: keda in the specified namespace
func isInteresting(request reconcile.Request) bool {
	return request.Name == kedaControllerResourceName && request.Namespace == kedaControllerResourceNamespace
}
