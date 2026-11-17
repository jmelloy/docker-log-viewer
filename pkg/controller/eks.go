package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	eksNameRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)
	awsRegionRe  = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-\d+$`)
	awsProfileRe = regexp.MustCompile(`^([A-Za-z0-9_.@+][A-Za-z0-9_.@+-]*)?$`)
	k8sNSRe      = regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?)?$`)
)

// LoadEKSConfigs converts the enabled saved clusters into manager configs.
func LoadEKSConfigs(s *store.Store) ([]logs.EKSClusterConfig, error) {
	clusters, err := s.ListEKSClusters()
	if err != nil {
		return nil, err
	}
	var cfgs []logs.EKSClusterConfig
	for _, c := range clusters {
		if !c.Enabled {
			continue
		}
		cfgs = append(cfgs, logs.EKSClusterConfig{
			Name:          c.Name,
			ClusterName:   c.ClusterName,
			Region:        c.Region,
			Profile:       c.Profile,
			Namespace:     c.Namespace,
			LabelSelector: c.LabelSelector,
			SinceMinutes:  c.SinceMinutes,
		})
	}
	return cfgs, nil
}

func validateEKSCluster(c *store.EKSCluster) error {
	switch {
	case !eksNameRe.MatchString(c.Name):
		return fmt.Errorf("invalid cluster name %q", c.Name)
	case c.ClusterName != "" && !eksNameRe.MatchString(c.ClusterName):
		return fmt.Errorf("invalid EKS cluster name %q", c.ClusterName)
	case !awsRegionRe.MatchString(c.Region):
		return fmt.Errorf("invalid region %q", c.Region)
	case !awsProfileRe.MatchString(c.Profile):
		return fmt.Errorf("invalid profile %q", c.Profile)
	case !k8sNSRe.MatchString(c.Namespace):
		return fmt.Errorf("invalid namespace %q", c.Namespace)
	case c.SinceMinutes < 0:
		return fmt.Errorf("sinceMinutes must be >= 0")
	}
	if _, err := labels.Parse(c.LabelSelector); err != nil {
		return fmt.Errorf("invalid label selector: %w", err)
	}
	return nil
}

func (c *Controller) reloadEKS() {
	if c.EKS == nil {
		return
	}
	cfgs, err := LoadEKSConfigs(c.store)
	if err != nil {
		slog.Error("failed to reload EKS clusters", "error", err)
		return
	}
	c.EKS.SetClusters(cfgs)
}

// HandleListEKSClusters returns saved clusters plus live connection status.
func (c *Controller) HandleListEKSClusters(w http.ResponseWriter, r *http.Request) {
	clusters, err := c.store.ListEKSClusters()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	status := []logs.EKSClusterStatus{}
	if c.EKS != nil {
		status = c.EKS.Status()
	}
	writeJSON(w, map[string]any{"clusters": clusters, "status": status})
}

// HandleSaveEKSCluster creates (POST) or updates (PUT /{id}) a cluster.
func (c *Controller) HandleSaveEKSCluster(w http.ResponseWriter, r *http.Request) {
	var cluster store.EKSCluster
	if err := json.NewDecoder(r.Body).Decode(&cluster); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cluster.ID = 0
	if idStr, ok := mux.Vars(r)["id"]; ok {
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		cluster.ID = uint(id)
	}
	if err := validateEKSCluster(&cluster); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.store.SaveEKSCluster(&cluster); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.reloadEKS()
	writeJSON(w, cluster)
}

