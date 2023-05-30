package keda

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/apis/keda/v1alpha1"
)

const (
	kedaControllerFinalizer = "finalizer.kedacontroller.keda.sh"
)

// finalizeKedaController is deleting resources for the respective KedaController
func (r *KedaControllerReconciler) finalizeKedaController(logger logr.Logger) error {
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

	// DO NOT manage deletion of namespace at the moment (as it was created manually)
	// if err := r.removeNamespace(installationNamespace); err != nil {
	// 	logger.Info("error finalized KedaController namespace", "error", err)
	// 	return err
	// }

	logger.Info("Successfully finalized KedaController")
	return nil
}

// addFinalizer adds finalizer to the KedaController
func (r *KedaControllerReconciler) addFinalizer(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Adding Finalizer for the KedaController")

	// Update CR
	patch := client.MergeFrom(instance.DeepCopy())
	instance.SetFinalizers(append(instance.GetFinalizers(), kedaControllerFinalizer))
	err := r.Client.Patch(ctx, instance, patch)
	if err != nil {
		logger.Error(err, "Failed to update KedaController with finalizer")
		return err
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}
