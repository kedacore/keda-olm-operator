package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KedaControllerPhase string

const (
	PhaseNone             KedaControllerPhase = ""
	PhaseInstallSucceeded                     = "Installation Succeeded"
	PhaseIgnored                              = "Installation Ignored"
	PhaseFailed                               = "Installation Failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KedaControllerSpec defines the desired state of KedaController
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type KedaControllerSpec struct {
	// +optional
	LogLevel string `json:"logLevel,omitempty"`
	// +optional
	LogLevelMetrics string `json:"logLevelMetrics,omitempty"`
	// +optional
	WatchNamespace string `json:"watchNamespace,omitempty"`

	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// KedaControllerStatus defines the observed state of KedaController
// +k8s:openapi-gen=true
type KedaControllerStatus struct {
	// +optional
	Phase KedaControllerPhase `json:"phase,omitempy"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Version string `json:"version,omitempty"`
	// +optional
	ConfigMapDataSum string `json:"configmadatasum,omitempty"`
	// +optional
	SecretDataSum string `json:"secretdatasum,omitempty"`

	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KedaController is the Schema for the kedacontrollers API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kedacontrollers,scope=Namespaced
type KedaController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KedaControllerSpec   `json:"spec,omitempty"`
	Status KedaControllerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
