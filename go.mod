module github.com/morvencao/multicluster-mesh-operator

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.3.0 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/thedevsaddam/gojsonq/v2 v2.5.2
	golang.org/x/net v0.0.0-20220114011407-0dd24b26b47d // indirect
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.22.1
	maistra.io/api v0.0.0-20211119171546-348bbce3ca27
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
