package util

import (
	"context"
	"crypto/md5"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/apis/keda/v1alpha1"
)

const (
	metricsServerPodLabelKey   = "app"
	metricsServerPodLabelValue = "keda-metrics-apiserver"
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

func DeleteMetricsServerPod(ctx context.Context, metricsServerNamespace string, logger logr.Logger, cl client.Client) error {
	selector := make(map[string]string)
	selector[metricsServerPodLabelKey] = metricsServerPodLabelValue

	podList := &corev1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(metricsServerNamespace),
		client.MatchingLabels(selector),
	}
	err := cl.List(ctx, podList, opts...)
	if err != nil {
		return err
	}
	if len(podList.Items) == 0 {
		logger.Info("KEDA Metrics Server is not running -> no need to restart it")
		return nil
	} else if len(podList.Items) != 1 {
		return fmt.Errorf("exactly one Pod object should match label %s", selector)
	}

	pod := &podList.Items[0]
	// restart Metrics Server Pod
	return cl.Delete(ctx, pod)
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
	if minor, err = strconv.Atoi(versionInfo.Minor); err != nil {
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
