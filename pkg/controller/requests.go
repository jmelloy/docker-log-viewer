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
	"strings"
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
		SampleID                 *uint  `json:"sampleId,omitempty"`
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
		RequestIDHeader:     requestIDHeader,
		RequestBody:         input.RequestData,
		ExecutedAt:          time.Now(),
		StatusCode:          0,
		IsSync:              input.Sync,
		BearerTokenOverride: input.BearerTokenOverride,
		DevIDOverride:       input.DevIDOverride,
		SampleID:            input.SampleID,
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

	pageURL, err := createNotionPageForRequest(notionAPIKey, notionDatabaseID, detail)
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
						if logFieldsI["experiment_type"] == "experimental" {
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

type experimentGroup struct {
	ExperimentID string
	Title        string
	Queries      []store.SQLQuery
}

func groupQueriesByExperimentID(queries []store.SQLQuery) []experimentGroup {
	// First sort queries
	sortedQueries := sortSQLQueries(queries)

	// Group by experiment_id, skipping queries without an experiment_id
	groupMap := make(map[string]*experimentGroup)
	var groupOrder []string

	for _, q := range sortedQueries {
		experimentID := ""
		experimentType := ""

		logFields := make(map[string]string)
		if err := json.Unmarshal([]byte(q.LogFields), &logFields); err == nil {
			experimentID = logFields["experiment_id"]
			experimentType = logFields["experiment_type"]
		}

		// Skip queries without an experiment_id
		if experimentID == "" {
			continue
		}

		if _, exists := groupMap[experimentID]; !exists {
			title := fmt.Sprintf("Experiment: %s", experimentID)
			if experimentType != "" {
				title = fmt.Sprintf("%s (%s)", title, experimentType)
			}
			groupMap[experimentID] = &experimentGroup{
				ExperimentID: experimentID,
				Title:        title,
				Queries:      []store.SQLQuery{},
			}
			groupOrder = append(groupOrder, experimentID)
		}
		groupMap[experimentID].Queries = append(groupMap[experimentID].Queries, q)
	}

	// Convert map to slice maintaining order
	var groups []experimentGroup
	for _, key := range groupOrder {
		g := *groupMap[key]
		// Sort queries within each group: experiment_type=experimental first
		sort.Slice(g.Queries, func(i, j int) bool {
			typeI := ""
			typeJ := ""
			logFieldsI := make(map[string]string)
			if err := json.Unmarshal([]byte(g.Queries[i].LogFields), &logFieldsI); err == nil {
				typeI = logFieldsI["experiment_type"]
			}
			logFieldsJ := make(map[string]string)
			if err := json.Unmarshal([]byte(g.Queries[j].LogFields), &logFieldsJ); err == nil {
				typeJ = logFieldsJ["experiment_type"]
			}
			// experimental comes first
			if typeI == "experimental" && typeJ != "experimental" {
				return true
			}
			if typeI != "experimental" && typeJ == "experimental" {
				return false
			}
			// If same type, maintain creation order
			return g.Queries[i].CreatedAt.Before(g.Queries[j].CreatedAt)
		})
		groups = append(groups, g)
	}

	return groups
}

func createNotionPageForRequest(apiKey, databaseID string, detail *store.RequestDetailResponse) (string, error) {
	// Build page title
	title := detail.Execution.Name
	if title == "" {
		title = detail.Execution.DisplayName
	}

	// Create blocks for the page content using jomei/notionapi types
	type blockOrRaw interface{}
	var blocks []blockOrRaw

	// Execution Information heading
	blocks = append(blocks, newHeading2Block("Execution Information"))

	// Metadata as bulleted list
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Status Code: %d", detail.Execution.StatusCode)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Duration: %dms", detail.Execution.DurationMS)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Executed At: %s", detail.Execution.ExecutedAt.Format(time.RFC3339))))

	// Add SQL Queries section if available
	if len(detail.SQLQueries) > 0 {
		blocks = append(blocks, newHeading2Block(fmt.Sprintf("SQL Queries (%d)", len(detail.SQLQueries))))

		// Group queries by experiment_id
		experimentGroups := groupQueriesByExperimentID(detail.SQLQueries)

		for _, group := range experimentGroups {
			// Build toggle children for this experiment group
			var toggleChildren []notionapi.Block

			for idx, q := range group.Queries {
				// Query header
				queryTitle := fmt.Sprintf("Query %d: %s on %s", idx+1, q.GraphQLOperation, q.QueriedTable)
				toggleChildren = append(toggleChildren, newHeading3Block(queryTitle))

				// Get the query to display (interpolated if variables exist)
				displayQuery := q.Query
				if q.Variables != "" {
					displayQuery = sqlutil.InterpolateSQLQuery(q.Query, q.Variables)
				}
				formattedQuery := sqlutil.FormatSQLForDisplay(displayQuery)

				// Add SQL query as code block
				statement := ""
				for _, line := range strings.Split(formattedQuery, "\n") {
					if len(statement)+len(line) > 1999 {
						toggleChildren = append(toggleChildren, newCodeBlock(statement, "sql"))
						statement = ""
					}
					statement += line + "\n"
				}
				if statement != "" {
					toggleChildren = append(toggleChildren, newCodeBlock(statement, "sql"))
				}

				// Add query metadata
				toggleChildren = append(toggleChildren, newBulletedListItemBlock(fmt.Sprintf("Duration: %.2fms", q.DurationMS)))
				toggleChildren = append(toggleChildren, newBulletedListItemBlock(fmt.Sprintf("Rows: %d", q.Rows)))

				logFields := make(map[string]string)
				if err := json.Unmarshal([]byte(q.LogFields), &logFields); err == nil {
					for key, value := range logFields {
						toggleChildren = append(toggleChildren, newBulletedListItemBlock(fmt.Sprintf("%s: %s", key, value)))
					}
				}

				// Add EXPLAIN plan if available
				if q.ExplainPlan != "" {
					daliboLink := generateDaliboExplainLink(q.ExplainPlan, formattedQuery, q.DisplayName())
					slog.Info("dalibo link", "daliboLink", daliboLink)
					if daliboLink != "" {
						toggleChildren = append(toggleChildren, newParagraphBlock(
							newTextRichText("View in Dalibo EXPLAIN: "),
							newTextRichTextWithLink(q.DisplayName(), daliboLink),
						))
					}

					explainText := sqlutil.FormatExplainPlanForNotion(q.ExplainPlan)
					toggleChildren = append(toggleChildren, newHeading3Block("EXPLAIN Plan"))

					// Add EXPLAIN plan as code block
					statement = ""
					for _, line := range strings.Split(explainText, "\n") {
						if len(statement)+len(line) > 1999 {
							toggleChildren = append(toggleChildren, newCodeBlock(statement, "json"))
							statement = ""
						}
						statement += line + "\n"
					}
					if statement != "" {
						toggleChildren = append(toggleChildren, newCodeBlock(statement, "json"))
					}
				}

				// Add separator between queries within the toggle
				if idx < len(group.Queries)-1 {
					toggleChildren = append(toggleChildren, newDividerBlock())
				}
			}

			// Create toggle block for this experiment group
			toggleTitle := fmt.Sprintf("%s (%d queries)", group.Title, len(group.Queries))
			blocks = append(blocks, newToggleBlock(toggleTitle, toggleChildren))
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
