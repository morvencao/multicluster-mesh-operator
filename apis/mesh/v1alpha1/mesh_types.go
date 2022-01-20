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
	maistrav2 "maistra.io/api/core/v2"
)

// MeshSpec defines the desired state of physical service mesh in a managed cluster
type MeshSpec struct {
	MeshProvider   MeshProvider      `json:"meshProvider,omitempty"`
	Cluster        string            `json:"cluster,omitempty"`
	ControlPlane   *MeshControlPlane `json:"controlPlane,omitempty"`
	MeshMemberRoll []string          `json:"meshMemberRoll,omitempty"`
	TrustDomain    string            `json:"trustDomain,omitempty"`
}

type MeshProvider string

const (
	MeshProviderOpenshift      MeshProvider = "Openshift Service Mesh"
	MeshProviderCommunityIstio MeshProvider = "Community Istio"
	// more providers come later
)

// MeshControlPlane defines the mesh control plane
type MeshControlPlane struct {
	Namespace          string              `json:"namespace,omitempty"`
	Version            string              `json:"version,omitempty"`
	Profiles           []string            `json:"profiles,omitempty"`
	Components         []string            `json:"components,omitempty"`
	FederationGateways []FederationGateway `json:"federationGateways,omitempty"`
}

// FederationGateway defines the ingressgateway and egressgateways used for mesh federation
type FederationGateway struct {
	MeshPeer string `json:"meshPeer,omitempty"`
	// additional setting for federation gateway...
}

// MeshStatus defines the observed state of Mesh
type MeshStatus struct {
	Readiness maistrav2.ReadinessStatus `json:"readiness"`
}

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Mesh is the Schema for the meshes API
type Mesh struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeshSpec   `json:"spec,omitempty"`
	Status MeshStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MeshList contains a list of Mesh
type MeshList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Mesh `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Mesh{}, &MeshList{})
}
