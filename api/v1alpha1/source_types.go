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

// SourceSpec defines the desired state of Source.
type SourceSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:printcolumn:JSONPath="spec.gitUrl",name=GitURL,type=string
	GitURL string `json:"gitUrl"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:printcolumn:JSONPath="spec.gitSecret",name=GitSecret,type=string
	GitSecret string `json:"gitSecret"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:printcolumn:JSONPath="spec.dockerRegistry",name=DockerRegistry,type=string
	DockerRegistry string `json:"dockerRegistry"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:printcolumn:JSONPath="spec.dockerSecret",name=DockerSecret,type=string
	DockerSecret string `json:"dockerSecret"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Source is the Schema for the sources API.
type Source struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SourceSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// SourceList contains a list of Source.
type SourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Source `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Source{}, &SourceList{})
}
