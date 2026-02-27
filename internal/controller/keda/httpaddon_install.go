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
	"os"
	"strconv"

	"github.com/go-logr/logr"
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/internal/controller/keda/transform"
	"github.com/kedacore/keda-olm-operator/internal/controller/keda/util"
)

func (r *KedaControllerReconciler) installHTTPAddon(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling KEDA HTTP Add-on")

	// If HTTP Add-on is not enabled, check if it needs to be uninstalled
	if !instance.Spec.HTTPAddon.Enabled {
		if instance.Status.HTTPAddonInstalled {
			logger.Info("HTTP Add-on is disabled but currently installed, uninstalling")
			return r.uninstallHTTPAddon(logger)
		}
		logger.Info("HTTP Add-on is not enabled, skipping installation")
		return nil
	}

	transforms := []mf.Transformer{
		transform.InjectHTTPAddonOwner(instance),
		transform.ReplaceHTTPAddonAllNamespaces(instance.Namespace),
	}

	// Handle additional labels
	if len(instance.Spec.HTTPAddon.AdditionalLabels) > 0 {
		transforms = append(transforms, transform.AddHTTPAddonAdditionalLabels(instance.Spec.HTTPAddon.AdditionalLabels))
	}

	// Handle global security contexts
	if instance.Spec.HTTPAddon.PodSecurityContext != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorPodSecurityContext(instance.Spec.HTTPAddon.PodSecurityContext, r.Scheme))
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerPodSecurityContext(instance.Spec.HTTPAddon.PodSecurityContext, r.Scheme))
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorPodSecurityContext(instance.Spec.HTTPAddon.PodSecurityContext, r.Scheme))
	}
	if instance.Spec.HTTPAddon.SecurityContext != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorSecurityContext(instance.Spec.HTTPAddon.SecurityContext, r.Scheme))
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerSecurityContext(instance.Spec.HTTPAddon.SecurityContext, r.Scheme))
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorSecurityContext(instance.Spec.HTTPAddon.SecurityContext, r.Scheme))
	}

	// Handle RBAC configuration
	if instance.Spec.HTTPAddon.RBAC.AggregateToDefaultRoles {
		transforms = append(transforms, transform.AddHTTPAddonAggregateToDefaultRoles())
	}

	// Handle operator component configuration
	transforms = r.httpAddonOperatorTransforms(ctx, logger, instance, transforms)

	// Handle scaler component configuration
	transforms = r.httpAddonScalerTransforms(logger, instance, transforms)

	// Handle interceptor component configuration
	transforms = r.httpAddonInterceptorTransforms(ctx, logger, instance, transforms)

	// Handle image overrides from environment variables
	transforms = r.httpAddonImageTransforms(transforms)

	// Apply custom images from spec
	transforms = r.httpAddonCustomImageTransforms(instance, transforms)

	manifest, err := r.resourcesHTTPAddon.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform HTTP Add-on manifest")
		return err
	}
	r.resourcesHTTPAddon = manifest

	if err := r.resourcesHTTPAddon.Apply(); err != nil {
		logger.Error(err, "Unable to install HTTP Add-on")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) httpAddonOperatorTransforms(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController, transforms []mf.Transformer) []mf.Transformer {
	spec := instance.Spec.HTTPAddon.Operator

	// on OpenShift 4.10 (kube 1.23) and earlier, the RuntimeDefault SeccompProfile won't validate against any SCC
	if util.RunningOnOpenshift(ctx, logger, r.Client) && util.RunningOnClusterWithoutSeccompProfileDefault(logger, r.discoveryClient) {
		transforms = append(transforms, transform.RemoveSeccompProfileFromHTTPAddonOperator(r.Scheme, logger))
	}

	if spec.Replicas != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorReplicas(*spec.Replicas, r.Scheme))
	}

	if spec.WatchNamespace != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorEnv("KEDA_HTTP_OPERATOR_WATCH_NAMESPACE", spec.WatchNamespace, r.Scheme))
	}

	if spec.Port > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorMetricsPort(spec.Port, r.Scheme))
	}

	// Metrics endpoint configuration
	if spec.Metrics.Secure != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorMetricsSecure(*spec.Metrics.Secure, r.Scheme))
	}
	if spec.Metrics.Auth != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorMetricsAuth(*spec.Metrics.Auth, r.Scheme))
	}
	if spec.Metrics.CertDir != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorMetricsCertDir(spec.Metrics.CertDir, r.Scheme))
	}

	// Logging configuration
	if spec.Logging.Level != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorLogLevel(spec.Logging.Level, r.Scheme, logger))
	}
	if spec.Logging.Format != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorLogEncoder(spec.Logging.Format, r.Scheme, logger))
	}
	if spec.Logging.TimeEncoding != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorLogTimeEncoding(spec.Logging.TimeEncoding, r.Scheme, logger))
	}
	if spec.Logging.StackTracesEnabled {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorEnv("KEDA_HTTP_OPERATOR_STACKTRACES", "true", r.Scheme))
	}

	// Resources
	if spec.Resources.Limits != nil || spec.Resources.Requests != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorResources(spec.Resources, r.Scheme))
	}

	// Node selector, tolerations, affinity, topology spread constraints
	if len(spec.NodeSelector) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorNodeSelector(spec.NodeSelector, r.Scheme))
	}
	if len(spec.Tolerations) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorTolerations(spec.Tolerations, r.Scheme))
	}
	if spec.Affinity != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorAffinity(spec.Affinity, r.Scheme))
	}
	if len(spec.TopologySpreadConstraints) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorTopologySpreadConstraints(spec.TopologySpreadConstraints, r.Scheme))
	}

	// Pod annotations
	if len(spec.PodAnnotations) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorPodAnnotations(spec.PodAnnotations, r.Scheme))
	}

	// Extra environment variables
	if len(spec.ExtraEnvs) > 0 {
		transforms = append(transforms, transform.AddHTTPAddonOperatorExtraEnvs(spec.ExtraEnvs, r.Scheme))
	}

	// Image pull secrets
	if len(spec.ImagePullSecrets) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorImagePullSecrets(spec.ImagePullSecrets, r.Scheme))
	}

	// Pull policy
	if spec.PullPolicy != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorPullPolicy(spec.PullPolicy, r.Scheme))
	}

	return transforms
}

