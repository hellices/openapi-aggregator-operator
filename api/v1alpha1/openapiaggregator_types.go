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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpenAPIAggregatorSpec defines the desired state of OpenAPIAggregator
type OpenAPIAggregatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of OpenAPIAggregator. Edit openapiaggregator_types.go to remove/update
	// Foo string `json:"foo,omitempty"`
	// 감시할 네임스페이스 (빈 값이면 전체)
	NamespaceSelector string `json:"namespaceSelector,omitempty"`

	// Deployment에 붙은 라벨 셀렉터
	LabelSelector map[string]string `json:"labelSelector,omitempty"`

	// OpenAPI 경로 (예: /v3/api-docs)
	// +kubebuilder:default="/v3/api-docs"
	DefaultPath string `json:"defaultPath,omitempty"`

	// 포트 이름 또는 번호 (예: "http" 또는 "8080")
	// +kubebuilder:default="8080"
	DefaultPort string `json:"defaultPort,omitempty"`

	// Swagger UI에 표시할 이름 prefix
	DisplayNamePrefix string `json:"displayNamePrefix,omitempty"`

	// OpenAPI 경로를 지정하는 annotation 키
	// +kubebuilder:default="openapi.aggregator.io/path"
	PathAnnotation string `json:"pathAnnotation,omitempty"`

	// OpenAPI 포트를 지정하는 annotation 키
	// +kubebuilder:default="openapi.aggregator.io/port"
	PortAnnotation string `json:"portAnnotation,omitempty"`

	// annotation 무시 여부 (true면 기본값만 사용)
	// +kubebuilder:default=false
	IgnoreAnnotations bool `json:"ignoreAnnotations,omitempty"`
}

// OpenAPIAggregatorStatus defines the observed state of OpenAPIAggregator
type OpenAPIAggregatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// CollectedAPIs contains information about the OpenAPI specs that have been collected
	CollectedAPIs []APIInfo `json:"collectedAPIs,omitempty"`
}

// APIInfo contains information about a collected OpenAPI spec
type APIInfo struct {
	// Name is the name of the API (usually from displayNamePrefix + deployment name)
	Name string `json:"name"`
	// URL is the full URL where the OpenAPI spec can be accessed
	URL string `json:"url"`
	// LastUpdated is when the spec was last successfully collected
	LastUpdated string `json:"lastUpdated"`
	// Error is set if there was an error collecting the spec
	Error string `json:"error,omitempty"`
	// ResourceType is the type of the kubernetes resource (Deployment, StatefulSet, etc)
	ResourceType string `json:"resourceType"`
	// ResourceName is the name of the kubernetes resource
	ResourceName string `json:"resourceName"`
	// Namespace is the namespace of the kubernetes resource
	Namespace string `json:"namespace"`
	// Annotations stores relevant annotations from the resource
	Annotations map[string]string `json:"annotations,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// OpenAPIAggregator is the Schema for the openapiaggregators API
type OpenAPIAggregator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenAPIAggregatorSpec   `json:"spec,omitempty"`
	Status OpenAPIAggregatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenAPIAggregatorList contains a list of OpenAPIAggregator
type OpenAPIAggregatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenAPIAggregator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenAPIAggregator{}, &OpenAPIAggregatorList{})
}
