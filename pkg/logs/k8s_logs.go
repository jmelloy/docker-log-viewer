package logs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// EKSIDPrefix marks container IDs that belong to EKS pods rather than Docker.
// Full form: "eks:<cluster>/<namespace>/<pod>/<container>".
const EKSIDPrefix = "eks:"

// IsEKSID reports whether a container ID refers to an EKS pod container.
func IsEKSID(id string) bool { return strings.HasPrefix(id, EKSIDPrefix) }

func eksContainerID(cluster, namespace, pod, container string) string {
	return EKSIDPrefix + cluster + "/" + namespace + "/" + pod + "/" + container
}

func parseEKSContainerID(id string) (cluster, namespace, pod, container string, err error) {
	parts := strings.Split(strings.TrimPrefix(id, EKSIDPrefix), "/")
	if !IsEKSID(id) || len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("invalid EKS container id %q", id)
	}
	return parts[0], parts[1], parts[2], parts[3], nil
}

// Cluster connection states reported to the UI.
const (
	EKSStateConnecting  = "connecting"
	EKSStateConnected   = "connected"
	EKSStateAuthExpired = "auth_expired" // AWS credentials/SSO session expired: user must sign in again
	EKSStateNeedsReauth = "needs_reauth" // AWS creds OK but the cluster rejected the token: Reconnect
	EKSStateError       = "error"
)

// While auth is expired, retry this rarely instead of every poll. A change to
// ~/.aws/credentials or ~/.aws/sso/cache (i.e. the user signed in again)
// triggers an immediate retry, as does Reconnect.
const authExpiredRetry = time.Minute

// EKSClusterStatus is the connection state shown in the settings UI.
type EKSClusterStatus struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	Error      string `json:"error,omitempty"`
	Containers int    `json:"containers"`
}

// ClassifyEKSError maps an error to a cluster state and a user-facing message
// with a recovery hint.
func ClassifyEKSError(ctx context.Context, err error, profile string) (state, message string) {
	if apierrors.IsUnauthorized(err) {
		return EKSStateNeedsReauth, "Cluster rejected the auth token (401). Click Reconnect to refresh credentials; " +
			"if it persists, check the IAM identity has an EKS access entry."
	}
	if apierrors.IsForbidden(err) {
		return EKSStateError, "Access denied listing pods (403): grant get/list on pods and pods/log. " + err.Error()
	}
	if IsAWSAuthError(err) {
		return EKSStateAuthExpired, ReauthHint(ctx, profile)
	}
	return EKSStateError, err.Error()
}

// IsAWSAuthError reports whether err means AWS credentials are missing,
// expired or invalid (expired SSO session, stale session token, ...).
// ponytail: substring match on SDK messages; these errors arrive wrapped in
// url.Error by client-go's transport, so typed matching wouldn't see them all.
func IsAWSAuthError(err error) bool {
	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"sso session", "sso token", "token has expired", "expiredtoken", "refresh cached credentials",
		"invalidgrant", "no valid credential", "failed to retrieve credentials", "unrecognizedclient",
		"invalidclienttokenid", "security token included in the request is", "unauthorizedexception",
		"session token not found",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

type eksCluster struct {
	cfg     EKSClusterConfig
	client  kubernetes.Interface
	ctx     context.Context // cancelled when the cluster is removed/changed
	cancel  context.CancelFunc
	state   string
	lastErr string
	retryAt time.Time   // don't auto-retry before this (auth expired)...
	credsAt time.Time   // ...unless AWS credentials changed after this
	last    []Container // last successful listing
}

// EKSManager tracks configured EKS clusters, lists their running pod
// containers as Containers, and streams pod logs into the shared pipeline.
type EKSManager struct {
	parent context.Context
	// NewClient builds a Kubernetes client for a cluster. Replaceable in tests.
	NewClient func(ctx context.Context, cfg EKSClusterConfig) (kubernetes.Interface, error)

	mu       sync.Mutex
	clusters map[string]*eksCluster
}

func NewEKSManager(parent context.Context) *EKSManager {
	return &EKSManager{parent: parent, NewClient: newEKSClient, clusters: map[string]*eksCluster{}}
}

func newEKSClient(ctx context.Context, cfg EKSClusterConfig) (kubernetes.Interface, error) {
	awsCfg, err := LoadAWSConfig(ctx, cfg.Profile, cfg.Region)
	if err != nil {
		return nil, err
	}
	restCfg, err := EKSRestConfig(ctx, awsCfg, cfg.EKSName())
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restCfg)
}

// SetClusters replaces the cluster set. Removed or changed clusters have
// their log streams cancelled; unchanged clusters keep their client.
func (m *EKSManager) SetClusters(cfgs []EKSClusterConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	want := map[string]EKSClusterConfig{}
	for _, c := range cfgs {
		want[c.Name] = c // ponytail: keyed by cluster name; same name in two accounts not supported
	}
	for name, c := range m.clusters {
		if w, ok := want[name]; !ok || w != c.cfg {
			c.cancel()
			delete(m.clusters, name)
		}
	}
	for name, cfg := range want {
		if _, ok := m.clusters[name]; !ok {
			ctx, cancel := context.WithCancel(m.parent)
			m.clusters[name] = &eksCluster{cfg: cfg, ctx: ctx, cancel: cancel, state: EKSStateConnecting}
		}
	}
}

