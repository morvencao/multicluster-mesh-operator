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
// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/morvencao/multicluster-mesh/apis/mesh/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// MeshFederationLister helps list MeshFederations.
// All objects returned here must be treated as read-only.
type MeshFederationLister interface {
	// List lists all MeshFederations in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.MeshFederation, err error)
	// MeshFederations returns an object that can list and get MeshFederations.
	MeshFederations(namespace string) MeshFederationNamespaceLister
	MeshFederationListerExpansion
}

// meshFederationLister implements the MeshFederationLister interface.
type meshFederationLister struct {
	indexer cache.Indexer
}

// NewMeshFederationLister returns a new MeshFederationLister.
func NewMeshFederationLister(indexer cache.Indexer) MeshFederationLister {
	return &meshFederationLister{indexer: indexer}
}

// List lists all MeshFederations in the indexer.
func (s *meshFederationLister) List(selector labels.Selector) (ret []*v1alpha1.MeshFederation, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.MeshFederation))
	})
	return ret, err
}

// MeshFederations returns an object that can list and get MeshFederations.
func (s *meshFederationLister) MeshFederations(namespace string) MeshFederationNamespaceLister {
	return meshFederationNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// MeshFederationNamespaceLister helps list and get MeshFederations.
// All objects returned here must be treated as read-only.
type MeshFederationNamespaceLister interface {
	// List lists all MeshFederations in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.MeshFederation, err error)
	// Get retrieves the MeshFederation from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.MeshFederation, error)
	MeshFederationNamespaceListerExpansion
}

// meshFederationNamespaceLister implements the MeshFederationNamespaceLister
// interface.
type meshFederationNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all MeshFederations in the indexer for a given namespace.
func (s meshFederationNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.MeshFederation, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.MeshFederation))
	})
	return ret, err
}

// Get retrieves the MeshFederation from the indexer for a given namespace and name.
func (s meshFederationNamespaceLister) Get(name string) (*v1alpha1.MeshFederation, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("meshfederation"), name)
	}
	return obj.(*v1alpha1.MeshFederation), nil
}
