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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// HTTPAddonSpec defines the desired state of the KEDA HTTP Add-on components
type HTTPAddonSpec struct {
	// Enabled specifies whether the HTTP Add-on should be installed
	// If not specified or set to false, the HTTP Add-on will not be installed
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Additional labels to be applied to HTTP Add-on resources
	// +optional
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`

	// Operator configuration for the HTTP Add-on operator component
	// +optional
	Operator HTTPAddonOperatorSpec `json:"operator,omitempty"`

	// Scaler configuration for the HTTP Add-on scaler component
	// +optional
	Scaler HTTPAddonScalerSpec `json:"scaler,omitempty"`

	// Interceptor configuration for the HTTP Add-on interceptor component
	// +optional
	Interceptor HTTPAddonInterceptorSpec `json:"interceptor,omitempty"`

	// Images configuration for HTTP Add-on components
	// +optional
	Images HTTPAddonImagesSpec `json:"images,omitempty"`

	// RBAC configuration for HTTP Add-on
	// +optional
	RBAC HTTPAddonRBACSpec `json:"rbac,omitempty"`

	// SecurityContext for all HTTP Add-on containers
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`

	// PodSecurityContext for all HTTP Add-on pods
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
}

// HTTPAddonCRDsSpec defines CRD-related configuration
type HTTPAddonCRDsSpec struct {
	// Install specifies whether to install the HTTPScaledObject CRD
	// default: true
	// +optional
	Install *bool `json:"install,omitempty"`
}

// HTTPAddonOperatorSpec defines the operator component configuration
type HTTPAddonOperatorSpec struct {
	// Replicas is the number of operator replicas
	// Set to 0 to disable the operator component
	// default: 1
	// +optional
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// WatchNamespace is the namespace to watch for HTTPScaledObject resources
	// Leave empty to watch all namespaces
	// +optional
	WatchNamespace string `json:"watchNamespace,omitempty"`

	// Port for the operator main server
	// default: 8443
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	HTTPAddonComponentSpec `json:",inline"`

	// Logging configuration for the operator
	// +optional
	Logging HTTPAddonLoggingSpec `json:"logging,omitempty"`
}

// HTTPAddonScalerSpec defines the scaler component configuration
type HTTPAddonScalerSpec struct {
	// Replicas is the number of scaler replicas
	// default: 3
	// +optional
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas,omitempty"`

	// Service is the name of the Kubernetes Service for the scaler
	// default: keda-http-add-on-external-scaler
	// +optional
	Service string `json:"service,omitempty"`

	// GRPCPort is the port for the scaler's gRPC server
	// default: 9090
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	GRPCPort int32 `json:"grpcPort,omitempty"`

	// PendingRequestsInterceptor is the target requests for scaling
	// default: 200
	// +optional
	// +kubebuilder:validation:Minimum=0
	PendingRequestsInterceptor int32 `json:"pendingRequestsInterceptor,omitempty"`

	// StreamInterval is the interval in ms for communicating IsActive to KEDA
	// default: 200
	// +optional
	// +kubebuilder:validation:Minimum=1
	StreamInterval int32 `json:"streamInterval,omitempty"`

	HTTPAddonComponentSpec `json:",inline"`

	// Logging configuration for the scaler
	// +optional
	Logging HTTPAddonLoggingSpec `json:"logging,omitempty"`
}

