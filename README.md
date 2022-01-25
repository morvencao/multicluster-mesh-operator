# Multicluster Mesh

multicluster-mesh is an enhanced service mesh operator applied in [Red Hat Advanced Cluster Management for Kubernetes](https://access.redhat.com/documentation/en-us/red_hat_advanced_cluster_management_for_kubernetes/). It is used to manages service meshes across multiple managed clusters and hybrid cloud providers.

## Core Concepts

1. Mesh - a `mesh` resource is mapping to a physical service mesh in a managed cluster, it contains the desired state and status of the backend service mesh. For each physical service mesh in a managed cluster, a `mesh` resource is created in hub cluster. An example of `mesh` resource would resemble the following yaml snippet:

```yaml
apiVersion: mesh.open-cluster-management.io/v1alpha1
kind: Mesh
metadata:
  name: mesh-sample
spec:
  clusters: managedcluster1
  controlPlane:
    components: ["istio-discovery", "istio-ingress", "mesh-config", "telemetry-common", "tracing"]
    namespace: istio-system
    profiles: ["default"]
    version: v2.1
  meshMemberRoll: ["istio-apps"]
  meshProvider: Openshift Service Mesh
  trustDomain: cluster.local
status:
  readiness:
    components:
      pending: []
      ready: ["istio-discovery", "istio-ingress", "mesh-config", "telemetry-common", "tracing"]
      unready: []
```

2. MeshDeployment - `meshdeployment` is used to deploy physical service meshes to managed cluster, it support deploy multiple physical service meshes to different managed clusters with one template. An example of `meshdeployment` resource would resemble the following yaml snippet:

```yaml
apiVersion: mesh.open-cluster-management.io/v1alpha1
kind: MeshDeployment
metadata:
  name: mesh
spec:
  clusters: ["managedcluster1", "managedcluster2"]
  controlPlane:
    components: ["prometheus", "istio-discovery", "istio-ingress", "mesh-config", "telemetry-common", "tracing"]
    namespace: mesh-system
    profiles: ["default"]
    version: v2.1
  meshMemberRoll: ["mesh-apps"]
  meshProvider: Openshift Service Mesh
  trustDomain: mesh.local
status:
  appliedMeshes: ["managedcluster1-mesh", "managedcluster2-mesh"]
```

3. MeshFederation - `meshfederation` resource is used to federate service meshes so that the physical service meshes located in different clusters to securely share and manage traffic between meshes while maintaining strong administrative boundaries in a multi-tenant environment. An example of `meshfederation` resource would resemble the following yaml snippet:

```yaml
apiVersion: mesh.open-cluster-management.io/v1alpha1
kind: MeshFederation
metadata:
  name: mcsm
spec:
  meshPeers:
  - peers:
    - managedcluster1-mesh
    - managedcluster1-mesh
  trustConfig:
    trustType: Limited
status:
  federatedMeshes:
  - peer:
    - managedcluster1-mesh
    - managedcluster1-mesh
```

## Getting Started

### Prerequisites

* Ensure [docker 17.03+](https://docs.docker.com/get-started) is installed.
* Ensure [golang 1.15+](https://golang.org/doc/install) is installed.
* Prepare an environment [Red Hat Advanced Cluster Management for Kubernetes](https://access.redhat.com/documentation/en-us/red_hat_advanced_cluster_management_for_kubernetes/) and login to the hub cluster with [oc](https://docs.openshift.com/container-platform/4.8/cli_reference/openshift_cli/getting-started-cli.html) command line tool.

1. Build and push docker image:

```bash
make docker-build docker-push IMG=quay.io/<your_quayio_username>/multicluster-mesh:latest
```

2. Install the multicluster-mesh to the hub cluster:

```bash
make deploy
```

3. If you have installed [Openshift Service Mesh](https://docs.openshift.com/container-platform/4.6/service_mesh/v2x/ossm-about.html) in a managed cluster, then you should find a `mesh` resource created in `open-cluster-management` namespace:

```bash
# oc get mesh -n open-cluster-management
NAME                                 CLUSTER           VERSION   PEERS   AGE
managedcluster1-istio-system-basic   managedcluster1   v2.1              20s
```

4. You can also deploy new service meshes to managed clusters, for example, creating the following `meshdeployment` resource to deploy new service meshes to managed cluster `managedcluster1` and `managedcluster2`:

```bash
cat << EOF | oc apply -f -
apiVersion: mesh.open-cluster-management.io/v1alpha1
kind: MeshDeployment
metadata:
  name: mesh
  namespace: open-cluster-management
spec:
  clusters: ["managedcluster1", "managedcluster2"]
  controlPlane:
    components: ["prometheus", "istio-discovery", "istio-ingress", "mesh-config", "telemetry-common", "tracing"]
    namespace: mesh-system
    profiles: ["default"]
    version: v2.1
  meshMemberRoll: ["mesh-apps"]
  meshProvider: Openshift Service Mesh
  trustDomain: mesh.local
EOF
```

5. Then verify the created service meshes:

```bash
# oc get mesh
NAME                                 CLUSTER           VERSION   PEERS   AGE
managedcluster1-istio-system-basic   managedcluster1   v2.1              59s
managedcluster1-mesh                 managedcluster1   v2.1              12s
managedcluster2-mesh                 managedcluster1   v2.1              12s
```

6. You can also federate `managedcluster1-mesh` and `managedcluster2-mesh` by creating `meshfederation` in hub cluster by the following command:

```bash
cat << EOF | oc apply -f -
apiVersion: mesh.open-cluster-management.io/v1alpha1
kind: MeshFederation
metadata:
  name: mcsm
  namespace: open-cluster-management
spec:
  meshPeers:
  - peers:
    - managedcluster1-mesh
    - managedcluster2-mesh
  trustConfig:
    trustType: Limited
EOF
```

7. To verify the meshes are federated, you can deploy part(productpage,details,reviews-v1) of the [bookinfo application](https://istio.io/latest/docs/examples/bookinfo/) in managed cluster `managedcluster1`:

_Note:_ currently the verify steps have to be executed in the managed cluster, we're working on the service discovery and service federation now.

```bash
oc create ns mesh-bookinfo
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l 'app in (productpage,details)'
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l app=reviews,version=v1
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l service=reviews
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l 'account'
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/networking/bookinfo-gateway.yaml
```

8. Then deploy the remaining part(reviews-v2, reviews-v3, ratings) of [bookinfo application](https://istio.io/latest/docs/examples/bookinfo/) in managed cluster `managedcluster2`:

```bash
oc create ns mesh-bookinfo
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l app=reviews,version=v2
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l app=reviews,version=v3
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l service=reviews
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l app=ratings
oc apply -n mesh-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l 'account'
```

9. Create `exportedserviceset` resource in managed cluster `managedcluster2` to export services(reviews and ratings) from `managedcluster2-mesh`:

```bash
cat << EOF | oc apply -f -
apiVersion: federation.maistra.io/v1
kind: ExportedServiceSet
metadata:
  name: managedcluster1-mesh
  namespace: mesh-system
spec:
  exportRules:
  - type: NameSelector
    nameSelector:
      namespace: mesh-bookinfo
      name: reviews
  - type: NameSelector
    nameSelector:
      namespace: mesh-bookinfo
      name: ratings
EOF
```

10. Create `importedserviceset` resource in managed cluster `managedcluster1` to import services(reviews and ratings) from `managedcluster1-mesh`:

```bash
cat << EOF | oc apply -f -
apiVersion: federation.maistra.io/v1
kind: ImportedServiceSet
metadata:
  name: managedcluster2-mesh
  namespace: mesh-system
spec:
  importRules:
    - type: NameSelector
      importAsLocal: true
      nameSelector:
        namespace: mesh-bookinfo
        name: reviews
        alias:
          namespace: mesh-bookinfo
    - type: NameSelector
      importAsLocal: true
      nameSelector:
        namespace: mesh-bookinfo
        name: ratings
        alias:
          namespace: mesh-bookinfo
EOF
```

11. Access the bookinfo from your browser with the following address from `managedcluster1` cluster:

```bash
echo http://$(oc -n mesh-system get route istio-ingressgateway -o jsonpath={.spec.host})/productpage
```

_Note_: The expected result is that by refreshing the page several times, you should see different versions of reviews shown in productpage, presented in a round robin style (red stars, black stars, no stars). Because reviews-v2, reviews-v3 and ratings service are running in another mesh, if you could see black stars and red stars reviews, then it means traffic across meshes are successfully routed.