// HandleDeleteEKSCluster deletes a cluster and stops its streams.
func (c *Controller) HandleDeleteEKSCluster(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := c.store.DeleteEKSCluster(uint(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.reloadEKS()
	w.WriteHeader(http.StatusNoContent)
}

// HandleDiscoverEKSClusters lists clusters visible to ?profile=&region=.
func (c *Controller) HandleDiscoverEKSClusters(w http.ResponseWriter, r *http.Request) {
	profile, region := r.URL.Query().Get("profile"), r.URL.Query().Get("region")
	if !awsProfileRe.MatchString(profile) || !awsRegionRe.MatchString(region) {
		http.Error(w, "invalid profile or region", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	cfg, err := logs.LoadAWSConfig(ctx, profile, region)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	identity, err := logs.AWSIdentity(ctx, cfg)
	if err != nil {
		http.Error(w, "AWS credentials invalid or expired (try SSO login): "+err.Error(), http.StatusUnauthorized)
		return
	}
	names, err := logs.ListEKSClusters(ctx, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if names == nil {
		names = []string{}
	}
	writeJSON(w, map[string]any{"identity": identity, "clusters": names})
}

// HandleEKSLogin starts re-authentication for {"profile": "..."}. SSO
// profiles get a device-code `aws sso login` whose verification URL the UI
// opens in the user's browser. Other profiles (credentials written by an
// Okta tool, credential_process, ...) can't be refreshed from here, so the
// response carries a hint instead. Either way the UI then polls
// /api/eks/auth-status and reconnects once credentials are valid.
func (c *Controller) HandleEKSLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !awsProfileRe.MatchString(req.Profile) {
		http.Error(w, "invalid profile", http.StatusBadRequest)
		return
	}
	resp := map[string]string{"kind": logs.AWSProfileKind(r.Context(), req.Profile), "hint": logs.ReauthHint(r.Context(), req.Profile)}
	if resp["kind"] == logs.ProfileKindSSO {
		url, out, err := logs.SSOLogin(req.Profile)
		resp["url"], resp["output"] = url, out
		if err != nil {
			resp["error"] = err.Error()
		}
	}
	writeJSON(w, resp)
}

// HandleEKSAuthStatus checks ?profile= credentials with a fresh credential
// load (re-reading ~/.aws/credentials and the SSO cache).
func (c *Controller) HandleEKSAuthStatus(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	if !awsProfileRe.MatchString(profile) {
		http.Error(w, "invalid profile", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	cfg, err := logs.LoadAWSConfig(ctx, profile, "us-east-1") // STS answers in any region
	identity := ""
	if err == nil {
		identity, err = logs.AWSIdentity(ctx, cfg)
	}
	if err != nil {
		writeJSON(w, map[string]any{"valid": false, "error": err.Error(), "hint": logs.ReauthHint(ctx, profile)})
		return
	}
	writeJSON(w, map[string]any{"valid": true, "identity": identity})
}

// HandleEKSReconnect forces a fresh credential load, token and Kubernetes
// client for a cluster and verifies access before reporting success.
func (c *Controller) HandleEKSReconnect(w http.ResponseWriter, r *http.Request) {
	if c.EKS == nil {
		http.Error(w, "EKS not available", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := c.EKS.Reconnect(ctx, mux.Vars(r)["name"]); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleListKubeconfigEKS lists EKS clusters found in kubeconfig, flagging
// ones already saved.
func (c *Controller) HandleListKubeconfigEKS(w http.ResponseWriter, r *http.Request) {
	found, err := logs.LoadKubeconfigEKSClusters()
	if err != nil {
		http.Error(w, "read kubeconfig: "+err.Error(), http.StatusInternalServerError)
		return
	}
	saved, err := c.store.ListEKSClusters()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type candidate struct {
		logs.KubeconfigEKSCluster
		Added bool `json:"added"`
	}
	out := make([]candidate, 0, len(found))
	for _, k := range found {
		out = append(out, candidate{k, findSaved(saved, k) != nil})
	}
	writeJSON(w, out)
}

// HandleImportKubeconfigEKS saves kubeconfig clusters not yet configured.
func (c *Controller) HandleImportKubeconfigEKS(w http.ResponseWriter, r *http.Request) {
	added, err := ImportKubeconfigEKS(c.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]int{"added": added})
}

// ImportKubeconfigEKS saves kubeconfig EKS clusters that aren't configured
// yet. They're added disabled so a kubeconfig full of prod contexts doesn't
// start streaming everything; the user enables the ones they want.
func ImportKubeconfigEKS(s *store.Store) (int, error) {
	found, err := logs.LoadKubeconfigEKSClusters()
	if err != nil {
		return 0, fmt.Errorf("read kubeconfig: %w", err)
	}
	saved, err := s.ListEKSClusters()
	if err != nil {
		return 0, err
	}
	added := 0
	for _, k := range found {
		if findSaved(saved, k) != nil {
			continue
		}
		cluster := store.EKSCluster{
			Name:         uniqueLabel(saved, k),
			ClusterName:  k.ClusterName,
			Region:       k.Region,
			Profile:      k.Profile,
			Namespace:    k.Namespace,
			SinceMinutes: 15,
		}
		if err := validateEKSCluster(&cluster); err != nil {
			slog.Warn("skipping kubeconfig cluster", "contexts", k.Contexts, "error", err)
			continue
		}
		if err := s.SaveEKSCluster(&cluster); err != nil {
			return added, err
		}
		saved = append(saved, cluster)
		added++
	}
	return added, nil
}

func effectiveName(c store.EKSCluster) string {
	if c.ClusterName != "" {
		return c.ClusterName
	}
	return c.Name
}

func findSaved(saved []store.EKSCluster, k logs.KubeconfigEKSCluster) *store.EKSCluster {
	for i, c := range saved {
		if effectiveName(c) == k.ClusterName && c.Region == k.Region && c.Profile == k.Profile {
			return &saved[i]
		}
	}
	return nil
}

func uniqueLabel(saved []store.EKSCluster, k logs.KubeconfigEKSCluster) string {
	taken := map[string]bool{}
	for _, c := range saved {
		taken[c.Name] = true
	}
	label := k.Label
	if taken[label] && k.Profile != "" {
		label = k.Label + "-" + k.Profile
	}
	for i := 2; taken[label]; i++ {
		label = fmt.Sprintf("%s-%d", k.Label, i)
	}
	return label
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
