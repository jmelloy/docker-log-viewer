package logs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// isolateAWS points AWS config/credentials at temp files so tests don't read
// the developer's ~/.aws. Returns the credentials file path.
func isolateAWS(t *testing.T) string {
	dir := t.TempDir()
	creds := filepath.Join(dir, "credentials")
	cfg := filepath.Join(dir, "config")
	os.WriteFile(creds, []byte("[okta]\naws_access_key_id=x\naws_secret_access_key=y\n"), 0o600)
	os.WriteFile(cfg, []byte("[profile sso-dev]\nsso_session=corp\nsso_account_id=1\nsso_role_name=r\n[sso-session corp]\nsso_start_url=https://corp.awsapps.com/start\nsso_region=us-east-1\n"), 0o600)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", creds)
	t.Setenv("AWS_CONFIG_FILE", cfg)
	t.Setenv("HOME", dir)
	t.Setenv("AWS_PROFILE", "")
	return creds
}

func TestClassifyEKSError(t *testing.T) {
	isolateAWS(t)
	ctx := context.Background()
	cases := []struct {
		err     error
		profile string
		state   string
		want    string
	}{
		{errors.New("operation error STS: GetCallerIdentity, failed to refresh cached credentials, the SSO session has expired or is invalid"), "sso-dev", EKSStateAuthExpired, "Run 'aws sso login --profile sso-dev'"},
		{errors.New(`Get "https://x.eks.amazonaws.com/api/v1/pods": presign EKS token: ExpiredToken: The security token included in the request is expired`), "okta", EKSStateAuthExpired, "Okta/AWS login tool"},
		{errors.New("api error InvalidClientTokenId: The security token included in the request is invalid"), "okta", EKSStateAuthExpired, "profile 'okta'"},
		{apierrors.NewUnauthorized("token expired"), "okta", EKSStateNeedsReauth, "Reconnect"},
		{apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("no")), "okta", EKSStateError, "pods/log"},
		{errors.New("dial tcp: no such host"), "okta", EKSStateError, "no such host"},
	}
	for _, tc := range cases {
		state, msg := ClassifyEKSError(ctx, tc.err, tc.profile)
		if state != tc.state || !strings.Contains(msg, tc.want) {
			t.Errorf("%v: got (%s, %q), want (%s, contains %q)", tc.err, state, msg, tc.state, tc.want)
		}
	}
}

