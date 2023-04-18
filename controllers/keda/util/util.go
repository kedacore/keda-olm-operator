package util

import (
	"context"
	"crypto/md5"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/apis/keda/v1alpha1"
)

const (
	metricsServerNamespace     = "keda"
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

func DeleteMetricsServerPod(ctx context.Context, logger logr.Logger, cl client.Client) error {
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
