package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"docker-log-parser/pkg/httputil"
	"docker-log-parser/pkg/sqlutil"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
	"github.com/jomei/notionapi"
)

// HandleCreateRequest creates a new request
func (c *Controller) HandleCreateRequest(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var input struct {
		ServerID                 *uint  `json:"serverId"`
		URLOverride              string `json:"urlOverride,omitempty"`
		BearerTokenOverride      string `json:"bearerTokenOverride,omitempty"`
		DevIDOverride            string `json:"devIdOverride,omitempty"`
		ExperimentalModeOverride string `json:"experimentalModeOverride,omitempty"`
		RequestData              string `json:"requestData"`
		Sync                     bool   `json:"sync,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if input.RequestData == "" {
		http.Error(w, "requestData is required", http.StatusBadRequest)
		return
	}

	if input.ServerID == nil {
		http.Error(w, "serverId is required", http.StatusBadRequest)
		return
	}

	server, err := c.store.GetServer(int64(*input.ServerID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if server == nil {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	url := server.URL
	bearerToken := server.BearerToken
	devID := server.DevID
	experimentalMode := server.ExperimentalMode

	if input.URLOverride != "" {
		url = input.URLOverride
	}
	if input.BearerTokenOverride != "" {
		bearerToken = input.BearerTokenOverride
	}
	if input.DevIDOverride != "" {
		devID = input.DevIDOverride
	}
	if input.ExperimentalModeOverride != "" {
		experimentalMode = input.ExperimentalModeOverride
	}

	requestIDHeader := httputil.GenerateRequestID()

	execution := &store.Request{
		ServerID:            input.ServerID,
		RequestBody:         input.RequestData,
		ExecutedAt:          time.Now(),
		StatusCode:          0,
		IsSync:              input.Sync,
		BearerTokenOverride: input.BearerTokenOverride,
		DevIDOverride:       input.DevIDOverride,
	}

	execID, err := c.store.CreateRequest(execution)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	executeRequest := func() {
		startTime := time.Now()
		statusCode, responseBody, responseHeaders, err := httputil.MakeHTTPRequest(url, []byte(input.RequestData), requestIDHeader, bearerToken, devID, experimentalMode)
		execution.DurationMS = time.Since(startTime).Milliseconds()
		execution.StatusCode = statusCode
		execution.ResponseBody = responseBody
		execution.ResponseHeaders = responseHeaders

		if err != nil {
			execution.Error = err.Error()
		}

		execution.ID = uint(execID)
		if err := c.store.UpdateRequest(execution); err != nil {
			slog.Error("failed to update execution", "error", err)
		}
	}

	if input.Sync {
		executeRequest()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "completed",
			"executionId": execID,
			"execution":   execution,
		})
	} else {
		go executeRequest()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "started",
			"executionId": execID,
		})
	}
}

// HandleListRequestsBySample lists executions for a request
func (c *Controller) HandleListRequestsBySample(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	type QueryParams struct {
		RequestID int64 `schema:"request_id,required"`
	}

	var params QueryParams
	if err := c.decoder.Decode(&params, r.URL.Query()); err != nil {
		http.Error(w, "request_id parameter required", http.StatusBadRequest)
		return
	}

	executions, err := c.store.ListRequestsBySample(params.RequestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

// HandleListAllRequests lists all executions with pagination
func (c *Controller) HandleListAllRequests(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	type QueryParams struct {
		Limit  int    `schema:"limit"`
		Offset int    `schema:"offset"`
		Search string `schema:"search"`
	}

	params := QueryParams{
		Limit:  20,
		Offset: 0,
	}

	if err := c.decoder.Decode(&params, r.URL.Query()); err != nil {
		slog.Warn("failed to decode query parameters", "error", err)
	}

	executions, total, err := c.store.ListRequests(params.Limit, params.Offset, params.Search, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"executions": executions,
		"total":      total,
		"limit":      params.Limit,
		"offset":     params.Offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetRequestDetail gets execution details by ID
func (c *Controller) HandleGetRequestDetail(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	detail, err := c.store.GetRequestDetail(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

// HandleNotionExportForRequest exports request to Notion
func (c *Controller) HandleNotionExportForRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	detail, err := c.store.GetRequestDetail(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	notionAPIKey := os.Getenv("NOTION_API_KEY")
	notionDatabaseID := os.Getenv("NOTION_DATABASE_ID")

	if notionAPIKey == "" {
		http.Error(w, "Notion API key not configured. Set NOTION_API_KEY environment variable.", http.StatusServiceUnavailable)
		return
	}

	if notionDatabaseID == "" {
		http.Error(w, "Notion database ID not configured. Set NOTION_DATABASE_ID environment variable.", http.StatusServiceUnavailable)
		return
	}

	pageURL, err := createNotionPageForExecution(notionAPIKey, notionDatabaseID, detail)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create Notion page: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":     pageURL,
		"message": "Successfully exported to Notion",
	})
}

// Helper functions

func sortSQLQueries(queries []store.SQLQuery) []store.SQLQuery {
	sort.Slice(queries, func(i, j int) bool {
		if queries[i].GraphQLOperation != queries[j].GraphQLOperation {
			return queries[i].GraphQLOperation < queries[j].GraphQLOperation
		}

		if queries[i].LogFields != "" && queries[j].LogFields != "" {
			logFieldsI := make(map[string]string)
			if err := json.Unmarshal([]byte(queries[i].LogFields), &logFieldsI); err == nil {
				logFieldsJ := make(map[string]string)
				if err := json.Unmarshal([]byte(queries[j].LogFields), &logFieldsJ); err == nil {
					if logFieldsI["experiment_id"] == logFieldsJ["experiment_id"] {
						if logFieldsI["experimental_type"] == "experimental" {
							return true
						}
						return false
					}
				}
			}
		}
		return queries[i].CreatedAt.Before(queries[j].CreatedAt)
	})
	return queries
}

func createNotionPageForExecution(apiKey, databaseID string, detail *store.RequestDetailResponse) (string, error) {
	sortedQueries := sortSQLQueries(detail.SQLQueries)

	title := "Execution"
	if detail.Execution.Name != "" {
		title = fmt.Sprintf("Execution: %s", detail.Execution.Name)
	}

	type blockOrRaw interface{}
	var blocks []blockOrRaw

	blocks = append(blocks, newHeading2Block("Request Information"))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Status Code: %d", detail.Execution.StatusCode)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Duration: %dms", detail.Execution.DurationMS)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Executed At: %s", detail.Execution.ExecutedAt.Format(time.RFC3339))))

	if detail.Execution.RequestBody != "" {
		blocks = append(blocks, newHeading2Block("Request Body"))
		requestBody := detail.Execution.RequestBody
		if len(requestBody) > 4000 {
			requestBody = requestBody[:4000] + "... (truncated)"
		}
		blocks = append(blocks, newCodeBlock(requestBody, "graphql"))
	}

	if len(detail.Logs) > 0 {
		blocks = append(blocks, newHeading2Block(fmt.Sprintf("Logs (%d)", len(detail.Logs))))

		logText := ""
		for _, log := range detail.Logs {
			logLine := fmt.Sprintf("[%s] %s\n", log.Timestamp.Format(time.RFC3339), log.Message)
			if len(logText)+len(logLine) > 1900 {
				blocks = append(blocks, newCodeBlock(logText, "plain text"))
				logText = ""
			}
			logText += logLine
		}
		if logText != "" {
			blocks = append(blocks, newCodeBlock(logText, "plain text"))
		}
	}

	if len(sortedQueries) > 0 {
		blocks = append(blocks, newHeading2Block(fmt.Sprintf("SQL Queries (%d)", len(sortedQueries))))

		for i, query := range sortedQueries {
			if i >= 20 {
				blocks = append(blocks, newParagraphBlock(newTextRichText(fmt.Sprintf("... and %d more queries", len(sortedQueries)-20))))
				break
			}

			queryHeader := fmt.Sprintf("%s on %s (%.2fms)", query.Operation, query.TableName, query.DurationMS)
			blocks = append(blocks, newHeading3Block(queryHeader))

			formattedQuery := sqlutil.FormatSQLForDisplay(query.Query)
			statement := ""
			for _, line := range formattedQuery[:min(len(formattedQuery), 2000)] {
				if len(statement)+1 > 1900 {
					blocks = append(blocks, newCodeBlock(statement, "sql"))
					statement = ""
				}
				statement += string(line)
			}
			if statement != "" {
				blocks = append(blocks, newCodeBlock(statement, "sql"))
			}
		}
	}

	children := make([]notionapi.Block, 0, len(blocks))
	for _, block := range blocks {
		switch b := block.(type) {
		case notionapi.Block:
			children = append(children, b)
		default:
			slog.Warn("skipping unsupported block type", "block", b)
		}
	}

	client := notionapi.NewClient(notionapi.Token(apiKey))

	req := &notionapi.PageCreateRequest{
		Parent: notionapi.Parent{
			Type:       notionapi.ParentTypeDatabaseID,
			DatabaseID: notionapi.DatabaseID(databaseID),
		},
		Properties: notionapi.Properties{
			"Name": notionapi.TitleProperty{
				Type:  notionapi.PropertyTypeTitle,
				Title: []notionapi.RichText{newTextRichText(truncateText(title, 100))},
			},
		},
		Children: children,
	}

	page, err := client.Page.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to create Notion page: %w", err)
	}

	return page.URL, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
