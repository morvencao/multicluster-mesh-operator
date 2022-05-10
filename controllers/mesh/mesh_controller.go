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

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	meshv1alpha1 "github.com/morvencao/multicluster-mesh-operator/apis/mesh/v1alpha1"
	maistrav1 "maistra.io/api/core/v1"
	maistrav2 "maistra.io/api/core/v2"

	constants "github.com/morvencao/multicluster-mesh-operator/pkg/constants"
	translate "github.com/morvencao/multicluster-mesh-operator/pkg/translate"
	configpolicyv1 "open-cluster-management.io/config-policy-controller/api/v1"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"
)

const (
	smcpEnforcePolicySuffix           = "sm-enf-"
	smcpEnforcePlacementRuleSuffix    = "sm-enf-"
	smcpEnforcePlacementBindingSuffix = "sm-enf-"
)

// MeshReconciler reconciles a Mesh object
type MeshReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mesh.open-cluster-management.io,resources=meshes/finalizers,verbs=update

func (r *MeshReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling request with mesh controller", "request name", req.Name, "namespace", req.Namespace)

	mesh := &meshv1alpha1.Mesh{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, mesh)
	if err != nil {
		log.Error(err, "unable to fetch Mesh")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// TODO(morvencao): handle mesh deletion
	// the policy (with complianceType musthave) will not delete resources from managed clusters after deletion

	if mesh.Spec.MeshProvider == "" {
		mesh.Spec.MeshProvider = meshv1alpha1.MeshProviderOpenshift
	}

	smcp, smmr, cluster, err := translate.TranslateToPhysicalMesh(mesh)
	if err != nil {
		log.Error(err, "failed to translate to physical mesh")
		return ctrl.Result{}, err
	}

	if err := r.buildAndApplyPolicyForSM(mesh, smcp, smmr, cluster, log); err != nil {
		log.Error(err, "failed to apply the enforce policy for mesh")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MeshReconciler) SetupWithManager(mgr ctrl.Manager) error {
	meshPred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			isDiscoveriedMesh, ok := e.Object.GetLabels()[constants.LabelKeyForDiscoveriedMesh]
			// skip create event for discoveried mesh
			if ok && isDiscoveriedMesh == "true" {
				return false
			}
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			isDiscoveriedMesh, ok := e.ObjectNew.GetLabels()[constants.LabelKeyForDiscoveriedMesh]
			// skip create event for discoveried mesh
			if ok && isDiscoveriedMesh == "true" {
				// for discoveried mesh, only requeue the change in Spec.FederationGateways
				if !reflect.DeepEqual(e.ObjectNew.(*meshv1alpha1.Mesh).Spec.ControlPlane.FederationGateways, e.ObjectOld.(*meshv1alpha1.Mesh).Spec.ControlPlane.FederationGateways) {
					return true
				}
				return false
			}
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&meshv1alpha1.Mesh{}, builder.WithPredicates(meshPred)).
		Complete(r)
}

func (r *MeshReconciler) buildAndApplyPolicyForSM(mesh *meshv1alpha1.Mesh, smcp *maistrav2.ServiceMeshControlPlane, smmr *maistrav1.ServiceMeshMemberRoll, cluster string, log logr.Logger) error {
	smcpNs := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: smcp.GetNamespace(),
		},
	}
	smcp.TypeMeta = metav1.TypeMeta{
		APIVersion: "maistra.io/v2",
		Kind:       "ServiceMeshControlPlane",
	}
	enforcedObjectTemplates := []*configpolicyv1.ObjectTemplate{
		&configpolicyv1.ObjectTemplate{
			ComplianceType: configpolicyv1.MustHave,
			ObjectDefinition: runtime.RawExtension{
				Object: smcpNs,
			},
		},
		&configpolicyv1.ObjectTemplate{
			ComplianceType: configpolicyv1.MustHave,
			ObjectDefinition: runtime.RawExtension{
				Object: smcp,
			},
		},
	}
	if smmr != nil {
		smmr.TypeMeta = metav1.TypeMeta{
			APIVersion: "maistra.io/v1",
			Kind:       "ServiceMeshMemberRoll",
		}
		enforcedObjectTemplates = append(enforcedObjectTemplates, &configpolicyv1.ObjectTemplate{
			ComplianceType: configpolicyv1.MustHave,
			ObjectDefinition: runtime.RawExtension{
				Object: smmr,
			},
		})
	}

	smcpEnforcePolicy := &policyv1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      smcpEnforcePolicySuffix + smcp.GetName(),
			Namespace: constants.ACMNamespace,
		},
		Spec: policyv1.PolicySpec{
			Disabled:          false,
			RemediationAction: policyv1.Enforce,
			PolicyTemplates: []*policyv1.PolicyTemplate{
				&policyv1.PolicyTemplate{
					ObjectDefinition: runtime.RawExtension{
						Object: &configpolicyv1.ConfigurationPolicy{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "policy.open-cluster-management.io/v1",
								Kind:       "ConfigurationPolicy",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name: smcpEnforcePolicySuffix + smcp.GetName(),
							},
							Spec: configpolicyv1.ConfigurationPolicySpec{
								Severity:          configpolicyv1.Severity("low"),
								RemediationAction: configpolicyv1.Enforce,
								NamespaceSelector: configpolicyv1.Target{
									Include: []configpolicyv1.NonEmptyString{configpolicyv1.NonEmptyString(smcp.GetNamespace())},
								},
								ObjectTemplates: enforcedObjectTemplates,
							},
						},
					},
				},
			},
		},
	}

	smcpEnforcePlacementRule := &placementrulev1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      smcpEnforcePlacementRuleSuffix + smcp.GetName(),
			Namespace: constants.ACMNamespace,
		},
		Spec: placementrulev1.PlacementRuleSpec{
			ClusterConditions: []placementrulev1.ClusterConditionFilter{
				placementrulev1.ClusterConditionFilter{
					Type:   "ManagedClusterConditionAvailable",
					Status: metav1.ConditionTrue,
				},
			},
			GenericPlacementFields: placementrulev1.GenericPlacementFields{
				Clusters: []placementrulev1.GenericClusterReference{
					placementrulev1.GenericClusterReference{
						Name: cluster,
					},
				},
			},
		},
	}

	smcpEnforcePlacementBinding := &policyv1.PlacementBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      smcpEnforcePlacementBindingSuffix + smcp.GetName(),
			Namespace: constants.ACMNamespace,
		},
		PlacementRef: policyv1.PlacementSubject{
			Name:     smcpEnforcePlacementRuleSuffix + smcp.GetName(),
			Kind:     "PlacementRule",
			APIGroup: placementrulev1.SchemeGroupVersion.Group,
		},
		Subjects: []policyv1.Subject{
			policyv1.Subject{
				Name:     smcpEnforcePolicySuffix + smcp.GetName(),
				Kind:     policyv1.Kind,
				APIGroup: policyv1.SchemeGroupVersion.Group,
			},
		},
	}

	if err := controllerutil.SetControllerReference(mesh, smcpEnforcePolicy, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", smcpEnforcePolicy.GetName())
	}

	if err := controllerutil.SetControllerReference(mesh, smcpEnforcePlacementRule, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", smcpEnforcePlacementRule.GetName())
	}

	if err := controllerutil.SetControllerReference(mesh, smcpEnforcePlacementBinding, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", smcpEnforcePlacementBinding.GetName())
	}

	// create or update the placementRule
	foundsmcpEnforcePlacementRule := &placementrulev1.PlacementRule{}
	err := r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      smcpEnforcePlacementRule.GetName(),
			Namespace: smcpEnforcePlacementRule.GetNamespace(),
		},
		foundsmcpEnforcePlacementRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new placementRule
			if err := r.Client.Create(context.TODO(), smcpEnforcePlacementRule); err != nil {
				log.Error(err, "failed to create the placementRule")
				return err
			}
			log.Info("created the placementRule", "placementRule", smcpEnforcePlacementRule.GetName())
		} else {
			// failed to get the placementRule
			log.Error(err, "failed to get the placementRule")
			return err
		}
	} else {
		// there is an existing placementRule
		// update if they are equal, update the existing placementRule if they are not equal
		if !equality.Semantic.DeepDerivative(smcpEnforcePlacementRule.Spec, foundsmcpEnforcePlacementRule.Spec) {
			log.Info("found difference between new placementRule and existing placementRule, updating...")
			// updated the placementRule
			smcpEnforcePlacementRule.ObjectMeta.ResourceVersion = foundsmcpEnforcePlacementRule.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), smcpEnforcePlacementRule); err != nil {
				// failed to update the placementRule
				log.Error(err, "failed to update the placementRule")
				return err
			}
			log.Info("updated the placementRule", "placementRule", smcpEnforcePlacementRule.GetName())
		} else {
			log.Info("new placementRule and existing placementRule are the same, no action needed.")
		}
	}

	// create or update placementBinding
	foundSmcpEnforcePlacementBinding := &policyv1.PlacementBinding{}
	err = r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      smcpEnforcePlacementBinding.GetName(),
			Namespace: smcpEnforcePlacementBinding.GetNamespace(),
		},
		foundSmcpEnforcePlacementBinding)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new placementBinding
			if err := r.Client.Create(context.TODO(), smcpEnforcePlacementBinding); err != nil {
				log.Error(err, "failed to create the placementBinding")
				return err
			}
			log.Info("created the placementBinding", "placementBinding", smcpEnforcePlacementBinding.GetName())
		} else {
			// failed to get the placementBinding
			log.Error(err, "failed to get the placementBinding")
			return err
		}
	} else {
		// there is an existing placementBinding
		// update if they are equal, update the existing placementBinding if they are not equal
		if !equality.Semantic.DeepDerivative(smcpEnforcePlacementBinding.PlacementRef, foundSmcpEnforcePlacementBinding.PlacementRef) || !equality.Semantic.DeepDerivative(smcpEnforcePlacementBinding.Subjects, foundSmcpEnforcePlacementBinding.Subjects) {
			log.Info("found difference between new placementBinding and existing placementBinding, updating...")
			// updated the placementBinding
			smcpEnforcePlacementBinding.ObjectMeta.ResourceVersion = foundSmcpEnforcePlacementBinding.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), smcpEnforcePlacementBinding); err != nil {
				// failed to update the placementBinding
				log.Error(err, "failed to update the placementBinding")
				return err
			}
			log.Info("updated the placementBinding", "placementBinding", smcpEnforcePlacementBinding.GetName())
		} else {
			log.Info("new placementBinding and existing placementBinding are the same, no action needed.")
		}
	}

	// create or update policy
	foundsmcpEnforcePolicy := &policyv1.Policy{}
	err = r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      smcpEnforcePolicy.GetName(),
			Namespace: smcpEnforcePolicy.GetNamespace(),
		},
		foundsmcpEnforcePolicy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new policy
			if err := r.Client.Create(context.TODO(), smcpEnforcePolicy); err != nil {
				log.Error(err, "failed to create the policy")
				return err
			}
			log.Info("created the policy", "policy", smcpEnforcePolicy.GetName())
		} else {
			// failed to get the policy
			log.Error(err, "failed to get the policy")
			return err
		}
	} else {
		// there is an existing policy
		// update if they are equal, update the existing policy if they are not equal
		if !equality.Semantic.DeepDerivative(smcpEnforcePolicy.Spec, foundsmcpEnforcePolicy.Spec) {
			log.Info("found difference between new policy and existing policy, updating...")
			// updated the policy
			smcpEnforcePolicy.ObjectMeta.ResourceVersion = foundsmcpEnforcePolicy.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), smcpEnforcePolicy); err != nil {
				// failed to update the policy
				log.Error(err, "failed to update the policy")
				return err
			}
			log.Info("updated the policy", "policy", smcpEnforcePolicy.GetName())
		} else {
			log.Info("new policy and existing policy are the same, no action needed.")
		}
	}

	return nil
}