// HTTPAddonInterceptorSpec defines the interceptor component configuration
type HTTPAddonInterceptorSpec struct {
	// Replicas configuration for the interceptor
	// +optional
	Replicas HTTPAddonInterceptorReplicasSpec `json:"replicas,omitempty"`

	// Admin service configuration
	// +optional
	Admin HTTPAddonInterceptorAdminSpec `json:"admin,omitempty"`

	// Proxy service configuration
	// +optional
	Proxy HTTPAddonInterceptorProxySpec `json:"proxy,omitempty"`

	// ScaledObject configuration
	// +optional
	ScaledObject HTTPAddonInterceptorScaledObjectSpec `json:"scaledObject,omitempty"`

	// TCPConnectTimeout is the timeout for establishing TCP connections
	// default: 500ms
	// +optional
	TCPConnectTimeout string `json:"tcpConnectTimeout,omitempty"`

	// KeepAlive is the connection keep alive timeout
	// default: 1s
	// +optional
	KeepAlive string `json:"keepAlive,omitempty"`

	// ResponseHeaderTimeout is the timeout for receiving response headers
	// default: 500ms
	// +optional
	ResponseHeaderTimeout string `json:"responseHeaderTimeout,omitempty"`

	// EndpointsCachePollingIntervalMS is how often the interceptor refreshes endpoints cache
	// default: 250
	// +optional
	// +kubebuilder:validation:Minimum=1
	EndpointsCachePollingIntervalMS int32 `json:"endpointsCachePollingIntervalMS,omitempty"`

	// ForceHTTP2 forces requests to use HTTP/2
	// default: false
	// +optional
	ForceHTTP2 bool `json:"forceHTTP2,omitempty"`

	// MaxIdleConns is the maximum number of idle connections
	// default: 100
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxIdleConns int32 `json:"maxIdleConns,omitempty"`

	// IdleConnTimeout is the timeout for idle connections
	// default: 90s
	// +optional
	IdleConnTimeout string `json:"idleConnTimeout,omitempty"`

	// TLSHandshakeTimeout is the timeout for TLS handshakes
	// default: 10s
	// +optional
	TLSHandshakeTimeout string `json:"tlsHandshakeTimeout,omitempty"`

	// ExpectContinueTimeout for 100-continue responses
	// default: 1s
	// +optional
	ExpectContinueTimeout string `json:"expectContinueTimeout,omitempty"`

	HTTPAddonComponentSpec `json:",inline"`

	// Logging configuration for the interceptor
	// +optional
	Logging HTTPAddonLoggingSpec `json:"logging,omitempty"`

	// TLS configuration for the interceptor proxy
	// +optional
	TLS HTTPAddonInterceptorTLSSpec `json:"tls,omitempty"`

	// PodDisruptionBudget configuration for the interceptor
	// +optional
	PodDisruptionBudget HTTPAddonInterceptorPodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
}

// HTTPAddonInterceptorReplicasSpec defines replica configuration for interceptor
type HTTPAddonInterceptorReplicasSpec struct {
	// Min is the minimum number of interceptor replicas
	// default: 3
	// +optional
	// +kubebuilder:validation:Minimum=0
	Min *int32 `json:"min,omitempty"`

	// Max is the maximum number of interceptor replicas
	// default: 50
	// +optional
	// +kubebuilder:validation:Minimum=1
	Max *int32 `json:"max,omitempty"`

	// WaitTimeout is the maximum time to wait for HTTP requests
	// default: 20s
	// +optional
	WaitTimeout string `json:"waitTimeout,omitempty"`
}

