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

// GlobalMeshSpec defines the desired state of GlobalMesh of a central view
type GlobalMeshSpec struct {
	Meshes []MeshFederation `json:"meshes,omitempty"`
}

// MeshFederation defines the mesh and its peers
type MeshFederation struct {
	Name  string     `json:"name,omitempty"`
	Peers []MeshPeer `json:"peers,omitempty"`
}

// MeshPeer defines the mesh peer information
type MeshPeer struct {
	Name string `json:"meshes,omitempty"`
}

// GlobalMeshStatus defines the observed state of GlobalMesh
type GlobalMeshStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GlobalMesh is the Schema for the globalmeshes API
type GlobalMesh struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GlobalMeshSpec   `json:"spec,omitempty"`
	Status GlobalMeshStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GlobalMeshList contains a list of GlobalMesh
type GlobalMeshList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GlobalMesh `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GlobalMesh{}, &GlobalMeshList{})
}
