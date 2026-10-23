/*
Copyright 2026 The KEDA Authors

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
	goerrors "errors"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/internal/controller/keda/util"
)

const (
	clusterMonitoringLabel          = "openshift.io/cluster-monitoring"
	prometheusMonitoringSAName      = "prometheus-k8s"
	prometheusMonitoringSANamespace = "openshift-monitoring"
)

func prometheusRBACName(namespace string) string {
	return namespace + "-prometheus"
}

func prometheusRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"services", "endpoints", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
}

func namespaceHasClusterMonitoringEnabled(ns *corev1.Namespace) bool {
	if ns == nil || ns.Labels == nil {
		return false
	}
	return ns.Labels[clusterMonitoringLabel] == "true"
}

// ensurePrometheusMonitoringRBAC creates a Role and RoleBinding that allow
// OpenShift cluster monitoring (prometheus-k8s) to discover scrape targets in
// the operator namespace. The OpenShift web console creates the same objects
// during OperatorHub installs; this ensures CLI installs behave identically.
// On non-OpenShift clusters or when cluster monitoring is not enabled for the
// namespace, this function is a no-op.
func (r *KedaControllerReconciler) ensurePrometheusMonitoringRBAC(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	if !util.RunningOnOpenshift(ctx, logger, r.Client) {
		return nil
	}

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Namespace}, ns); err != nil {
		if errors.IsNotFound(err) {
			logger.V(4).Info("Operator namespace not found, skipping Prometheus monitoring RBAC")
			return nil
		}
		return err
	}

	if !namespaceHasClusterMonitoringEnabled(ns) {
		logger.V(4).Info("Cluster monitoring not enabled for namespace, skipping Prometheus monitoring RBAC", "namespace", instance.Namespace)
		return nil
	}

	rbacName := prometheusRBACName(instance.Namespace)
	logger.Info("Ensuring Prometheus monitoring RBAC exists", "namespace", instance.Namespace, "name", rbacName)

	if err := r.ensurePrometheusRole(ctx, logger, instance, rbacName); err != nil {
		return err
	}
	return r.ensurePrometheusRoleBinding(ctx, logger, instance, rbacName)
}

func (r *KedaControllerReconciler) ensurePrometheusRole(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController, rbacName string) error {
	desired := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: instance.Namespace,
		},
		Rules: prometheusRoleRules(),
	}

	existing := &rbacv1.Role{}
	err := r.Get(ctx, types.NamespacedName{Name: rbacName, Namespace: instance.Namespace}, existing)
	if errors.IsNotFound(err) {
		if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference for Prometheus Role")
			return err
		}
		if err := r.Create(ctx, desired); err != nil {
			logger.Error(err, "Failed to create Prometheus monitoring Role")
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}

	needsUpdate := false

	if !reflect.DeepEqual(existing.Rules, desired.Rules) {
		existing.Rules = desired.Rules
		needsUpdate = true
	}

	if err := controllerutil.SetControllerReference(instance, existing, r.Scheme); err != nil {
		if _, ok := goerrors.AsType[*controllerutil.AlreadyOwnedError](err); !ok {
			logger.Error(err, "Failed to set controller reference for Prometheus Role")
			return err
		}
	} else {
		needsUpdate = true
	}

	if needsUpdate {
		if err := r.Update(ctx, existing); err != nil {
			logger.Error(err, "Failed to update Prometheus monitoring Role")
			return err
		}
	}

	return nil
}

func (r *KedaControllerReconciler) ensurePrometheusRoleBinding(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController, rbacName string) error {
	desired := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbacName,
			Namespace: instance.Namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     rbacName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      prometheusMonitoringSAName,
				Namespace: prometheusMonitoringSANamespace,
			},
		},
	}

	existing := &rbacv1.RoleBinding{}
	err := r.Get(ctx, types.NamespacedName{Name: rbacName, Namespace: instance.Namespace}, existing)
	if errors.IsNotFound(err) {
		if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference for Prometheus RoleBinding")
			return err
		}
		if err := r.Create(ctx, desired); err != nil {
			logger.Error(err, "Failed to create Prometheus monitoring RoleBinding")
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}

	// RoleRef is immutable in the Kubernetes API. If it has drifted,
	// we must delete the RoleBinding and recreate it.
	if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
		if err := r.Delete(ctx, existing); err != nil {
			logger.Error(err, "Failed to delete Prometheus monitoring RoleBinding with stale RoleRef")
			return err
		}
		if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference for Prometheus RoleBinding")
			return err
		}
		if err := r.Create(ctx, desired); err != nil {
			logger.Error(err, "Failed to recreate Prometheus monitoring RoleBinding")
			return err
		}
		return nil
	}

	needsUpdate := false

	if !reflect.DeepEqual(existing.Subjects, desired.Subjects) {
		existing.Subjects = desired.Subjects
		needsUpdate = true
	}

	if err := controllerutil.SetControllerReference(instance, existing, r.Scheme); err != nil {
		if _, ok := goerrors.AsType[*controllerutil.AlreadyOwnedError](err); !ok {
			logger.Error(err, "Failed to set controller reference for Prometheus RoleBinding")
			return err
		}
	} else {
		needsUpdate = true
	}

	if needsUpdate {
		if err := r.Update(ctx, existing); err != nil {
			logger.Error(err, "Failed to update Prometheus monitoring RoleBinding")
			return err
		}
	}

	return nil
}
