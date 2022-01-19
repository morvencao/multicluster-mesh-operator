package translate

import (
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	constants "github.com/morvencao/multicluster-mesh/pkg/constants"

	meshv1alpha1 "github.com/morvencao/multicluster-mesh/apis/mesh/v1alpha1"
	maistrav1 "maistra.io/api/core/v1"
	maistrav2 "maistra.io/api/core/v2"
)

var profileComponentsMap map[string][]string

func init() {
	profileComponentsMap = make(map[string][]string)
	profileComponentsMap["default"] = []string{"grafana", "istio-discovery", "istio-egress", "istio-ingress", "kiali", "mesh-config", "prometheus", "telemetry-common", "tracing"}
}

func TranslateToLogicMesh(smcpJson, smmrJson []byte, cluster string) (*meshv1alpha1.Mesh, error) {
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
			Existing: true,
			Cluster:  cluster,
			ControlPlane: &meshv1alpha1.MeshControlPlane{
				Namespace:  smcp.GetNamespace(),
				Profiles:   smcp.Spec.Profiles,
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

func TranslateToPhysicalMesh(mesh *meshv1alpha1.Mesh) (*maistrav2.ServiceMeshControlPlane, *maistrav1.ServiceMeshMemberRoll, string, error) {
	if mesh.Spec.Cluster == "" {
		return nil, nil, "", fmt.Errorf("cluster field in mesh object is empty")
	}
	cluster := mesh.Spec.Cluster
	if mesh.Spec.ControlPlane == nil {
		return nil, nil, "", fmt.Errorf("controlPlane field in mesh object is empty")
	}
	if mesh.Spec.ControlPlane.Namespace == "" {
		return nil, nil, "", fmt.Errorf("controlPlane namespace field in mesh object is empty")
	}
	namespace := mesh.Spec.ControlPlane.Namespace
	version := "v2.0"
	if mesh.Spec.ControlPlane.Version != "" {
		version = mesh.Spec.ControlPlane.Version
	}
	profiles := []string{"default"}
	if mesh.Spec.ControlPlane.Profiles != nil {
		profiles = mesh.Spec.ControlPlane.Profiles
	}

	addonEnabled := false
	gatewayEnabled := false
	var tracingConfig *maistrav2.TracingConfig
	var prometheusAddonConfig *maistrav2.PrometheusAddonConfig
	var stackdriverAddonConfig *maistrav2.StackdriverAddonConfig
	var jaegerAddonConfig *maistrav2.JaegerAddonConfig
	var grafanaAddonConfig *maistrav2.GrafanaAddonConfig
	var kialiAddonConfig *maistrav2.KialiAddonConfig
	var threeScaleAddonConfig *maistrav2.ThreeScaleAddonConfig
	var ingressGatewayConfig *maistrav2.ClusterIngressGatewayConfig
	var egressGatewayConfig *maistrav2.EgressGatewayConfig
	// var additionalIngressGatewayConfig map[string]*maistrav2.IngressGatewayConfig
	// var additionalEgressGatewayConfig map[string]*maistrav2.EgressGatewayConfig

	if mesh.Spec.ControlPlane.Components != nil {
		for _, c := range profiles {
			for _, p := range profiles {
				if sliceContainsString(profileComponentsMap[p], c) {
					continue
				}
				switch c {
				case "tracing":
					tracingConfig = &maistrav2.TracingConfig{
						Type: maistrav2.TracerTypeJaeger,
					}
				case "prometheus":
					prometheusAddonConfig = &maistrav2.PrometheusAddonConfig{
						Enablement: maistrav2.Enablement{
							Enabled: &[]bool{true}[0],
						},
					}
					addonEnabled = true
				case "grafana":
					grafanaAddonConfig = &maistrav2.GrafanaAddonConfig{
						Enablement: maistrav2.Enablement{
							Enabled: &[]bool{true}[0],
						},
					}
					addonEnabled = true
				case "kiali":
					kialiAddonConfig = &maistrav2.KialiAddonConfig{
						Enablement: maistrav2.Enablement{
							Enabled: &[]bool{true}[0],
						},
					}
					addonEnabled = true
				case "3scale":
					threeScaleAddonConfig = &maistrav2.ThreeScaleAddonConfig{
						Enablement: maistrav2.Enablement{
							Enabled: &[]bool{true}[0],
						},
					}
					addonEnabled = true
				case "istio-ingress":
					ingressGatewayConfig = &maistrav2.ClusterIngressGatewayConfig{
						IngressGatewayConfig: maistrav2.IngressGatewayConfig{
							GatewayConfig: maistrav2.GatewayConfig{
								Enablement: maistrav2.Enablement{
									Enabled: &[]bool{true}[0],
								},
							},
						},
					}
					gatewayEnabled = true
				case "istio-egress":
					egressGatewayConfig = &maistrav2.EgressGatewayConfig{
						GatewayConfig: maistrav2.GatewayConfig{
							Enablement: maistrav2.Enablement{
								Enabled: &[]bool{true}[0],
							},
						},
					}
					gatewayEnabled = true
				}
			}
		}
	}

	// default smcp
	smcp := &maistrav2.ServiceMeshControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mesh.GetName(),
			Namespace: namespace,
		},
		Spec: maistrav2.ControlPlaneSpec{
			Version:  version,
			Profiles: profiles,
		},
	}

	// set addons
	if addonEnabled {
		addonsConfig := &maistrav2.AddonsConfig{}
		if prometheusAddonConfig != nil {
			addonsConfig.Prometheus = prometheusAddonConfig
		}
		if stackdriverAddonConfig != nil {
			addonsConfig.Stackdriver = stackdriverAddonConfig
		}
		if jaegerAddonConfig != nil {
			addonsConfig.Jaeger = jaegerAddonConfig
		}
		if grafanaAddonConfig != nil {
			addonsConfig.Grafana = grafanaAddonConfig
		}
		if kialiAddonConfig != nil {
			addonsConfig.Kiali = kialiAddonConfig
		}
		if threeScaleAddonConfig != nil {
			addonsConfig.ThreeScale = threeScaleAddonConfig
		}
		smcp.Spec.Addons = addonsConfig
	}

	// set gateways
	if gatewayEnabled {
		gatewaysConfig := &maistrav2.GatewaysConfig{
			Enablement: maistrav2.Enablement{
				Enabled: &[]bool{true}[0],
			},
		}
		if ingressGatewayConfig != nil {
			gatewaysConfig.ClusterIngress = ingressGatewayConfig
		}
		if egressGatewayConfig != nil {
			gatewaysConfig.ClusterEgress = egressGatewayConfig
		}
		smcp.Spec.Gateways = gatewaysConfig
	}

	if tracingConfig != nil {
		smcp.Spec.Tracing = tracingConfig
	}

	// set trust domain
	if mesh.Spec.TrustDomain != "" {
		smcp.Spec.Security = &maistrav2.SecurityConfig{
			Trust: &maistrav2.TrustConfig{
				Domain: mesh.Spec.TrustDomain,
			},
		}
	}

	var smmr *maistrav1.ServiceMeshMemberRoll
	if len(mesh.Spec.MeshMemberRoll) > 0 {
		smmr = &maistrav1.ServiceMeshMemberRoll{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: namespace,
			},
			Spec: maistrav1.ServiceMeshMemberRollSpec{
				Members: mesh.Spec.MeshMemberRoll,
			},
		}
	}

	return smcp, smmr, cluster, nil
}

func sliceContainsString(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}
