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
	"github.com/kedacore/keda-olm-operator/controllers/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
)

const (
	configMapName      = "keda-metrics-apiserver"
	configMapNamespace = "keda"
)

// ConfigMapReconciler reconciles a ConfigMap object
type ConfigMapReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs="*"

func (r *ConfigMapReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("configmap", req.NamespacedName)

	r.Log.Info("Reconciling ConfigMap containing CA Bundle")

	// Fetch the ConfigMap instance
	instance := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
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

	kedaController := &kedav1alpha1.KedaController{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "keda", Namespace: "keda"}, kedaController)
	if err != nil {
		if errors.IsNotFound(err) {
			// there isn't any keda KedaController CR created in namespace keda -> do nothing
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	newCheckSum := util.CalculateConfigMapDataCheckSum(instance.Data)
	if kedaController.Status.ConfigMapDataSum == "" {
		// ConfigMap was just created -> we store it's Data Checksum
		r.Log.Info("ConfigMap containing CA Bundle was created for the first time -> do nothing")
	} else {

		if kedaController.Status.ConfigMapDataSum == newCheckSum {
			//  ConfigMap.Data were not changed -> no need to anything, return
			return ctrl.Result{}, nil
		} else {
			// ConfigMap.Data were changed -> let's restart KEDA Metrics Server
			r.Log.Info("ConfigMap containing CA Bundle was changed -> let's restart KEDA Metrics Server")
			if err := util.DeleteMetricsServerPod(r.Log, r.Client); err != nil {
				r.Log.Error(err, "Unable to restart KEDA Metrics Server")
				return ctrl.Result{}, err
			}
		}
	}

	status := kedaController.Status.DeepCopy()
	status.ConfigMapDataSum = newCheckSum
	return ctrl.Result{}, util.UpdateKedaControllerStatus(r.Client, kedaController, status)
}

func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// we are interested only in one particular ConfigMap and only to it's creation/updates
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Meta.GetName() == configMapName && e.Meta.GetNamespace() == configMapNamespace {
				return true
			} else {
				return false
			}
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.MetaNew.GetName() == configMapName && e.MetaNew.GetNamespace() == configMapNamespace {
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

	if util.RunningOnOpenshift(r.Log, mgr.GetClient()) {
		return ctrl.NewControllerManagedBy(mgr).
			For(&corev1.ConfigMap{}, builder.WithPredicates(pred)).
			Complete(r)
	}
	return nil
}
