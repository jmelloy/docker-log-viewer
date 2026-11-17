package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestEKSClusterCRUD(t *testing.T) {
	db, err := store.NewStore(filepath.Join(t.TempDir(), "eks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctrl := NewController(nil, logstore.NewLogStore(100, time.Hour), db, ctx, cancel, nil)
	ctrl.EKS = logs.NewEKSManager(ctx)
	ctrl.EKS.NewClient = func(context.Context, logs.EKSClusterConfig) (kubernetes.Interface, error) {
		return fake.NewClientset(), nil
	}

	r := mux.NewRouter()
	r.HandleFunc("/api/eks/clusters", ctrl.HandleListEKSClusters).Methods("GET")
	r.HandleFunc("/api/eks/clusters", ctrl.HandleSaveEKSCluster).Methods("POST")
	r.HandleFunc("/api/eks/clusters/{id}", ctrl.HandleSaveEKSCluster).Methods("PUT")
	r.HandleFunc("/api/eks/clusters/{id}", ctrl.HandleDeleteEKSCluster).Methods("DELETE")
	do := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	status := func() []logs.EKSClusterStatus {
		var resp struct {
			Clusters []store.EKSCluster      `json:"clusters"`
			Status   []logs.EKSClusterStatus `json:"status"`
		}
		json.NewDecoder(do("GET", "/api/eks/clusters", "").Body).Decode(&resp)
		return resp.Status
	}

	for _, bad := range []string{
		`{"name":"prod","region":"nowhere"}`,
		`{"name":"a/b","region":"us-east-1"}`,
		`{"name":"prod","region":"us-east-1","profile":"--debug"}`,
		`{"name":"prod","region":"us-east-1","labelSelector":"app in ("}`,
	} {
		if w := do("POST", "/api/eks/clusters", bad); w.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", bad, w.Code)
		}
	}

	w := do("POST", "/api/eks/clusters", `{"name":"prod","region":"us-east-1","profile":"dev","namespace":"api","labelSelector":"app=web","sinceMinutes":30,"enabled":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var created store.EKSCluster
	json.NewDecoder(w.Body).Decode(&created)
	if st := status(); len(st) != 1 || st[0].Name != "prod" {
		t.Fatalf("manager should track enabled cluster, got %+v", st)
	}

	// Disabling persists false and removes the cluster from the manager.
	w = do("PUT", "/api/eks/clusters/"+itoa(created.ID), `{"name":"prod","region":"us-east-1","enabled":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body)
	}
	if cfgs, _ := LoadEKSConfigs(db); len(cfgs) != 0 {
		t.Fatalf("disabled cluster should not load, got %+v", cfgs)
	}
	if st := status(); len(st) != 0 {
		t.Fatalf("manager should drop disabled cluster, got %+v", st)
	}

	if w := do("DELETE", "/api/eks/clusters/"+itoa(created.ID), ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	if list, _ := db.ListEKSClusters(); len(list) != 0 {
		t.Fatalf("expected empty list, got %+v", list)
	}
}

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }

func TestImportKubeconfigEKS(t *testing.T) {
	dir := t.TempDir()
	kubeconfig := filepath.Join(dir, "config")
	os.WriteFile(kubeconfig, []byte(`apiVersion: v1
kind: Config
clusters:
- name: arn:aws:eks:us-east-1:111111111111:cluster/dnsid-dev
  cluster: {server: "https://A.gr7.us-east-1.eks.amazonaws.com"}
- name: arn:aws:eks:us-east-1:222222222222:cluster/dnsid-dev
  cluster: {server: "https://B.gr7.us-east-1.eks.amazonaws.com"}
users:
- name: a
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args: [--region, us-east-1, eks, get-token, --cluster-name, dnsid-dev]
      env: [{name: AWS_PROFILE, value: dnsid-development}]
- name: b
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args: [--region, us-east-1, eks, get-token, --cluster-name, dnsid-dev]
      env: [{name: AWS_PROFILE, value: idil-dev}]
contexts:
- name: dnsid-dev
  context: {cluster: "arn:aws:eks:us-east-1:111111111111:cluster/dnsid-dev", user: a}
- name: arn:aws:eks:us-east-1:222222222222:cluster/dnsid-dev
  context: {cluster: "arn:aws:eks:us-east-1:222222222222:cluster/dnsid-dev", user: b}
`), 0o600)
	t.Setenv("KUBECONFIG", kubeconfig)

	db, err := store.NewStore(filepath.Join(dir, "eks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	added, err := ImportKubeconfigEKS(db)
	if err != nil || added != 2 {
		t.Fatalf("first import: added=%d err=%v", added, err)
	}
	if added, _ := ImportKubeconfigEKS(db); added != 0 {
		t.Fatalf("re-import should add nothing, added %d", added)
	}
	list, _ := db.ListEKSClusters()
	labels := map[string]store.EKSCluster{}
	for _, c := range list {
		if c.Enabled {
			t.Errorf("imported cluster %q should be disabled", c.Name)
		}
		labels[c.Name] = c
	}
	// Same EKS name in two accounts: distinct labels, same ClusterName.
	a, b := labels["dnsid-dev"], labels["dnsid-dev-idil-dev"]
	if a.ClusterName != "dnsid-dev" || a.Profile != "dnsid-development" || b.ClusterName != "dnsid-dev" || b.Profile != "idil-dev" {
		t.Fatalf("unexpected import: %+v", list)
	}
	if cfgs, _ := LoadEKSConfigs(db); len(cfgs) != 0 {
		t.Fatalf("disabled imports must not stream, got %+v", cfgs)
	}
}
