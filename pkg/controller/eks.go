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

// HandleEKSLogin starts `aws sso login` for {"profile": "..."} and returns
// the CLI output (verification URL/code). Clusters reconnect on the next poll.
func (c *Controller) HandleEKSLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !awsProfileRe.MatchString(req.Profile) {
		http.Error(w, "invalid profile", http.StatusBadRequest)
		return
	}
	out, err := logs.SSOLogin(req.Profile)
	if err != nil {
		http.Error(w, err.Error()+"\n"+out, http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]string{"output": out})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
