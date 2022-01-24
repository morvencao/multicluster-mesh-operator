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
// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	internalinterfaces "github.com/morvencao/multicluster-mesh/apis/client/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// Meshes returns a MeshInformer.
	Meshes() MeshInformer
	// MeshDeployments returns a MeshDeploymentInformer.
	MeshDeployments() MeshDeploymentInformer
	// MeshFederations returns a MeshFederationInformer.
	MeshFederations() MeshFederationInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// Meshes returns a MeshInformer.
func (v *version) Meshes() MeshInformer {
	return &meshInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// MeshDeployments returns a MeshDeploymentInformer.
func (v *version) MeshDeployments() MeshDeploymentInformer {
	return &meshDeploymentInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// MeshFederations returns a MeshFederationInformer.
func (v *version) MeshFederations() MeshFederationInformer {
	return &meshFederationInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
