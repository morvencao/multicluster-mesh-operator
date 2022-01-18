package translate

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	constants "github.com/morvencao/multicluster-mesh/pkg/constants"

	meshv1alpha1 "github.com/morvencao/multicluster-mesh/apis/mesh/v1alpha1"
	maistrav1 "maistra.io/api/core/v1"
	maistrav2 "maistra.io/api/core/v2"
)

func TranslateToMesh(smcpJson, smmrJson []byte, cluster string) (*meshv1alpha1.Mesh, error) {
	smcp := &maistrav2.ServiceMeshControlPlane{}
	err := json.Unmarshal(smcpJson, smcp)
	if err != nil {
		return nil, err
	}

	smmr := &maistrav1.ServiceMeshMemberRoll{}
	err = json.Unmarshal(smmrJson, smmr)
	if err != nil {
		return nil, err
	}

	trustDomain := "cluster.local"
	if smcp.Spec.Security != nil && smcp.Spec.Security.Trust != nil && smcp.Spec.Security.Trust.Domain != "" {
		trustDomain = smcp.Spec.Security.Trust.Domain
	}

	allComponents := make([]string, 2)
	for _, v := range smcp.Status.Readiness.Components {
		if len(v) == 1 && v[0] == "" {
			continue
		}
		allComponents = append(allComponents, v...)
	}

	meshName := cluster + "-" + smcp.GetNamespace() + "-" + smcp.GetName()
	mesh := &meshv1alpha1.Mesh{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meshName,
			Namespace: constants.ACMNamespace,
		},
		Spec: meshv1alpha1.MeshSpec{
			Cluster: cluster,
			ControlPlane: &meshv1alpha1.MeshControlPlane{
				Namespace:  smcp.GetNamespace(),
				Version:    smcp.Spec.Version,
				Components: allComponents,
			},
			MeshMemberRoll: smmr.Spec.Members,
			TrustDomain:    trustDomain,
		},
		Status: meshv1alpha1.MeshStatus{
			Readiness: smcp.Status.Readiness,
		},
	}

	return mesh, nil
}
