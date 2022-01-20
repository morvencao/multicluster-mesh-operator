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

package mesh

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	constants "github.com/morvencao/multicluster-mesh/pkg/constants"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshv1alpha1 "github.com/morvencao/multicluster-mesh/apis/mesh/v1alpha1"
)

// MeshFederationReconciler reconciles a MeshFederation object
type MeshFederationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshfederations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshfederations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshfederations/finalizers,verbs=update

func (r *MeshFederationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling request with meshfederation controller", "request name", req.Name, "namespace", req.Namespace)

	meshFederation := &meshv1alpha1.MeshFederation{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, meshFederation)
	if err != nil {
		log.Error(err, "unable to fetch MeshFederation")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	trustType := meshv1alpha1.TrustTypeComplete
	if meshFederation.Spec.TrustConfig != nil && meshFederation.Spec.TrustConfig.TrustType != "" {
		trustType = meshFederation.Spec.TrustConfig.TrustType
	}

	switch trustType {
	case meshv1alpha1.TrustTypeComplete:
		// for upstream istio
		log.Info("federate meshes with complete trust by shared CA")
	case meshv1alpha1.TrustTypeLimited:
		// for Openshift service mesh
		log.Info("federate meshes with limited trust gated at gateways")
	default:
		err := fmt.Errorf("invalid trust type")
		return ctrl.Result{}, err
	}

	meshPeers := meshFederation.Spec.MeshPeers
	for _, meshPeer := range meshPeers {
		peers := meshPeer.Peers
		if peers == nil || len(peers) != 2 || peers[0] == peers[1] || peers[0] == "" || peers[1] == "" {
			err := fmt.Errorf("two different meshes must specified in peers")
			return ctrl.Result{}, err
		}
		err := r.federateMeshPeers(peers, log)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MeshFederationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshv1alpha1.MeshFederation{}).
		Complete(r)
}

func (r *MeshFederationReconciler) federateMeshPeers(peers []string, log logr.Logger) error {
	mesh1, mesh2 := &meshv1alpha1.Mesh{}, &meshv1alpha1.Mesh{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      peers[0],
		Namespace: constants.ACMNamespace,
	}, mesh1)
	if err != nil {
		return err
	}

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      peers[1],
		Namespace: constants.ACMNamespace,
	}, mesh2)
	if err != nil {
		return err
	}

	existing := false
	for _, fgw := range mesh1.Spec.ControlPlane.FederationGateways {
		if fgw.MeshPeer == mesh2.GetName() {
			existing = true
			break
		}
	}
	if !existing {
		mesh1.Spec.ControlPlane.FederationGateways = append(mesh1.Spec.ControlPlane.FederationGateways, meshv1alpha1.FederationGateway{MeshPeer: mesh2.GetName()})
	}
	if err := r.Client.Update(context.TODO(), mesh1); err != nil {
		// failed to update the mesh
		log.Error(err, "failed to update the mesh", "mesh name", mesh1.GetName())
		return err
	}

	existing = false
	for _, fgw := range mesh2.Spec.ControlPlane.FederationGateways {
		if fgw.MeshPeer == mesh1.GetName() {
			existing = true
			break
		}
	}
	if !existing {
		mesh2.Spec.ControlPlane.FederationGateways = append(mesh2.Spec.ControlPlane.FederationGateways, meshv1alpha1.FederationGateway{MeshPeer: mesh1.GetName()})
	}
	if err := r.Client.Update(context.TODO(), mesh2); err != nil {
		// failed to update the mesh
		log.Error(err, "failed to update the mesh", "mesh name", mesh2.GetName())
		return err
	}

	return nil
}
