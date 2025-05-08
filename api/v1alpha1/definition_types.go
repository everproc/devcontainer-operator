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
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EqualParsedDefinitions(a, b *ParsedDefinition) bool {
	return reflect.DeepEqual(a, b)
}

type BuildSpec struct {
	// +kubebuilder:validation:Optional
	Dockerfile string `json:"dockerfile"`
	// +kubebuilder:validation:Optional
	Args map[string]string `json:"args"`
}

type RunSpec struct {
	// +kubebuilder:validation:Optional
	Args []string `json:"args"`
}

// DefinitionSpec defines the desired state of Definition.
type DefinitionSpec struct {
}

// The parsed devcontainer json.
type ParsedDefinition struct {
	// +kubebuilder:validation:Optional
	PodTpl *corev1.PodTemplateSpec `json:"podTemplateSpec"`
	// +kubebuilder:validation:Optional
	RawDefinition string `json:"rawDefinition"`
	// +kubebuilder:validation:Optional
	Image string `json:"image"`
	// +kubebuilder:validation:Optional
	Build BuildSpec `json:"build"`
	// +kubebuilder:validation:Optional
	Run RunSpec `json:"run"`
	// +kubebuilder:validation:Optional
	GitHash string `json:"gitHash"`
}

const (
	DefinitionCondTypeReady        = "Ready"
	DefinitionCondTypeParsed       = "Parsed"
	DefinitionCondTypeBuilt        = "Built"
	DefinitionCondTypeRemoteCloned = "RemoteCloned"
)

func InitialConditionsDefinition() []metav1.Condition {
	return []metav1.Condition{
		{
			Type:   DefinitionCondTypeReady,
			Status: metav1.ConditionUnknown,
			Reason: "NotStarted",
		},
		{
			Type:   DefinitionCondTypeParsed,
			Status: metav1.ConditionUnknown,
			Reason: "NotStarted",
		},
		{
			Type:   DefinitionCondTypeRemoteCloned,
			Status: metav1.ConditionUnknown,
			Reason: "NotStarted",
		},
	}
}

// DefinitionStatus defines the observed state of Definition.
type DefinitionStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=ReadyState,type=string,JSONPath=".status.conditions[?(@.type=='Parsed')].reason"

// Definition is the Schema for the definitions API.
type Definition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DefinitionSpec   `json:"spec,omitempty"`
	Parsed ParsedDefinition `json:"pasedDefinition,omitempty"`
	Status DefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DefinitionList contains a list of Definition.
type DefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Definition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Definition{}, &DefinitionList{})
}
