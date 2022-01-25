/*
Copyright 2022.

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

// MeshDeploymentSpec defines the desired state of MeshDeployment
type MeshDeploymentSpec struct {
	MeshProvider   MeshProvider      `json:"meshProvider,omitempty"`
	Clusters       []string          `json:"clusters,omitempty"`
	ControlPlane   *MeshControlPlane `json:"controlPlane,omitempty"`
	MeshMemberRoll []string          `json:"meshMemberRoll,omitempty"`
	TrustDomain    string            `json:"trustDomain,omitempty"`
}

// MeshDeploymentStatus defines the observed state of MeshDeployment
type MeshDeploymentStatus struct {
}

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="CLUSTERS",type="string",JSONPath=".spec.clusters",description="Target clusters of the mesh deployment"
//+kubebuilder:printcolumn:name="VERSION",type="string",JSONPath=".spec.controlPlane.version",description="Version of the mesh"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// MeshDeployment is the Schema for the meshdeployments API
type MeshDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeshDeploymentSpec   `json:"spec,omitempty"`
	Status MeshDeploymentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MeshDeploymentList contains a list of MeshDeployment
type MeshDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeshDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeshDeployment{}, &MeshDeploymentList{})
}
