package graphql

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	graphql "github.com/hasura/go-graphql-client"
	"golang.org/x/oauth2"
)

const (
	serviceAccountFile = "/var/run/secrets/kubernetes.io/serviceaccount"
	graphqlServerAddr  = "https://search-search-api.open-cluster-management:4010/searchapi/graphql"
)

var (
	serviceAccountToken string
	graphqlClient       *graphql.Client
)

func init() {
	serviceAccountTokenBytes, _ := os.ReadFile(serviceAccountFile)
	serviceAccountToken = string(serviceAccountTokenBytes)

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: serviceAccountToken},
	)
	httpClient := oauth2.NewClient(context.TODO(), src)
	httpTransport := &http.Transport{
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
	}
	httpClient.Transport.(*oauth2.Transport).Base = httpTransport
	graphqlClient = graphql.NewClient(graphqlServerAddr, httpClient)
}

func QuerySmcp(name, namespace, cluster string) {
	var query struct {
		GetResource struct {
			Name graphql.String
		} `graphql:"getResource(apiVersion: $apiVersion, kind: $kind, name: $name, namespace: $namespace, cluster: $cluster)"`
	}
	variables := map[string]interface{}{
		"apiVersion": "maistra.io/v2",
		"kind":       "servicemeshcontrolplane",
		"name":       graphql.String(name),
		"namespace":  graphql.String(namespace),
		"cluster":    graphql.String(cluster),
	}

	err := graphqlClient.Query(context.TODO(), &query, variables, graphql.OperationName("getResource"))
	if err != nil {
		fmt.Printf("==== error: %#v\n", err)
	}
	fmt.Printf("==== %#v\n", query.GetResource)
}
