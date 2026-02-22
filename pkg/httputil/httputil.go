package httputil

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/utils"
)

// GenerateRequestID generates a random 8-character hex string for request tracking
func GenerateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// MakeHTTPRequest executes an HTTP POST request with the given parameters
// Returns: statusCode, responseBody, responseHeaders (as JSON), error
func MakeHTTPRequest(url string, data []byte, requestID, bearerToken, devID, experimentalMode string) (int, string, string, error) {
	// Replace localhost with host.docker.internal if running in Docker
	url = utils.ReplaceLocalhostWithDockerHost(url)

	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return 0, "", "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", requestID)

	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if devID != "" {
		req.Header.Set("X-GlueDev-UserID", devID)
	}
	if experimentalMode != "" {
		req.Header.Set("x-glue-experimental-mode", experimentalMode)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	// Capture response headers as JSON
	headersJSON, _ := json.Marshal(resp.Header)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", string(headersJSON), err
	}

	return resp.StatusCode, string(bodyBytes), string(headersJSON), nil
}

// CollectLogsForRequest searches the log store for logs matching the given request ID
func CollectLogsForRequest(requestID string, logStore *logstore.LogStore, timeout time.Duration) []logs.ContainerMessage {
	// Wait for logs to arrive
	time.Sleep(timeout)

	// Search LogStore for matching request ID
	filters := []logstore.FieldFilter{
		{Name: "request_id", Value: requestID},
	}
	storeResults := logStore.SearchByFields(filters, 100000)

	// Convert pointers to values
	collected := make([]logs.ContainerMessage, 0, len(storeResults))
	for _, storeMsg := range storeResults {
		collected = append(collected, *storeMsg)
	}

	return collected
}

// ContainsErrorsKey recursively checks if the data contains an "errors" key
// Returns: hasErrors, errorMessage, keyPath
func ContainsErrorsKey(data any, key string) (bool, string, string) {
	errors := map[string]string{}

	switch v := data.(type) {
	case map[string]any:
		if _, exists := v["errors"]; exists {
			if errors, ok := v["errors"].([]any); ok && len(errors) > 0 {
				if first, ok := errors[0].(map[string]any); ok {
					message, _ := json.Marshal(first)
					return true, string(message), key
				}
			}
			return true, "Unknown error", key
		}
		for k, value := range v {
			if hasErrors, message, key := ContainsErrorsKey(value, fmt.Sprintf("%s.%s", key, k)); hasErrors {
				return true, message, key
			}
		}
	case []any:
		for i, item := range v {
			if hasErrors, message, key := ContainsErrorsKey(item, fmt.Sprintf("%s.[%d]", key, i)); hasErrors {
				errors[key] = message
			}
		}
	}
	if len(errors) > 0 {
		var errorsString strings.Builder
		for k, v := range errors {
			errorsString.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
		return true, errorsString.String(), ""
	}
	return false, "", key
}
