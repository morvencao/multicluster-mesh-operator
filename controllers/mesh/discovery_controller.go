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

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	constants "github.com/morvencao/multicluster-mesh/pkg/constants"
	graphql "github.com/morvencao/multicluster-mesh/pkg/graphql"
	translate "github.com/morvencao/multicluster-mesh/pkg/translate"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"

	meshv1alpha1 "github.com/morvencao/multicluster-mesh/apis/mesh/v1alpha1"
)

const (
	nonComplianceNoMappingResourcesErr = "couldn't find mapping resource with kind"
)

var MeshMap map[string]bool

func init() {
	MeshMap = make(map[string]bool)
}

// DiscoveryReconciler reconciles a Policy object
type DiscoveryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=policy.open-cluster-management.io,resources=policies/finalizers,verbs=update

func (r *DiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	policyInstance := &policyv1.Policy{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      smcpDiscoveryPolicyName,
		Namespace: constants.ACMNamespace,
	}, policyInstance)
	if err != nil {
		log.Error(err, "unable to fetch Policy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// reset mesh map
	for m := range MeshMap {
		MeshMap[m] = false
	}

	err = r.discoveryMeshFromPolicyStatus(policyInstance, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.pruneMeshes()
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	policyPred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object.GetName() == smcpDiscoveryPolicyName && e.Object.GetNamespace() == constants.ACMNamespace {
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew.GetName() == smcpDiscoveryPolicyName && e.ObjectNew.GetNamespace() == constants.ACMNamespace {
				return e.ObjectOld.GetResourceVersion() != e.ObjectNew.GetResourceVersion()
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&policyv1.Policy{}, builder.WithPredicates(policyPred)).
		Complete(r)
}

func (r *DiscoveryReconciler) discoveryMeshFromPolicyStatus(discoveryPolicy *policyv1.Policy, log logr.Logger) error {
	clusterSmcpMap, err := r.getClusterSmcp(discoveryPolicy.Status.Status, log)
	if err != nil {
		log.Error(err, "failed to get cluster-SMCP map")
		return err
	}
	log.Info("get the cluster-SMCP", "clusterSmcpMap", clusterSmcpMap)

	for clusterName, smcpLists := range clusterSmcpMap {
		for _, smcpNamespacedName := range smcpLists {
			namespacedName := strings.Split(smcpNamespacedName, "/")
			smcpJson, err := graphql.QueryK8sResource("maistra.io/v2", "ServiceMeshControlPlane", namespacedName[0], namespacedName[1], clusterName)
			if err != nil {
				log.Error(err, "failed to get the SMCP from graphql", "name", namespacedName[0], "namespace", namespacedName[1], "cluster", clusterName)
				return err
			}
			//log.Info("get the SMCP", "SMCP", string(smcpJson))
			// the ServiceMeshMemberRoll resource is created in the same namespace as ServiceMeshControlPlane with fixed name "default"
			smmrJson, err := graphql.QueryK8sResource("maistra.io/v1", "ServiceMeshMemberRoll", "default", namespacedName[1], clusterName)
			if err != nil {
				log.Error(err, "failed to get the SMMR from graphql", "name", "default", "namespace", namespacedName[1], "cluster", clusterName)
				return err
			}
			//log.Info("get the SMMR", "SMMR", string(smmrJson))
			mesh, err := translate.TranslateToLogicMesh(smcpJson, smmrJson, clusterName)
			if err != nil {
				log.Error(err, "failed to translate to mesh")
				return err
			}
			if mesh != nil {
				if err := controllerutil.SetControllerReference(discoveryPolicy, mesh, r.Scheme); err != nil {
					log.Error(err, "failed to set controller reference", "name", mesh.GetName())
				}
			}
			err = r.createOrUpdateMesh(mesh, log)
			if err != nil {
				return err
			}
			MeshMap[mesh.GetName()] = true
		}
	}

	return nil
}

func (r *DiscoveryReconciler) getClusterSmcp(compliancePerClusterStatus []*policyv1.CompliancePerClusterStatus, log logr.Logger) (map[string][]string, error) {
	clusterSmcpMap := make(map[string][]string)

	// check the nonCompliance clusters
	clusterPolicyInstance := &policyv1.Policy{}
	for _, v := range compliancePerClusterStatus {
		if v.ComplianceState == policyv1.Compliant {
			continue // skip compliance cluster
		}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      constants.ACMNamespace + "." + smcpDiscoveryPolicyName,
			Namespace: v.ClusterNamespace,
		}, clusterPolicyInstance); err != nil {
			log.Error(err, "failed to get the cluster level policy")
			return nil, err
		}

		if clusterPolicyInstance.Status.ComplianceState != policyv1.NonCompliant {
			err := fmt.Errorf("incorrect status for cluster level policy and global policy")
			//log.Error(err, "incorrect status", "policy name", clusterPolicyInstance.GetName(), "policy namespace", clusterPolicyInstance.GetNamespace())
			return nil, err
		}

		if len(clusterPolicyInstance.Status.Details) <= 0 {
			err := fmt.Errorf("empty details for cluster level policy")
			log.Error(err, "empty details", "policy name", clusterPolicyInstance.GetName(), "policy namespace", clusterPolicyInstance.GetNamespace())
			return nil, err
		}

		clusterPolicyStatusDetail := clusterPolicyInstance.Status.Details[0]
		if len(clusterPolicyStatusDetail.History) <= 0 {
			err := fmt.Errorf("empty history for cluster level policy")
			log.Error(err, "empty history", "policy name", clusterPolicyInstance.GetName(), "policy namespace", clusterPolicyInstance.GetNamespace())
			return nil, err
		}

		message := clusterPolicyStatusDetail.History[0].Message
		if message == "" {
			err := fmt.Errorf("empty non compliance message for cluster level policy")
			log.Error(err, "empty non compliance ", "policy name", clusterPolicyInstance.GetName(), "policy namespace", clusterPolicyInstance.GetNamespace())
			return nil, err
		}

		// skip no resource mapping cluster due to the OpenShift ServiceMesh operator is not installed in this case
		if strings.Contains(message, nonComplianceNoMappingResourcesErr) {
			log.Info("skip non compliance cluster caused by no mapping resource", "policy name", clusterPolicyInstance.GetName(), "policy namespace", clusterPolicyInstance.GetNamespace())
			continue
		}

		// get the smcp name-namespace map from the compliance history message
		smcpList, err := getSmcpList(message)
		if err != nil {
			log.Error(err, "failed to get the smcp map from non compliance message", "policy name", clusterPolicyInstance.GetName(), "policy namespace", clusterPolicyInstance.GetNamespace())
			return nil, err
		}

		log.Info("get SMCPs from non compliance message", "map", smcpList, "policy name", clusterPolicyInstance.GetName(), "policy namespace", clusterPolicyInstance.GetNamespace())
		clusterSmcpMap[v.ClusterName] = smcpList
	}

	return clusterSmcpMap, nil
}

func getSmcpList(msg string) ([]string, error) {
	res := make([]string, 0)
	msgs := strings.Split(msg, ";")
	for i, v := range msgs {
		if i == 0 {
			if v != "NonCompliant" {
				return nil, fmt.Errorf("invalid non complicance message: %s", msg)
			}
			continue
		}
		smcpName := v[strings.IndexByte(v, '[')+1 : strings.IndexByte(v, ']')]
		if smcpName == "" {
			return nil, fmt.Errorf("empty smcp name from message: %s", msg)
		}
		vs := strings.Split(v, " ")
		if len(vs) <= 1 || vs[len(vs)-2] != "namespace" {
			return nil, fmt.Errorf("no smcp namespace detail found in message: %s", msg)
		}
		smcpNamespace := vs[len(vs)-1]
		res = append(res, smcpName+"/"+smcpNamespace)
	}

	return res, nil
}

func (r *DiscoveryReconciler) createOrUpdateMesh(mesh *meshv1alpha1.Mesh, log logr.Logger) error {
	// create or update mesh
	foundMesh := &meshv1alpha1.Mesh{}
	err := r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      mesh.GetName(),
			Namespace: constants.ACMNamespace,
		},
		foundMesh)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new mesh
			if err := r.Client.Create(context.TODO(), mesh); err != nil {
				log.Error(err, "failed to create the mesh", "name", mesh.GetName())
				return err
			}
			log.Info("created the mesh", "name", mesh.GetName())
			emptyMeshStatus := meshv1alpha1.MeshStatus{}
			if !equality.Semantic.DeepDerivative(mesh.Status, emptyMeshStatus) { // update mesh status if it is not empty
				if err := r.Client.Status().Update(context.TODO(), mesh); err != nil {
					log.Error(err, "failed to update the mesh status", "name", mesh.GetName())
					return err
				}
			}
		} else {
			// failed to get the mesh
			log.Error(err, "failed to get the mesh", "name", mesh.GetName())
			return err
		}
	} else {
		// there is an existing mesh
		// update if they are equal, update the existing mesh if they are not equal
		if !equality.Semantic.DeepDerivative(mesh.Spec, foundMesh.Spec) {
			log.Info("found difference between new mesh and existing mesh spec, updating...")
			// updated the mesh
			mesh.ObjectMeta.ResourceVersion = foundMesh.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), mesh); err != nil {
				// failed to update the mesh
				log.Error(err, "failed to update the mesh")
				return err
			}
			log.Info("updated the mesh", "name", mesh.GetName())
		} else if !equality.Semantic.DeepDerivative(mesh.Status, foundMesh.Status) {
			log.Info("found difference between new mesh and existing mesh status, updating...")
			// updated the mesh
			mesh.ObjectMeta.ResourceVersion = foundMesh.ObjectMeta.ResourceVersion
			if err := r.Client.Status().Update(context.TODO(), mesh); err != nil {
				// failed to update the mesh status
				log.Error(err, "failed to update the mesh status")
				return err
			}
			log.Info("updated the mesh status", "name", mesh.GetName())
		} else {
			log.Info("new mesh and existing mesh are the same, no action needed.")
		}
	}
	return nil
}

func (r *DiscoveryReconciler) pruneMeshes() error {
	foundMesh := &meshv1alpha1.Mesh{}
	for m, ok := range MeshMap {
		if !ok {
			err := r.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      m,
					Namespace: constants.ACMNamespace,
				},
				foundMesh)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// do nothing
					continue
				}
				return err
			}
			if err := r.Client.Delete(context.TODO(), foundMesh); err != nil {
				return err
			}
		}
	}
	return nil
}
