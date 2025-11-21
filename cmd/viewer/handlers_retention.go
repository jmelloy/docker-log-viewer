package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/store"
)

// ============================================================================
// HTTP Handlers - Container Retention Settings
// ============================================================================

func (wa *WebApp) handleRetention(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		retentions, err := wa.store.ListContainerRetentions()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(retentions)
	case http.MethodPost:
		var retention store.ContainerRetention
		if err := json.NewDecoder(r.Body).Decode(&retention); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := wa.store.SaveContainerRetention(&retention); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, container := range wa.containers {
			if container.Name == retention.ContainerName {
				containerID := container.ID
				slog.Info("setting container retention", "containerID", containerID, "retention", retention)
				wa.logStore.SetContainerRetention(containerID, logstore.ContainerRetentionPolicy{
					Type:  retention.RetentionType,
					Value: retention.RetentionValue,
				})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(retention)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) handleRetentionDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract container name from path
	containerName := strings.TrimPrefix(r.URL.Path, "/api/retention/")
	if containerName == "" {
		http.Error(w, "Container name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		retention, err := wa.store.GetContainerRetention(containerName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if retention == nil {
			http.Error(w, "Retention settings not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(retention)
	case http.MethodDelete:
		if err := wa.store.DeleteContainerRetention(containerName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
