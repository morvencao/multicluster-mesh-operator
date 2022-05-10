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
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshv1alpha1 "github.com/morvencao/multicluster-mesh-operator/apis/mesh/v1alpha1"
	constants "github.com/morvencao/multicluster-mesh-operator/pkg/constants"
)

// MeshDeploymentReconciler reconciles a MeshDeployment object
type MeshDeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshdeployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshdeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshdeployments/finalizers,verbs=update

func (r *MeshDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling request with meshdeployment controller", "request name", req.Name, "namespace", req.Namespace)

	meshDeploy := &meshv1alpha1.MeshDeployment{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, meshDeploy)
	if err != nil {
		log.Error(err, "unable to fetch MeshDeployment")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	for i, c := range meshDeploy.Spec.Clusters {
		trustDomain := "cluster.local" // default trust domain
		if meshDeploy.Spec.TrustDomain != "" {
			trustDomain = meshDeploy.Spec.TrustDomain
		}
		trustDomainSplit := strings.Split(trustDomain, ".")
		// workaround for trust domain issue in mesh federation
		trustDomainSplit[0] = fmt.Sprintf("%s%d", trustDomainSplit[0], i+1)
		trustDomain = strings.Join(trustDomainSplit, ".")

		mesh := &meshv1alpha1.Mesh{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", c, meshDeploy.GetName()),
				Namespace: constants.ACMNamespace,
			},
			Spec: meshv1alpha1.MeshSpec{
				MeshProvider:   meshDeploy.Spec.MeshProvider,
				Cluster:        c,
				ControlPlane:   meshDeploy.Spec.ControlPlane,
				MeshMemberRoll: meshDeploy.Spec.MeshMemberRoll,
				TrustDomain:    trustDomain,
			},
		}

		if err := controllerutil.SetControllerReference(meshDeploy, mesh, r.Scheme); err != nil {
			log.Error(err, "failed to set controller reference", "name", mesh.GetName())
			return ctrl.Result{}, err
		}

		// create or update mesh
		foundMesh := &meshv1alpha1.Mesh{}
		err = r.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Name:      mesh.GetName(),
				Namespace: mesh.GetNamespace(),
			},
			foundMesh)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// create new mesh
				if err := r.Client.Create(context.TODO(), mesh); err != nil {
					log.Error(err, "failed to create the mesh", "mesh name", mesh.GetName())
					return ctrl.Result{}, err
				}
				log.Info("created the mesh", "mesh name", mesh.GetName())
			} else {
				// failed to get the mesh
				log.Error(err, "failed to get the mesh")
				return ctrl.Result{}, err
			}
		} else {
			// there is an existing mesh
			// update if they are equal, update the existing mesh if they are not equal
			if !equality.Semantic.DeepDerivative(mesh.Spec, foundMesh.Spec) {
				log.Info("found difference between new mesh and existing mesh, updating...")
				// updated the mesh
				mesh.ObjectMeta.ResourceVersion = foundMesh.ObjectMeta.ResourceVersion
				if err := r.Client.Update(context.TODO(), mesh); err != nil {
					// failed to update the mesh
					log.Error(err, "failed to update the mesh", "mesh name", mesh.GetName())
					return ctrl.Result{}, err
				}
				log.Info("updated the mesh", "mesh name", mesh.GetName())
			} else {
				log.Info("new mesh and existing mesh are the same, no action needed.")
			}
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MeshDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshv1alpha1.MeshDeployment{}).
		Complete(r)
}
