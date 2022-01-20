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

	"k8s.io/apimachinery/pkg/runtime"
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

	meshFederation := &meshv1alpha1.MeshFederation{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, meshFederation)
	if err != nil {
		log.Error(err, "unable to fetch MeshFederation")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MeshFederationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshv1alpha1.MeshFederation{}).
		Complete(r)
}
