//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"context"
	"docker-log-parser/pkg/logs"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type CompareConfig struct {
	URL1        string
	URL2        string
	DataFile    string
	Output      string
	Timeout     time.Duration
	BearerToken string
	DevID       string
}

type RequestResult struct {
	URL         string
	RequestID   string
	Duration    time.Duration
	StatusCode  int
	PostData    string
	Logs        []logs.LogMessage
	SQLAnalysis *SQLAnalysis
	Error       error
}

type SQLQuery struct {
	Query      string
	Duration   float64
	Table      string
	Operation  string
	Rows       int
	Normalized string
}

type SQLAnalysis struct {
	TotalQueries    int
	UniqueQueries   int
	AvgDuration     float64
	TotalDuration   float64
	SlowestQueries  []SQLQuery
	FrequentQueries []QueryGroup
	NPlusOneIssues  []QueryGroup
	TablesAccessed  map[string]int
}

type QueryGroup struct {
	Normalized  string
	Count       int
	Example     SQLQuery
	AvgDuration float64
}

func main() {
	config := parseFlags()

	if err := runComparison(config); err != nil {
		log.Fatalf("Comparison failed: %v", err)
	}
}

func parseFlags() CompareConfig {
	var config CompareConfig

	flag.StringVar(&config.URL1, "url1", "", "First URL to test (required)")
	flag.StringVar(&config.URL2, "url2", "", "Second URL to test (required)")
	flag.StringVar(&config.DataFile, "data", "", "GraphQL or JSON data file (required)")
	flag.StringVar(&config.Output, "output", "comparison.html", "Output HTML file")
	flag.DurationVar(&config.Timeout, "timeout", 10*time.Second, "Timeout for log collection")
	flag.StringVar(&config.BearerToken, "token", os.Getenv("BEARER_TOKEN"), "Bearer token for authentication")
	flag.StringVar(&config.DevID, "dev-id", os.Getenv("X_GLUE_DEV_ID"), "X-Glue-Dev-Id header value")
	flag.Parse()

	if config.URL1 == "" || config.URL2 == "" || config.DataFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	return config
}

func runComparison(config CompareConfig) error {
	// Read data file
	data, err := os.ReadFile(config.DataFile)
	if err != nil {
		return fmt.Errorf("failed to read data file: %w", err)
	}

	// Create Docker client for log monitoring
	docker, err := logs.NewDockerClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer docker.Close()

	ctx := context.Background()

	// Start log collection
	logChan := make(chan logs.LogMessage, 10000)
	containers, err := docker.ListRunningContainers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		if err := docker.StreamLogs(ctx, c.ID, logChan); err != nil {
			log.Printf("Failed to stream logs for container %s: %v", c.ID, err)
		}
	}

	// Test URL1
	log.Printf("Testing URL1: %s", config.URL1)
	result1 := testURL(config.URL1, data, logChan, config.Timeout, &config)

	// Wait a bit between requests
	time.Sleep(2 * time.Second)

	// Test URL2
	log.Printf("Testing URL2: %s", config.URL2)
	result2 := testURL(config.URL2, data, logChan, config.Timeout, &config)

	// Generate HTML report
	if err := generateHTML(config.Output, result1, result2, string(data)); err != nil {
		return fmt.Errorf("failed to generate HTML: %w", err)
	}

	log.Printf("Comparison report generated: %s", config.Output)
	return nil
}

func testURL(url string, data []byte, logChan <-chan logs.LogMessage, timeout time.Duration, config *CompareConfig) *RequestResult {
	result := &RequestResult{
		URL:  url,
		Logs: make([]logs.LogMessage, 0),
	}

	// Format the post data nicely
	var prettyData bytes.Buffer
	if err := json.Indent(&prettyData, data, "", "  "); err == nil {
		result.PostData = prettyData.String()
	} else {
		result.PostData = string(data)
	}

	// Determine content type
	contentType := "application/json"
	var jsonData map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		// Not valid JSON, might be GraphQL
		contentType = "application/json"
	}

	// Make request
	startTime := time.Now()
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		result.Error = err
		return result
	}

	req.Header.Set("Content-Type", contentType)

	// Add authentication headers
	if config.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.BearerToken)
	}
	if config.DevID != "" {
		req.Header.Set("X-Glue-Dev-Id", config.DevID)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err
		return result
	}
	defer resp.Body.Close()

	result.Duration = time.Since(startTime)
	result.StatusCode = resp.StatusCode
	result.RequestID = resp.Header.Get("X-Request-Id")

	// Read response body
	io.Copy(io.Discard, resp.Body)

	if result.RequestID == "" {
		result.Error = fmt.Errorf("no X-Request-Id header found in response")
		return result
	}

	log.Printf("Request ID: %s, Status: %d, Duration: %v", result.RequestID, result.StatusCode, result.Duration)

	// Collect logs for this request ID
	result.Logs = collectLogs(result.RequestID, logChan, timeout)
	log.Printf("Collected %d logs for request %s", len(result.Logs), result.RequestID)

	// Analyze SQL queries
	result.SQLAnalysis = analyzeSQLQueries(result.Logs)

	return result
}

func collectLogs(requestID string, logChan <-chan logs.LogMessage, timeout time.Duration) []logs.LogMessage {
	logs := make([]logs.LogMessage, 0)
	deadline := time.After(timeout)

	// Keep collecting until timeout or no more logs
	lastLogTime := time.Now()
	noLogTimeout := 2 * time.Second

	for {
		select {
		case <-deadline:
			return logs
		case <-time.After(noLogTimeout):
			if time.Since(lastLogTime) > noLogTimeout {
				return logs
			}
		case msg := <-logChan:
			lastLogTime = time.Now()
			if matchesRequestID(msg, requestID) {
				logs = append(logs, msg)
			}
		}
	}
}

