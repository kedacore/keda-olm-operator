/*
Copyright 2020 The KEDA Authors

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KedaControllerPhase string

const (
	PhaseNone             KedaControllerPhase = ""
	PhaseInstallSucceeded KedaControllerPhase = "Installation Succeeded"
	PhaseIgnored          KedaControllerPhase = "Installation Ignored"
	PhaseFailed           KedaControllerPhase = "Installation Failed"
)

// KedaControllerSpec defines the desired state of KedaController
// +kubebuilder:subresource:status
type KedaControllerSpec struct {

	// +optional
	WatchNamespace string `json:"watchNamespace,omitempty"`

	// +optional
	Operator KedaOperatorSpec `json:"operator"`

	// +optional
	MetricsServer KedaMetricsServerSpec `json:"metricsServer"`

	// +optional
	ServiceAccount KedaServiceAccountSpec `json:"serviceAccount"`

	// DEPRECATED fields - use `.spec.operator` or `spec.metricsServer` instead
	KedaControllerDeprecatedSpec `json:",inline"`

	// Important: Run "make" to regenerate code after modifying this file
}

type KedaServiceAccountSpec struct {

	// Annotations applied to the Service Account
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels applied to the Service Account
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

type KedaOperatorSpec struct {

	// Logging level for KEDA Controller
	// allowed values: 'debug', 'info', 'error', or an integer value greater than 0, specified as string
	// default value: info
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	// Logging format for KEDA Controller
	// allowed values are 'json' and 'console'
	// default value: console
	// +optional
	LogEncoder string `json:"logEncoder,omitempty"`

	// Logging time encoding for KEDA Controller
	// allowed values are 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'
	// default value: rfc3339
	// +optional
	LogTimeEncoding string `json:"logTimeEncoding,omitempty"`

	GenericDeploymentSpec `json:",inline"`
}

type KedaMetricsServerSpec struct {

	// Logging level for Metrics Server
	// allowed values: "0" for info, "4" for debug, or an integer value greater than 0, specified as string
	// default value: "0"
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	GenericDeploymentSpec `json:",inline"`
}

type GenericDeploymentSpec struct {

	// Annotations applied to the Deployment
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
	// +optional
	DeploymentAnnotations map[string]string `json:"deploymentAnnotations,omitempty"`

	// Labels applied to the Deployment
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	// +optional
	DeploymentLabels map[string]string `json:"deploymentLabels,omitempty"`

	// Annotations applied to the Pod
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// Labels applied to the Pod
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
	// +optional
	PodLabels map[string]string `json:"podLabels,omitempty"`

	// Node selector for pod scheduling
	// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod scheduling
	// https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity for pod scheduling
	// https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Pod priority
	// https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// Manage resource requests & limits
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type KedaControllerDeprecatedSpec struct {
	// Logging level for KEDA Controller
	// allowed values: 'debug', 'info', 'error', or an integer value greater than 0, specified as string
	// default value: info
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	// Logging format for KEDA Controller
	// allowed values are json and console
	// default value: console
	// +optional
	LogEncoder string `json:"logEncoder,omitempty"`

	// Logging level for Metrics Server
	// allowed values: "0" for info, "4" for debug, or an integer value greater than 0, specified as string
	// default value: "0"
	// +optional
	LogLevelMetrics string `json:"logLevelMetrics,omitempty"`

	// Node selector for pod scheduling - both KEDA Operator and Metrics Server
	// https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod scheduling - both KEDA Operator and Metrics Server
	// https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity for pod scheduling - both KEDA Operator and Metrics Server
	// https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Pod priority for KEDA Operator and Metrics Adapter
	// https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// Manage resource requests & limits for KEDA Operator
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	ResourcesKedaOperator corev1.ResourceRequirements `json:"resourcesKedaOperator,omitempty"`

	// Manage resource requests & limits for KEDA Metrics Server
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	// +optional
	ResourcesMetricsServer corev1.ResourceRequirements `json:"resourcesMetricsServer,omitempty"`
}

// KedaControllerStatus defines the observed state of KedaController
type KedaControllerStatus struct {
	// +optional
	Phase KedaControllerPhase `json:"phase,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Version string `json:"version,omitempty"`
	// +optional
	ConfigMapDataSum string `json:"configmadatasum,omitempty"`
	// +optional
	SecretDataSum string `json:"secretdatasum,omitempty"`

	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kedacontrollers,scope=Namespaced

// KedaController is the Schema for the kedacontrollers API
type KedaController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KedaControllerSpec   `json:"spec,omitempty"`
	Status KedaControllerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KedaControllerList contains a list of KedaController
type KedaControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KedaController `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KedaController{}, &KedaControllerList{})
}

func (kcs *KedaControllerStatus) SetPhase(p KedaControllerPhase) {
	kcs.Phase = p
}

func (kcs *KedaControllerStatus) SetReason(r string) {
	kcs.Reason = r
}

func (kcs *KedaControllerStatus) MarkIgnored(r string) {
	kcs.Phase = PhaseIgnored
	kcs.Reason = r
}

func (kcs *KedaControllerStatus) MarkInstallSucceeded(r string) {
	kcs.Phase = PhaseInstallSucceeded
	kcs.Reason = r
}

func (kcs *KedaControllerStatus) MarkInstallFailed(r string) {
	kcs.Phase = PhaseFailed
	kcs.Reason = r
}
