package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"docker-log-parser/pkg/logs"
	"encoding/hex"
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
	AllQueries      []SQLQuery
}

type QueryDiff struct {
	Query1       string
	Query2       string
	Diff         string
	Added        []string
	Removed      []string
	Changed      []string
	IsSame       bool
}

type ComparisonAnalysis struct {
	QueryDiffs      []QueryDiff
	QueriesOnlyIn1  []SQLQuery
	QueriesOnlyIn2  []SQLQuery
	CommonQueries   []QueryComparison
	PerfImprovements []QueryComparison
	PerfRegressions  []QueryComparison
}

type QueryComparison struct {
	Query        string
	Duration1    float64
	Duration2    float64
	DiffPercent  float64
	Improvement  bool
	Table        string
	Operation    string
	Rows1        int
	Rows2        int
	Count1       int
	Count2       int
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
	flag.StringVar(&config.DevID, "dev-id", os.Getenv("X_GLUE_DEV_USER_ID"), "X-GlueDev-UserID header value")
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

func generateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func testURL(url string, data []byte, logChan <-chan logs.LogMessage, timeout time.Duration, config *CompareConfig) *RequestResult {
	result := &RequestResult{
		URL:  url,
		Logs: make([]logs.LogMessage, 0),
	}

	// Generate a unique request ID
	result.RequestID = generateRequestID()

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
	req.Header.Set("X-Request-Id", result.RequestID)

	// Add authentication headers
	if config.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.BearerToken)
	}
	if config.DevID != "" {
		req.Header.Set("X-GlueDev-UserID", config.DevID)
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

	// Read response body
	io.Copy(io.Discard, resp.Body)

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
		AllQueries:     queries,
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

func ansiToHTML(text string) template.HTML {
	// ANSI color codes mapping to HTML colors
	colorMap := map[string]string{
		"30": "#000000", "31": "#f85149", "32": "#3fb950", "33": "#d29922",
		"34": "#58a6ff", "35": "#bc8cff", "36": "#56d4dd", "37": "#c9d1d9",
		"90": "#6e7681", "91": "#ff7b72", "92": "#7ee787", "93": "#f2cc60",
		"94": "#79c0ff", "95": "#d2a8ff", "96": "#a5d6ff", "97": "#f0f6fc",
	}

	// Remove ANSI escape sequences and convert to HTML
	ansiRegex := regexp.MustCompile(`\x1b\[([0-9;]+)m`)
	var result strings.Builder
	lastPos := 0

	for _, match := range ansiRegex.FindAllStringSubmatchIndex(text, -1) {
		// Add text before this match
		result.WriteString(template.HTMLEscapeString(text[lastPos:match[0]]))

		// Parse ANSI code
		codes := text[match[2]:match[3]]
		if codes == "0" || codes == "" {
			result.WriteString("</span>")
		} else if color, ok := colorMap[codes]; ok {
			result.WriteString(fmt.Sprintf(`<span style="color: %s">`, color))
		}

		lastPos = match[1]
	}
	result.WriteString(template.HTMLEscapeString(text[lastPos:]))

	return template.HTML(result.String())
}

func formatSQL(sql string) string {
	depth := 0
	
	// Replace major keywords with newlines
	formatted := sql
	formatted = regexp.MustCompile(`\bSELECT\b`).ReplaceAllString(formatted, "\nSELECT")
	formatted = regexp.MustCompile(`\bFROM\b`).ReplaceAllString(formatted, "\nFROM")
	formatted = regexp.MustCompile(`\bWHERE\b`).ReplaceAllString(formatted, "\nWHERE")
	formatted = regexp.MustCompile(`\bAND\b`).ReplaceAllString(formatted, "\n  AND")
	formatted = regexp.MustCompile(`\bOR\b`).ReplaceAllString(formatted, "\n  OR")
	formatted = regexp.MustCompile(`\bLEFT JOIN\b`).ReplaceAllString(formatted, "\nLEFT JOIN")
	formatted = regexp.MustCompile(`\bINNER JOIN\b`).ReplaceAllString(formatted, "\nINNER JOIN")
	formatted = regexp.MustCompile(`\bRIGHT JOIN\b`).ReplaceAllString(formatted, "\nRIGHT JOIN")
	formatted = regexp.MustCompile(`\bGROUP BY\b`).ReplaceAllString(formatted, "\nGROUP BY")
	formatted = regexp.MustCompile(`\bORDER BY\b`).ReplaceAllString(formatted, "\nORDER BY")
	formatted = regexp.MustCompile(`\bLIMIT\b`).ReplaceAllString(formatted, "\nLIMIT")
	
	// Handle parentheses
	formatted = strings.ReplaceAll(formatted, "(", "\n(\n")
	formatted = strings.ReplaceAll(formatted, ")", "\n)\n")
	
	// Split into lines and indent
	lines := strings.Split(formatted, "\n")
	var result strings.Builder
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		if line == ")" {
			depth--
		}
		
		result.WriteString(strings.Repeat("  ", depth))
		result.WriteString(line)
		result.WriteString("\n")
		
		if line == "(" {
			depth++
		}
	}
	
	return strings.TrimSpace(result.String())
}