func (r *KedaControllerReconciler) httpAddonScalerTransforms(logger logr.Logger, instance *kedav1alpha1.KedaController, transforms []mf.Transformer) []mf.Transformer {
	spec := instance.Spec.HTTPAddon.Scaler

	if spec.Replicas != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerReplicas(*spec.Replicas, r.Scheme))
	}

	// Service name configuration
	if spec.Service != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonExternalScalerServiceName(spec.Service, instance.Namespace, spec.GRPCPort, r.Scheme))
	}

	if spec.GRPCPort > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerEnv("KEDA_HTTP_SCALER_PORT", strconv.Itoa(int(spec.GRPCPort)), r.Scheme))
	}

	if spec.StreamInterval > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerEnv("KEDA_HTTP_SCALER_STREAM_INTERVAL_MS", strconv.Itoa(int(spec.StreamInterval)), r.Scheme))
	}

	if spec.PendingRequestsInterceptor > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorScaledObjectPendingRequests(spec.PendingRequestsInterceptor, r.Scheme))
	}

	// Logging configuration
	if spec.Logging.Level != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerLogLevel(spec.Logging.Level, r.Scheme, logger))
	}
	if spec.Logging.Format != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerLogEncoder(spec.Logging.Format, r.Scheme, logger))
	}
	if spec.Logging.TimeEncoding != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerLogTimeEncoding(spec.Logging.TimeEncoding, r.Scheme, logger))
	}
	if spec.Logging.StackTracesEnabled {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerEnv("KEDA_HTTP_SCALER_STACKTRACES", "true", r.Scheme))
	}

	// Resources
	if spec.Resources.Limits != nil || spec.Resources.Requests != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerResources(spec.Resources, r.Scheme))
	}

	// Node selector, tolerations, affinity, topology spread constraints
	if len(spec.NodeSelector) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerNodeSelector(spec.NodeSelector, r.Scheme))
	}
	if len(spec.Tolerations) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerTolerations(spec.Tolerations, r.Scheme))
	}
	if spec.Affinity != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerAffinity(spec.Affinity, r.Scheme))
	}
	if len(spec.TopologySpreadConstraints) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerTopologySpreadConstraints(spec.TopologySpreadConstraints, r.Scheme))
	}

	// Pod annotations
	if len(spec.PodAnnotations) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerPodAnnotations(spec.PodAnnotations, r.Scheme))
	}

	// Extra environment variables
	if len(spec.ExtraEnvs) > 0 {
		transforms = append(transforms, transform.AddHTTPAddonScalerExtraEnvs(spec.ExtraEnvs, r.Scheme))
	}

	// Image pull secrets
	if len(spec.ImagePullSecrets) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerImagePullSecrets(spec.ImagePullSecrets, r.Scheme))
	}

	// Pull policy
	if spec.PullPolicy != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerPullPolicy(spec.PullPolicy, r.Scheme))
	}

	return transforms
}

