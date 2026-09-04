package keda

import (
	"context"

	"github.com/go-logr/logr"
	mf "github.com/manifestival/manifestival"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/internal/controller/keda/transform"
)

const (
	kedaControllerFinalizer = "finalizer.kedacontroller.keda.sh"
)

// deleteOperand renders an untransformed operand manifest into the namespace it was
// installed in and removes it.
func (r *KedaControllerReconciler) deleteOperand(logger logr.Logger, manifest mf.Manifest, namespace, component string, extra ...mf.Transformer) error {
	rendered, err := renderForDelete(manifest, namespace, extra...)
	if err != nil {
		logger.Info("error rendering KedaController manifest for deletion", "component", component, "error", err)
		return err
	}
	if err := rendered.Delete(); err != nil {
		logger.Info("error finalized KedaController "+component, "error", err)
		return err
	}
	return nil
}

// finalizeKedaController is deleting resources for the respective KedaController
func (r *KedaControllerReconciler) finalizeKedaController(logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	if err := r.deleteHTTPAddon(logger, instance.Namespace); err != nil {
		logger.Info("error finalized KedaController HTTP Add-on", "error", err)
		return err
	}

	if err := r.deleteOperand(logger, r.resourcesGeneral, instance.Namespace, "general"); err != nil {
		return err
	}
	if err := r.deleteOperand(logger, r.resourcesController, instance.Namespace, "controller"); err != nil {
		return err
	}
	// the metrics server RoleBinding is installed into kube-system rather than the
	// KedaController namespace, so it has to be pointed back there to be found
	if err := r.deleteOperand(logger, r.resourcesMetrics, instance.Namespace, "metrics",
		transform.ReplaceNamespace(roleBindingName, roleBindingNamespace, r.Scheme, logger)); err != nil {
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
