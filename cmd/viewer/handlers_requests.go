package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"docker-log-parser/pkg/httputil"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/sqlutil"
	"docker-log-parser/pkg/store"

	"github.com/jomei/notionapi"
)

func (wa *WebApp) handleExecute(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var input struct {
		ServerID            *uint  `json:"serverId"`
		URLOverride         string `json:"urlOverride,omitempty"`
		BearerTokenOverride string `json:"bearerTokenOverride,omitempty"`
		DevIDOverride       string `json:"devIdOverride,omitempty"`
		RequestData         string `json:"requestData"`
		Sync                bool   `json:"sync,omitempty"`
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

	// Get server info
	server, err := wa.store.GetServer(int64(*input.ServerID))
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
	connectionString := ""
	if server.DefaultDatabase != nil {
		connectionString = server.DefaultDatabase.ConnectionString
	}

	// Apply overrides
	if input.URLOverride != "" {
		url = input.URLOverride
	}
	if input.BearerTokenOverride != "" {
		bearerToken = input.BearerTokenOverride
	}
	if input.DevIDOverride != "" {
		devID = input.DevIDOverride
	}

	// Generate request ID
	requestIDHeader := httputil.GenerateRequestID()

	// Create execution record immediately with pending status
	execution := &store.ExecutedRequest{
		ServerID:            input.ServerID,
		RequestIDHeader:     requestIDHeader,
		RequestBody:         input.RequestData,
		ExecutedAt:          time.Now(),
		StatusCode:          0, // 0 indicates pending
		IsSync:              input.Sync,
		BearerTokenOverride: input.BearerTokenOverride,
		DevIDOverride:       input.DevIDOverride,
	}

	// Save execution immediately
	execID, err := wa.store.CreateExecution(execution)
	if err != nil {
		slog.Error("failed to save execution", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Define execution logic as a function
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

		// Check for GraphQL errors in response body (even with 200 status)
		if execution.Error == "" && statusCode == 200 && responseBody != "" {
			var responseData interface{}
			if err := json.Unmarshal([]byte(responseBody), &responseData); err == nil {
				if hasErrors, message, key := httputil.ContainsErrorsKey(responseData, ""); hasErrors {
					slog.Warn("GraphQL errors in response", "message", message, "key", key)
					msg := fmt.Sprintf("GraphQL errors: %s", message)
					if key != "" {
						msg += fmt.Sprintf(" at %s", key)
					}
					execution.Error = msg
				}
			}
		}

		// Update execution with results
		execution.ID = uint(execID)
		if err := wa.store.UpdateExecution(execution); err != nil {
			slog.Error("failed to update execution", "error", err)
			return
		}

		slog.Info("request executed", "header_id", requestIDHeader, "status", statusCode, "duration_ms", execution.DurationMS)

		// Collect logs
		collectedLogs := httputil.CollectLogsForRequest(requestIDHeader, wa.logStore, 10*time.Second)

		// Save logs
		if len(collectedLogs) > 0 {
			if err := wa.store.SaveExecutionLogs(execID, collectedLogs); err != nil {
				slog.Error("failed to save logs", "error", err)
			}
		}

		// Extract and save SQL queries
		sqlQueries := sqlutil.ExtractSQLQueries(collectedLogs)
		if len(sqlQueries) > 0 {
			if err := wa.store.SaveSQLQueries(execID, sqlQueries); err != nil {
				slog.Error("failed to save SQL queries", "error", err)
			}

			// Auto-execute EXPLAIN for queries taking longer than 2ms
			for i, q := range sqlQueries {
				if q.DurationMS > 2.0 {
					// Parse db.vars to extract parameters
					variables := make(map[string]string)
					if q.Variables != "" {
						var varsArray []interface{}
						if err := json.Unmarshal([]byte(q.Variables), &varsArray); err == nil {
							// Convert array values to map with 1-based indices
							for idx, val := range varsArray {
								variables[fmt.Sprintf("%d", idx+1)] = fmt.Sprintf("%v", val)
							}
						} else {
							slog.Warn("failed to parse db.vars", "query_index", i, "error", err)
						}
					}

					// Execute EXPLAIN
					req := sqlexplain.Request{
						Query:            q.Query,
						Variables:        variables,
						ConnectionString: connectionString,
					}
					resp := sqlexplain.Explain(req)

					if resp.Error != "" {
						slog.Warn("auto-EXPLAIN failed", "query_index", i, "error", resp.Error)
						continue
					}

					// Save the EXPLAIN plan to database
					planJSON, _ := json.Marshal(resp.QueryPlan)
					if err := wa.store.UpdateQueryExplainPlan(execID, q.QueryHash, string(planJSON)); err != nil {
						slog.Error("failed to save EXPLAIN plan", "query_index", i, "error", err)
					}
				}
			}
		}
	}

	// If sync is true, execute synchronously and return response
	if input.Sync {
		executeRequest()

		// Return the execution result with response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "completed",
			"executionId":  execID,
			"responseBody": execution.ResponseBody,
			"statusCode":   execution.StatusCode,
			"durationMs":   execution.DurationMS,
			"error":        execution.Error,
		})
	} else {
		// Execute HTTP request in background
		go executeRequest()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "started",
			"executionId": execID,
		})
	}
}

