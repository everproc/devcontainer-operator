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
	// +kubebuilder:validation:Required
	// +kubebuilder:printcolumn:JSONPath="spec.gitUrl",name=GitURL,type=string,description="The git url to download the repository."
	// The git url to download the repository.
	GitURL string `json:"gitUrl"`
	// +kubebuilder:validation:Optional
	// The secret name that stores the SSH private key to download the private repository.
	GitSecret string `json:"gitSecret,omitempty"`
	// +kubebuilder:validation:Optional
	// The git reference to checkout a specific hash or tag.
	GitHashOrTag string `json:"gitHashOrTag,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:printcolumn:JSONPath="spec.containerRegistry",name=ContainerRegistry,type=string
	ContainerRegistry string `json:"containerRegistry,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:printcolumn:JSONPath="spec.registryCredentials",name=RegistryCredentials,type=string
	RegistryCredentials string `json:"registryCredentials,omitempty"`
	// +kubebuilder:validation:Optional
	// The owner of this builder
	Owner string `json:"owner,omitempty"`
	// +kubebuilder:validation:Optional
	// The storage class that is used for PVC creation.
	StorageClassName string `json:"storageClassName,omitempty"`
}

const (
	WorkspaceCondTypePending = "Pending"
	WorkspaceCondTypeInUse   = "InUse"
	WorkspaceCondTypeReady   = "Ready"
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
	// Reference to the source responsible for storing the external resources
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
