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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KedaControllerPhase string

const (
	PhaseNone             KedaControllerPhase = ""
	PhaseInstallSucceeded                     = "Installation Succeeded"
	PhaseIgnored                              = "Installation Ignored"
	PhaseFailed                               = "Installation Failed"
)

// KedaControllerSpec defines the desired state of KedaController
// +kubebuilder:subresource:status
type KedaControllerSpec struct {
	// Important: Run "make" to regenerate code after modifying this file
	// +optional
	LogLevel string `json:"logLevel,omitempty"`
	// +optional
	LogTimeFormat string `json:"logTimeFormat,omitempty"`
	// +optional
	LogLevelMetrics string `json:"logLevelMetrics,omitempty"`
	// +optional
	WatchNamespace string `json:"watchNamespace,omitempty"`
}

// KedaControllerStatus defines the observed state of KedaController
type KedaControllerStatus struct {
	// Important: Run "make" to regenerate code after modifying this file
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
