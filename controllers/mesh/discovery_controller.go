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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	constants "github.com/morvencao/multicluster-mesh/pkg/constants"
	graphql "github.com/morvencao/multicluster-mesh/pkg/graphql"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
)

const (
	nonComplianceNoMappingResourcesErr = "couldn't find mapping resource with kind"
)

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
		Name:      smcpPolicyName,
		Namespace: constants.ACMNamespace,
	}, policyInstance)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	err = r.discoveryMeshFromPolicyStatus(policyInstance.Status.Status, log)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	policyPred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object.GetName() == smcpPolicyName && e.Object.GetNamespace() == constants.ACMNamespace {
				return true
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew.GetName() == smcpPolicyName && e.ObjectNew.GetNamespace() == constants.ACMNamespace {
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

func (r *DiscoveryReconciler) discoveryMeshFromPolicyStatus(compliancePerClusterStatus []*policyv1.CompliancePerClusterStatus, log logr.Logger) error {
	clusterSmcpMap, err := r.getClusterSmcp(compliancePerClusterStatus, log)
	if err != nil {
		return err
	}

	log.Info("========", "clusterSmcpMap", clusterSmcpMap)

	for clusterName, smcpLists := range clusterSmcpMap {
		for _, smcpNamespacedName := range smcpLists {
			namespacedName := strings.Split(smcpNamespacedName, "/")
			graphql.QuerySmcp(namespacedName[0], namespacedName[1], clusterName)
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
			Name:      constants.ACMNamespace + "." + smcpPolicyName,
			Namespace: v.ClusterNamespace,
		}, clusterPolicyInstance); err != nil {
			log.Error(err, "failed to get the cluster level policy")
			return nil, err
		}

		if clusterPolicyInstance.Status.ComplianceState != policyv1.NonCompliant {
			err := fmt.Errorf("incorrect status for cluster level policy and global policy")
			log.Error(err, "incorrect status", "policy name", clusterPolicyInstance.GetName(), "policy namespace", clusterPolicyInstance.GetNamespace())
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
