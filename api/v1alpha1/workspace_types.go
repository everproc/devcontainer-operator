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

// WorkspaceSpec defines the desired state of Workspace.
type WorkspaceSpec struct {
	// TODO (juf): Should we use a PodSpecTemplate approach here or not? Need to do research
	// DefinitionSpecTemplate DefinitionSpecTemplate `json:"definition_spec_template"`

	// +kubebuilder:validation:Required
	// The owner of this workspace
	Owner string `json:"owner"`

	// +kubebuilder:validation:Required
	// The link to the workspace definition
	DefinitionRef string `json:"definitionRef"`

	// +kubebuilder:validation:Optional
	// The storage class that is used for PVC creation.
	StorageClassName string `json:"storageClassName"`
}

const (
	WorkspaceCondTypeInUse = "InUse"
	WorkspaceCondTypeReady = "Ready"
)

func initialConditionsWorkspace() []metav1.Condition {
	return []metav1.Condition{
		{
			Type:   WorkspaceCondTypeInUse,
			Status: metav1.ConditionUnknown,
			Reason: "NotStarted",
		},
		{
			Type:   WorkspaceCondTypeReady,
			Status: metav1.ConditionUnknown,
			Reason: "NotStarted",
		},
	}
}

// WorkspaceStatus defines the observed state of Workspace.
type WorkspaceStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=ReadyState,type=string,JSONPath=".status['Ready'].status"
// +kubebuilder:printcolumn:JSONPath=".spec.owner",name=Owner,type=string
// +kubebuilder:printcolumn:JSONPath=".spec.definitionRef",name=DefinitionRef,type=string

// Workspace is the Schema for the workspaces API.
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkspaceList contains a list of Workspace.
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}