func formatAndHighlightSQL(sql string) template.HTML {
	// First format the SQL
	formatted := formatSQL(sql)
	
	// Apply syntax highlighting with proper escaping
	// We use ReplaceAllStringFunc to escape each part individually
	keywords := regexp.MustCompile(`\b(SELECT|FROM|WHERE|INSERT|UPDATE|DELETE|CREATE|DROP|ALTER|TABLE|INDEX|JOIN|LEFT|RIGHT|INNER|OUTER|ON|AND|OR|NOT|IN|EXISTS|LIKE|IS|NULL|ORDER|BY|GROUP|HAVING|LIMIT|OFFSET|AS|SET|VALUES|INTO|DISTINCT|UNION|CASE|WHEN|THEN|ELSE|END)\b`)
	strings_re := regexp.MustCompile(`('[^']*')`)
	numbers := regexp.MustCompile(`\b(\d+(?:\.\d+)?)\b`)
	
	// Process in order: keywords first, then strings, then numbers
	result := keywords.ReplaceAllStringFunc(formatted, func(match string) string {
		return fmt.Sprintf(`<span class="sql-keyword">%s</span>`, template.HTMLEscapeString(match))
	})
	
	result = strings_re.ReplaceAllStringFunc(result, func(match string) string {
		// Check if already inside a span tag
		if strings.Contains(match, "sql-keyword") {
			return match
		}
		return fmt.Sprintf(`<span class="sql-string">%s</span>`, template.HTMLEscapeString(match))
	})
	
	result = numbers.ReplaceAllStringFunc(result, func(match string) string {
		// Check if already inside a span tag
		if strings.Contains(match, "sql-keyword") || strings.Contains(match, "sql-string") {
			return match
		}
		return fmt.Sprintf(`<span class="sql-number">%s</span>`, template.HTMLEscapeString(match))
	})
	
	// Escape any remaining untagged content
	// Split by tags and escape the text between them
	var builder strings.Builder
	inTag := false
	current := ""
	
	for i := 0; i < len(result); i++ {
		if result[i] == '<' {
			if !inTag && current != "" {
				builder.WriteString(template.HTMLEscapeString(current))
				current = ""
			}
			inTag = true
			builder.WriteByte('<')
		} else if result[i] == '>' {
			inTag = false
			builder.WriteByte('>')
		} else if inTag {
			builder.WriteByte(result[i])
		} else {
			current += string(result[i])
			if i == len(result)-1 || result[i+1] == '<' {
				builder.WriteString(template.HTMLEscapeString(current))
				current = ""
			}
		}
	}
	
	return template.HTML(builder.String())
}

