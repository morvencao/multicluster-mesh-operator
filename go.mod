module github.com/morvencao/multicluster-mesh

go 1.16

require (
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.22.1
	open-cluster-management.io/config-policy-controller v0.5.1-0.20211220212124-e08b41e77b46
	open-cluster-management.io/governance-policy-propagator v0.5.1-0.20211220161230-0e35d5833c52
	open-cluster-management.io/multicloud-operators-subscription v0.5.0
	sigs.k8s.io/controller-runtime v0.10.0
)

replace (
	github.com/go-openapi/spec => github.com/go-openapi/spec v0.19.5
	k8s.io/api => k8s.io/api v0.22.1
	k8s.io/client-go => k8s.io/client-go v0.22.1
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
)
