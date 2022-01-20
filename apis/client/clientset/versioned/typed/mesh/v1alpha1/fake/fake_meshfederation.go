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
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/morvencao/multicluster-mesh/apis/mesh/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMeshFederations implements MeshFederationInterface
type FakeMeshFederations struct {
	Fake *FakeMeshV1alpha1
	ns   string
}

var meshfederationsResource = schema.GroupVersionResource{Group: "mesh.open-cluster-management.io", Version: "v1alpha1", Resource: "meshfederations"}

var meshfederationsKind = schema.GroupVersionKind{Group: "mesh.open-cluster-management.io", Version: "v1alpha1", Kind: "MeshFederation"}

// Get takes name of the meshFederation, and returns the corresponding meshFederation object, and an error if there is any.
func (c *FakeMeshFederations) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.MeshFederation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(meshfederationsResource, c.ns, name), &v1alpha1.MeshFederation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MeshFederation), err
}

// List takes label and field selectors, and returns the list of MeshFederations that match those selectors.
func (c *FakeMeshFederations) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.MeshFederationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(meshfederationsResource, meshfederationsKind, c.ns, opts), &v1alpha1.MeshFederationList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MeshFederationList{ListMeta: obj.(*v1alpha1.MeshFederationList).ListMeta}
	for _, item := range obj.(*v1alpha1.MeshFederationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested meshFederations.
func (c *FakeMeshFederations) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(meshfederationsResource, c.ns, opts))

}

// Create takes the representation of a meshFederation and creates it.  Returns the server's representation of the meshFederation, and an error, if there is any.
func (c *FakeMeshFederations) Create(ctx context.Context, meshFederation *v1alpha1.MeshFederation, opts v1.CreateOptions) (result *v1alpha1.MeshFederation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(meshfederationsResource, c.ns, meshFederation), &v1alpha1.MeshFederation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MeshFederation), err
}

// Update takes the representation of a meshFederation and updates it. Returns the server's representation of the meshFederation, and an error, if there is any.
func (c *FakeMeshFederations) Update(ctx context.Context, meshFederation *v1alpha1.MeshFederation, opts v1.UpdateOptions) (result *v1alpha1.MeshFederation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(meshfederationsResource, c.ns, meshFederation), &v1alpha1.MeshFederation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MeshFederation), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMeshFederations) UpdateStatus(ctx context.Context, meshFederation *v1alpha1.MeshFederation, opts v1.UpdateOptions) (*v1alpha1.MeshFederation, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(meshfederationsResource, "status", c.ns, meshFederation), &v1alpha1.MeshFederation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MeshFederation), err
}

// Delete takes name of the meshFederation and deletes it. Returns an error if one occurs.
func (c *FakeMeshFederations) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(meshfederationsResource, c.ns, name), &v1alpha1.MeshFederation{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMeshFederations) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(meshfederationsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.MeshFederationList{})
	return err
}

// Patch applies the patch and returns the patched meshFederation.
func (c *FakeMeshFederations) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.MeshFederation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(meshfederationsResource, c.ns, name, pt, data, subresources...), &v1alpha1.MeshFederation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MeshFederation), err
}