// ListContainers returns running containers across all clusters. A cluster
// that fails (e.g. expired SSO session) reports its last known containers so
// existing streams aren't treated as stopped and restarted as duplicates.
func (m *EKSManager) ListContainers(ctx context.Context) []Container {
	m.mu.Lock()
	clusters := make([]*eksCluster, 0, len(m.clusters))
	for _, c := range m.clusters {
		clusters = append(clusters, c)
	}
	m.mu.Unlock()

	var result []Container
	for _, c := range clusters {
		m.mu.Lock()
		waiting := time.Now().Before(c.retryAt)
		credsAt := c.credsAt
		containers := c.last
		m.mu.Unlock()
		if waiting && awsCredsModTime().After(credsAt) {
			slog.Info("AWS credentials changed, retrying EKS cluster", "cluster", c.cfg.Name)
			waiting = false
		}
		if !waiting {
			var err error
			if containers, err = m.listCluster(ctx, c); err != nil {
				containers = m.recordError(c, err)
			}
		}
		result = append(result, containers...)
	}
	return result
}

// recordError marks the cluster failed and drops its client so the next
// attempt reloads AWS credentials and mints a fresh token. Returns the last
// known containers.
func (m *EKSManager) recordError(c *eksCluster, err error) []Container {
	state, msg := ClassifyEKSError(m.parent, err, c.cfg.Profile)
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg != c.lastErr {
		slog.Warn("EKS cluster unavailable", "cluster", c.cfg.Name, "state", state, "error", err)
	}
	c.state, c.lastErr, c.client = state, msg, nil
	c.retryAt = time.Time{}
	if state == EKSStateAuthExpired {
		c.retryAt, c.credsAt = time.Now().Add(authExpiredRetry), awsCredsModTime()
	}
	return c.last
}

// Reconnect discards the cluster's client and cached token, reloads AWS
// credentials (picking up a fresh SSO login), re-describes the cluster and
// verifies access by listing pods before marking it connected.
func (m *EKSManager) Reconnect(ctx context.Context, name string) error {
	m.mu.Lock()
	c := m.clusters[name]
	m.mu.Unlock()
	if c == nil {
		return fmt.Errorf("EKS cluster %q is not enabled", name)
	}
	client, err := m.NewClient(ctx, c.cfg)
	if err == nil {
		_, err = client.CoreV1().Pods(c.cfg.Namespace).List(ctx, metav1.ListOptions{LabelSelector: c.cfg.LabelSelector, Limit: 1})
	}
	if err != nil {
		m.recordError(c, err)
		_, msg := ClassifyEKSError(ctx, err, c.cfg.Profile)
		return errors.New(msg)
	}
	m.mu.Lock()
	c.client, c.state, c.lastErr, c.retryAt = client, EKSStateConnected, "", time.Time{}
	m.mu.Unlock()
	slog.Info("EKS cluster reconnected", "cluster", name)
	return nil
}

func (m *EKSManager) listCluster(ctx context.Context, c *eksCluster) ([]Container, error) {
	m.mu.Lock()
	client := c.client
	m.mu.Unlock()
	if client == nil {
		var err error
		if client, err = m.NewClient(ctx, c.cfg); err != nil {
			return nil, err
		}
		m.mu.Lock()
		c.client = client
		m.mu.Unlock()
	}

	pods, err := client.CoreV1().Pods(c.cfg.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: c.cfg.LabelSelector,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	containers := podContainers(c.cfg.Name, pods.Items)
	m.mu.Lock()
	c.state, c.lastErr, c.last = EKSStateConnected, "", containers
	m.mu.Unlock()
	return containers, nil
}

func podContainers(cluster string, pods []corev1.Pod) []Container {
	var result []Container
	for _, pod := range pods {
		images := map[string]string{}
		for _, spec := range pod.Spec.Containers {
			images[spec.Name] = spec.Image
		}
		service := pod.Labels["app.kubernetes.io/name"]
		if service == "" {
			service = pod.Labels["app"]
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Running == nil {
				continue
			}
			result = append(result, Container{
				ID:      eksContainerID(cluster, pod.Namespace, pod.Name, cs.Name),
				Name:    pod.Namespace + "/" + pod.Name + "/" + cs.Name,
				Image:   images[cs.Name],
				Ports:   []PortMapping{},
				Project: EKSIDPrefix + cluster,
				Service: service,
			})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// StreamLogsSince follows a pod container's logs (like `kubectl logs -f
// --timestamps --since-time`). A zero since loads the cluster's configured
// history window. The stream stops when the pod ends or the cluster is removed.
func (m *EKSManager) StreamLogsSince(id string, logChan chan<- ContainerMessage, onStreamEnd func(), since time.Time) error {
	clusterName, namespace, pod, container, err := parseEKSContainerID(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	c := m.clusters[clusterName]
	var client kubernetes.Interface
	if c != nil {
		client = c.client
	}
	m.mu.Unlock()
	if client == nil {
		return fmt.Errorf("EKS cluster %q not connected", clusterName)
	}

	if since.IsZero() {
		minutes := c.cfg.SinceMinutes
		if minutes <= 0 {
			minutes = 15
		}
		since = time.Now().Add(-time.Duration(minutes) * time.Minute)
	}
	// ponytail: SinceTime has 1s resolution, so a resumed stream may repeat lines from the last second.
	sinceTime := metav1.NewTime(since)
	stream, err := client.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container:  container,
		Follow:     true,
		Timestamps: true,
		SinceTime:  &sinceTime,
	}).Stream(c.ctx)
	if err != nil {
		return fmt.Errorf("stream logs for %s: %w", id, err)
	}
	slog.Info("Starting EKS log stream", "id", id, "since", since)
	go streamLines(c.ctx, id, stream, false, logChan, onStreamEnd)
	return nil
}

// Status reports per-cluster connection state, sorted by name.
func (m *EKSManager) Status() []EKSClusterStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]EKSClusterStatus, 0, len(m.clusters))
	for name, c := range m.clusters {
		out = append(out, EKSClusterStatus{Name: name, State: c.state, Error: c.lastErr, Containers: len(c.last)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
