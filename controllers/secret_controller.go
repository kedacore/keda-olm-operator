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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/v1alpha1"
	"github.com/kedacore/keda-olm-operator/controllers/util"
)

const (
	secretName      = "keda-metrics-apiserver"
	secretNamespace = "keda"
)

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
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

	if util.RunningOnOpenshift(r.Log, mgr.GetClient()) {
		return ctrl.NewControllerManagedBy(mgr).
			For(&corev1.Secret{}, builder.WithPredicates(pred)).
			Complete(r)
	}
	return nil
}

// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs="*"

func (r *SecretReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log.WithValues("secret", req.NamespacedName)

	logger.Info("Reconciling Secret containing Certificates")

	// Fetch the Secret instance
	instance := &corev1.Secret{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after ctrl req.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the req.
		return ctrl.Result{}, err
	}

	kedaController := &kedav1alpha1.KedaController{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: "keda", Namespace: "keda"}, kedaController)
	if err != nil {
		if errors.IsNotFound(err) {
			// there isn't any keda KedaController CR created in namespace keda -> do nothing
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the req.
		return ctrl.Result{}, err
	}

	newCheckSum := util.CalculateSecretedDataCheckSum(instance.Data)
	if kedaController.Status.SecretDataSum == "" {
		// Secret was just created -> we store it's Data Checksum
		logger.Info("Secret containing Certificates was created for the first time -> do nothing")
	} else {

		if kedaController.Status.SecretDataSum == newCheckSum {
			//  Secret.Data were not changed -> no need to anything, return
			return ctrl.Result{}, nil
		} else {
			// Secret.Data were changed -> let's restart KEDA Metrics Server
			logger.Info("Secret containing Certificates was changed -> let's restart KEDA Metrics Server")
			if err := util.DeleteMetricsServerPod(logger, r.Client); err != nil {
				logger.Error(err, "Unable to restart KEDA Metrics Server")
				return ctrl.Result{}, err
			}
		}
	}

	status := kedaController.Status.DeepCopy()
	status.SecretDataSum = newCheckSum
	return ctrl.Result{}, util.UpdateKedaControllerStatus(r.Client, kedaController, status)
}