// HTTPAddonInterceptorAdminSpec defines admin service configuration
type HTTPAddonInterceptorAdminSpec struct {
	// Service is the name of the admin service
	// default: keda-http-add-on-interceptor-admin
	// +optional
	Service string `json:"service,omitempty"`

	// Port for the admin service
	// default: 9090
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// HTTPAddonInterceptorProxySpec defines proxy service configuration
type HTTPAddonInterceptorProxySpec struct {
	// Service is the name of the proxy service
	// default: keda-http-add-on-interceptor-proxy
	// +optional
	Service string `json:"service,omitempty"`

	// Port for the proxy service
	// default: 8080
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// HTTPAddonInterceptorScaledObjectSpec defines ScaledObject configuration
type HTTPAddonInterceptorScaledObjectSpec struct {
	// PollingInterval is how often KEDA polls the external scaler
	// default: 1
	// +optional
	// +kubebuilder:validation:Minimum=1
	PollingInterval int32 `json:"pollingInterval,omitempty"`
}

// HTTPAddonInterceptorTLSSpec defines TLS configuration for interceptor
type HTTPAddonInterceptorTLSSpec struct {
	// Enabled enables TLS server on the interceptor proxy
	// default: false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// CertPath is the mount path of the certificate file
	// default: /certs/tls.crt
	// +optional
	CertPath string `json:"certPath,omitempty"`

	// KeyPath is the mount path of the certificate key file
	// default: /certs/tls.key
	// +optional
	KeyPath string `json:"keyPath,omitempty"`

	// CertSecret is the name of the Kubernetes secret containing certificates
	// default: keda-tls-certs
	// +optional
	CertSecret string `json:"certSecret,omitempty"`

	// Port for the TLS server
	// default: 8443
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// TLSSkipVerify is a boolean flag to specify whether the interceptor should skip TLS verification for upstreams
	// default: false
	// +optional
	TLSSkipVerify bool `json:"tlsSkipVerify,omitempty"`
}

// HTTPAddonInterceptorPodDisruptionBudgetSpec defines PodDisruptionBudget configuration
type HTTPAddonInterceptorPodDisruptionBudgetSpec struct {
	// Enabled enables the PodDisruptionBudget for interceptor
	// default: true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// MinAvailable is the minimum number of available replicas
	// default: 0
	// +optional
	// +kubebuilder:validation:Minimum=0
	MinAvailable *int32 `json:"minAvailable,omitempty"`

	// MaxUnavailable is the maximum number of unavailable replicas
	// default: 1
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

// HTTPAddonComponentSpec defines common component configuration
type HTTPAddonComponentSpec struct {
	// Extra environment variables
	// +optional
	ExtraEnvs map[string]string `json:"extraEnvs,omitempty"`

	// ImagePullSecrets for pulling component images
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// PullPolicy for the component image
	// default: Always
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// NodeSelector for pod scheduling
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for pod scheduling
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity for pod scheduling
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// TopologySpreadConstraints for pod scheduling
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// PodAnnotations to be added to the pods
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// Resources for the component
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// HTTPAddonLoggingSpec defines logging configuration for HTTP Add-on components
type HTTPAddonLoggingSpec struct {
	// Level is the logging level
	// allowed values: 'debug', 'info', 'error', or an integer value greater than 0
	// default: info
	// +optional
	Level string `json:"level,omitempty"`

	// Format is the logging format
	// allowed values: 'json', 'console'
	// default: console
	// +optional
	// +kubebuilder:validation:Enum=json;console
	Format string `json:"format,omitempty"`

	// TimeEncoding is the logging time encoding
	// allowed values: 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339', 'rfc3339nano'
	// default: rfc3339
	// +optional
	// +kubebuilder:validation:Enum=epoch;millis;nano;iso8601;rfc3339;rfc3339nano
	TimeEncoding string `json:"timeEncoding,omitempty"`

	// StackTracesEnabled enables stack traces in logs
	// default: false
	// +optional
	StackTracesEnabled bool `json:"stackTracesEnabled,omitempty"`
}

// HTTPAddonImagesSpec defines image configuration for HTTP Add-on components
type HTTPAddonImagesSpec struct {
	// Tag is the image tag for all HTTP Add-on images
	// If empty, the default tag from the embedded manifest is used
	// +optional
	Tag string `json:"tag,omitempty"`

	// Operator is the image name for the operator component
	// default: ghcr.io/kedacore/http-add-on-operator
	// +optional
	Operator string `json:"operator,omitempty"`

	// Interceptor is the image name for the interceptor component
	// default: ghcr.io/kedacore/http-add-on-interceptor
	// +optional
	Interceptor string `json:"interceptor,omitempty"`

	// Scaler is the image name for the scaler component
	// default: ghcr.io/kedacore/http-add-on-scaler
	// +optional
	Scaler string `json:"scaler,omitempty"`
}

// HTTPAddonRBACSpec defines RBAC configuration for HTTP Add-on
type HTTPAddonRBACSpec struct {
	// AggregateToDefaultRoles enables aggregate roles for edit and view
	// default: false
	// +optional
	AggregateToDefaultRoles bool `json:"aggregateToDefaultRoles,omitempty"`
}
