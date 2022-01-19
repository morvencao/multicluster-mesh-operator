#!/bin/bash

set -exo pipefail

# Point to the internal API server hostname
APISERVER=https://kubernetes.default.svc

# Path to ServiceAccount token
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount

# Read this Pod's namespace
NAMESPACE=$(cat ${SERVICEACCOUNT}/namespace)

# Read the ServiceAccount bearer token
TOKEN=$(cat ${SERVICEACCOUNT}/token)

# Reference the internal certificate authority (CA)
CACERT=${SERVICEACCOUNT}/ca.crt

smcpDiscoveryPolicyNamespace="open-cluster-management"
smcpDiscoveryPolicyName="policy-smcp-discovery"
smcpDiscoveryPlacementRuleName="placement-policy-smcp-discovery"
smcpDiscoveryPlacementBindingName="binding-policy-smcp-discovery"

# Delete the smcp discovery policy, placementrule and placementbinding resources with TOKEN
curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X DELETE ${APISERVER}/apis/policy.open-cluster-management.io/v1/namespaces/${smcpDiscoveryPolicyNamespace}/policies/${smcpDiscoveryPolicyName}
curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X DELETE ${APISERVER}/apis/apps.open-cluster-management.io/v1/namespaces/${smcpDiscoveryPolicyNamespace}/placementrules/${smcpDiscoveryPlacementRuleName}
curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X DELETE ${APISERVER}/apis/policy.open-cluster-management.io/v1/namespaces/${smcpDiscoveryPolicyNamespace}/placementbindings/${smcpDiscoveryPlacementBindingName}
