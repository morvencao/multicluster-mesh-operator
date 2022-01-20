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

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	configpolicyv1 "open-cluster-management.io/config-policy-controller/api/v1"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"

	constants "github.com/morvencao/multicluster-mesh/pkg/constants"
)

const (
	smcpDiscoveryPolicyName           = "sm-discovery"
	smcpDiscoveryPlacementRuleName    = "sm-discovery"
	smcpDiscoveryPlacementBindingName = "sm-discovery"
)

var (
	smcpDiscoveryPolicy = &policyv1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      smcpDiscoveryPolicyName,
			Namespace: constants.ACMNamespace,
		},
		Spec: policyv1.PolicySpec{
			Disabled: false,
			PolicyTemplates: []*policyv1.PolicyTemplate{
				&policyv1.PolicyTemplate{
					ObjectDefinition: runtime.RawExtension{
						Object: &configpolicyv1.ConfigurationPolicy{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "policy.open-cluster-management.io/v1",
								Kind:       "ConfigurationPolicy",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name: smcpDiscoveryPolicyName,
							},
							Spec: configpolicyv1.ConfigurationPolicySpec{
								Severity:          configpolicyv1.Severity("low"),
								RemediationAction: configpolicyv1.Inform,
								NamespaceSelector: configpolicyv1.Target{
									Exclude: []configpolicyv1.NonEmptyString{configpolicyv1.NonEmptyString("kube-*")},
									Include: []configpolicyv1.NonEmptyString{configpolicyv1.NonEmptyString("*")},
								},
								ObjectTemplates: []*configpolicyv1.ObjectTemplate{
									&configpolicyv1.ObjectTemplate{
										ComplianceType: configpolicyv1.MustNotHave,
										ObjectDefinition: runtime.RawExtension{
											Raw: []byte(`{
"apiVersion": "maistra.io/v2",
"kind": "ServiceMeshControlPlane"}`),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	smcpDiscoveryPlacementRule = &placementrulev1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      smcpDiscoveryPlacementRuleName,
			Namespace: constants.ACMNamespace,
		},
		Spec: placementrulev1.PlacementRuleSpec{
			ClusterConditions: []placementrulev1.ClusterConditionFilter{
				placementrulev1.ClusterConditionFilter{
					Type:   "ManagedClusterConditionAvailable",
					Status: metav1.ConditionTrue,
				},
			},
		},
	}

	smcpDiscoveryPlacementBinding = &policyv1.PlacementBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      smcpDiscoveryPlacementBindingName,
			Namespace: constants.ACMNamespace,
		},
		PlacementRef: policyv1.PlacementSubject{
			Name:     smcpDiscoveryPlacementRuleName,
			Kind:     "PlacementRule",
			APIGroup: placementrulev1.SchemeGroupVersion.Group,
		},
		Subjects: []policyv1.Subject{
			policyv1.Subject{
				Name:     smcpDiscoveryPolicyName,
				Kind:     policyv1.Kind,
				APIGroup: policyv1.SchemeGroupVersion.Group,
			},
		},
	}
)

type PolicyController struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewPolicyController create the PolicyController.
func NewPolicyController(client client.Client, scheme *runtime.Scheme) *PolicyController {
	return &PolicyController{
		Client: client,
		Scheme: scheme,
	}
}

// Start runs the PolicyController with the given context.
// it will create the corresponding Policy instance with the client
// at starting and remove it when context done singal is received.
func (pc *PolicyController) Start(ctx context.Context) error {
	log := log.FromContext(ctx)

	// create or update placementRule
	foundsmcpDiscoveryPlacementRule := &placementrulev1.PlacementRule{}
	err := pc.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      smcpDiscoveryPlacementRule.GetName(),
			Namespace: smcpDiscoveryPlacementRule.GetNamespace(),
		},
		foundsmcpDiscoveryPlacementRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new placementRule
			if err := pc.Client.Create(context.TODO(), smcpDiscoveryPlacementRule); err != nil {
				log.Error(err, "failed to create the placementRule")
				return err
			}
			log.Info("created the placementRule", "placementRule", smcpDiscoveryPlacementRule.GetName())
		} else {
			// failed to get the placementRule
			log.Error(err, "failed to get the placementRule")
			return err
		}
	} else {
		// there is an existing placementRule
		// update if they are equal, update the existing placementRule if they are not equal
		if !equality.Semantic.DeepDerivative(smcpDiscoveryPlacementRule.Spec, foundsmcpDiscoveryPlacementRule.Spec) {
			log.Info("found difference between new placementRule and existing placementRule, updating...")
			// updated the placementRule
			smcpDiscoveryPlacementRule.ObjectMeta.ResourceVersion = foundsmcpDiscoveryPlacementRule.ObjectMeta.ResourceVersion
			if err := pc.Client.Update(context.TODO(), smcpDiscoveryPlacementRule); err != nil {
				// failed to update the placementRule
				log.Error(err, "failed to update the placementRule")
				return err
			}
			log.Info("updated the placementRule", "placementRule", smcpDiscoveryPlacementRule.GetName())
		} else {
			log.Info("new placementRule and existing placementRule are the same, no action needed.")
		}
	}

	// create or update placementBinding
	foundSmcpDiscoveryPlacementBinding := &policyv1.PlacementBinding{}
	err = pc.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      smcpDiscoveryPlacementBinding.GetName(),
			Namespace: smcpDiscoveryPlacementBinding.GetNamespace(),
		},
		foundSmcpDiscoveryPlacementBinding)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new placementBinding
			if err := pc.Client.Create(context.TODO(), smcpDiscoveryPlacementBinding); err != nil {
				log.Error(err, "failed to create the placementBinding")
				return err
			}
			log.Info("created the placementBinding", "placementBinding", smcpDiscoveryPlacementBinding.GetName())
		} else {
			// failed to get the placementBinding
			log.Error(err, "failed to get the placementBinding")
			return err
		}
	} else {
		// there is an existing placementBinding
		// update if they are equal, update the existing placementBinding if they are not equal
		if !equality.Semantic.DeepDerivative(smcpDiscoveryPlacementBinding.PlacementRef, foundSmcpDiscoveryPlacementBinding.PlacementRef) || !equality.Semantic.DeepDerivative(smcpDiscoveryPlacementBinding.Subjects, foundSmcpDiscoveryPlacementBinding.Subjects) {
			log.Info("found difference between new placementBinding and existing placementBinding, updating...")
			// updated the placementBinding
			smcpDiscoveryPlacementBinding.ObjectMeta.ResourceVersion = foundSmcpDiscoveryPlacementBinding.ObjectMeta.ResourceVersion
			if err := pc.Client.Update(context.TODO(), smcpDiscoveryPlacementBinding); err != nil {
				// failed to update the placementBinding
				log.Error(err, "failed to update the placementBinding")
				return err
			}
			log.Info("updated the placementBinding", "placementBinding", smcpDiscoveryPlacementBinding.GetName())
		} else {
			log.Info("new placementBinding and existing placementBinding are the same, no action needed.")
		}
	}

	// create or update policy
	foundsmcpDiscoveryPolicy := &policyv1.Policy{}
	err = pc.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      smcpDiscoveryPolicy.GetName(),
			Namespace: smcpDiscoveryPolicy.GetNamespace(),
		},
		foundsmcpDiscoveryPolicy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new policy
			if err := pc.Client.Create(context.TODO(), smcpDiscoveryPolicy); err != nil {
				log.Error(err, "failed to create the policy")
				return err
			}
			log.Info("created the policy", "policy", smcpDiscoveryPolicy.GetName())
		} else {
			// failed to get the policy
			log.Error(err, "failed to get the policy")
			return err
		}
	} else {
		// there is an existing policy
		// update if they are equal, update the existing policy if they are not equal
		if !equality.Semantic.DeepDerivative(smcpDiscoveryPolicy.Spec, foundsmcpDiscoveryPolicy.Spec) {
			log.Info("found difference between new policy and existing policy, updating...")
			// updated the policy
			smcpDiscoveryPolicy.ObjectMeta.ResourceVersion = foundsmcpDiscoveryPolicy.ObjectMeta.ResourceVersion
			if err := pc.Client.Update(context.TODO(), smcpDiscoveryPolicy); err != nil {
				// failed to update the policy
				log.Error(err, "failed to update the policy")
				return err
			}
			log.Info("updated the policy", "policy", smcpDiscoveryPolicy.GetName())
		} else {
			log.Info("new policy and existing policy are the same, no action needed.")
		}
	}

	// wait for context done signal
	<-ctx.Done()

	return nil
}