// ============================================================================
// HTTP Handlers - Request & Execution Management
// ============================================================================

// Request management handlers
func (wa *WebApp) handleRequests(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		wa.listRequests(w, r)
	case http.MethodPost:
		wa.createRequest(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) listRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := wa.store.ListRequests()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

func (wa *WebApp) createRequest(w http.ResponseWriter, r *http.Request) {
	// Parse the incoming request which may have server fields or a serverID
	var input struct {
		Name        string `json:"name"`
		ServerID    *uint  `json:"serverId,omitempty"`
		URL         string `json:"url,omitempty"`
		BearerToken string `json:"bearerToken,omitempty"`
		DevID       string `json:"devId,omitempty"`
		RequestData string `json:"requestData"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If URL is provided but no serverID, create a new server
	var serverID *uint
	if input.ServerID != nil {
		// Use existing server
		serverID = input.ServerID
	} else if input.URL != "" {
		// Create new server with URL and credentials
		server := &store.Server{
			Name:        input.URL, // Use URL as name for now
			URL:         input.URL,
			BearerToken: input.BearerToken,
			DevID:       input.DevID,
		}

		sid, err := wa.store.CreateServer(server)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create server: %v", err), http.StatusInternalServerError)
			return
		}
		sidUint := uint(sid)
		serverID = &sidUint
	}

	// Create the request
	req := &store.SampleQuery{
		Name:        input.Name,
		ServerID:    serverID,
		RequestData: input.RequestData,
	}

	id, err := wa.store.CreateRequest(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (wa *WebApp) handleRequestDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		req, err := wa.store.GetRequest(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if req == nil {
			http.Error(w, "Request not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(req)
	case http.MethodDelete:
		if err := wa.store.DeleteRequest(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (wa *WebApp) handleExecuteRequest(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	path = strings.TrimSuffix(path, "/execute")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	// Parse request body for overrides
	var input struct {
		ServerID            *uint  `json:"serverId,omitempty"`
		URLOverride         string `json:"urlOverride,omitempty"`
		BearerTokenOverride string `json:"bearerTokenOverride,omitempty"`
		DevIDOverride       string `json:"devIdOverride,omitempty"`
		RequestDataOverride string `json:"requestDataOverride,omitempty"`
	}

	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			// Ignore decode errors for backward compatibility
			slog.Warn("failed to decode execute request body", "error", err)
		}
	}

	// Execute request in background with overrides
	executionID := wa.executeRequestWithOverrides(id, input.ServerID, input.URLOverride, input.BearerTokenOverride, input.DevIDOverride, input.RequestDataOverride)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "started",
		"executionId": executionID,
	})
}

func (wa *WebApp) handleExecutions(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract request ID from query param
	requestIDStr := r.URL.Query().Get("request_id")
	if requestIDStr == "" {
		http.Error(w, "request_id parameter required", http.StatusBadRequest)
		return
	}

	requestID, err := strconv.ParseInt(requestIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request_id", http.StatusBadRequest)
		return
	}

	executions, err := wa.store.ListExecutions(requestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

func (wa *WebApp) handleAllExecutions(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	limit := 20
	if limitStr := query.Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offset := 0
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	search := query.Get("search")

	executions, total, err := wa.store.ListAllExecutions(limit, offset, search, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"executions": executions,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (wa *WebApp) handleExecutionDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/executions/")

	// Check if this is an export-notion request
	if strings.HasSuffix(path, "/export-notion") {
		wa.handleExecutionNotionExport(w, r)
		return
	}

	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	detail, err := wa.store.GetExecutionDetail(id)
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

func (wa *WebApp) handleSQLDetail(w http.ResponseWriter, r *http.Request) {
	if wa.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Extract query hash from path
	path := strings.TrimPrefix(r.URL.Path, "/api/sql/")

	// Check if this is an export-notion request
	if strings.HasSuffix(path, "/export-notion") {
		wa.handleSQLNotionExport(w, r)
		return
	}

	queryHash := strings.TrimSpace(path)
	if queryHash == "" {
		http.Error(w, "Invalid query hash", http.StatusBadRequest)
		return
	}

	detail, err := wa.store.GetSQLQueryDetailByHash(queryHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "SQL query not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

// handleSQLNotionExport exports SQL query details to Notion
func (wa *WebApp) handleSQLNotionExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract query hash from path (remove "/export-notion" suffix)
	path := strings.TrimPrefix(r.URL.Path, "/api/sql/")
	queryHash := strings.TrimSuffix(path, "/export-notion")
	queryHash = strings.TrimSpace(queryHash)

	if queryHash == "" {
		http.Error(w, "Invalid query hash", http.StatusBadRequest)
		return
	}

	// Get SQL query details
	detail, err := wa.store.GetSQLQueryDetailByHash(queryHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "SQL query not found", http.StatusNotFound)
		return
	}

	// Get Notion API key and database ID from environment
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

	// Create Notion page
	pageURL, err := createNotionPage(notionAPIKey, notionDatabaseID, detail)
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

// handleExecutionNotionExport exports execution details to Notion
func (wa *WebApp) handleExecutionNotionExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract execution ID from path (remove "/export-notion" suffix)
	path := strings.TrimPrefix(r.URL.Path, "/api/executions/")
	path = strings.TrimSuffix(path, "/export-notion")
	id, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		http.Error(w, "Invalid execution ID", http.StatusBadRequest)
		return
	}

	// Get execution details
	detail, err := wa.store.GetExecutionDetail(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "Execution not found", http.StatusNotFound)
		return
	}

	// Get Notion API key and database ID from environment
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

	// Create Notion page
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

// Helper functions to create Notion blocks using jomei/notionapi types

// newTextRichText creates a RichText object with plain text content
func newTextRichText(content string) notionapi.RichText {
	return notionapi.RichText{
		Type:      notionapi.ObjectTypeText,
		PlainText: content,
		Text: &notionapi.Text{
			Content: content,
		},
	}
}

// newTextRichTextWithLink creates a RichText object with a link
func newTextRichTextWithLink(content, url string) notionapi.RichText {
	return notionapi.RichText{
		Type:      notionapi.ObjectTypeText,
		PlainText: content,
		Text: &notionapi.Text{
			Content: content,
			Link: &notionapi.Link{
				Url: url,
			},
		},
	}
}

// newHeading2Block creates a heading_2 block
func newHeading2Block(text string) notionapi.Block {
	return &notionapi.Heading2Block{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeHeading2,
		},
		Heading2: notionapi.Heading{
			RichText: []notionapi.RichText{newTextRichText(text)},
		},
	}
}

// newHeading3Block creates a heading_3 block
func newHeading3Block(text string) notionapi.Block {
	return &notionapi.Heading3Block{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeHeading3,
		},
		Heading3: notionapi.Heading{
			RichText: []notionapi.RichText{newTextRichText(text)},
		},
	}
}

// newHeading4Block creates a heading_4 block as a map (since jomei/notionapi doesn't support heading_4)
func newHeading4Block(text string) map[string]interface{} {
	return map[string]interface{}{
		"object": "block",
		"type":   "heading_4",
		"heading_4": map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{
					"type": "text",
					"text": map[string]string{
						"content": text,
					},
				},
			},
		},
	}
}

// newBulletedListItemBlock creates a bulleted_list_item block
func newBulletedListItemBlock(text string) notionapi.Block {
	return &notionapi.BulletedListItemBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeBulletedListItem,
		},
		BulletedListItem: notionapi.ListItem{
			RichText: []notionapi.RichText{newTextRichText(text)},
		},
	}
}

// newParagraphBlock creates a paragraph block
func newParagraphBlock(texts ...notionapi.RichText) notionapi.Block {
	return &notionapi.ParagraphBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeParagraph,
		},
		Paragraph: notionapi.Paragraph{
			RichText: texts,
		},
	}
}

// newCodeBlock creates a code block
func newCodeBlock(content, language string) notionapi.Block {
	return &notionapi.CodeBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeCode,
		},
		Code: notionapi.Code{
			RichText: []notionapi.RichText{newTextRichText(content)},
			Language: language,
		},
	}
}

// newDividerBlock creates a divider block
func newDividerBlock() notionapi.Block {
	return &notionapi.DividerBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeDivider,
		},
		Divider: notionapi.Divider{},
	}
}

// createNotionPage creates a new page in Notion with the SQL query details
func createNotionPage(apiKey, databaseID string, detail *store.SQLQueryDetail) (string, error) {
	// Format SQL query with basic formatting
	formattedQuery := sqlutil.FormatSQLForDisplay(detail.Query)

	// Get execution info
	var requestID string
	var executedAt string
	if len(detail.RelatedExecutions) > 0 {
		firstExec := detail.RelatedExecutions[0]
		requestID = firstExec.RequestIDHeader
		executedAt = firstExec.ExecutedAt.Format(time.RFC3339)
	}

	// Build page content
	title := fmt.Sprintf("SQL Query: %s on %s", detail.Operation, detail.TableName)

	// Create blocks for the page content using jomei/notionapi types
	// We'll use a slice of interface{} to hold both notionapi.Block and raw blocks (for heading_4)
	type blockOrRaw interface{}
	var blocks []blockOrRaw

	// Metadata heading
	blocks = append(blocks, newHeading2Block("Query Information"))

	// Metadata as bulleted list
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Operation: %s", detail.Operation)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Table: %s", detail.TableName)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Total Executions: %d", detail.TotalExecutions)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Average Duration: %.2fms", detail.AvgDuration)))

	// Add request ID and execution date if available
	if requestID != "" {
		blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Request ID: %s", requestID)))
	}

	if executedAt != "" {
		blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Last Executed: %s", executedAt)))
	}

	// SQL Query section
	blocks = append(blocks, newHeading2Block("SQL Query"))
	statement := ""
	for _, line := range strings.Split(formattedQuery, "\n") {
		if len(statement)+len(line) > 2000 {
			blocks = append(blocks, newCodeBlock(statement, "sql"))
			statement = ""
		}
		statement += line + "\n"
	}
	if statement != "" {
		blocks = append(blocks, newCodeBlock(statement, "sql"))
	}

	// EXPLAIN Plan section (if available)
	if detail.ExplainPlan != "" {
		explainText := sqlutil.FormatExplainPlanForNotion(detail.ExplainPlan)
		blocks = append(blocks, newHeading2Block("EXPLAIN Plan"))

		// Add Dalibo explain link
		daliboLink := generateDaliboExplainLink(detail.ExplainPlan, formattedQuery, title)
		slog.Info("dalibo link", "daliboLink", daliboLink)
		if daliboLink != "" {
			blocks = append(blocks, newParagraphBlock(
				newTextRichText("View in Dalibo EXPLAIN: "),
				newTextRichTextWithLink(title, daliboLink),
			))
		}

		statement = ""
		for _, line := range strings.Split(explainText, "\n") {
			if len(statement)+len(line) > 2000 {
				blocks = append(blocks, newCodeBlock(statement, "plain text"))
				statement = ""
			}
			statement += line + "\n"
		}
		if statement != "" {
			blocks = append(blocks, newCodeBlock(statement, "plain text"))
		}
	}

	// Index/Seq Scan Recommendations section (if available)
	if detail.IndexAnalysis != nil && (len(detail.IndexAnalysis.Recommendations) > 0 || len(detail.IndexAnalysis.SequentialScans) > 0) {
		blocks = append(blocks, newHeading2Block("Index & Scan Recommendations"))

		// Add sequential scan issues
		if len(detail.IndexAnalysis.SequentialScans) > 0 {
			blocks = append(blocks, newHeading3Block(fmt.Sprintf("Sequential Scans (%d)", len(detail.IndexAnalysis.SequentialScans))))

			for _, scan := range detail.IndexAnalysis.SequentialScans {
				scanText := fmt.Sprintf("Table: %s | Rows: %.0f | Cost: %.2f | Occurrences: %d",
					scan.QueriedTable, scan.EstimatedRows, scan.Cost, scan.Occurrences)
				if scan.FilterCondition != "" {
					scanText += fmt.Sprintf(" | Filter: %s", scan.FilterCondition)
				}
				blocks = append(blocks, newBulletedListItemBlock(scanText))
			}
		}

		// Add index recommendations
		if len(detail.IndexAnalysis.Recommendations) > 0 {
			blocks = append(blocks, newHeading3Block(fmt.Sprintf("Index Recommendations (%d)", len(detail.IndexAnalysis.Recommendations))))

			for _, rec := range detail.IndexAnalysis.Recommendations {
				recText := fmt.Sprintf("[%s] %s on %s: %s", strings.ToUpper(rec.Priority), strings.Join(rec.Columns, ", "), rec.QueriedTable, rec.Reason)
				blocks = append(blocks, newBulletedListItemBlock(recText))

				// Add SQL command as code block
				if rec.SQLCommand != "" {
					blocks = append(blocks, newCodeBlock(rec.SQLCommand, "sql"))
				}
			}
		}
	}

	// Convert blocks to notionapi.Block format (mix of notionapi.Block and raw blocks for heading_4)
	children := make([]notionapi.Block, 0, len(blocks))
	for _, block := range blocks {
		switch b := block.(type) {
		case notionapi.Block:
			children = append(children, b)
		case map[string]interface{}:
			// For heading_4 and other unsupported blocks, we'll need to append them after page creation
			// For now, skip unsupported blocks in initial creation
			slog.Warn("skipping unsupported block type (heading_4)", "block", b)
		default:
			return "", fmt.Errorf("unknown block type: %T", block)
		}
	}

	// Create Notion client
	client := notionapi.NewClient(notionapi.Token(apiKey))

	// Build page creation request
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

	// Create the page
	page, err := client.Page.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to create Notion page: %w", err)
	}

	// Append heading_4 blocks if any were skipped
	var heading4Blocks []map[string]interface{}
	for _, block := range blocks {
		if m, ok := block.(map[string]interface{}); ok {
			heading4Blocks = append(heading4Blocks, m)
		}
	}

	if len(heading4Blocks) > 0 {
		// Note: heading_4 blocks are not supported by jomei/notionapi
		// They would need to be appended using Block.AppendChildren with raw JSON
		// For now, log a warning
		slog.Warn("heading_4 blocks are not supported by jomei/notionapi and were skipped", "count", len(heading4Blocks))
	}

	return page.URL, nil
}

// createNotionPageForExecution creates a new page in Notion with execution details and SQL queries
func createNotionPageForExecution(apiKey, databaseID string, detail *store.ExecutionDetail) (string, error) {
	// Build page title
	title := detail.Execution.Name
	if title == "" {
		title = detail.Execution.RequestIDHeader
	}
	if title == "" {
		title = "Execution Details"
	}

	// Create blocks for the page content using jomei/notionapi types
	type blockOrRaw interface{}
	var blocks []blockOrRaw

	// Execution Information heading
	blocks = append(blocks, newHeading2Block("Execution Information"))

	// Metadata as bulleted list
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Request ID: %s", detail.Execution.RequestIDHeader)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Status Code: %d", detail.Execution.StatusCode)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Duration: %dms", detail.Execution.DurationMS)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Executed At: %s", detail.Execution.ExecutedAt.Format(time.RFC3339))))

	// Add SQL Queries section if available
	if len(detail.SQLQueries) > 0 {
		blocks = append(blocks, newHeading2Block(fmt.Sprintf("SQL Queries (%d)", len(detail.SQLQueries))))

		// Add each SQL query
		for idx, q := range detail.SQLQueries {
			// Query header
			queryTitle := fmt.Sprintf("Query %d: %s on %s", idx+1, q.Operation, q.QueriedTable)
			blocks = append(blocks, newHeading3Block(queryTitle))

			// Get the query to display (interpolated if variables exist)
			displayQuery := q.Query
			if q.Variables != "" {
				displayQuery = sqlutil.InterpolateSQLQuery(q.Query, q.Variables)
			}
			formattedQuery := sqlutil.FormatSQLForDisplay(displayQuery)

			// Add SQL query as code block
			statement := ""
			for _, line := range strings.Split(formattedQuery, "\n") {
				if len(statement)+len(line) > 2000 {
					blocks = append(blocks, newCodeBlock(statement, "sql"))
					statement = ""
				}
				statement += line + "\n"
			}
			if statement != "" {
				blocks = append(blocks, newCodeBlock(statement, "sql"))
			}

			// Add query metadata
			blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Duration: %.2fms", q.DurationMS)))

			if q.GraphQLOperation != "" {
				blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("GraphQL Operation: %s", q.GraphQLOperation)))
			}

			// Add EXPLAIN plan if available
			if q.ExplainPlan != "" {
				explainText := sqlutil.FormatExplainPlanForNotion(q.ExplainPlan)
				blocks = append(blocks, newHeading4Block("EXPLAIN Plan"))

				// Add EXPLAIN plan as code block
				statement = ""
				for _, line := range strings.Split(explainText, "\n") {
					if len(statement)+len(line) > 2000 {
						blocks = append(blocks, newCodeBlock(statement, "json"))
						statement = ""
					}
					statement += line + "\n"
				}
				if statement != "" {
					blocks = append(blocks, newCodeBlock(statement, "json"))
				}
			}

			// Add separator between queries
			if idx < len(detail.SQLQueries)-1 {
				blocks = append(blocks, newDividerBlock())
			}
		}
	}

	// Convert blocks to notionapi.Block format (mix of notionapi.Block and raw blocks for heading_4)
	children := make([]notionapi.Block, 0, len(blocks))
	var heading4Blocks []map[string]interface{}
	for _, block := range blocks {
		switch b := block.(type) {
		case notionapi.Block:
			children = append(children, b)
		case map[string]interface{}:
			// Collect heading_4 blocks to append later
			heading4Blocks = append(heading4Blocks, b)
		default:
			return "", fmt.Errorf("unknown block type: %T", block)
		}
	}

	// Create Notion client
	client := notionapi.NewClient(notionapi.Token(apiKey))

	// Build page creation request
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

	// Create the page
	page, err := client.Page.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to create Notion page: %w", err)
	}

	// Append heading_4 blocks if any were collected
	if len(heading4Blocks) > 0 {
		// Note: heading_4 blocks would need to be appended using Block.AppendChildren
		// For now, we'll log a warning since jomei/notionapi doesn't support heading_4
		slog.Warn("heading_4 blocks are not supported by jomei/notionapi and were skipped", "count", len(heading4Blocks))
	}

	return page.URL, nil
}

// truncateText truncates text to maxLen characters, adding ellipsis if truncated
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// generateDaliboExplainLink generates a shareable link to Dalibo's EXPLAIN tool
// by posting the plan data to their API
func generateDaliboExplainLink(explainPlan, query, title string) string {
	if explainPlan == "" {
		return ""
	}

	// Dalibo's explain tool accepts plan data via POST form
	// We'll create a form and POST it to get a shareable link
	// However, since we can't easily get the redirect URL from a POST,
	// we'll create a link that opens the explain tool with instructions

	// Try to POST to Dalibo's API to create a shareable link
	formData := url.Values{}
	formData.Set("title", truncateText(title, 200))
	formData.Set("plan", explainPlan)
	formData.Set("query", truncateText(query, 5000)) // Limit query size

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://explain.dalibo.com/new", strings.NewReader(formData.Encode()))
	if err != nil {
		slog.Error("failed to create request", "error", err)
		// If request creation fails, return base URL
		return "https://explain.dalibo.com/new"
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	redirectURL := ""
	// Make request with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			slog.Info("redirect", "redirect", req.URL.String())
			redirectURL = req.URL.String()

			// Follow redirects
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// If request fails, return base URL
		slog.Error("failed to make request", "error", err)
		return "https://explain.dalibo.com/new"
	}
	defer resp.Body.Close()

	if redirectURL != "" {
		return redirectURL
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			return location
		}
	}

	slog.Info("response", "response", resp)
	slog.Info("redirectURL", "redirectURL", redirectURL)

	// Read response body to check for a link
	body, err := io.ReadAll(resp.Body)
	slog.Info("body", "body", string(body))
	if err == nil {
		// Try to extract URL from response
		// Dalibo might return a JSON response with a URL
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err == nil {
			slog.Info("result", "result", result)
			if urlStr, ok := result["url"].(string); ok {
				return urlStr
			}
		}
		// Try to find URL in HTML response
		urlPattern := regexp.MustCompile(`https://explain\.dalibo\.com/plan/[a-zA-Z0-9]+`)
		matches := urlPattern.FindString(string(body))
		slog.Info("matches", "matches", matches)
		if matches != "" {
			return matches
		}
	}

	// Fallback: return base URL
	return "https://explain.dalibo.com/new"
}
