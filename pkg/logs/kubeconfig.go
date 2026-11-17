package logs

import (
	"regexp"
	"sort"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// KubeconfigEKSCluster is an EKS cluster found in the user's kubeconfig.
type KubeconfigEKSCluster struct {
	Label       string   `json:"label"`       // suggested unique label (preferred context name)
	ClusterName string   `json:"clusterName"` // EKS cluster name
	Region      string   `json:"region"`
	Profile     string   `json:"profile"`
	Namespace   string   `json:"namespace"`
	Contexts    []string `json:"contexts"` // all contexts pointing at this cluster+profile
}

var (
	eksARNRe    = regexp.MustCompile(`^arn:aws[\w-]*:eks:([a-z0-9-]+):(\d+):cluster/(.+)$`)
	eksServerRe = regexp.MustCompile(`\.([a-z]{2}(?:-[a-z]+)+-\d+)\.eks\.amazonaws\.com`)
	labelBadRe  = regexp.MustCompile(`[^A-Za-z0-9_-]+`)
)

// LoadKubeconfigEKSClusters reads kubeconfig (honouring $KUBECONFIG, default
// ~/.kube/config) and returns its EKS clusters. A missing file yields none.
func LoadKubeconfigEKSClusters() ([]KubeconfigEKSCluster, error) {
	cfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}
	return kubeconfigEKSClusters(cfg), nil
}

// kubeconfigEKSClusters extracts EKS clusters from kubeconfig contexts. Name,
// region and profile come from the `aws eks get-token` (or
// aws-iam-authenticator) exec args/env, falling back to the cluster ARN and
// API server hostname. Contexts sharing a cluster+profile are merged.
func kubeconfigEKSClusters(cfg *clientcmdapi.Config) []KubeconfigEKSCluster {
	byKey := map[string]*KubeconfigEKSCluster{}
	for ctxName, kctx := range cfg.Contexts {
		cluster := cfg.Clusters[kctx.Cluster]
		if cluster == nil || !strings.Contains(cluster.Server, ".eks.amazonaws.com") {
			continue
		}
		var name, region, profile string
		if m := eksARNRe.FindStringSubmatch(kctx.Cluster); m != nil {
			region, name = m[1], m[3]
		}
		if m := eksServerRe.FindStringSubmatch(cluster.Server); m != nil && region == "" {
			region = m[1]
		}
		if user := cfg.AuthInfos[kctx.AuthInfo]; user != nil && user.Exec != nil {
			args := user.Exec.Args
			for i := 0; i+1 < len(args); i++ {
				switch args[i] {
				case "--cluster-name", "-i", "--cluster-id":
					name = args[i+1]
				case "--region":
					region = args[i+1]
				case "--profile":
					profile = args[i+1]
				}
			}
			for _, env := range user.Exec.Env {
				if env.Name == "AWS_PROFILE" && profile == "" {
					profile = env.Value
				}
			}
		}
		if name == "" || region == "" {
			continue
		}
		key := name + "|" + region + "|" + profile
		c := byKey[key]
		if c == nil {
			c = &KubeconfigEKSCluster{ClusterName: name, Region: region, Profile: profile, Namespace: kctx.Namespace}
			byKey[key] = c
		}
		c.Contexts = append(c.Contexts, ctxName)
	}

	out := make([]KubeconfigEKSCluster, 0, len(byKey))
	for _, c := range byKey {
		sort.Strings(c.Contexts)
		c.Label = preferredLabel(c.ClusterName, c.Contexts)
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out
}

// preferredLabel picks a context named after the cluster, else the shortest
// non-ARN context name, else the cluster name, sanitized for use in IDs.
func preferredLabel(clusterName string, contexts []string) string {
	best := ""
	for _, ctx := range contexts {
		if ctx == clusterName {
			best = ctx
			break
		}
		if !strings.HasPrefix(ctx, "arn:") && (best == "" || len(ctx) < len(best)) {
			best = ctx
		}
	}
	if best == "" {
		best = clusterName
	}
	return strings.Trim(labelBadRe.ReplaceAllString(best, "-"), "-")
}
