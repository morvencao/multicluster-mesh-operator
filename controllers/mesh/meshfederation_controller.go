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
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	constants "github.com/morvencao/multicluster-mesh/pkg/constants"
	graphql "github.com/morvencao/multicluster-mesh/pkg/graphql"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	meshv1alpha1 "github.com/morvencao/multicluster-mesh/apis/mesh/v1alpha1"
	maistrafederationv1 "maistra.io/api/federation/v1"
	configpolicyv1 "open-cluster-management.io/config-policy-controller/api/v1"
	policyv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"
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

	meshUpdated := false
	meshPeers := meshFederation.Spec.MeshPeers
	for _, meshPeer := range meshPeers {
		peers := meshPeer.Peers
		if peers == nil || len(peers) != 2 || peers[0] == peers[1] || peers[0] == "" || peers[1] == "" {
			err := fmt.Errorf("two different meshes must specified in peers")
			return ctrl.Result{}, err
		}
		var err error
		meshUpdated, err = r.federateMeshPeers(peers, log)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if meshUpdated {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// create ServiceMeshPeer anc Configmap for CA
	for _, meshPeer := range meshPeers {
		peers := meshPeer.Peers
		if peers == nil || len(peers) != 2 || peers[0] == peers[1] || peers[0] == "" || peers[1] == "" {
			err := fmt.Errorf("two different meshes must specified in peers")
			return ctrl.Result{}, err
		}
		err := r.connectMeshPeers(meshFederation, peers, log)
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

func (r *MeshFederationReconciler) federateMeshPeers(peers []string, log logr.Logger) (bool, error) {
	meshUpdated := false
	mesh1, mesh2 := &meshv1alpha1.Mesh{}, &meshv1alpha1.Mesh{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      peers[0],
		Namespace: constants.ACMNamespace,
	}, mesh1)
	if err != nil {
		return false, err
	}

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      peers[1],
		Namespace: constants.ACMNamespace,
	}, mesh2)
	if err != nil {
		return false, err
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
		// only update mesh if needed
		if err := r.Client.Update(context.TODO(), mesh1); err != nil {
			// failed to update the mesh
			log.Error(err, "failed to update the mesh", "mesh name", mesh1.GetName())
			return false, err
		}
		meshUpdated = true
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
		// only update mesh if needed
		if err := r.Client.Update(context.TODO(), mesh2); err != nil {
			// failed to update the mesh
			log.Error(err, "failed to update the mesh", "mesh name", mesh2.GetName())
			return false, err
		}
		meshUpdated = true
	}

	return meshUpdated, nil
}

