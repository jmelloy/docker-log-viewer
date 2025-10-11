//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"context"
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
	URL1     string
	URL2     string
	DataFile string
	Output   string
	Timeout  time.Duration
}

type RequestResult struct {
	URL         string
	RequestID   string
	Duration    time.Duration
	StatusCode  int
	Logs        []LogMessage
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
	TotalQueries  int
	UniqueQueries int
	AvgDuration   float64
	TotalDuration float64
	SlowestQueries []SQLQuery
	FrequentQueries []QueryGroup
	NPlusOneIssues []QueryGroup
	TablesAccessed map[string]int
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
	docker, err := NewDockerClient()
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer docker.Close()
	
	ctx := context.Background()
	
	// Start log collection
	logChan := make(chan LogMessage, 10000)
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
	result1 := testURL(config.URL1, data, logChan, config.Timeout)
	
	// Wait a bit between requests
	time.Sleep(2 * time.Second)
	
	// Test URL2
	log.Printf("Testing URL2: %s", config.URL2)
	result2 := testURL(config.URL2, data, logChan, config.Timeout)
	
	// Generate HTML report
	if err := generateHTML(config.Output, result1, result2); err != nil {
		return fmt.Errorf("failed to generate HTML: %w", err)
	}
	
	log.Printf("Comparison report generated: %s", config.Output)
	return nil
}

