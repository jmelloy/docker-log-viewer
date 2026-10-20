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
	"github.com/gorilla/mux"
)

func TestCustomHeaders(t *testing.T) {
	db, err := store.NewStore(filepath.Join(t.TempDir(), "headers.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(r.Header)
	}))
	defer target.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctrl := NewController(nil, logstore.NewLogStore(100, time.Hour), db, ctx, cancel, nil)
	send := func(handler http.HandlerFunc, payload any) *httptest.ResponseRecorder {
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequest("POST", "/", strings.NewReader(string(body))))
		return response
	}
	serverResponse := send(ctrl.HandleCreateServer, map[string]any{
		"name": "custom", "url": target.URL, "devId": "legacy-user", "experimentalMode": "legacy-mode",
		"bearerToken": "default-token", "headers": map[string]string{"X-Layer": "server", "X-Server": "kept"},
	})
	if serverResponse.Code != http.StatusOK {
		t.Fatal(serverResponse.Body.String())
	}
	var serverID struct {
		ID uint `json:"id"`
	}
	json.Unmarshal(serverResponse.Body.Bytes(), &serverID)
	saved, err := db.GetServer(int64(serverID.ID))
	if err != nil || saved.Headers["X-Layer"] != "server" {
		t.Fatalf("server headers not saved: %+v, %v", saved, err)
	}
	sampleResponse := send(ctrl.HandleCreateSampleQuery, map[string]any{
		"name": "sample", "serverId": serverID.ID, "requestData": "{}",
		"headers": map[string]string{"x-layer": "sample", "X-Sample": "kept"},
	})
	if sampleResponse.Code != http.StatusOK {
		t.Fatal(sampleResponse.Body.String())
	}
	var sampleID struct {
		ID uint `json:"id"`
	}
	json.Unmarshal(sampleResponse.Body.Bytes(), &sampleID)
	sample, err := db.GetSampleQuery(int64(sampleID.ID))
	if err != nil || sample.Headers["x-layer"] != "sample" {
		t.Fatalf("sample headers not saved: %+v, %v", sample, err)
	}
	for _, tc := range []struct {
		name, layer string
		sample      bool
		headers     map[string]string
	}{
		{name: "server defaults", layer: "server"},
		{name: "sample defaults", layer: "sample", sample: true},
		{name: "execution override", layer: "execution", sample: true, headers: map[string]string{"X-LAYER": "execution", "X-GlueDev-UserID": "custom-user", "X-Glue-Experimental-Mode": "custom-mode", "Authorization": "Bearer custom"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]any{"serverId": serverID.ID, "requestData": "{}", "sync": true, "headers": tc.headers, "urlOverride": "/explicit"}
			if tc.sample {
				payload["sampleId"] = sampleID.ID
			}
			response := send(ctrl.HandleCreateRequest, payload)
			if response.Code != http.StatusOK {
				t.Fatal(response.Body.String())
			}
			var result struct {
				Execution store.Request `json:"execution"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			var received http.Header
			if err := json.Unmarshal([]byte(result.Execution.ResponseBody), &received); err != nil {
				t.Fatal(err)
			}
			if received.Get("X-Layer") != tc.layer || received.Get("X-Server") != "kept" {
				t.Fatalf("wrong headers: %v", received)
			}
			if tc.sample && received.Get("X-Sample") != "kept" {
				t.Fatalf("sample lost with explicit endpoint: %v", received)
			}
			if received.Get("X-Request-Id") != result.Execution.RequestIDHeader || result.Execution.RequestIDHeader == "" {
				t.Fatal("correlation ID lost")
			}
			wantUser, wantMode, wantToken := "legacy-user", "legacy-mode", "Bearer default-token"
			if tc.headers != nil {
				wantUser, wantMode, wantToken = "custom-user", "custom-mode", "Bearer custom"
			}
			if received.Get("X-Gluedev-Userid") != wantUser || received.Get("X-Glue-Experimental-Mode") != wantMode || received.Get("Authorization") != wantToken {
				t.Fatalf("auth headers changed: %v", received)
			}
			persisted, err := db.GetRequest(int64(result.Execution.ID))
			if err != nil || persisted.Headers["X-Layer"] != tc.layer {
				t.Fatalf("headers not persisted: %+v, %v", persisted, err)
			}
		})
	}
	updated := send(func(w http.ResponseWriter, r *http.Request) {
		ctrl.HandleUpdateServer(w, mux.SetURLVars(r, map[string]string{"id": fmt.Sprint(serverID.ID)}))
	}, map[string]any{
		"name": "edited", "url": target.URL, "headers": map[string]string{"X-GlueDev-UserID": "legacy-user", "X-Glue-Experimental-Mode": "legacy-mode", "X-Edited": "saved"},
	})
	if updated.Code != http.StatusNoContent {
		t.Fatal(updated.Body.String())
	}
	saved, err = db.GetServer(int64(serverID.ID))
	if err != nil || saved.Headers["X-Edited"] != "saved" || saved.DevID != "" || saved.ExperimentalMode != "" {
		t.Fatalf("edited headers not saved: %+v, %v", saved, err)
	}

	for name, handler := range map[string]http.HandlerFunc{
		"server": ctrl.HandleCreateServer, "sample": ctrl.HandleCreateSampleQuery, "execute": ctrl.HandleCreateRequest,
		"edit server": func(w http.ResponseWriter, r *http.Request) {
			ctrl.HandleUpdateServer(w, mux.SetURLVars(r, map[string]string{"id": fmt.Sprint(serverID.ID)}))
		},
	} {
		t.Run("reject invalid headers/"+name, func(t *testing.T) {
			response := send(handler, map[string]any{"serverId": serverID.ID, "requestData": "{}", "headers": map[string]string{"X-Request-Id": "override"}})
			if response.Code != http.StatusBadRequest {
				t.Fatalf("got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
