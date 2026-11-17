package logs

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"k8s.io/client-go/rest"
)

// EKSClusterConfig identifies a cluster and which pods to follow in it.
type EKSClusterConfig struct {
	Name          string // EKS cluster name
	Region        string
	Profile       string // AWS profile; empty = default credential chain
	Namespace     string // empty = all namespaces
	LabelSelector string // e.g. "app=api,tier!=batch"
	SinceMinutes  int    // initial history to load; 0 = 15
}

// ListEKSClusters returns the cluster names visible to the credentials in cfg.
func ListEKSClusters(ctx context.Context, cfg aws.Config) ([]string, error) {
	var names []string
	p := eks.NewListClustersPaginator(eks.NewFromConfig(cfg), &eks.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list EKS clusters: %w", err)
		}
		names = append(names, page.Clusters...)
	}
	return names, nil
}

// EKSRestConfig describes the cluster and returns a client-go config that
// authenticates with auto-refreshed IAM tokens.
func EKSRestConfig(ctx context.Context, cfg aws.Config, clusterName string) (*rest.Config, error) {
	out, err := eks.NewFromConfig(cfg).DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
	if err != nil {
		return nil, fmt.Errorf("describe EKS cluster %q: %w", clusterName, err)
	}
	c := out.Cluster
	if c == nil {
		return nil, fmt.Errorf("EKS cluster %q not found", clusterName)
	}
	if c.Endpoint == nil || c.CertificateAuthority == nil || c.CertificateAuthority.Data == nil {
		return nil, fmt.Errorf("EKS cluster %q has no endpoint/CA (status %s)", clusterName, c.Status)
	}
	ca, err := base64.StdEncoding.DecodeString(*c.CertificateAuthority.Data)
	if err != nil {
		return nil, fmt.Errorf("decode CA for %q: %w", clusterName, err)
	}
	return &rest.Config{
		Host:            *c.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{CAData: ca},
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			return &eksTokenTransport{base: rt, cfg: cfg, cluster: clusterName}
		},
	}, nil
}
