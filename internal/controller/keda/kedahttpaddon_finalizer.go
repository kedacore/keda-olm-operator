/*
Copyright 2025 The KEDA Authors

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

package keda

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
)

const (
	kedaHTTPAddOnFinalizer = "finalizer.kedahttpaddon.keda.sh"
)

// finalizeKedaHTTPAddOn is deleting resources for the respective KedaHTTPAddOn
func (r *KedaHTTPAddOnReconciler) finalizeKedaHTTPAddOn(logger logr.Logger) error {
	if err := r.resources.Delete(); err != nil {
		logger.Info("error finalized KedaHTTPAddOn", "error", err)
		return err
	}

	logger.Info("Successfully finalized KedaHTTPAddOn")
	return nil
}

// addFinalizer adds finalizer to the KedaHTTPAddOn
func (r *KedaHTTPAddOnReconciler) addHTTPAddOnFinalizer(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaHTTPAddOn) error {
	logger.Info("Adding Finalizer for the KedaHTTPAddOn")

	// Update CR
	patch := client.MergeFrom(instance.DeepCopy())
	instance.SetFinalizers(append(instance.GetFinalizers(), kedaHTTPAddOnFinalizer))
	err := r.Client.Patch(ctx, instance, patch)
	if err != nil {
		logger.Error(err, "Failed to update KedaHTTPAddOn with finalizer")
		return err
	}
	return nil
}