func (r *MeshFederationReconciler) connectMeshPeers(meshFederation *meshv1alpha1.MeshFederation, peers []string, log logr.Logger) error {
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

	mesh1CAConfigMapJson, err := graphql.QueryK8sResource("v1", "ConfigMap", constants.IstioCAConfigmapName, mesh1.Spec.ControlPlane.Namespace, mesh1.Spec.Cluster)
	if err != nil {
		log.Error(err, "failed to get the Configmap from graphql", "name", constants.IstioCAConfigmapName, "namespace", mesh1.Spec.ControlPlane.Namespace, "cluster", mesh1.Spec.Cluster)
		return err
	}

	mesh1CAConfigMap := &corev1.ConfigMap{}
	if err = json.Unmarshal(mesh1CAConfigMapJson, mesh1CAConfigMap); err != nil {
		return err
	}

	mesh1CA, ok := mesh1CAConfigMap.Data[constants.IstioCAConfigmapKey]
	if !ok {
		return fmt.Errorf("invalid in configmap %s", mesh1CAConfigMap.GetName())
	}

	mesh2CAConfigMapJson, err := graphql.QueryK8sResource("v1", "ConfigMap", constants.IstioCAConfigmapName, mesh2.Spec.ControlPlane.Namespace, mesh2.Spec.Cluster)
	if err != nil {
		log.Error(err, "failed to get the Configmap from graphql", "name", constants.IstioCAConfigmapName, "namespace", mesh2.Spec.ControlPlane.Namespace, "cluster", mesh2.Spec.Cluster)
		return err
	}

	mesh2CAConfigMap := &corev1.ConfigMap{}
	if err = json.Unmarshal(mesh2CAConfigMapJson, mesh2CAConfigMap); err != nil {
		return err
	}

	mesh2CA, ok := mesh2CAConfigMap.Data[constants.IstioCAConfigmapKey]
	if !ok {
		return fmt.Errorf("invalid in configmap %s", mesh2CAConfigMap.GetName())
	}

	mesh2IngressSvcInMesh1Json, err := graphql.QueryK8sResource("v1", "Service", mesh2.GetName()+"-ingress", mesh1.Spec.ControlPlane.Namespace, mesh1.Spec.Cluster)
	if err != nil {
		log.Error(err, "failed to get the Service from graphql", "name", mesh2.GetName()+"-ingress", "namespace", mesh1.Spec.ControlPlane.Namespace, "cluster", mesh1.Spec.Cluster)
		return err
	}

	mesh2IngressSvcInMesh1 := &corev1.Service{}
	if err = json.Unmarshal(mesh2IngressSvcInMesh1Json, mesh2IngressSvcInMesh1); err != nil {
		return err
	}

	if len(mesh2IngressSvcInMesh1.Status.LoadBalancer.Ingress) <= 0 {
		return fmt.Errorf("no public ingress address found in service %s", mesh2IngressSvcInMesh1.GetName())
	}

	mesh2IngressAddressInMesh1 := ""
	if mesh2IngressSvcInMesh1.Status.LoadBalancer.Ingress[0].IP != "" {
		mesh2IngressAddressInMesh1 = mesh2IngressSvcInMesh1.Status.LoadBalancer.Ingress[0].IP
	} else if mesh2IngressSvcInMesh1.Status.LoadBalancer.Ingress[0].Hostname != "" {
		mesh2IngressAddressInMesh1 = mesh2IngressSvcInMesh1.Status.LoadBalancer.Ingress[0].Hostname
	} else {
		return fmt.Errorf("no public IP or hostname found in service %s", mesh2IngressSvcInMesh1.GetName())
	}

	mesh1IngressSvcInMesh2Json, err := graphql.QueryK8sResource("v1", "Service", mesh1.GetName()+"-ingress", mesh2.Spec.ControlPlane.Namespace, mesh2.Spec.Cluster)
	if err != nil {
		log.Error(err, "failed to get the Service from graphql", "name", mesh1.GetName()+"-ingress", "namespace", mesh2.Spec.ControlPlane.Namespace, "cluster", mesh2.Spec.Cluster)
		return err
	}

	mesh1IngressSvcInMesh2 := &corev1.Service{}
	if err = json.Unmarshal(mesh1IngressSvcInMesh2Json, mesh1IngressSvcInMesh2); err != nil {
		return err
	}

	if len(mesh1IngressSvcInMesh2.Status.LoadBalancer.Ingress) <= 0 {
		return fmt.Errorf("no public ingress address found in service %s", mesh1IngressSvcInMesh2.GetName())
	}

	mesh1IngressAddressInMesh2 := ""
	if mesh1IngressSvcInMesh2.Status.LoadBalancer.Ingress[0].IP != "" {
		mesh1IngressAddressInMesh2 = mesh1IngressSvcInMesh2.Status.LoadBalancer.Ingress[0].IP
	} else if mesh1IngressSvcInMesh2.Status.LoadBalancer.Ingress[0].Hostname != "" {
		mesh1IngressAddressInMesh2 = mesh1IngressSvcInMesh2.Status.LoadBalancer.Ingress[0].Hostname
	} else {
		return fmt.Errorf("no public IP or hostname found in service %s", mesh1IngressSvcInMesh2.GetName())
	}

	newMesh2CAConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh2.GetName() + "-ca-root-cert",
			Namespace: mesh1.Spec.ControlPlane.Namespace,
		},
		Data: map[string]string{
			constants.IstioCAConfigmapKey: mesh2CA,
		},
	}

	mesh1ToMesh2Peer := &maistrafederationv1.ServiceMeshPeer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "federation.maistra.io/v1",
			Kind:       "ServiceMeshPeer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh2.GetName(),
			Namespace: mesh1.Spec.ControlPlane.Namespace,
		},
		Spec: maistrafederationv1.ServiceMeshPeerSpec{
			Remote: maistrafederationv1.ServiceMeshPeerRemote{
				Addresses:     []string{mesh1IngressAddressInMesh2},
				DiscoveryPort: 8188,
				ServicePort:   15443,
			},
			Gateways: maistrafederationv1.ServiceMeshPeerGateways{
				Ingress: corev1.LocalObjectReference{
					Name: mesh2.GetName() + "-ingress",
				},
				Egress: corev1.LocalObjectReference{
					Name: mesh2.GetName() + "-egress",
				},
			},
			Security: maistrafederationv1.ServiceMeshPeerSecurity{
				ClientID:    mesh2.Spec.TrustDomain + "/ns/" + mesh2.Spec.ControlPlane.Namespace + "/sa/" + mesh1.GetName() + "-egress-service-account",
				TrustDomain: mesh2.Spec.TrustDomain,
				CertificateChain: corev1.TypedLocalObjectReference{
					Kind: "ConfigMap",
					Name: mesh2.GetName() + "-ca-root-cert",
				},
			},
		},
	}

	newMesh1CAConfigMap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh1.GetName() + "-ca-root-cert",
			Namespace: mesh2.Spec.ControlPlane.Namespace,
		},
		Data: map[string]string{
			constants.IstioCAConfigmapKey: mesh1CA,
		},
	}

	mesh2ToMesh1Peer := &maistrafederationv1.ServiceMeshPeer{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "federation.maistra.io/v1",
			Kind:       "ServiceMeshPeer",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh1.GetName(),
			Namespace: mesh2.Spec.ControlPlane.Namespace,
		},
		Spec: maistrafederationv1.ServiceMeshPeerSpec{
			Remote: maistrafederationv1.ServiceMeshPeerRemote{
				Addresses:     []string{mesh2IngressAddressInMesh1},
				DiscoveryPort: 8188,
				ServicePort:   15443,
			},
			Gateways: maistrafederationv1.ServiceMeshPeerGateways{
				Ingress: corev1.LocalObjectReference{
					Name: mesh1.GetName() + "-ingress",
				},
				Egress: corev1.LocalObjectReference{
					Name: mesh1.GetName() + "-egress",
				},
			},
			Security: maistrafederationv1.ServiceMeshPeerSecurity{
				ClientID:    mesh1.Spec.TrustDomain + "/ns/" + mesh1.Spec.ControlPlane.Namespace + "/sa/" + mesh2.GetName() + "-egress-service-account",
				TrustDomain: mesh1.Spec.TrustDomain,
				CertificateChain: corev1.TypedLocalObjectReference{
					Kind: "ConfigMap",
					Name: mesh1.GetName() + "-ca-root-cert",
				},
			},
		},
	}

	// build enforce policy for mesh1 to mesh2
	mesh1ToMesh2EnforcePolicy := &policyv1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh1.GetName() + "-" + mesh2.GetName(),
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
								Name: mesh1.GetName() + "-" + mesh2.GetName(),
							},
							Spec: configpolicyv1.ConfigurationPolicySpec{
								Severity:          configpolicyv1.Severity("low"),
								RemediationAction: configpolicyv1.Enforce,
								NamespaceSelector: configpolicyv1.Target{
									Include: []configpolicyv1.NonEmptyString{configpolicyv1.NonEmptyString(mesh1.Spec.ControlPlane.Namespace)},
								},
								ObjectTemplates: []*configpolicyv1.ObjectTemplate{
									&configpolicyv1.ObjectTemplate{
										ComplianceType: configpolicyv1.MustHave,
										ObjectDefinition: runtime.RawExtension{
											Object: newMesh2CAConfigMap,
										},
									},
									&configpolicyv1.ObjectTemplate{
										ComplianceType: configpolicyv1.MustHave,
										ObjectDefinition: runtime.RawExtension{
											Object: mesh1ToMesh2Peer,
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

	mesh1ToMesh2EnforcePlacementRule := &placementrulev1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh1.GetName() + "-" + mesh2.GetName(),
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
						Name: mesh1.Spec.Cluster,
					},
				},
			},
		},
	}

	mesh1ToMesh2EnforcePlacementBinding := &policyv1.PlacementBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh1.GetName() + "-" + mesh2.GetName(),
			Namespace: constants.ACMNamespace,
		},
		PlacementRef: policyv1.PlacementSubject{
			Name:     mesh1.GetName() + "-" + mesh2.GetName(),
			Kind:     "PlacementRule",
			APIGroup: placementrulev1.SchemeGroupVersion.Group,
		},
		Subjects: []policyv1.Subject{
			policyv1.Subject{
				Name:     mesh1.GetName() + "-" + mesh2.GetName(),
				Kind:     policyv1.Kind,
				APIGroup: policyv1.SchemeGroupVersion.Group,
			},
		},
	}

	if err := controllerutil.SetControllerReference(meshFederation, mesh1ToMesh2EnforcePolicy, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", mesh1ToMesh2EnforcePolicy.GetName())
	}

	if err := controllerutil.SetControllerReference(meshFederation, mesh1ToMesh2EnforcePlacementRule, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", mesh1ToMesh2EnforcePlacementRule.GetName())
	}

	if err := controllerutil.SetControllerReference(meshFederation, mesh1ToMesh2EnforcePlacementBinding, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", mesh1ToMesh2EnforcePlacementBinding.GetName())
	}

	// create or update the placementRule
	foundMesh1ToMesh2EnforcePlacementRule := &placementrulev1.PlacementRule{}
	err = r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      mesh1ToMesh2EnforcePlacementRule.GetName(),
			Namespace: mesh1ToMesh2EnforcePlacementRule.GetNamespace(),
		},
		foundMesh1ToMesh2EnforcePlacementRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new placementRule
			if err := r.Client.Create(context.TODO(), mesh1ToMesh2EnforcePlacementRule); err != nil {
				log.Error(err, "failed to create the placementRule")
				return err
			}
			log.Info("created the placementRule", "placementRule", mesh1ToMesh2EnforcePlacementRule.GetName())
		} else {
			// failed to get the placementRule
			log.Error(err, "failed to get the placementRule")
			return err
		}
	} else {
		// there is an existing placementRule
		// update if they are equal, update the existing placementRule if they are not equal
		if !equality.Semantic.DeepDerivative(mesh1ToMesh2EnforcePlacementRule.Spec, foundMesh1ToMesh2EnforcePlacementRule.Spec) {
			log.Info("found difference between new placementRule and existing placementRule, updating...")
			// updated the placementRule
			mesh1ToMesh2EnforcePlacementRule.ObjectMeta.ResourceVersion = foundMesh1ToMesh2EnforcePlacementRule.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), mesh1ToMesh2EnforcePlacementRule); err != nil {
				// failed to update the placementRule
				log.Error(err, "failed to update the placementRule")
				return err
			}
			log.Info("updated the placementRule", "placementRule", mesh1ToMesh2EnforcePlacementRule.GetName())
		} else {
			log.Info("new placementRule and existing placementRule are the same, no action needed.")
		}
	}

	// create or update placementBinding
	foundMesh1ToMesh2EnforcePlacementBinding := &policyv1.PlacementBinding{}
	err = r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      mesh1ToMesh2EnforcePlacementBinding.GetName(),
			Namespace: mesh1ToMesh2EnforcePlacementBinding.GetNamespace(),
		},
		foundMesh1ToMesh2EnforcePlacementBinding)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new placementBinding
			if err := r.Client.Create(context.TODO(), mesh1ToMesh2EnforcePlacementBinding); err != nil {
				log.Error(err, "failed to create the placementBinding")
				return err
			}
			log.Info("created the placementBinding", "placementBinding", mesh1ToMesh2EnforcePlacementBinding.GetName())
		} else {
			// failed to get the placementBinding
			log.Error(err, "failed to get the placementBinding")
			return err
		}
	} else {
		// there is an existing placementBinding
		// update if they are equal, update the existing placementBinding if they are not equal
		if !equality.Semantic.DeepDerivative(mesh1ToMesh2EnforcePlacementBinding.PlacementRef, foundMesh1ToMesh2EnforcePlacementBinding.PlacementRef) || !equality.Semantic.DeepDerivative(mesh1ToMesh2EnforcePlacementBinding.Subjects, foundMesh1ToMesh2EnforcePlacementBinding.Subjects) {
			log.Info("found difference between new placementBinding and existing placementBinding, updating...")
			// updated the placementBinding
			mesh1ToMesh2EnforcePlacementBinding.ObjectMeta.ResourceVersion = foundMesh1ToMesh2EnforcePlacementBinding.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), mesh1ToMesh2EnforcePlacementBinding); err != nil {
				// failed to update the placementBinding
				log.Error(err, "failed to update the placementBinding")
				return err
			}
			log.Info("updated the placementBinding", "placementBinding", mesh1ToMesh2EnforcePlacementBinding.GetName())
		} else {
			log.Info("new placementBinding and existing placementBinding are the same, no action needed.")
		}
	}

	// create or update policy
	foundMesh1ToMesh2EnforcePolicy := &policyv1.Policy{}
	err = r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      mesh1ToMesh2EnforcePolicy.GetName(),
			Namespace: mesh1ToMesh2EnforcePolicy.GetNamespace(),
		},
		foundMesh1ToMesh2EnforcePolicy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new policy
			if err := r.Client.Create(context.TODO(), mesh1ToMesh2EnforcePolicy); err != nil {
				log.Error(err, "failed to create the policy")
				return err
			}
			log.Info("created the policy", "policy", mesh1ToMesh2EnforcePolicy.GetName())
		} else {
			// failed to get the policy
			log.Error(err, "failed to get the policy")
			return err
		}
	} else {
		// there is an existing policy
		// update if they are equal, update the existing policy if they are not equal
		if !equality.Semantic.DeepDerivative(mesh1ToMesh2EnforcePolicy.Spec, foundMesh1ToMesh2EnforcePolicy.Spec) {
			log.Info("found difference between new policy and existing policy, updating...")
			// updated the policy
			mesh1ToMesh2EnforcePolicy.ObjectMeta.ResourceVersion = foundMesh1ToMesh2EnforcePolicy.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), mesh1ToMesh2EnforcePolicy); err != nil {
				// failed to update the policy
				log.Error(err, "failed to update the policy")
				return err
			}
			log.Info("updated the policy", "policy", mesh1ToMesh2EnforcePolicy.GetName())
		} else {
			log.Info("new policy and existing policy are the same, no action needed.")
		}
	}

	// build enforce policy for mesh2 to mesh1
	mesh2ToMesh1EnforcePolicy := &policyv1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh2.GetName() + "-" + mesh1.GetName(),
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
								Name: mesh2.GetName() + "-" + mesh1.GetName(),
							},
							Spec: configpolicyv1.ConfigurationPolicySpec{
								Severity:          configpolicyv1.Severity("low"),
								RemediationAction: configpolicyv1.Enforce,
								NamespaceSelector: configpolicyv1.Target{
									Include: []configpolicyv1.NonEmptyString{configpolicyv1.NonEmptyString(mesh2.Spec.ControlPlane.Namespace)},
								},
								ObjectTemplates: []*configpolicyv1.ObjectTemplate{
									&configpolicyv1.ObjectTemplate{
										ComplianceType: configpolicyv1.MustHave,
										ObjectDefinition: runtime.RawExtension{
											Object: newMesh1CAConfigMap,
										},
									},
									&configpolicyv1.ObjectTemplate{
										ComplianceType: configpolicyv1.MustHave,
										ObjectDefinition: runtime.RawExtension{
											Object: mesh2ToMesh1Peer,
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

	mesh2ToMesh1EnforcePlacementRule := &placementrulev1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh2.GetName() + "-" + mesh1.GetName(),
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
						Name: mesh2.Spec.Cluster,
					},
				},
			},
		},
	}

	mesh2ToMesh1EnforcePlacementBinding := &policyv1.PlacementBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh2.GetName() + "-" + mesh1.GetName(),
			Namespace: constants.ACMNamespace,
		},
		PlacementRef: policyv1.PlacementSubject{
			Name:     mesh2.GetName() + "-" + mesh1.GetName(),
			Kind:     "PlacementRule",
			APIGroup: placementrulev1.SchemeGroupVersion.Group,
		},
		Subjects: []policyv1.Subject{
			policyv1.Subject{
				Name:     mesh2.GetName() + "-" + mesh1.GetName(),
				Kind:     policyv1.Kind,
				APIGroup: policyv1.SchemeGroupVersion.Group,
			},
		},
	}

	if err := controllerutil.SetControllerReference(meshFederation, mesh2ToMesh1EnforcePolicy, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", mesh2ToMesh1EnforcePolicy.GetName())
	}

	if err := controllerutil.SetControllerReference(meshFederation, mesh2ToMesh1EnforcePlacementRule, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", mesh2ToMesh1EnforcePlacementRule.GetName())
	}

	if err := controllerutil.SetControllerReference(meshFederation, mesh2ToMesh1EnforcePlacementBinding, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference", "name", mesh2ToMesh1EnforcePlacementBinding.GetName())
	}

	// create or update the placementRule
	foundMesh2ToMesh1EnforcePlacementRule := &placementrulev1.PlacementRule{}
	err = r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      mesh2ToMesh1EnforcePlacementRule.GetName(),
			Namespace: mesh2ToMesh1EnforcePlacementRule.GetNamespace(),
		},
		foundMesh2ToMesh1EnforcePlacementRule)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new placementRule
			if err := r.Client.Create(context.TODO(), mesh2ToMesh1EnforcePlacementRule); err != nil {
				log.Error(err, "failed to create the placementRule")
				return err
			}
			log.Info("created the placementRule", "placementRule", mesh2ToMesh1EnforcePlacementRule.GetName())
		} else {
			// failed to get the placementRule
			log.Error(err, "failed to get the placementRule")
			return err
		}
	} else {
		// there is an existing placementRule
		// update if they are equal, update the existing placementRule if they are not equal
		if !equality.Semantic.DeepDerivative(mesh2ToMesh1EnforcePlacementRule.Spec, foundMesh2ToMesh1EnforcePlacementRule.Spec) {
			log.Info("found difference between new placementRule and existing placementRule, updating...")
			// updated the placementRule
			mesh2ToMesh1EnforcePlacementRule.ObjectMeta.ResourceVersion = foundMesh2ToMesh1EnforcePlacementRule.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), mesh2ToMesh1EnforcePlacementRule); err != nil {
				// failed to update the placementRule
				log.Error(err, "failed to update the placementRule")
				return err
			}
			log.Info("updated the placementRule", "placementRule", mesh2ToMesh1EnforcePlacementRule.GetName())
		} else {
			log.Info("new placementRule and existing placementRule are the same, no action needed.")
		}
	}

	// create or update placementBinding
	foundMesh2ToMesh1EnforcePlacementBinding := &policyv1.PlacementBinding{}
	err = r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      mesh2ToMesh1EnforcePlacementBinding.GetName(),
			Namespace: mesh2ToMesh1EnforcePlacementBinding.GetNamespace(),
		},
		foundMesh2ToMesh1EnforcePlacementBinding)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new placementBinding
			if err := r.Client.Create(context.TODO(), mesh2ToMesh1EnforcePlacementBinding); err != nil {
				log.Error(err, "failed to create the placementBinding")
				return err
			}
			log.Info("created the placementBinding", "placementBinding", mesh2ToMesh1EnforcePlacementBinding.GetName())
		} else {
			// failed to get the placementBinding
			log.Error(err, "failed to get the placementBinding")
			return err
		}
	} else {
		// there is an existing placementBinding
		// update if they are equal, update the existing placementBinding if they are not equal
		if !equality.Semantic.DeepDerivative(mesh2ToMesh1EnforcePlacementBinding.PlacementRef, foundMesh2ToMesh1EnforcePlacementBinding.PlacementRef) || !equality.Semantic.DeepDerivative(mesh2ToMesh1EnforcePlacementBinding.Subjects, foundMesh2ToMesh1EnforcePlacementBinding.Subjects) {
			log.Info("found difference between new placementBinding and existing placementBinding, updating...")
			// updated the placementBinding
			mesh2ToMesh1EnforcePlacementBinding.ObjectMeta.ResourceVersion = foundMesh2ToMesh1EnforcePlacementBinding.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), mesh2ToMesh1EnforcePlacementBinding); err != nil {
				// failed to update the placementBinding
				log.Error(err, "failed to update the placementBinding")
				return err
			}
			log.Info("updated the placementBinding", "placementBinding", mesh2ToMesh1EnforcePlacementBinding.GetName())
		} else {
			log.Info("new placementBinding and existing placementBinding are the same, no action needed.")
		}
	}

	// create or update policy
	foundMesh2ToMesh1EnforcePolicy := &policyv1.Policy{}
	err = r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      mesh2ToMesh1EnforcePolicy.GetName(),
			Namespace: mesh2ToMesh1EnforcePolicy.GetNamespace(),
		},
		foundMesh2ToMesh1EnforcePolicy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create new policy
			if err := r.Client.Create(context.TODO(), mesh2ToMesh1EnforcePolicy); err != nil {
				log.Error(err, "failed to create the policy")
				return err
			}
			log.Info("created the policy", "policy", mesh2ToMesh1EnforcePolicy.GetName())
		} else {
			// failed to get the policy
			log.Error(err, "failed to get the policy")
			return err
		}
	} else {
		// there is an existing policy
		// update if they are equal, update the existing policy if they are not equal
		if !equality.Semantic.DeepDerivative(mesh2ToMesh1EnforcePolicy.Spec, foundMesh2ToMesh1EnforcePolicy.Spec) {
			log.Info("found difference between new policy and existing policy, updating...")
			// updated the policy
			mesh2ToMesh1EnforcePolicy.ObjectMeta.ResourceVersion = foundMesh2ToMesh1EnforcePolicy.ObjectMeta.ResourceVersion
			if err := r.Client.Update(context.TODO(), mesh2ToMesh1EnforcePolicy); err != nil {
				// failed to update the policy
				log.Error(err, "failed to update the policy")
				return err
			}
			log.Info("updated the policy", "policy", mesh2ToMesh1EnforcePolicy.GetName())
		} else {
			log.Info("new policy and existing policy are the same, no action needed.")
		}
	}

	return nil
}
