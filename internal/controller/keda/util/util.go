package util

import (
	"context"
	"crypto/md5"
	"fmt"
	"strconv"
	"time"
	"unicode"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/api/keda/v1alpha1"
)

const (
	metricsServerPodLabelKey    = "app"
	metricsServerPodLabelValue  = "keda-metrics-apiserver"
	metricsServerDeploymentName = "keda-metrics-apiserver"
	restartAnnotationKey        = "kubectl.kubernetes.io/restartedAt"
)

func CalculateConfigMapDataCheckSum(m map[string]string) string {
	var data string
	for k, v := range m {
		data = data + k + v
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}

func CalculateSecretedDataCheckSum(m map[string][]byte) string {
	var data string
	for k, v := range m {
		data = data + k + string(v)
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}

// DeleteMetricsServerPod triggers a rolling restart of the metrics server deployment
// by updating the pod template restart annotation. This works with any number of replicas
// and ensures zero-downtime restarts for multi-replica deployments.
//
// The function name is preserved for backward compatibility, but the implementation now
// performs a rolling restart instead of deleting a single pod.
func DeleteMetricsServerPod(ctx context.Context, metricsServerNamespace string, logger logr.Logger, cl client.Client) error {
	// Get the metrics server Deployment
	deployment := &appsv1.Deployment{}
	err := cl.Get(ctx, types.NamespacedName{
		Name:      metricsServerDeploymentName,
		Namespace: metricsServerNamespace,
	}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("KEDA Metrics Server Deployment not found -> no need to restart it")
			return nil
		}
		return fmt.Errorf("failed to get metrics server deployment: %w", err)
	}

	// Check if deployment has any replicas configured
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
		logger.Info("KEDA Metrics Server is scaled to zero -> no need to restart it")
		return nil
	}

	// Trigger rolling restart by updating the pod template restart annotation
	// This is the same approach used by 'kubectl rollout restart'
	patch := client.MergeFrom(deployment.DeepCopy())
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations[restartAnnotationKey] = time.Now().Format(time.RFC3339)

	logger.Info("Triggering rolling restart of KEDA Metrics Server",
		"replicas", deployment.Spec.Replicas,
		"restartedAt", deployment.Spec.Template.Annotations[restartAnnotationKey])

	return cl.Patch(ctx, deployment, patch)
}

func UpdateKedaControllerStatus(ctx context.Context, cl client.Client, kedaController *kedav1alpha1.KedaController, status *kedav1alpha1.KedaControllerStatus) error {
	patch := client.MergeFrom(kedaController.DeepCopy())
	kedaController.Status = *status
	return cl.Status().Patch(ctx, kedaController, patch)
}

func RunningOnOpenshift(ctx context.Context, logger logr.Logger, cl client.Client) bool {
	gvk := schema.GroupVersionKind{Group: "route.openshift.io", Version: "v1", Kind: "route"}
	return isGvkPresent(ctx, logger, cl, gvk)
}

// RunningOnClusterWithoutSeccompProfileDefault returns true if running on cluster <= 1.23.Z which lacks the RuntimeDefault seccomp profile
func RunningOnClusterWithoutSeccompProfileDefault(logger logr.Logger, discoveryClient *discovery.DiscoveryClient) bool {
	var major, minor int

	if discoveryClient == nil {
		logger.Error(nil, "Unable to get cluster version without discoveryClient")
		return false
	}
	versionInfo, err := discoveryClient.ServerVersion()
	if err != nil {
		logger.Error(err, "Unable to get cluster version from ServerVersion()")
		return false
	}
	if major, err = strconv.Atoi(versionInfo.Major); err != nil {
		logger.Error(err, "Unable to get numeric major cluster version", "major", versionInfo.Major)
		return false
	}
	// assume that any runes that follow digits can be ignored. So, "28" -> 28, and also "28+" -> 28
	digitsLen := 0
	for _, r := range versionInfo.Minor {
		if !unicode.IsDigit(r) {
			break
		}
		digitsLen++
	}
	if minor, err = strconv.Atoi(string([]rune(versionInfo.Minor)[0:digitsLen])); err != nil {
		logger.Error(err, "Unable to get numeric minor cluster version", "minor", versionInfo.Minor)
		return false
	}
	return major <= 1 && minor <= 23
}

// HasServiceMonitorCRD returns true if the ServiceMonitor CRD is present in the cluster, false otherwise
func HasServiceMonitorCRD(ctx context.Context, logger logr.Logger, cl client.Client) bool {
	gvk := schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "ServiceMonitor"}
	return isGvkPresent(ctx, logger, cl, gvk)
}

// HasPodMonitorCRD returns true if the monitoring stack (i.e. Prometheus) is present, false otherwise
func HasPodMonitorCRD(ctx context.Context, logger logr.Logger, cl client.Client) bool {
	gvk := schema.GroupVersionKind{Group: "monitoring.coreos.com", Version: "v1", Kind: "PodMonitor"}
	return isGvkPresent(ctx, logger, cl, gvk)
}

// isGvkPresent returns whether the given gvk is present or not
func isGvkPresent(ctx context.Context, logger logr.Logger, cl client.Client, gvk schema.GroupVersionKind) bool {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)
	if err := cl.List(ctx, list, &client.ListOptions{}); err != nil {
		if !meta.IsNoMatchError(err) {
			logger.Error(err, "Unable to query", "gvk", gvk.String())
		}
		return false
	}
	return true
}
