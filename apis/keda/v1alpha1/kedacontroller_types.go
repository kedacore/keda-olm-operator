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
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
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

	// Any user-defined arguments with possibility to override any existing or
	// previously defined arguments. Allowed formats are '--argument=value',
	// 'argument=value' or just 'value'. Ex.: '--v=0' or 'ENV_ARGUMENT'
	// +optional
	Args []string `json:"args,omitempty"`
}

type KedaMetricsServerSpec struct {

	// Logging level for Metrics Server
	// allowed values: "0" for info, "4" for debug, or an integer value greater than 0, specified as string
	// default value: "0"
	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	GenericDeploymentSpec `json:",inline"`

	// Audit config for auditing log files. If a user wants to config other audit
	// flags, he can do so manually with Args field.
	// +optional
	AuditConfig `json:"auditConfig,omitempty"`

	// Any user-defined arguments with possibility to override any existing or
	// previously defined arguments. Allowed formats are '--argument=value',
	// 'argument=value' or just 'value'. Ex.: '--v=0' or 'ENV_ARGUMENT'
	// +optional
	Args []string `json:"args,omitempty"`
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

// AuditConfig defines basic audit logging arguments user can define. If more
// advanced flags are required, use 'Args' field to add them manually.
type AuditConfig struct {
	// All requests coming to api server will be logged to this persistent volume
	// claim. Leaving this empty means logging to stdout. (if Policy is not empty)
	// +optional
	LogOutputVolumeClaim string `json:"logOutputVolumeClaim,omitempty" protobuf:"bytes,1,opt,name=logOutputVolumeClaim"`

	// Policy which describes audit configuration. (Required for audit logging)
	// ex: https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy
	// +optional
	Policy AuditPolicy `json:"policy,omitempty" protobuf:"bytes,2,opt,name=policy"`

	// Logging format of saved audits. Known formats are "legacy" & "json".
	// default value: json
	// +optional
	LogFormat string `json:"logFormat,omitempty" protobuf:"bytes,3,opt,name=logFormat"`

	// +optional
	AuditLifetime `json:"lifetime,omitempty" protobuf:"bytes,4,opt,name=lifetime"`
}

// AuditLifetime defines size and life-span of audit files.
type AuditLifetime struct {
	// The maximum number of days to retain old audit log files based on the timestamp encoded in their filename.
	// + optional
	MaxAge string `json:"maxAge,omitempty"  protobuf:"bytes,1,opt,name=maxAge"`

	// The maximum number of old audit log files to retain. Setting a value of 0 will mean there's no restriction.
	// +optional
	MaxBackup string `json:"maxBackup,omitempty"  protobuf:"bytes,2,opt,name=maxBackup"`

	// The maximum size in megabytes of the audit log file before it gets rotated.
	// +optional
	MaxSize string `json:"maxSize,omitempty"  protobuf:"bytes,3,opt,name=maxSize"`
}

// AuditPolicy is a wrapper for auditv1.Policy structure used in AuditConfig
// as higher level structure exposed to user without metadata which is filled
// automatically.
type AuditPolicy struct {
	// Rules specify the audit Level a request should be recorded at.
	// A request may match multiple rules, in which case the FIRST matching rule is used.
	// The default audit level is None, but can be overridden by a catch-all rule at the end of the list.
	// PolicyRules are strictly ordered.
	// +optional
	Rules []auditv1.PolicyRule `json:"rules" protobuf:"bytes,1,rep,name=rules"`

	// OmitStages is a list of stages for which no events are created. Note that this can also
	// be specified per rule in which case the union of both are omitted.
	// +optional
	OmitStages []auditv1.Stage `json:"omitStages,omitempty" protobuf:"bytes,2,rep,name=omitStages"`

	// OmitManagedFields indicates whether to omit the managed fields of the request
	// and response bodies from being written to the API audit log.
	// This is used as a global default - a value of 'true' will omit the managed fileds,
	// otherwise the managed fields will be included in the API audit log.
	// Note that this can also be specified per rule in which case the value specified
	// in a rule will override the global default.
	// +optional
	OmitManagedFields bool `json:"omitManagedFields,omitempty" protobuf:"varint,3,opt,name=omitManagedFields"`
}
