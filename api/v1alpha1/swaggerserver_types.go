/*
Copyright 2023.

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

// SwaggerServerSpec defines the desired state of SwaggerServer
type SwaggerServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ConfigMapName is the name of the ConfigMap containing the OpenAPI specifications.
	// +kubebuilder:validation:Required
	ConfigMapName string `json:"configMapName"`

	// Port is the port number on which the Swagger UI will be exposed.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Image is the Docker image to use for the Swagger UI server.
	// If not specified, defaults to "ghcr.io/hellices/openapi-multi-swagger:latest".
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy defines the policy for pulling the Docker image.
	// Defaults to "IfNotPresent".
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// Resources defines the CPU and memory resources for the Swagger UI server.
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// WatchIntervalSeconds is the interval in seconds for the server to check for updates to the ConfigMap.
	// Defaults to "10".
	// +optional
	WatchIntervalSeconds string `json:"watchIntervalSeconds,omitempty"`

	// LogLevel is the logging level for the Swagger UI server.
	// Valid values are: "trace", "debug", "info", "warn", "error", "fatal", "panic".
	// Defaults to "info".
	// +optional
	// +kubebuilder:validation:Enum=trace;debug;info;warn;error;fatal;panic
	LogLevel string `json:"logLevel,omitempty"`

	// DevMode enables or disables development mode for the Swagger UI server, which provides more verbose logging.
	// Valid values are: "true", "false".
	// Defaults to "false".
	// +optional
	// +kubebuilder:validation:Enum="true";"false"
	DevMode string `json:"devMode,omitempty"`
}

// ResourceRequirements describes the compute resource requirements
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed
	// +optional
	Limits ResourceList `json:"limits,omitempty"`

	// Requests describes the minimum amount of compute resources required
	// +optional
	Requests ResourceList `json:"requests,omitempty"`
}

// ResourceList is a map of resource names to quantities
type ResourceList map[string]string

// SwaggerServerStatus defines the observed state of SwaggerServer
type SwaggerServerStatus struct {
	// Ready indicates whether the Swagger UI server is ready to serve requests
	Ready bool `json:"ready"`

	// URL is the URL where the Swagger UI is accessible
	URL string `json:"url,omitempty"`

	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="READY",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// SwaggerServer is the Schema for the swaggerservers API
type SwaggerServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwaggerServerSpec   `json:"spec,omitempty"`
	Status SwaggerServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SwaggerServerList contains a list of SwaggerServer
type SwaggerServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SwaggerServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SwaggerServer{}, &SwaggerServerList{})
}