func TestVerificationURL(t *testing.T) {
	out := "Browser will not be automatically opened.\nPlease visit the following URL:\n\nhttps://device.sso.us-east-1.amazonaws.com/\n\nThen enter the code:\n\nABCD-EFGH\n\nAlternatively, you may visit the following URL which will autofill the code upon loading:\nhttps://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH\n"
	if got := verificationURL(out); got != "https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH" {
		t.Fatalf("got %q", got)
	}
	if got := verificationURL("no url here"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestEKSManagerAuthExpiredBackoffAndCredsChange(t *testing.T) {
	creds := isolateAWS(t)
	expired := true
	calls := 0
	client := fake.NewClientset(runningPod("default", "api-1", nil, "app"))
	m := NewEKSManager(t.Context())
	m.NewClient = func(context.Context, EKSClusterConfig) (kubernetes.Interface, error) {
		calls++
		if expired {
			return nil, errors.New("ExpiredToken: The security token included in the request is expired")
		}
		return client, nil
	}
	m.SetClusters([]EKSClusterConfig{{Name: "dev", Profile: "okta"}})

	m.ListContainers(t.Context())
	if st := m.Status()[0]; st.State != EKSStateAuthExpired || !strings.Contains(st.Error, "profile 'okta'") {
		t.Fatalf("want auth_expired with hint, got %+v", st)
	}
	m.ListContainers(t.Context())
	if calls != 1 {
		t.Fatalf("auth-expired cluster should back off, got %d attempts", calls)
	}

	// User signs in again with their Okta tool -> credentials file changes -> immediate retry.
	expired = false
	future := time.Now().Add(time.Second)
	os.Chtimes(creds, future, future)
	if got := m.ListContainers(t.Context()); len(got) != 1 || calls != 2 {
		t.Fatalf("expected retry after credentials change, calls=%d got=%+v", calls, got)
	}
	if st := m.Status()[0]; st.State != EKSStateConnected || st.Error != "" {
		t.Fatalf("want connected, got %+v", st)
	}
}

func TestEKSManagerReconnect(t *testing.T) {
	isolateAWS(t)
	var fail error
	client := fake.NewClientset(runningPod("default", "api-1", nil, "app"))
	m := NewEKSManager(t.Context())
	m.NewClient = func(context.Context, EKSClusterConfig) (kubernetes.Interface, error) {
		if fail != nil {
			return nil, fail
		}
		return client, nil
	}
	m.SetClusters([]EKSClusterConfig{{Name: "dev", Profile: "okta"}})

	fail = errors.New("ExpiredToken: expired")
	if err := m.Reconnect(t.Context(), "dev"); err == nil || !strings.Contains(err.Error(), "expired or invalid") {
		t.Fatalf("want auth hint error, got %v", err)
	}
	fail = nil
	if err := m.Reconnect(t.Context(), "dev"); err != nil {
		t.Fatal(err)
	}
	if st := m.Status()[0]; st.State != EKSStateConnected {
		t.Fatalf("want connected, got %+v", st)
	}
	if err := m.Reconnect(t.Context(), "missing"); err == nil {
		t.Fatal("want error for unknown cluster")
	}
}

func TestKubeconfigEKSClusters(t *testing.T) {
	cfg := clientcmdapi.NewConfig()
	addCluster := func(arn, server, profile string, args ...string) {
		cfg.Clusters[arn] = &clientcmdapi.Cluster{Server: server}
		exec := &clientcmdapi.ExecConfig{Command: "aws", Args: args}
		if profile != "" {
			exec.Env = []clientcmdapi.ExecEnvVar{{Name: "AWS_PROFILE", Value: profile}}
		}
		cfg.AuthInfos[arn] = &clientcmdapi.AuthInfo{Exec: exec}
	}
	ctx := func(name, arn, ns string) {
		cfg.Contexts[name] = &clientcmdapi.Context{Cluster: arn, AuthInfo: arn, Namespace: ns}
	}
	a := "arn:aws:eks:us-east-1:111111111111:cluster/dnsid-dev"
	b := "arn:aws:eks:us-east-1:222222222222:cluster/dnsid-dev"
	c := "arn:aws:eks:eu-west-1:333333333333:cluster/legacy"
	addCluster(a, "https://A.gr7.us-east-1.eks.amazonaws.com", "dnsid-development",
		"--region", "us-east-1", "eks", "get-token", "--cluster-name", "dnsid-dev", "--output", "json")
	addCluster(b, "https://B.gr7.us-east-1.eks.amazonaws.com", "idil-dev",
		"--region", "us-east-1", "eks", "get-token", "--cluster-name", "dnsid-dev")
	addCluster(c, "https://C.yl4.eu-west-1.eks.amazonaws.com", "", "token", "-i", "legacy", "--profile", "ops")
	ctx(a, a, "")
	ctx("dnsid-dev", a, "")
	ctx("dnsid-development", a, "")
	ctx("dev", b, "dnsid")
	ctx("legacy-ctx", c, "")
	cfg.Clusters["minikube"] = &clientcmdapi.Cluster{Server: "https://127.0.0.1:8443"}
	cfg.Contexts["minikube"] = &clientcmdapi.Context{Cluster: "minikube"}

	got := kubeconfigEKSClusters(cfg)
	if len(got) != 3 {
		t.Fatalf("want 3 clusters (contexts merged, non-EKS skipped), got %+v", got)
	}
	want := []KubeconfigEKSCluster{
		{Label: "dev", ClusterName: "dnsid-dev", Region: "us-east-1", Profile: "idil-dev", Namespace: "dnsid"},
		{Label: "dnsid-dev", ClusterName: "dnsid-dev", Region: "us-east-1", Profile: "dnsid-development"},
		{Label: "legacy-ctx", ClusterName: "legacy", Region: "eu-west-1", Profile: "ops"},
	}
	for i, w := range want {
		g := got[i]
		if g.Label != w.Label || g.ClusterName != w.ClusterName || g.Region != w.Region || g.Profile != w.Profile || g.Namespace != w.Namespace {
			t.Errorf("cluster %d: got %+v, want %+v", i, g, w)
		}
	}
	if len(got[1].Contexts) != 3 {
		t.Errorf("want 3 merged contexts, got %v", got[1].Contexts)
	}
}
