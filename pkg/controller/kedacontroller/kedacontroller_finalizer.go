package kedacontroller

import (
	"context"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda-olm-operator/pkg/apis/keda/v1alpha1"
)

const (
	kedaControllerFinalizer = "finalizer.kedacontroller.keda.k8s.io"
)

// finalizeKedaController is deleting resources for the respective KedaController
func (r *ReconcileKedaController) finalizeKedaController(logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	if err := r.resourcesGeneral.DeleteAll(); err != nil {
		logger.Info("error finalized KedaController general", "error", err)
		return err
	}
	if err := r.resourcesController.DeleteAll(); err != nil {
		logger.Info("error finalized KedaController controller", "error", err)
		return err
	}
	if err := r.resourcesMetrics.DeleteAll(); err != nil {
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
func (r *ReconcileKedaController) addFinalizer(logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Adding Finalizer for the KedaController")
	instance.SetFinalizers(append(instance.GetFinalizers(), kedaControllerFinalizer))

	// Update CR
	err := r.client.Update(context.TODO(), instance)
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
