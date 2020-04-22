package secret

import (
	"context"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/pkg/apis/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/pkg/controller/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	secretName      = "keda-metrics-apiserver"
	secretNamespace = "keda"
)

var log = logf.Log.WithName("controller_secret")

// Add creates a new Secret Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSecret{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("secret-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// we are interested only in one particular Secret and only to it's creation/updates
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Meta.GetName() == secretName && e.Meta.GetNamespace() == secretNamespace {
				return true
			} else {
				return false
			}
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.MetaNew.GetName() == secretName && e.MetaNew.GetNamespace() == secretNamespace {
				return e.MetaOld.GetResourceVersion() != e.MetaNew.GetResourceVersion()
			} else {
				return false
			}
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	// Watch for changes to primary resource Secret
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, pred)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSecret implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSecret{}

// ReconcileSecret reconciles a Secret object
type ReconcileSecret struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Secret object and makes changes based on the state read
// and what is in the Secret.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Secret containing Certificates")

	// Fetch the Secret instance
	instance := &corev1.Secret{}
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

	kedaController := &kedav1alpha1.KedaController{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "keda", Namespace: "keda"}, kedaController)
	if err != nil {
		if errors.IsNotFound(err) {
			// there isn't any keda KedaController CR created in namespace keda -> do nothing
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	newCheckSum := util.CalculateSecretedDataCheckSum(instance.Data)
	if kedaController.Status.SecretDataSum == "" {
		// Secret was just created -> we store it's Data Checksum
		reqLogger.Info("Secret containing Certificates was created for the first time -> do nothing")
	} else {

		if kedaController.Status.SecretDataSum == newCheckSum {
			//  Secret.Data were not changed -> no need to anything, return
			return reconcile.Result{}, nil
		} else {
			// Secret.Data were changed -> let's restart KEDA Metrics Server
			reqLogger.Info("Secret containing Certificates was changed -> let's restart KEDA Metrics Server")
			if err := util.DeleteMetricsServerPod(reqLogger, r.client); err != nil {
				reqLogger.Error(err, "Unable to restart KEDA Metrics Server")
				return reconcile.Result{}, err
			}
		}
	}

	status := kedaController.Status.DeepCopy()
	status.SecretDataSum = newCheckSum
	return reconcile.Result{}, util.UpdateKedaControllerStatus(r.client, kedaController, status)
}
