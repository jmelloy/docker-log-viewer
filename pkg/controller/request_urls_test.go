package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/store"
)

func TestRequestURLs(t *testing.T) {
	db, err := store.NewStore(filepath.Join(t.TempDir(), "requests.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.URL.RequestURI())
	}))
	defer target.Close()
	serverID, err := db.CreateServer(&store.Server{Name: "test", URL: target.URL + "/graphql"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctrl := NewController(nil, logstore.NewLogStore(100, time.Hour), db, ctx, cancel, nil)
	saved := httptest.NewRecorder()
	ctrl.HandleCreateSampleQuery(saved, httptest.NewRequest("POST", "/api/samples/", strings.NewReader(fmt.Sprintf(`{"name":"search","serverId":%d,"urlOverride":"/api/search?q=test","requestData":"{}"}`, serverID))))
	if saved.Code != http.StatusOK {
		t.Fatal(saved.Body.String())
	}
	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(saved.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	sample, err := db.GetSampleQuery(result.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sample.URLOverride != "/api/search?q=test" {
		t.Fatalf("URL not saved: %+v", sample)
	}

	for _, tc := range []struct {
		name, override, path string
		sample               bool
	}{
		{name: "legacy server URL", path: "/graphql"},
		{name: "sample URL", path: "/api/search?q=test", sample: true},
		{name: "override sample", override: "/other", path: "/other", sample: true},
		{name: "absolute URL", override: target.URL + "/absolute", path: "/absolute"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{"serverId": serverID, "requestData": "{}", "sync": true, "urlOverride": tc.override}
			if tc.sample {
				payload["sampleId"] = result.ID
			}
			body, _ := json.Marshal(payload)
			response := httptest.NewRecorder()
			ctrl.HandleCreateRequest(response, httptest.NewRequest("POST", "/api/requests", strings.NewReader(string(body))))
			if response.Code != http.StatusOK {
				t.Fatal(response.Body.String())
			}
			var execution struct {
				Execution store.Request `json:"execution"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &execution); err != nil {
				t.Fatal(err)
			}
			if execution.Execution.ResponseBody != tc.path {
				t.Fatalf("got %q, want %q", execution.Execution.ResponseBody, tc.path)
			}
			detail, err := db.GetRequestDetail(int64(execution.Execution.ID))
			if err != nil {
				t.Fatal(err)
			}
			if detail.Execution.URL != target.URL+tc.path {
				t.Fatalf("URL not persisted: %q", detail.Execution.URL)
			}
		})
	}
}