func matchesRequestID(msg logs.LogMessage, requestID string) bool {
	if msg.Entry == nil || msg.Entry.Fields == nil {
		return false
	}

	// Check common request ID field names
	for _, field := range []string{"request_id", "requestId", "requestID", "req_id"} {
		if val, ok := msg.Entry.Fields[field]; ok && val == requestID {
			return true
		}
	}

	return false
}

func analyzeSQLQueries(logs []logs.LogMessage) *SQLAnalysis {
	queries := extractSQLQueries(logs)

	if len(queries) == 0 {
		return &SQLAnalysis{
			TablesAccessed: make(map[string]int),
		}
	}

	analysis := &SQLAnalysis{
		TotalQueries:   len(queries),
		TablesAccessed: make(map[string]int),
	}

	// Calculate total and average duration
	for _, q := range queries {
		analysis.TotalDuration += q.Duration
		analysis.TablesAccessed[q.Table]++
	}
	analysis.AvgDuration = analysis.TotalDuration / float64(len(queries))

	// Group queries by normalized form
	queryGroups := make(map[string]*QueryGroup)
	for _, q := range queries {
		if _, exists := queryGroups[q.Normalized]; !exists {
			queryGroups[q.Normalized] = &QueryGroup{
				Normalized: q.Normalized,
				Count:      0,
				Example:    q,
			}
		}
		group := queryGroups[q.Normalized]
		group.Count++
		group.AvgDuration = (group.AvgDuration*float64(group.Count-1) + q.Duration) / float64(group.Count)
	}

	analysis.UniqueQueries = len(queryGroups)

	// Get slowest queries
	sortedQueries := make([]SQLQuery, len(queries))
	copy(sortedQueries, queries)
	sort.Slice(sortedQueries, func(i, j int) bool {
		return sortedQueries[i].Duration > sortedQueries[j].Duration
	})
	if len(sortedQueries) > 5 {
		analysis.SlowestQueries = sortedQueries[:5]
	} else {
		analysis.SlowestQueries = sortedQueries
	}

	// Get most frequent queries
	frequentQueries := make([]QueryGroup, 0, len(queryGroups))
	for _, group := range queryGroups {
		frequentQueries = append(frequentQueries, *group)
	}
	sort.Slice(frequentQueries, func(i, j int) bool {
		return frequentQueries[i].Count > frequentQueries[j].Count
	})
	if len(frequentQueries) > 5 {
		analysis.FrequentQueries = frequentQueries[:5]
	} else {
		analysis.FrequentQueries = frequentQueries
	}

	// Detect N+1 issues (queries executed more than 5 times)
	for _, group := range frequentQueries {
		if group.Count > 5 {
			analysis.NPlusOneIssues = append(analysis.NPlusOneIssues, group)
		}
	}

	return analysis
}

func extractSQLQueries(logs []logs.LogMessage) []SQLQuery {
	queries := make([]SQLQuery, 0)
	sqlRegex := regexp.MustCompile(`\[sql\]:\s*(.+)`)

	for _, log := range logs {
		if log.Entry == nil {
			continue
		}

		message := log.Entry.Message
		if strings.Contains(message, "[sql]") {
			matches := sqlRegex.FindStringSubmatch(message)
			if len(matches) > 1 {
				query := SQLQuery{
					Query:     strings.TrimSpace(matches[1]),
					Table:     getField(log.Entry.Fields, "db.table", "unknown"),
					Operation: getField(log.Entry.Fields, "db.operation", "unknown"),
				}

				if durStr := getField(log.Entry.Fields, "duration", "0"); durStr != "" {
					fmt.Sscanf(durStr, "%f", &query.Duration)
				}

				if rowsStr := getField(log.Entry.Fields, "db.rows", "0"); rowsStr != "" {
					fmt.Sscanf(rowsStr, "%d", &query.Rows)
				}

				query.Normalized = normalizeQuery(query.Query)
				queries = append(queries, query)
			}
		}
	}

	return queries
}

func getField(fields map[string]string, key, defaultVal string) string {
	if fields == nil {
		return defaultVal
	}
	if val, ok := fields[key]; ok {
		return val
	}
	return defaultVal
}

func normalizeQuery(query string) string {
	// Replace parameter placeholders
	normalized := regexp.MustCompile(`\$\d+`).ReplaceAllString(query, "$N")
	// Replace quoted strings
	normalized = regexp.MustCompile(`'[^']*'`).ReplaceAllString(normalized, "'?'")
	// Replace numbers
	normalized = regexp.MustCompile(`\d+`).ReplaceAllString(normalized, "N")
	// Normalize whitespace
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func generateHTML(filename string, result1, result2 *RequestResult, postData string) error {
	tmpl := template.Must(template.New("comparison-report.tmpl").Funcs(template.FuncMap{
		"escapeHTML": template.HTMLEscapeString,
		"formatDuration": func(d time.Duration) string {
			return d.Round(time.Millisecond).String()
		},
		"formatFloat": func(f float64) string {
			return fmt.Sprintf("%.2f", f)
		},
	}).ParseFiles("comparison-report.tmpl"))

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	data := struct {
		Result1   *RequestResult
		Result2   *RequestResult
		PostData  string
		Generated time.Time
	}{
		Result1:   result1,
		Result2:   result2,
		PostData:  postData,
		Generated: time.Now(),
	}

	return tmpl.Execute(f, data)
}
