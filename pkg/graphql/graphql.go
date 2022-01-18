package graphql

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	gojsonq "github.com/thedevsaddam/gojsonq/v2"
)

const (
	serviceAccountFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	graphqlServerAddr  = "https://console-api.open-cluster-management:4000/hcmuiapi/graphql"
)

var (
	serviceAccountToken string
	httpClient          *http.Client
)

func init() {
	serviceAccountTokenBytes, _ := os.ReadFile(serviceAccountFile)
	serviceAccountToken = string(serviceAccountTokenBytes)

	httpClient = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		},
	}
}

type k8sVariables struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Cluster    string `json:"cluster"`
}

var query = "query getResource($apiVersion: String, $kind: String, $name: String, $namespace: String, $cluster: String, $selfLink: String, $updateInterval: Int, $deleteAfterUse: Boolean) {\n  getResource(\n    apiVersion: $apiVersion\n    kind: $kind\n    name: $name\n    namespace: $namespace\n    cluster: $cluster\n    selfLink: $selfLink\n    updateInterval: $updateInterval\n    deleteAfterUse: $deleteAfterUse\n  )\n}\n"
var operationName = "getResource"

type k8sResourceQuery struct {
	OperationName string       `json:"operationName"`
	Query         string       `json:"query"`
	Variables     k8sVariables `json:"variables"`
}

func QueryK8sResource(apiVersion, kind, name, namespace, cluster string) ([]byte, error) {
	k8sResQuery := k8sResourceQuery{
		OperationName: operationName,
		Query:         query,
		Variables: k8sVariables{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       name,
			Namespace:  namespace,
			Cluster:    cluster,
		},
	}
	k8sResQueryJson, err := json.Marshal(k8sResQuery)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, graphqlServerAddr, bytes.NewBuffer(k8sResQueryJson))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+serviceAccountToken)
	req.Header.Set("content-type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	k8sResource := gojsonq.New().FromString(string(res)).Find("data.getResource")
	k8sResourceJson, err := json.Marshal(k8sResource)
	if err != nil {
		return nil, err
	}

	return k8sResourceJson, nil
}
