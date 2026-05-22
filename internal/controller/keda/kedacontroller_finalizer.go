package keda

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
)

const (
	kedaControllerFinalizer = "finalizer.kedacontroller.keda.sh"
)

// finalizeKedaController is deleting resources for the respective KedaController
func (r *KedaControllerReconciler) finalizeKedaController(logger logr.Logger) error {
	if err := r.deleteHTTPAddon(logger); err != nil {
		logger.Info("error finalized KedaController HTTP Add-on", "error", err)
		return err
	}

	if err := r.resourcesGeneral.Delete(); err != nil {
		logger.Info("error finalized KedaController general", "error", err)
		return err
	}
	if err := r.resourcesController.Delete(); err != nil {
		logger.Info("error finalized KedaController controller", "error", err)
		return err
	}
	if err := r.resourcesMetrics.Delete(); err != nil {
		logger.Info("error finalized KedaController metrics", "error", err)
		return err
	}

	logger.Info("Successfully finalized KedaController")
	return nil
}

// addFinalizer adds finalizer to the KedaController
func (r *KedaControllerReconciler) addFinalizer(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Adding Finalizer for the KedaController")

	patch := client.MergeFrom(instance.DeepCopy())
	controllerutil.AddFinalizer(instance, kedaControllerFinalizer)
	if err := r.Patch(ctx, instance, patch); err != nil {
		logger.Error(err, "Failed to update KedaController with finalizer")
		return err
	}
	return nil
}