func (r *KedaControllerReconciler) httpAddonInterceptorTransforms(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController, transforms []mf.Transformer) []mf.Transformer {
	spec := instance.Spec.HTTPAddon.Interceptor

	// on OpenShift 4.10 (kube 1.23) and earlier, the RuntimeDefault SeccompProfile won't validate against any SCC
	if util.RunningOnOpenshift(ctx, logger, r.Client) && util.RunningOnClusterWithoutSeccompProfileDefault(logger, r.discoveryClient) {
		transforms = append(transforms, transform.RemoveSeccompProfileFromHTTPAddonInterceptor(r.Scheme, logger))
	}

	// Replicas configuration for ScaledObject
	if spec.Replicas.Min != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorScaledObjectMinReplicas(*spec.Replicas.Min, r.Scheme))
	}
	if spec.Replicas.Max != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorScaledObjectMaxReplicas(*spec.Replicas.Max, r.Scheme))
	}
	if spec.Replicas.WaitTimeout != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_CONDITION_WAIT_TIMEOUT", spec.Replicas.WaitTimeout, r.Scheme))
	}

	// ScaledObject configuration
	if spec.ScaledObject.PollingInterval > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorScaledObjectPollingInterval(spec.ScaledObject.PollingInterval, r.Scheme))
	}

	// Interceptor settings
	if spec.TCPConnectTimeout != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_CONNECT_TIMEOUT", spec.TCPConnectTimeout, r.Scheme))
	}
	if spec.KeepAlive != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_KEEP_ALIVE", spec.KeepAlive, r.Scheme))
	}
	if spec.ResponseHeaderTimeout != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_RESPONSE_HEADER_TIMEOUT", spec.ResponseHeaderTimeout, r.Scheme))
	}
	if spec.EndpointsCachePollingIntervalMS > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_ENDPOINTS_CACHE_POLLING_INTERVAL_MS", strconv.Itoa(int(spec.EndpointsCachePollingIntervalMS)), r.Scheme))
	}
	if spec.ForceHTTP2 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_FORCE_HTTP2", "true", r.Scheme))
	}
	if spec.MaxIdleConns > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_MAX_IDLE_CONNS", strconv.Itoa(int(spec.MaxIdleConns)), r.Scheme))
	}
	if spec.IdleConnTimeout != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_IDLE_CONN_TIMEOUT", spec.IdleConnTimeout, r.Scheme))
	}
	if spec.TLSHandshakeTimeout != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_TLS_HANDSHAKE_TIMEOUT", spec.TLSHandshakeTimeout, r.Scheme))
	}
	if spec.ExpectContinueTimeout != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_EXPECT_CONTINUE_TIMEOUT", spec.ExpectContinueTimeout, r.Scheme))
	}

	// Admin service configuration
	if spec.Admin.Service != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorAdminServiceName(spec.Admin.Service))
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorAdminServiceNameInScaler(spec.Admin.Service, r.Scheme))
	}
	if spec.Admin.Port > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_ADMIN_PORT", strconv.Itoa(int(spec.Admin.Port)), r.Scheme))
	}

	// Proxy service configuration
	if spec.Proxy.Service != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorProxyServiceName(spec.Proxy.Service))
	}
	if spec.Proxy.Port > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_PROXY_PORT", strconv.Itoa(int(spec.Proxy.Port)), r.Scheme))
	}

	// TLS configuration
	if spec.TLS.Enabled {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_PROXY_TLS_ENABLED", "true", r.Scheme))
		certPath := spec.TLS.CertPath
		keyPath := spec.TLS.KeyPath
		if certPath != "" {
			transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_PROXY_TLS_CERT_PATH", certPath, r.Scheme))
		}
		if keyPath != "" {
			transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_PROXY_TLS_KEY_PATH", keyPath, r.Scheme))
		}
		if spec.TLS.Port > 0 {
			transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_PROXY_TLS_PORT", strconv.Itoa(int(spec.TLS.Port)), r.Scheme))
		}
		if spec.TLS.TLSSkipVerify {
			transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_PROXY_TLS_SKIP_VERIFY", "true", r.Scheme))
		}
		// Mount TLS secret as volume if specified
		if spec.TLS.CertSecret != "" {
			transforms = append(transforms, transform.AddHTTPAddonInterceptorTLSSecretVolume(spec.TLS.CertSecret, certPath, keyPath, r.Scheme, logger))
		}
	}

	// Logging configuration
	if spec.Logging.Level != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorLogLevel(spec.Logging.Level, r.Scheme, logger))
	}
	if spec.Logging.Format != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorLogEncoder(spec.Logging.Format, r.Scheme, logger))
	}
	if spec.Logging.TimeEncoding != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorLogTimeEncoding(spec.Logging.TimeEncoding, r.Scheme, logger))
	}
	if spec.Logging.StackTracesEnabled {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorEnv("KEDA_HTTP_INTERCEPTOR_STACKTRACES", "true", r.Scheme))
	}

	// Resources
	if spec.Resources.Limits != nil || spec.Resources.Requests != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorResources(spec.Resources, r.Scheme))
	}

	// Node selector, tolerations, affinity, topology spread constraints
	if len(spec.NodeSelector) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorNodeSelector(spec.NodeSelector, r.Scheme))
	}
	if len(spec.Tolerations) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorTolerations(spec.Tolerations, r.Scheme))
	}
	if spec.Affinity != nil {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorAffinity(spec.Affinity, r.Scheme))
	}
	if len(spec.TopologySpreadConstraints) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorTopologySpreadConstraints(spec.TopologySpreadConstraints, r.Scheme))
	}

	// Pod annotations
	if len(spec.PodAnnotations) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorPodAnnotations(spec.PodAnnotations, r.Scheme))
	}

	// Extra environment variables
	if len(spec.ExtraEnvs) > 0 {
		transforms = append(transforms, transform.AddHTTPAddonInterceptorExtraEnvs(spec.ExtraEnvs, r.Scheme))
	}

	// Image pull secrets
	if len(spec.ImagePullSecrets) > 0 {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorImagePullSecrets(spec.ImagePullSecrets, r.Scheme))
	}

	// Pull policy
	if spec.PullPolicy != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorPullPolicy(spec.PullPolicy, r.Scheme))
	}

	// PodDisruptionBudget configuration
	// Check if PDB is disabled
	if spec.PodDisruptionBudget.Enabled != nil && !*spec.PodDisruptionBudget.Enabled {
		transforms = append(transforms, transform.DeleteHTTPAddonPDB())
	} else {
		// MinAvailable and MaxUnavailable are mutually exclusive - warn if both are set
		if spec.PodDisruptionBudget.MinAvailable != nil && spec.PodDisruptionBudget.MaxUnavailable != nil {
			logger.Info("Warning: both minAvailable and maxUnavailable are set in podDisruptionBudget; only minAvailable will be used (they are mutually exclusive)")
		}
		if spec.PodDisruptionBudget.MinAvailable != nil {
			transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorPDBMinAvailable(*spec.PodDisruptionBudget.MinAvailable, r.Scheme))
		} else if spec.PodDisruptionBudget.MaxUnavailable != nil {
			transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorPDBMaxUnavailable(*spec.PodDisruptionBudget.MaxUnavailable, r.Scheme))
		}
	}

	return transforms
}

