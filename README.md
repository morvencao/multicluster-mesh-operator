# multicluster-mesh

Service mesh used in multicluster scenario.

## How to use

1. Create a default mesh in managed cluster `mcsmtest1`:

```bash
cat << EOF | oc --context=${mcsmtest1} apply -f -
apiVersion: maistra.io/v2
kind: ServiceMeshControlPlane
metadata:
  name: basic
  namespace: default
spec:
  version: v2.1
  profiles:
  - default
  gateways:
    ingress:
      enabled: false
    egress:
      enabled: false
  security:
    trust:
      domain: cluster.local
---
apiVersion: maistra.io/v1
kind: ServiceMeshMemberRoll
metadata:
  name: default
  namespace: default
spec:
  members:
    - istio-apps
---
apiVersion: v1
kind: Namespace
metadata:
  name: istio-apps
EOF
```

2. Deploy the multicluster-mesh in hub cluster:

```bash
make install
oc apply -f deploy/deploy.yaml
```

3. Check the discoveried mesh from hub cluster:

```bash
# oc get mesh -A
NAMESPACE                 NAME                           AGE
open-cluster-management   mcsmtest1-default-basic        53s
```

4. Configure a new mesh `mesh1` to managed cluster `mcsmtest2`:

```bash
cat << EOF | oc apply -f -
apiVersion: mesh.open-cluster-management.io/v1alpha1
kind: Mesh
metadata:
  name: mesh1
  namespace: open-cluster-management
spec:
  cluster: mcsmtest1
  controlPlane:
    components:
    - grafana
    - istio-discovery
    - istio-egress
    - istio-ingress
    - kiali
    - mesh-config
    - prometheus
    - telemetry-common
    - tracing
    namespace: mesh1-system
    profiles:
    - default
    version: v2.1
  meshMemberRoll:
  - mesh1-bookinfo
  meshProvider: Openshift Service Mesh
  trustDomain: mesh1.local
EOF
```

5. Configure a new mesh `mesh2` to managed cluster `mcsmtest2`:

```bash
cat << EOF | oc apply -f -
apiVersion: mesh.open-cluster-management.io/v1alpha1
kind: Mesh
metadata:
  name: mesh2
  namespace: open-cluster-management
spec:
  cluster: mcsmtest2
  controlPlane:
    components:
    - grafana
    - istio-discovery
    - istio-egress
    - istio-ingress
    - kiali
    - mesh-config
    - prometheus
    - telemetry-common
    - tracing
    namespace: mesh2-system
    profiles:
    - default
    version: v2.1
  meshMemberRoll:
  - mesh2-bookinfo
  meshProvider: Openshift Service Mesh
  trustDomain: mesh2.local
EOF
```

6. Federate `mesh1` and `mesh2` by creating `MeshFederation` in hub cluster:

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
    - mesh1
    - mesh2
  trustConfig:
    trustType: Limited
EOF
```

7. Install bookinfo(productpage,details,reviews-v1) in `mesh1`:

```bash
oc apply -n mesh1-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l 'app in (productpage,details)'
oc apply -n mesh1-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l app=reviews,version=v1
oc apply -n mesh1-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l service=reviews
oc apply -n mesh1-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l 'account'
oc apply -n mesh1-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/networking/bookinfo-gateway.yaml
```

8. Install bookinfo(reviews-v2, reviews-v3, ratings) in `mesh2`:

```bash
oc apply -n mesh2-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l app=reviews,version=v2
oc apply -n mesh2-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l app=reviews,version=v3
oc apply -n mesh2-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l service=reviews
oc apply -n mesh2-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l app=ratings
oc apply -n mesh2-bookinfo -f https://raw.githubusercontent.com/maistra/istio/maistra-2.1/samples/bookinfo/platform/kube/bookinfo.yaml -l 'account'
```

9. Export services(reviews and ratings) from `mesh2`:

```bash
cat << EOF | oc apply -f -
apiVersion: federation.maistra.io/v1
kind: ExportedServiceSet
metadata:
  name: mesh1
  namespace: mesh2-system
spec:
  exportRules:
  - type: NameSelector
    nameSelector:
      namespace: mesh2-bookinfo
      name: reviews
      alias:
        namespace: bookinfo
        name: reviews
  - type: NameSelector
    nameSelector:
      namespace: mesh2-bookinfo
      name: ratings
      alias:
        namespace: bookinfo
        name: ratings
EOF
```

10. Import services to `mesh1`:

```bash
cat << EOF | oc apply -f -
apiVersion: federation.maistra.io/v1
kind: ImportedServiceSet
metadata:
  name: mesh2
  namespace: mesh1-system
spec:
  importRules:
    - type: NameSelector
      importAsLocal: true
      nameSelector:
        namespace: bookinfo
        alias:
          # services will be imported as <name>.mesh1-bookinfo.svc.mesh2-imports.local
          namespace: mesh1-bookinfo
EOF
```

11. Access the bookinfo from browser with the following address:

```bash
echo http://$(oc -n mesh1-system get route istio-ingressgateway -o jsonpath={.spec.host})/productpage
```

