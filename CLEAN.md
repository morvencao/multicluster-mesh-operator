## Clean

1. Clean up resources in hub cluster:

```bash
oc -n open-cluster-management delete meshfederation,meshdeployment,mesh --all
```

2. Scale the replicas of multicluster-mesh-operator to `0`:

```bash
oc -n open-cluster-management scale deploy/multicluster-mesh-operator --replicas 0
```

3. Verify the resources in hub clusters are creaned up:

```bash
oc get meshfederation,meshdeployment,mesh -A
oc get policy,placementrule,placementbinding -A
```

4. Clean up resources in managed clusters:

```bash
oc -n bookinfo delete vs,gw,dr,se --all
oc -n bookinfo delete deploy,svc,sa --all

oc -n istio-bookinfo delete vs,gw,dr,se --all
oc -n istio-bookinfo delete deploy,svc,sa --all

oc -n mesh-bookinfo delete vs,gw,dr,se --all
oc -n mesh-bookinfo delete deploy,svc,sa --all

oc -n mesh1-bookinfo delete vs,gw,dr,se --all
oc -n mesh1-bookinfo delete deploy,svc,sa --all

oc -n mesh2-bookinfo delete vs,gw,dr,se --all
oc -n mesh2-bookinfo delete deploy,svc,sa --all

oc -n istio-system delete ExportedServiceSet,ImportedServiceSet,servicemeshpeer,smcp,smmr --all
oc -n mesh-system delete ExportedServiceSet,ImportedServiceSet,servicemeshpeer,smcp,smmr --all
oc -n mesh1-system delete ExportedServiceSet,ImportedServiceSet,servicemeshpeer,smcp,smmr --all
oc -n mesh2-system delete ExportedServiceSet,ImportedServiceSet,servicemeshpeer,smcp,smmr --all

oc delete ns bookinfo istio-bookinfo mesh-bookinfo mesh1-bookinfo mesh2-bookinfo
oc delete ns istio-system mesh-system mesh1-system mesh2-system
```

5. Verify the resources in managed clusters are creaned up:

```bash
oc get vs,se,gw,dr -A
oc get policy -A
oc get ExportedServiceSet,ImportedServiceSet -A
oc get servicemeshpeer,smcp,smmr -A
oc get ns | grep -E "bookinfo|mesh|istio"
```
