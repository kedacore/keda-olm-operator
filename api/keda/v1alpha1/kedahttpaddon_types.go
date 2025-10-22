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

type KedaHTTPAddOnPhase string

const (
	HTTPAddOnPhaseNone             KedaHTTPAddOnPhase = ""
	HTTPAddOnPhaseInstallSucceeded KedaHTTPAddOnPhase = "Installation Succeeded"
	HTTPAddOnPhaseIgnored          KedaHTTPAddOnPhase = "Installation Ignored"
	HTTPAddOnPhaseFailed           KedaHTTPAddOnPhase = "Installation Failed"
)

// KedaHTTPAddOnSpec defines the desired state of KedaHTTPAddOn
type KedaHTTPAddOnSpec struct {
	// Namespace that the add-on should watch; empty means all namespaces
	// +optional
	WatchNamespace string `json:"watchNamespace,omitempty"`

	// Version (image tag) of the HTTP Add-on to deploy, e.g. "0.11.0"
	// If empty, uses the default version
	// +optional
	Version string `json:"version,omitempty"`

	// Optional custom image for the HTTP Add-on operator
	// +optional
	OperatorImage string `json:"operatorImage,omitempty"`

	// Optional custom image for the HTTP Add-on scaler
	// +optional
	ScalerImage string `json:"scalerImage,omitempty"`

	// Optional custom image for the HTTP Add-on interceptor
	// +optional
	InterceptorImage string `json:"interceptorImage,omitempty"`
}

// KedaHTTPAddOnStatus defines the observed state of KedaHTTPAddOn
type KedaHTTPAddOnStatus struct {
	// Phase describes the current state of the HTTP Add-on installation
	// +optional
	Phase KedaHTTPAddOnPhase `json:"phase,omitempty"`

	// Reason provides a human-readable message for the current phase
	// +optional
	Reason string `json:"reason,omitempty"`

	// Version indicates the deployed HTTP Add-on version
	// +optional
	Version string `json:"version,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=kedahttpaddons,scope=Namespaced

// KedaHTTPAddOn is the Schema for the kedahttpaddons API
type KedaHTTPAddOn struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KedaHTTPAddOnSpec   `json:"spec,omitempty"`
	Status KedaHTTPAddOnStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KedaHTTPAddOnList contains a list of KedaHTTPAddOn
type KedaHTTPAddOnList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KedaHTTPAddOn `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KedaHTTPAddOn{}, &KedaHTTPAddOnList{})
}

func (khs *KedaHTTPAddOnStatus) SetPhase(p KedaHTTPAddOnPhase) {
	khs.Phase = p
}

func (khs *KedaHTTPAddOnStatus) SetReason(r string) {
	khs.Reason = r
}

func (khs *KedaHTTPAddOnStatus) MarkIgnored(r string) {
	khs.Phase = HTTPAddOnPhaseIgnored
	khs.Reason = r
}

func (khs *KedaHTTPAddOnStatus) MarkInstallSucceeded(r string) {
	khs.Phase = HTTPAddOnPhaseInstallSucceeded
	khs.Reason = r
}

func (khs *KedaHTTPAddOnStatus) MarkInstallFailed(r string) {
	khs.Phase = HTTPAddOnPhaseFailed
	khs.Reason = r
}