func (r *KedaControllerReconciler) httpAddonImageTransforms(transforms []mf.Transformer) []mf.Transformer {
	// Use alternate image specs if env vars are set
	if operatorImage := os.Getenv("KEDA_HTTP_ADDON_OPERATOR_IMAGE"); operatorImage != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorImage(operatorImage, r.Scheme))
	}
	if scalerImage := os.Getenv("KEDA_HTTP_ADDON_SCALER_IMAGE"); scalerImage != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerImage(scalerImage, r.Scheme))
	}
	if interceptorImage := os.Getenv("KEDA_HTTP_ADDON_INTERCEPTOR_IMAGE"); interceptorImage != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorImage(interceptorImage, r.Scheme))
	}

	return transforms
}

func (r *KedaControllerReconciler) httpAddonCustomImageTransforms(instance *kedav1alpha1.KedaController, transforms []mf.Transformer) []mf.Transformer {
	images := instance.Spec.HTTPAddon.Images
	tag := images.Tag

	if images.Operator != "" {
		image := images.Operator
		if tag != "" {
			image = image + ":" + tag
		}
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorImage(image, r.Scheme))
	} else if tag != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonOperatorImage("ghcr.io/kedacore/http-add-on-operator:"+tag, r.Scheme))
	}

	if images.Scaler != "" {
		image := images.Scaler
		if tag != "" {
			image = image + ":" + tag
		}
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerImage(image, r.Scheme))
	} else if tag != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonScalerImage("ghcr.io/kedacore/http-add-on-scaler:"+tag, r.Scheme))
	}

	if images.Interceptor != "" {
		image := images.Interceptor
		if tag != "" {
			image = image + ":" + tag
		}
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorImage(image, r.Scheme))
	} else if tag != "" {
		transforms = append(transforms, transform.ReplaceHTTPAddonInterceptorImage("ghcr.io/kedacore/http-add-on-interceptor:"+tag, r.Scheme))
	}

	return transforms
}

func (r *KedaControllerReconciler) uninstallHTTPAddon(logger logr.Logger) error {
	logger.Info("Uninstalling KEDA HTTP Add-on")

	// Filter out CRDs - we don't want to delete them as they might have user data
	resources := []unstructured.Unstructured{}
	for _, resource := range r.resourcesHTTPAddon.Resources() {
		if resource.GetKind() != "CustomResourceDefinition" {
			resources = append(resources, resource)
		}
	}

	manifest, err := mf.ManifestFrom(mf.Slice(resources))
	if err != nil {
		logger.Error(err, "Unable to create HTTP Add-on manifest for deletion")
		return err
	}
	manifest.Client = r.resourcesHTTPAddon.Client

	if err := manifest.Delete(); err != nil {
		logger.Error(err, "Unable to delete HTTP Add-on resources")
		return err
	}

	return nil
}
