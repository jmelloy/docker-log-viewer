package logs

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func runningPod(ns, name string, labels map[string]string, containers ...string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: labels},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	for _, c := range containers {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: c, Image: c + ":latest"})
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{
			Name: c, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		})
	}
	return pod
}

func newTestManager(t *testing.T, client kubernetes.Interface, fail *bool) *EKSManager {
	m := NewEKSManager(t.Context())
	m.NewClient = func(ctx context.Context, cfg EKSClusterConfig) (kubernetes.Interface, error) {
		if fail != nil && *fail {
			return nil, errors.New("sso session expired")
		}
		return client, nil
	}
	return m
}

func TestEKSContainerIDRoundTrip(t *testing.T) {
	id := eksContainerID("prod", "default", "api-123", "app")
	if !IsEKSID(id) || IsEKSID(strings.Repeat("a", 64)) {
		t.Fatalf("IsEKSID wrong for %q", id)
	}
	c, ns, pod, ctr, err := parseEKSContainerID(id)
	if err != nil || c != "prod" || ns != "default" || pod != "api-123" || ctr != "app" {
		t.Fatalf("got %q %q %q %q %v", c, ns, pod, ctr, err)
	}
	if _, _, _, _, err := parseEKSContainerID("eks:prod/default"); err == nil {
		t.Fatal("expected error for short id")
	}
}

func TestEKSManagerListContainers(t *testing.T) {
	client := fake.NewClientset(
		runningPod("default", "api-1", map[string]string{"app": "api"}, "app", "sidecar"),
		runningPod("default", "worker-1", map[string]string{"app": "worker"}, "app"),
	)
	m := newTestManager(t, client, nil)
	m.SetClusters([]EKSClusterConfig{{Name: "prod", Region: "us-east-1", Namespace: "default", LabelSelector: "app=api"}})

	got := m.ListContainers(t.Context())
	if len(got) != 2 {
		t.Fatalf("want 2 containers (api app+sidecar), got %+v", got)
	}
	if got[0].ID != "eks:prod/default/api-1/app" || got[0].Name != "default/api-1/app" ||
		got[0].Project != "eks:prod" || got[0].Service != "api" || got[0].Image != "app:latest" {
		t.Fatalf("unexpected container %+v", got[0])
	}
	if st := m.Status(); len(st) != 1 || st[0].State != EKSStateConnected || st[0].Containers != 2 {
		t.Fatalf("unexpected status %+v", st)
	}
}

func TestEKSManagerKeepsLastContainersOnError(t *testing.T) {
	client := fake.NewClientset(runningPod("default", "api-1", nil, "app"))
	fail := false
	m := newTestManager(t, client, &fail)
	m.SetClusters([]EKSClusterConfig{{Name: "prod", Region: "us-east-1"}})
	if len(m.ListContainers(t.Context())) != 1 {
		t.Fatal("expected 1 container")
	}

	// Force a reconnect that fails: last known containers are kept.
	m.clusters["prod"].client = nil
	fail = true
	if got := m.ListContainers(t.Context()); len(got) != 1 {
		t.Fatalf("expected cached container on error, got %+v", got)
	}
	if st := m.Status(); st[0].State != EKSStateAuthExpired || st[0].Error == "" {
		t.Fatalf("expected error status, got %+v", st)
	}
}

func TestEKSManagerStreamLogs(t *testing.T) {
	client := fake.NewClientset(runningPod("default", "api-1", nil, "app"))
	m := newTestManager(t, client, nil)
	m.SetClusters([]EKSClusterConfig{{Name: "prod", Region: "us-east-1"}})
	m.ListContainers(t.Context()) // connects client

	logChan := make(chan ContainerMessage, 10)
	ended := make(chan struct{})
	id := "eks:prod/default/api-1/app"
	if err := m.StreamLogsSince(id, logChan, func() { close(ended) }, time.Time{}); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-logChan:
		// The fake clientset always returns the body "fake logs".
		if msg.ContainerID != id || msg.Entry == nil || !strings.Contains(msg.Entry.Raw, "fake logs") {
			t.Fatalf("unexpected message %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no log received")
	}
	<-ended

	if err := m.StreamLogsSince("eks:other/default/x/y", logChan, nil, time.Time{}); err == nil {
		t.Fatal("expected error for unknown cluster")
	}
}

func TestEKSManagerSetClustersCancelsRemoved(t *testing.T) {
	m := newTestManager(t, fake.NewClientset(), nil)
	m.SetClusters([]EKSClusterConfig{{Name: "a"}, {Name: "b"}})
	a, b := m.clusters["a"], m.clusters["b"]
	m.SetClusters([]EKSClusterConfig{{Name: "b"}, {Name: "a", Namespace: "changed"}})
	if a.ctx.Err() == nil {
		t.Fatal("changed cluster should be cancelled")
	}
	if b.ctx.Err() != nil || m.clusters["b"] != b {
		t.Fatal("unchanged cluster should be kept")
	}
	m.SetClusters(nil)
	if b.ctx.Err() == nil || len(m.clusters) != 0 {
		t.Fatal("removed cluster should be cancelled")
	}
}

func TestEKSToken(t *testing.T) {
	cfg := aws.Config{
		Region:      "us-west-2",
		Credentials: credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secret", ""),
	}
	token, err := EKSToken(t.Context(), cfg, "my-cluster")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, eksTokenPrefix) {
		t.Fatalf("missing prefix: %s", token)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, eksTokenPrefix))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "sts.us-west-2.amazonaws.com" || q.Get("Action") != "GetCallerIdentity" ||
		!strings.Contains(q.Get("X-Amz-SignedHeaders"), "x-k8s-aws-id") || q.Get("X-Amz-Expires") != "60" {
		t.Fatalf("unexpected presigned URL: %s", u)
	}
}
