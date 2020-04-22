package util

import (
	"context"
	"crypto/md5"
	"fmt"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda-olm-operator/pkg/apis/keda/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kedaControllerName      = "keda"
	kedaControllerNamespace = "keda"

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

func DeleteMetricsServerPod(logger logr.Logger, cl client.Client) error {
	selector := make(map[string]string)
	selector[metricsServerPodLabelKey] = metricsServerPodLabelValue

	podList := &corev1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(metricsServerNamespace),
		client.MatchingLabels(selector),
	}
	err := cl.List(context.TODO(), podList, opts...)
	if err != nil {
		return err
	} else if len(podList.Items) == 0 {
		logger.Info("KEDA Metrics Server is not running -> no need to restart it")
		return nil
	} else if len(podList.Items) != 1 {
		return fmt.Errorf("Exactly one Pod object should match label %s", selector)
	}

	pod := &podList.Items[0]
	// restart Metrics Server Pod
	if err := cl.Delete(context.TODO(), pod); err != nil {
		return err
	}

	return nil
}

func UpdateKedaControllerStatus(cl client.Client, kedaController *kedav1alpha1.KedaController, status *kedav1alpha1.KedaControllerStatus) error {
	patch := client.MergeFrom(kedaController.DeepCopy())
	kedaController.Status = *status
	return cl.Status().Patch(context.TODO(), kedaController, patch)
}