func compareQuerySequences(result1, result2 *RequestResult) *ComparisonAnalysis {
	analysis := &ComparisonAnalysis{}
	
	if result1.SQLAnalysis == nil || result2.SQLAnalysis == nil {
		return analysis
	}
	
	queries1 := result1.SQLAnalysis.AllQueries
	queries2 := result2.SQLAnalysis.AllQueries
	
	// Group queries by normalized form
	map1 := make(map[string][]SQLQuery)
	map2 := make(map[string][]SQLQuery)
	
	for _, q := range queries1 {
		map1[q.Normalized] = append(map1[q.Normalized], q)
	}
	for _, q := range queries2 {
		map2[q.Normalized] = append(map2[q.Normalized], q)
	}
	
	// Find queries only in result1
	for norm, queries := range map1 {
		if _, exists := map2[norm]; !exists {
			analysis.QueriesOnlyIn1 = append(analysis.QueriesOnlyIn1, queries[0])
		}
	}
	
	// Find queries only in result2
	for norm, queries := range map2 {
		if _, exists := map1[norm]; !exists {
			analysis.QueriesOnlyIn2 = append(analysis.QueriesOnlyIn2, queries[0])
		}
	}
	
	// Compare common queries
	for norm, queries1 := range map1 {
		if queries2, exists := map2[norm]; exists {
			avgDur1 := 0.0
			avgDur2 := 0.0
			totalRows1 := 0
			totalRows2 := 0
			
			for _, q := range queries1 {
				avgDur1 += q.Duration
				totalRows1 += q.Rows
			}
			for _, q := range queries2 {
				avgDur2 += q.Duration
				totalRows2 += q.Rows
			}
			avgDur1 /= float64(len(queries1))
			avgDur2 /= float64(len(queries2))
			avgRows1 := 0
			avgRows2 := 0
			if len(queries1) > 0 {
				avgRows1 = totalRows1 / len(queries1)
			}
			if len(queries2) > 0 {
				avgRows2 = totalRows2 / len(queries2)
			}
			
			comp := QueryComparison{
				Query:      queries1[0].Query,
				Duration1:  avgDur1,
				Duration2:  avgDur2,
				Table:      queries1[0].Table,
				Operation:  queries1[0].Operation,
				Rows1:      avgRows1,
				Rows2:      avgRows2,
				Count1:     len(queries1),
				Count2:     len(queries2),
			}
			
			if avgDur1 > 0 {
				comp.DiffPercent = ((avgDur2 - avgDur1) / avgDur1) * 100
				comp.Improvement = avgDur2 < avgDur1
			}
			
			analysis.CommonQueries = append(analysis.CommonQueries, comp)
			
			if comp.Improvement && comp.DiffPercent < -10 {
				analysis.PerfImprovements = append(analysis.PerfImprovements, comp)
			} else if !comp.Improvement && comp.DiffPercent > 10 {
				analysis.PerfRegressions = append(analysis.PerfRegressions, comp)
			}
		}
	}
	
	// Sort by performance impact
	sort.Slice(analysis.PerfImprovements, func(i, j int) bool {
		return analysis.PerfImprovements[i].DiffPercent < analysis.PerfImprovements[j].DiffPercent
	})
	sort.Slice(analysis.PerfRegressions, func(i, j int) bool {
		return analysis.PerfRegressions[i].DiffPercent > analysis.PerfRegressions[j].DiffPercent
	})
	
	// Generate sequential diffs for all queries
	maxLen := len(queries1)
	if len(queries2) > maxLen {
		maxLen = len(queries2)
	}
	
	for i := 0; i < maxLen; i++ {
		diff := QueryDiff{}
		
		if i < len(queries1) {
			diff.Query1 = queries1[i].Query
		}
		if i < len(queries2) {
			diff.Query2 = queries2[i].Query
		}
		
		diff.IsSame = diff.Query1 == diff.Query2
		if !diff.IsSame && diff.Query1 != "" && diff.Query2 != "" {
			diff.Added, diff.Removed, diff.Changed = computeQueryDiff(diff.Query1, diff.Query2)
		}
		
		analysis.QueryDiffs = append(analysis.QueryDiffs, diff)
	}
	
	return analysis
}

func computeQueryDiff(q1, q2 string) ([]string, []string, []string) {
	// Simple word-based diff
	words1 := strings.Fields(q1)
	words2 := strings.Fields(q2)
	
	wordMap1 := make(map[string]bool)
	wordMap2 := make(map[string]bool)
	
	for _, w := range words1 {
		wordMap1[w] = true
	}
	for _, w := range words2 {
		wordMap2[w] = true
	}
	
	var added, removed, changed []string
	
	for _, w := range words2 {
		if !wordMap1[w] {
			added = append(added, w)
		}
	}
	for _, w := range words1 {
		if !wordMap2[w] {
			removed = append(removed, w)
		}
	}
	
	return added, removed, changed
}

func generateHTML(filename string, result1, result2 *RequestResult, postData string) error {
	// Find template file relative to executable or in common locations
	templatePath := "comparison-report.tmpl"
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		templatePath = "cmd/compare/comparison-report.tmpl"
	}
	
	comparison := compareQuerySequences(result1, result2)
	
	tmpl := template.Must(template.New("comparison-report.tmpl").Funcs(template.FuncMap{
		"escapeHTML": template.HTMLEscapeString,
		"ansiToHTML": ansiToHTML,
		"formatDuration": func(d time.Duration) string {
			return d.Round(time.Millisecond).String()
		},
		"formatFloat": func(f float64) string {
			return fmt.Sprintf("%.2f", f)
		},
		"formatPercent": func(f float64) string {
			if f > 0 {
				return fmt.Sprintf("+%.1f%%", f)
			}
			return fmt.Sprintf("%.1f%%", f)
		},
		"shortenQuery": func(q string) string {
			if len(q) > 100 {
				return q[:97] + "..."
			}
			return q
		},
		"add": func(a, b int) int {
			return a + b
		},
		"formatSQL": formatAndHighlightSQL,
	}).ParseFiles(templatePath))

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	data := struct {
		Result1    *RequestResult
		Result2    *RequestResult
		PostData   string
		Generated  time.Time
		Comparison *ComparisonAnalysis
	}{
		Result1:    result1,
		Result2:    result2,
		PostData:   postData,
		Generated:  time.Now(),
		Comparison: comparison,
	}

	return tmpl.Execute(f, data)
}
