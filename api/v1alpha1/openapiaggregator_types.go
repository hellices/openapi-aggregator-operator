/*
Copyright 2025.

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

// OpenAPIAggregatorSpec defines the desired state of OpenAPIAggregator
type OpenAPIAggregatorSpec struct {
	// LabelSelector selects target deployments to collect OpenAPI specs from
	LabelSelector map[string]string `json:"labelSelector,omitempty"`

	// DefaultPath is the default path for OpenAPI documentation
	// +kubebuilder:default="/v2/api-docs"
	DefaultPath string `json:"defaultPath,omitempty"`

	// DefaultPort is the default port for OpenAPI documentation
	// +kubebuilder:default="8080"
	DefaultPort string `json:"defaultPort,omitempty"`

	// SwaggerAnnotation is the annotation key that indicates if the Service should be included
	// +kubebuilder:default="openapi.aggregator.io/swagger"
	SwaggerAnnotation string `json:"swaggerAnnotation,omitempty"`

	// PathAnnotation is the annotation key for OpenAPI path
	// +kubebuilder:default="openapi.aggregator.io/path"
	PathAnnotation string `json:"pathAnnotation,omitempty"`

	// PortAnnotation is the annotation key for OpenAPI port
	// +kubebuilder:default="openapi.aggregator.io/port"
	PortAnnotation string `json:"portAnnotation,omitempty"`

	// AllowedMethodsAnnotation is the annotation key for allowed HTTP methods in Swagger UI
	// +kubebuilder:default="openapi.aggregator.io/allowed-methods"
	AllowedMethodsAnnotation string `json:"allowedMethodsAnnotation,omitempty"`
}

// OpenAPIAggregatorStatus defines the observed state of OpenAPIAggregator
type OpenAPIAggregatorStatus struct {
	// CollectedAPIs contains information about the OpenAPI specs that have been collected
	CollectedAPIs []APIInfo `json:"collectedAPIs,omitempty"`
}

// APIInfo contains information about a collected OpenAPI spec
type APIInfo struct {
	// Name is the name of the API (usually same as deployment name)
	Name string `json:"name"`

	// URL is the full URL where the OpenAPI spec can be accessed
	URL string `json:"url"`

	// LastUpdated is when the spec was last successfully collected
	LastUpdated string `json:"lastUpdated"`

	// Error is set if there was an error collecting the spec
	Error string `json:"error,omitempty"`

	// ResourceType is the type of the kubernetes resource (Deployment)
	ResourceType string `json:"resourceType"`

	// ResourceName is the name of the kubernetes resource
	ResourceName string `json:"resourceName"`

	// Namespace is the namespace of the kubernetes resource
	Namespace string `json:"namespace"`

	// Path is the OpenAPI spec path for this service
	Path string `json:"path"`

	// Port is the port for this service's OpenAPI spec
	Port string `json:"port"`

	// Annotations stores relevant annotations from the resource
	Annotations map[string]string `json:"annotations,omitempty"`

	// AllowedMethods stores the allowed HTTP methods for Swagger UI
	AllowedMethods []string `json:"allowedMethods,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OpenAPIAggregator is the Schema for the openapiaggregators API
type OpenAPIAggregator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenAPIAggregatorSpec   `json:"spec,omitempty"`
	Status OpenAPIAggregatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenAPIAggregatorList contains a list of OpenAPIAggregator
type OpenAPIAggregatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenAPIAggregator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenAPIAggregator{}, &OpenAPIAggregatorList{})
}