func testURL(url string, data []byte, logChan <-chan LogMessage, timeout time.Duration) *RequestResult {
	result := &RequestResult{
		URL:  url,
		Logs: make([]LogMessage, 0),
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

func collectLogs(requestID string, logChan <-chan LogMessage, timeout time.Duration) []LogMessage {
	logs := make([]LogMessage, 0)
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

func matchesRequestID(msg LogMessage, requestID string) bool {
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

func analyzeSQLQueries(logs []LogMessage) *SQLAnalysis {
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

func extractSQLQueries(logs []LogMessage) []SQLQuery {
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
					Query:      strings.TrimSpace(matches[1]),
					Table:      getField(log.Entry.Fields, "db.table", "unknown"),
					Operation:  getField(log.Entry.Fields, "db.operation", "unknown"),
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

func generateHTML(filename string, result1, result2 *RequestResult) error {
	tmpl := template.Must(template.New("report").Funcs(template.FuncMap{
		"escapeHTML": template.HTMLEscapeString,
		"formatDuration": func(d time.Duration) string {
			return d.Round(time.Millisecond).String()
		},
		"formatFloat": func(f float64) string {
			return fmt.Sprintf("%.2f", f)
		},
	}).Parse(htmlTemplate))
	
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	
	data := struct {
		Result1 *RequestResult
		Result2 *RequestResult
		Generated time.Time
	}{
		Result1: result1,
		Result2: result2,
		Generated: time.Now(),
	}
	
	return tmpl.Execute(f, data)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>URL Comparison Report</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
            background: #0d1117;
            color: #c9d1d9;
            padding: 20px;
            line-height: 1.6;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        h1 { color: #58a6ff; margin-bottom: 20px; font-size: 2rem; }
        h2 { color: #58a6ff; margin: 30px 0 15px; font-size: 1.5rem; }
        h3 { color: #8b949e; margin: 20px 0 10px; font-size: 1.2rem; }
        .comparison { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
        .result {
            background: #161b22;
            border: 1px solid #30363d;
            border-radius: 6px;
            padding: 20px;
        }
        .result.error { border-color: #f85149; }
        .header { margin-bottom: 20px; }
        .header .url {
            font-size: 1.1rem;
            font-weight: bold;
            color: #58a6ff;
            word-break: break-all;
        }
        .header .request-id {
            color: #79c0ff;
            font-family: monospace;
            font-size: 0.9rem;
            margin-top: 5px;
        }
        .metrics {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 10px;
            margin-bottom: 20px;
        }
        .metric {
            background: #0d1117;
            padding: 10px;
            border-radius: 4px;
            border: 1px solid #30363d;
        }
        .metric-label {
            font-size: 0.75rem;
            color: #8b949e;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .metric-value {
            font-size: 1.25rem;
            font-weight: bold;
            color: #58a6ff;
            margin-top: 5px;
        }
        .metric-value.error { color: #f85149; }
        .metric-value.success { color: #3fb950; }
        .section {
            margin-top: 20px;
            padding: 15px;
            background: #0d1117;
            border-radius: 4px;
            border: 1px solid #30363d;
        }
        .section h4 {
            color: #8b949e;
            font-size: 0.85rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            margin-bottom: 10px;
        }
        .query-item {
            background: #161b22;
            border: 1px solid #30363d;
            border-radius: 4px;
            padding: 10px;
            margin-bottom: 10px;
        }
        .query-header {
            display: flex;
            justify-content: space-between;
            margin-bottom: 8px;
            font-size: 0.85rem;
        }
        .query-count { color: #58a6ff; font-weight: bold; }
        .query-duration { color: #79c0ff; }
        .query-slow { color: #f85149; }
        .query-text {
            font-family: monospace;
            font-size: 0.85rem;
            color: #c9d1d9;
            background: #0d1117;
            padding: 8px;
            border-radius: 3px;
            overflow-x: auto;
            margin-bottom: 8px;
        }
        .query-meta {
            display: flex;
            gap: 15px;
            font-size: 0.75rem;
            color: #8b949e;
        }
        .table-badge {
            display: inline-block;
            background: #1f6feb;
            color: #fff;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 0.75rem;
            margin-right: 5px;
            margin-bottom: 5px;
        }
        .table-count {
            margin-left: 5px;
            opacity: 0.8;
        }
        .log-line {
            font-family: monospace;
            font-size: 0.8rem;
            padding: 4px 8px;
            border-left: 2px solid #30363d;
            margin-bottom: 2px;
            white-space: pre-wrap;
            word-break: break-all;
        }
        .log-line.error { border-left-color: #f85149; }
        .log-line.warn { border-left-color: #d29922; }
        .log-line.info { border-left-color: #58a6ff; }
        .error-message {
            color: #f85149;
            background: #1f1416;
            padding: 10px;
            border-radius: 4px;
            border: 1px solid #f85149;
        }
        .timestamp {
            color: #8b949e;
            font-size: 0.75rem;
            margin-bottom: 20px;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 10px;
            margin-bottom: 15px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>URL Comparison Report</h1>
        <div class="timestamp">Generated: {{.Generated.Format "2006-01-02 15:04:05 MST"}}</div>
        
        <div class="comparison">
            <!-- Result 1 -->
            <div class="result{{if .Result1.Error}} error{{end}}">
                <div class="header">
                    <div class="url">URL 1: {{.Result1.URL}}</div>
                    {{if .Result1.RequestID}}
                    <div class="request-id">Request ID: {{.Result1.RequestID}}</div>
                    {{end}}
                </div>
                
                {{if .Result1.Error}}
                <div class="error-message">Error: {{.Result1.Error}}</div>
                {{else}}
                
                <div class="metrics">
                    <div class="metric">
                        <div class="metric-label">Status Code</div>
                        <div class="metric-value {{if eq .Result1.StatusCode 200}}success{{else}}error{{end}}">
                            {{.Result1.StatusCode}}
                        </div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">Duration</div>
                        <div class="metric-value">{{formatDuration .Result1.Duration}}</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">Logs Collected</div>
                        <div class="metric-value">{{len .Result1.Logs}}</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">SQL Queries</div>
                        <div class="metric-value">{{.Result1.SQLAnalysis.TotalQueries}}</div>
                    </div>
                </div>
                
                {{if .Result1.SQLAnalysis}}
                {{if gt .Result1.SQLAnalysis.TotalQueries 0}}
                <div class="section">
                    <h4>SQL Analysis</h4>
                    <div class="stats-grid">
                        <div class="metric">
                            <div class="metric-label">Total Queries</div>
                            <div class="metric-value">{{.Result1.SQLAnalysis.TotalQueries}}</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Unique Queries</div>
                            <div class="metric-value">{{.Result1.SQLAnalysis.UniqueQueries}}</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Avg Duration</div>
                            <div class="metric-value">{{formatFloat .Result1.SQLAnalysis.AvgDuration}}ms</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Total Duration</div>
                            <div class="metric-value">{{formatFloat .Result1.SQLAnalysis.TotalDuration}}ms</div>
                        </div>
                    </div>
                </div>
                
                {{if .Result1.SQLAnalysis.SlowestQueries}}
                <div class="section">
                    <h4>Slowest Queries</h4>
                    {{range .Result1.SQLAnalysis.SlowestQueries}}
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-duration{{if gt .Duration 10.0}} query-slow{{end}}">{{formatFloat .Duration}}ms</span>
                        </div>
                        <div class="query-text">{{.Query}}</div>
                        <div class="query-meta">
                            <span>Table: {{.Table}}</span>
                            <span>Op: {{.Operation}}</span>
                            <span>Rows: {{.Rows}}</span>
                        </div>
                    </div>
                    {{end}}
                </div>
                {{end}}
                
                {{if .Result1.SQLAnalysis.NPlusOneIssues}}
                <div class="section">
                    <h4>Potential N+1 Issues</h4>
                    {{range .Result1.SQLAnalysis.NPlusOneIssues}}
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-count">{{.Count}}x executions</span>
                        </div>
                        <div class="query-text">{{.Example.Query}}</div>
                        <div class="query-meta">
                            <span>Table: {{.Example.Table}}</span>
                            <span>Consider batching or eager loading</span>
                        </div>
                    </div>
                    {{end}}
                </div>
                {{end}}
                
                {{if .Result1.SQLAnalysis.TablesAccessed}}
                <div class="section">
                    <h4>Tables Accessed</h4>
                    {{range $table, $count := .Result1.SQLAnalysis.TablesAccessed}}
                    <span class="table-badge">{{$table}}<span class="table-count">({{$count}})</span></span>
                    {{end}}
                </div>
                {{end}}
                {{end}}
                {{end}}
                
                {{if .Result1.Logs}}
                <div class="section">
                    <h4>Logs ({{len .Result1.Logs}} entries)</h4>
                    {{range .Result1.Logs}}
                    <div class="log-line {{if .Entry}}{{if eq .Entry.Level "ERR"}}error{{else if eq .Entry.Level "ERROR"}}error{{else if eq .Entry.Level "WRN"}}warn{{else if eq .Entry.Level "WARN"}}warn{{else}}info{{end}}{{end}}">{{if .Entry}}{{.Entry.Raw}}{{end}}</div>
                    {{end}}
                </div>
                {{end}}
                {{end}}
            </div>
            
            <!-- Result 2 -->
            <div class="result{{if .Result2.Error}} error{{end}}">
                <div class="header">
                    <div class="url">URL 2: {{.Result2.URL}}</div>
                    {{if .Result2.RequestID}}
                    <div class="request-id">Request ID: {{.Result2.RequestID}}</div>
                    {{end}}
                </div>
                
                {{if .Result2.Error}}
                <div class="error-message">Error: {{.Result2.Error}}</div>
                {{else}}
                
                <div class="metrics">
                    <div class="metric">
                        <div class="metric-label">Status Code</div>
                        <div class="metric-value {{if eq .Result2.StatusCode 200}}success{{else}}error{{end}}">
                            {{.Result2.StatusCode}}
                        </div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">Duration</div>
                        <div class="metric-value">{{formatDuration .Result2.Duration}}</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">Logs Collected</div>
                        <div class="metric-value">{{len .Result2.Logs}}</div>
                    </div>
                    <div class="metric">
                        <div class="metric-label">SQL Queries</div>
                        <div class="metric-value">{{.Result2.SQLAnalysis.TotalQueries}}</div>
                    </div>
                </div>
                
                {{if .Result2.SQLAnalysis}}
                {{if gt .Result2.SQLAnalysis.TotalQueries 0}}
                <div class="section">
                    <h4>SQL Analysis</h4>
                    <div class="stats-grid">
                        <div class="metric">
                            <div class="metric-label">Total Queries</div>
                            <div class="metric-value">{{.Result2.SQLAnalysis.TotalQueries}}</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Unique Queries</div>
                            <div class="metric-value">{{.Result2.SQLAnalysis.UniqueQueries}}</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Avg Duration</div>
                            <div class="metric-value">{{formatFloat .Result2.SQLAnalysis.AvgDuration}}ms</div>
                        </div>
                        <div class="metric">
                            <div class="metric-label">Total Duration</div>
                            <div class="metric-value">{{formatFloat .Result2.SQLAnalysis.TotalDuration}}ms</div>
                        </div>
                    </div>
                </div>
                
                {{if .Result2.SQLAnalysis.SlowestQueries}}
                <div class="section">
                    <h4>Slowest Queries</h4>
                    {{range .Result2.SQLAnalysis.SlowestQueries}}
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-duration{{if gt .Duration 10.0}} query-slow{{end}}">{{formatFloat .Duration}}ms</span>
                        </div>
                        <div class="query-text">{{.Query}}</div>
                        <div class="query-meta">
                            <span>Table: {{.Table}}</span>
                            <span>Op: {{.Operation}}</span>
                            <span>Rows: {{.Rows}}</span>
                        </div>
                    </div>
                    {{end}}
                </div>
                {{end}}
                
                {{if .Result2.SQLAnalysis.NPlusOneIssues}}
                <div class="section">
                    <h4>Potential N+1 Issues</h4>
                    {{range .Result2.SQLAnalysis.NPlusOneIssues}}
                    <div class="query-item">
                        <div class="query-header">
                            <span class="query-count">{{.Count}}x executions</span>
                        </div>
                        <div class="query-text">{{.Example.Query}}</div>
                        <div class="query-meta">
                            <span>Table: {{.Example.Table}}</span>
                            <span>Consider batching or eager loading</span>
                        </div>
                    </div>
                    {{end}}
                </div>
                {{end}}
                
                {{if .Result2.SQLAnalysis.TablesAccessed}}
                <div class="section">
                    <h4>Tables Accessed</h4>
                    {{range $table, $count := .Result2.SQLAnalysis.TablesAccessed}}
                    <span class="table-badge">{{$table}}<span class="table-count">({{$count}})</span></span>
                    {{end}}
                </div>
                {{end}}
                {{end}}
                {{end}}
                
                {{if .Result2.Logs}}
                <div class="section">
                    <h4>Logs ({{len .Result2.Logs}} entries)</h4>
                    {{range .Result2.Logs}}
                    <div class="log-line {{if .Entry}}{{if eq .Entry.Level "ERR"}}error{{else if eq .Entry.Level "ERROR"}}error{{else if eq .Entry.Level "WRN"}}warn{{else if eq .Entry.Level "WARN"}}warn{{else}}info{{end}}{{end}}">{{if .Entry}}{{.Entry.Raw}}{{end}}</div>
                    {{end}}
                </div>
                {{end}}
                {{end}}
            </div>
        </div>
    </div>
</body>
</html>
`
