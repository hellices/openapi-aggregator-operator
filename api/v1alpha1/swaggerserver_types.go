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
	// Port is the port number the Swagger UI server will listen on
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=9090
	Port int32 `json:"port,omitempty"`

	// ConfigMapName is the name of the ConfigMap containing OpenAPI specs
	// +kubebuilder:validation:Required
	ConfigMapName string `json:"configMapName"`

	// BasePath is the base path for the Swagger UI server (for Ingress/Route support)
	// +optional
	BasePath string `json:"basePath,omitempty"`

	// Image is the Swagger UI server container image
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// ImagePullPolicy defines the policy for pulling the container image
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +kubebuilder:default=IfNotPresent
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// Resources defines the compute resource requirements for the server container
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`
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
