package controller

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
	"slices"
	"strconv"
	"strings"
	"time"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/sqlutil"
	"docker-log-parser/pkg/store"

	"github.com/gorilla/mux"
	"github.com/jomei/notionapi"
)

// HandleExplain executes SQL EXPLAIN analysis
func (c *Controller) HandleExplain(w http.ResponseWriter, r *http.Request) {
	var req sqlexplain.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := sqlexplain.Explain(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleSaveTrace saves a trace with associated logs and SQL queries
func (c *Controller) HandleSaveTrace(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var input struct {
		Name               string             `json:"name"`
		TraceID            string             `json:"traceId"`
		RequestID          string             `json:"requestId"`
		Filters            []TraceFilterValue `json:"filters"`
		SelectedContainers []string           `json:"selectedContainers"`
		SearchQuery        string             `json:"searchQuery"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fieldFilters := []logstore.FieldFilter{}
	for _, filter := range input.Filters {
		fieldFilters = append(fieldFilters, logstore.FieldFilter{
			Name:  filter.Type,
			Value: filter.Value,
		})
		if filter.Type == "trace_id" && input.TraceID == "" {
			input.TraceID = filter.Value
		}
		if filter.Type == "request_id" && input.RequestID == "" {
			input.RequestID = filter.Value
		}
	}

	slog.Info("[save trace]", "name", input.Name, "traceId", input.TraceID, "requestId", input.RequestID, "filters", input.Filters, "selectedContainers", input.SelectedContainers)

	c.containerMutex.RLock()
	containers := c.containers
	containerIDNames := c.containerIDNames
	c.containerMutex.RUnlock()

	containerIDs := make([]string, 0, len(input.SelectedContainers))
	for _, container := range containers {
		for _, containerName := range input.SelectedContainers {
			if containerName == containerIDNames[container.ID] {
				containerIDs = append(containerIDs, container.ID)
				break
			}
		}
	}

	// Convert search query to search terms (split by whitespace and lowercase)
	searchTerms := []string{}
	if input.SearchQuery != "" {
		terms := strings.Fields(input.SearchQuery)
		for _, term := range terms {
			if term != "" {
				searchTerms = append(searchTerms, strings.ToLower(term))
			}
		}
	}

	logMessages := c.logStore.Filter(logstore.FilterOptions{
		FieldFilters: fieldFilters,
		ContainerIDs: containerIDs,
		SearchTerms:  searchTerms,
	}, 1000)

	messages := make([]logs.LogMessage, 0, len(logMessages))
	for _, msg := range logMessages {
		messages = append(messages, logs.LogMessage{
			Timestamp:   msg.Timestamp,
			ContainerID: msg.ContainerID,
			Entry:       deserializeLogEntry(msg),
		})
	}
	slog.Info("[save trace] found", "count", len(messages), "containerIDs", containerIDs)

	requestIDHeader := input.RequestID
	if requestIDHeader == "" {
		requestIDHeader = input.TraceID
	}
	if requestIDHeader == "" {
		requestIDHeader = input.Name
	}

	requestBody := ""
	minTimestamp := time.Now().Add(1 * time.Hour)
	maxTimestamp := time.Time{}
	statusCode := 200

	for _, logMsg := range logMessages {
		if logMsg.Timestamp.Before(minTimestamp) {
			minTimestamp = logMsg.Timestamp
		}
		if logMsg.Timestamp.After(maxTimestamp) {
			maxTimestamp = logMsg.Timestamp
		}
		if logMsg.Fields != nil {
			if query, ok := logMsg.Fields["Operations"]; ok {
				requestBody = query
			}
			if status, ok := logMsg.Fields["status"]; ok {
				statusCodeVal, err := strconv.Atoi(status)
				if err != nil {
					slog.Error("failed to parse status", "error", err)
				}
				statusCode = statusCodeVal
			}
		}
	}

	var durationMS int64
	if len(messages) > 1 {
		if !minTimestamp.IsZero() && !maxTimestamp.IsZero() {
			durationMS = maxTimestamp.Sub(minTimestamp).Milliseconds()
		}
	}

	exec := &store.Request{
		RequestIDHeader: requestIDHeader,
		RequestBody:     requestBody,
		StatusCode:      statusCode,
		DurationMS:      durationMS,
		ExecutedAt:      time.Now(),
		Name:            input.Name,
	}

	id, err := c.store.CreateRequest(exec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(messages) > 0 {
		if err := c.store.SaveRequestLogs(id, messages); err != nil {
			slog.Error("failed to save execution logs", "error", err)
		}
	}

	sqlQueries := sqlutil.ExtractSQLQueries(messages)
	if len(sqlQueries) > 0 {
		if err := c.store.SaveSQLQueries(id, sqlQueries); err != nil {
			slog.Error("failed to save SQL queries from trace", "error", err)
		} else {
			containerIDToConnectionString := map[string]string{}
			buildPortToServerMap := c.buildPortToServerMap(containers)

			for _, container := range containers {
				if slices.Contains(containerIDs, container.ID) {
					if len(container.Ports) > 0 {
						for _, port := range container.Ports {
							if buildPortToServerMap[port.PublicPort] != "" {
								containerIDToConnectionString[container.ID] = buildPortToServerMap[port.PublicPort]
								break
							}
						}
					}
				}
			}

			for i, q := range sqlQueries {
				connectionString := containerIDToConnectionString[q.ContainerID]
				if q.DurationMS > 2.0 && connectionString != "" {
					variables := make(map[string]string)
					if q.Variables != "" {
						var varsArray []interface{}
						if err := json.Unmarshal([]byte(q.Variables), &varsArray); err == nil {
							for idx, val := range varsArray {
								variables[fmt.Sprintf("%d", idx+1)] = fmt.Sprintf("%v", val)
							}
						} else {
							slog.Warn("failed to parse db.vars", "query_index", i, "error", err)
						}
					}

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

					planJSON, _ := json.Marshal(resp.QueryPlan)
					if err := c.store.UpdateQueryExplainPlan(id, q.QueryHash, string(planJSON)); err != nil {
						slog.Error("failed to save EXPLAIN plan", "query_index", i, "error", err)
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"message": "Trace saved successfully as execution",
	})
}

// HandleSQLDetail retrieves details for a specific SQL query by hash
func (c *Controller) HandleSQLDetail(w http.ResponseWriter, r *http.Request) {
	if c.store == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	queryHash := strings.TrimSpace(vars["hash"])
	if queryHash == "" {
		http.Error(w, "Invalid query hash", http.StatusBadRequest)
		return
	}

	detail, err := c.store.GetSQLQueryDetailByHash(queryHash)
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

// HandleSQLNotionExport exports SQL query details to Notion
func (c *Controller) HandleSQLNotionExport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queryHash := strings.TrimSpace(vars["hash"])

	if queryHash == "" {
		http.Error(w, "Invalid query hash", http.StatusBadRequest)
		return
	}

	detail, err := c.store.GetSQLQueryDetailByHash(queryHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "SQL query not found", http.StatusNotFound)
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

// buildPortToServerMap builds a map of container ports to database connection strings

func newTextRichText(content string) notionapi.RichText {
	return notionapi.RichText{
		Type:      notionapi.ObjectTypeText,
		PlainText: content,
		Text: &notionapi.Text{
			Content: content,
		},
	}
}

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

func newDividerBlock() notionapi.Block {
	return &notionapi.DividerBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeDivider,
		},
		Divider: notionapi.Divider{},
	}
}

func newToggleBlock(title string, children []notionapi.Block) notionapi.Block {
	return &notionapi.ToggleBlock{
		BasicBlock: notionapi.BasicBlock{
			Object: notionapi.ObjectTypeBlock,
			Type:   notionapi.BlockTypeToggle,
		},
		Toggle: notionapi.Toggle{
			RichText: []notionapi.RichText{newTextRichText(title)},
			Children: children,
		},
	}
}

func createNotionPage(apiKey, databaseID string, detail *store.SQLQueryDetail) (string, error) {
	formattedQuery := sqlutil.FormatSQLForDisplay(detail.Query)

	var executedAt string
	if len(detail.RelatedExecutions) > 0 {
		firstExec := detail.RelatedExecutions[0]
		executedAt = firstExec.ExecutedAt.Format(time.RFC3339)
	}

	title := fmt.Sprintf("SQL Query: %s on %s", detail.Operation, detail.TableName)

	type blockOrRaw interface{}
	var blocks []blockOrRaw

	blocks = append(blocks, newHeading2Block("Query Information"))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Operation: %s", detail.Operation)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Table: %s", detail.TableName)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Total Executions: %d", detail.TotalExecutions)))
	blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Average Duration: %.2fms", detail.AvgDuration)))

	if executedAt != "" {
		blocks = append(blocks, newBulletedListItemBlock(fmt.Sprintf("Last Executed: %s", executedAt)))
	}

	blocks = append(blocks, newHeading2Block("SQL Query"))
	statement := ""
	for _, line := range strings.Split(formattedQuery, "\n") {
		if len(statement)+len(line) > 1999 {
			blocks = append(blocks, newCodeBlock(statement, "sql"))
			statement = ""
		}
		statement += line + "\n"
	}
	if statement != "" {
		blocks = append(blocks, newCodeBlock(statement, "sql"))
	}

	if detail.ExplainPlan != "" {
		explainText := sqlutil.FormatExplainPlanForNotion(detail.ExplainPlan)
		blocks = append(blocks, newHeading2Block("EXPLAIN Plan"))

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
			if len(statement)+len(line) > 1999 {
				blocks = append(blocks, newCodeBlock(statement, "plain text"))
				statement = ""
			}
			statement += line + "\n"
		}
		if statement != "" {
			blocks = append(blocks, newCodeBlock(statement, "plain text"))
		}
	}

	if detail.IndexAnalysis != nil && (len(detail.IndexAnalysis.Recommendations) > 0 || len(detail.IndexAnalysis.SequentialScans) > 0) {
		blocks = append(blocks, newHeading2Block("Index & Scan Recommendations"))

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

		if len(detail.IndexAnalysis.Recommendations) > 0 {
			blocks = append(blocks, newHeading3Block(fmt.Sprintf("Index Recommendations (%d)", len(detail.IndexAnalysis.Recommendations))))

			for _, rec := range detail.IndexAnalysis.Recommendations {
				recText := fmt.Sprintf("[%s] %s on %s: %s", strings.ToUpper(rec.Priority), strings.Join(rec.Columns, ", "), rec.QueriedTable, rec.Reason)
				blocks = append(blocks, newBulletedListItemBlock(recText))

				if rec.SQLCommand != "" {
					blocks = append(blocks, newCodeBlock(rec.SQLCommand, "sql"))
				}
			}
		}
	}

	children := make([]notionapi.Block, 0, len(blocks))
	for _, block := range blocks {
		switch b := block.(type) {
		case notionapi.Block:
			children = append(children, b)
		case map[string]interface{}:
			slog.Warn("skipping unsupported block type", "block", b)
		default:
			return "", fmt.Errorf("unknown block type: %T", block)
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

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

func generateDaliboExplainLink(explainPlan, query, title string) string {
	if explainPlan == "" {
		return ""
	}

	formData := url.Values{}
	formData.Set("title", truncateText(title, 200))
	formData.Set("plan", explainPlan)
	formData.Set("query", truncateText(query, 5000))

	req, err := http.NewRequest("POST", "https://explain.dalibo.com/new", strings.NewReader(formData.Encode()))
	if err != nil {
		slog.Error("failed to create request", "error", err)
		return "https://explain.dalibo.com/new"
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	redirectURL := ""
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			slog.Info("redirect", "redirect", req.URL.String())
			redirectURL = req.URL.String()
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
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

	body, err := io.ReadAll(resp.Body)
	slog.Info("body", "body", string(body))
	if err == nil {
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err == nil {
			slog.Info("result", "result", result)
			if urlStr, ok := result["url"].(string); ok {
				return urlStr
			}
		}

		urlPattern := regexp.MustCompile(`https://explain\.dalibo\.com/plan/[a-zA-Z0-9]+`)
		matches := urlPattern.FindString(string(body))
		slog.Info("matches", "matches", matches)
		if matches != "" {
			return matches
		}
	}

	return "https://explain.dalibo.com/new"
}
