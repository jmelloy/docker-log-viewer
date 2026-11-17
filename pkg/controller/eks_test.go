package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
