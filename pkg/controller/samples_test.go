package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
)

func TestSavedRequestEditing(t *testing.T) {
	db, err := store.NewStore(filepath.Join(t.TempDir(), "saved.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctrl := &Controller{store: db}
	router := mux.NewRouter()
	router.HandleFunc("/api/samples/", ctrl.HandleCreateSampleQuery).Methods("POST")
	router.HandleFunc("/api/samples/{id}", ctrl.HandleUpdateSampleQuery).Methods("PUT")
	router.HandleFunc("/api/samples/{id}", ctrl.HandleDeleteSampleQuery).Methods("DELETE")
	call := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	created := call("POST", "/api/samples/", `{"name":"Original","url":"https://example.com/graphql","headers":{"X-Test":"old"},"urlOverride":"/old","requestData":"{}"}`, http.StatusOK)
	var result struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != 1 {
		t.Fatalf("unexpected ID: %d", result.ID)
	}
	original, err := db.GetSampleQuery(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateRequest(&store.Request{SampleID: &result.ID, RequestIDHeader: "history", RequestBody: "{}"}); err != nil {
		t.Fatal(err)
	}
	call("PUT", "/api/samples/1", `{"name":"Updated","requestData":"{\"query\":\"{ users { id } }\"}","headers":{"X-Test":"new"},"urlOverride":"/new","serverId":1}`, http.StatusOK)
	updated, err := db.GetSampleQuery(1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Updated" || updated.Headers["X-Test"] != "new" || updated.URLOverride != "/new" || updated.ServerID == nil || !strings.Contains(updated.RequestData, "users") || !updated.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("incorrect update: %+v", updated)
	}
	call("PUT", "/api/samples/1", `{"name":"bad","headers":{"Bad\nHeader":"value"}}`, http.StatusBadRequest)
	call("PUT", "/api/samples/1", `{"headers":`, http.StatusBadRequest)
	unchanged, _ := db.GetSampleQuery(1)
	if unchanged.Name != "Updated" {
		t.Fatal("invalid update modified the saved request")
	}
	call("PUT", "/api/samples/1", `{"name":"","requestData":"","headers":{},"urlOverride":"","serverId":null}`, http.StatusOK)
	cleared, err := db.GetSampleQuery(1)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Name != "" || cleared.RequestData != "" || len(cleared.Headers) != 0 || cleared.URLOverride != "" || cleared.ServerID != nil {
		t.Fatalf("empty fields were not saved: %+v", cleared)
	}
	call("PUT", "/api/samples/1", `{"name":"New server","url":"https://other.example/graphql","bearerToken":"example","requestData":"{}"}`, http.StatusOK)
	withServer, err := db.GetSampleQuery(1)
	if err != nil {
		t.Fatal(err)
	}
	if withServer.Server == nil || withServer.Server.URL != "https://other.example/graphql" || withServer.ServerID == nil || *withServer.ServerID == *original.ServerID {
		t.Fatalf("server replacement failed: %+v", withServer)
	}
	for _, id := range []string{"bad", "0", "-1"} {
		call("PUT", "/api/samples/"+id, `{}`, http.StatusBadRequest)
	}
	call("PUT", "/api/samples/999", `{}`, http.StatusNotFound)
	call("DELETE", "/api/samples/1", "", http.StatusNoContent)
	deleted, err := db.GetSampleQuery(1)
	if err != nil || deleted != nil {
		t.Fatalf("request was not deleted: %v, %v", deleted, err)
	}
	history, err := db.GetRequestDetail(1)
	if err != nil || history == nil || history.Execution.RequestIDHeader != "history" {
		t.Fatalf("editing/deleting lost execution history: %v, %v", history, err)
	}
	call("PUT", "/api/samples/1", `{}`, http.StatusNotFound)
}
