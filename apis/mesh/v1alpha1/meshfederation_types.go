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

// MeshFederationSpec defines the desired state of MeshFederation of a central view
type MeshFederationSpec struct {
	MeshPeer    MeshPeer    `json:",inline"`
	TrustConfig TrustConfig `json:"trustConfig,omitempty"`
}

// MeshPeer defines mesh peers
type MeshPeer struct {
	Peers []string `json:"peers,omitempty"`
}

type TrustType string

const (
	TrustTypeLimited  TrustType = "Limited"  // limited trust gated at gateways, used by OSSM
	TrustTypeComplete TrustType = "Complete" // complete trust by shared CA, used by community istio
)

// TrustConfig defines the trust configuratin for mesh peers
type TrustConfig struct {
	TrustType TrustType `json:"trustType,omitempty"`
}

// MeshFederationStatus defines the observed state of MeshFederation
type MeshFederationStatus struct {
}

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MeshFederation is the Schema for the meshfederations API
type MeshFederation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MeshFederationSpec   `json:"spec,omitempty"`
	Status MeshFederationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MeshFederationList contains a list of MeshFederation
type MeshFederationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MeshFederation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MeshFederation{}, &MeshFederationList{})
}
