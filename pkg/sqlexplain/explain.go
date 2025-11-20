package sqlexplain

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	_ "github.com/lib/pq"
)

type Request struct {
	Query            string            `json:"query"`
	Variables        map[string]string `json:"variables,omitempty"`
	ConnectionString string            `json:"connectionString,omitempty"` // Optional: use specific connection instead of default
}

type Response struct {
	QueryPlan      []map[string]interface{} `json:"queryPlan"`
	Error          string                   `json:"error,omitempty"`
	Query          string                   `json:"query"`
	FormattedQuery string                   `json:"formattedQuery"`
}

var db *sql.DB

// Init initializes the database connection for EXPLAIN queries
func Init() error {
	// Try to get connection string from environment
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		// Default connection for local PostgreSQL
		connStr = "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	}

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		db = nil
		return err
	}

	return nil
}

// substituteVariables replaces $1, $2, etc. with actual values from variables map
func substituteVariables(query string, variables map[string]string) string {
	// Match $1, $2, $3, etc.
	re := regexp.MustCompile(`\$(\d+)`)

	result := re.ReplaceAllStringFunc(query, func(match string) string {
		// Extract the number
		num := match[1:]
		if val, ok := variables[num]; ok {
			// Handle NULL (but only if it's exactly "NULL" or empty, case-insensitive)
			trimmedVal := strings.TrimSpace(val)
			if trimmedVal == "" || strings.EqualFold(trimmedVal, "NULL") {
				return "NULL"
			}
			// Handle booleans
			if val == "true" || val == "false" || val == "TRUE" || val == "FALSE" {
				return val
			}
			// Handle numbers (integers and floats)
			if regexp.MustCompile(`^-?\d+(\.\d+)?$`).MatchString(val) {
				return val
			}
			// Quote strings (including timestamps, UUIDs, etc.)
			return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
		}
		return match
	})

	return result
}

// postgres://username:password@host:port/dbname
func noPasswordConnectionString(connectionString string) string {
	parts := strings.Split(connectionString, "@")
	if len(parts) != 2 {
		return connectionString
	}

	return fmt.Sprintf("postgres://%s", parts[1])
}

// formatSQL applies basic SQL formatting with indentation and newlines
func formatSQL(query string) string {
	// Remove leading/trailing whitespace
	query = strings.TrimSpace(query)

	// Keywords that should start on a new line
	keywords := []string{
		"SELECT", "FROM", "WHERE", "AND", "OR", "JOIN", "INNER JOIN", "LEFT JOIN", "RIGHT JOIN",
		"GROUP BY", "ORDER BY", "HAVING", "LIMIT", "OFFSET", "UNION", "UNION ALL",
		"INSERT INTO", "UPDATE", "DELETE FROM", "VALUES", "SET",
	}

	// Add newlines before major keywords
	for _, keyword := range keywords {
		// Match keyword with word boundaries
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(keyword) + `\b`)
		query = re.ReplaceAllStringFunc(query, func(match string) string {
			if strings.HasPrefix(strings.ToUpper(match), "AND") || strings.HasPrefix(strings.ToUpper(match), "OR") {
				return "\n  " + strings.ToUpper(match)
			}
			return "\n" + strings.ToUpper(match)
		})
	}

	// Clean up multiple newlines and trim each line
	lines := strings.Split(query, "\n")
	var formatted []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			formatted = append(formatted, trimmed)
		}
	}

	return strings.Join(formatted, "\n")
}

// Explain runs EXPLAIN (ANALYZE, FORMAT JSON) on the given query
func Explain(req Request) Response {
	resp := Response{
		Query: req.Query,
	}

	// Use connection string from request if provided, otherwise use default db
	var targetDB *sql.DB

	if req.ConnectionString != "" {
		// Open a temporary connection for this request
		tempDB, err := sql.Open("postgres", req.ConnectionString)
		if err != nil {
			resp.Error = fmt.Sprintf("Error connecting to database: %v", err)
			return resp
		}
		defer tempDB.Close()
		// Test the connection
		if err := tempDB.Ping(); err != nil {
			resp.Error = fmt.Sprintf("Error connecting to database: %v", err)
			return resp
		}
		targetDB = tempDB
	} else {
		// Use default connection
		if db == nil {
			resp.Error = "Database connection not configured. Set DATABASE_URL environment variable or provide connectionString."
			return resp
		}
		targetDB = db
	}

	var err error
	query := req.Query
	displayQuery := query

	// If we have variables, show substituted query for display
	if len(req.Variables) > 0 {
		displayQuery = substituteVariables(query, req.Variables)
	}
	resp.Query = displayQuery
	resp.FormattedQuery = formatSQL(displayQuery)

	// Detect if query modifies data (INSERT, UPDATE, DELETE)
	// For those, use EXPLAIN without ANALYZE to avoid actually executing them
	queryUpper := strings.ToUpper(strings.TrimSpace(query))
	useAnalyze := true
	if strings.Contains(queryUpper, "INSERT INTO") ||
		strings.Contains(queryUpper, "UPDATE ") ||
		strings.Contains(queryUpper, "DELETE FROM") {
		useAnalyze = false
	}

	// Run EXPLAIN with or without ANALYZE based on query type
	var explainQuery string
	if useAnalyze {
		explainQuery = fmt.Sprintf("EXPLAIN (ANALYZE, COSTS, VERBOSE, BUFFERS, FORMAT JSON) %s", query)
	} else {
		explainQuery = fmt.Sprintf("EXPLAIN (COSTS, VERBOSE, FORMAT JSON) %s", query)
	}

	var rows *sql.Rows

	// If we have variables, use them as bind parameters
	if len(req.Variables) > 0 {
		// Convert variables map to ordered slice based on $1, $2, $3...
		args := make([]interface{}, 0)
		for i := 1; ; i++ {
			val, ok := req.Variables[fmt.Sprintf("%d", i)]
			if !ok {
				break
			}
			args = append(args, val)
		}
		slog.Info("EXPLAIN query", "query", explainQuery, "args", args, "connection", noPasswordConnectionString(req.ConnectionString))
		rows, err = targetDB.Query(explainQuery, args...)
	} else {
		slog.Debug("EXPLAIN query", "query", explainQuery)
		rows, err = targetDB.Query(explainQuery)
	}

	if err != nil {
		resp.Error = fmt.Sprintf("Error running EXPLAIN: %v", err)
		return resp
	}
	defer rows.Close()

	// Collect all rows (usually just one for EXPLAIN JSON)
	var planJSON string
	for rows.Next() {
		var plan string
		if err := rows.Scan(&plan); err != nil {
			resp.Error = fmt.Sprintf("Error scanning result: %v", err)
			return resp
		}
		planJSON += plan
	}

	if err := rows.Err(); err != nil {
		resp.Error = fmt.Sprintf("Error iterating results: %v", err)
		return resp
	}

	// Parse the JSON plan
	var plan []map[string]interface{}
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		resp.Error = fmt.Sprintf("Error parsing EXPLAIN JSON: %v", err)
		return resp
	}

	resp.QueryPlan = plan
	return resp
}

// Close closes the database connection
func Close() {
	if db != nil {
		db.Close()
	}
}

type PlanNode struct {
	NodeType            string      `json:"Node Type"`
	RelationName        string      `json:"Relation Name,omitempty"`
	Alias               string      `json:"Alias,omitempty"`
	StartupCost         float64     `json:"Startup Cost,omitempty"`
	TotalCost           float64     `json:"Total Cost,omitempty"`
	PlanRows            int         `json:"Plan Rows,omitempty"`
	PlanWidth           int         `json:"Plan Width,omitempty"`
	ActualStartupTime   float64     `json:"Actual Startup Time,omitempty"`
	ActualTotalTime     float64     `json:"Actual Total Time,omitempty"`
	ActualRows          int         `json:"Actual Rows,omitempty"`
	ActualLoops         int         `json:"Actual Loops,omitempty"`
	Filter              string      `json:"Filter,omitempty"`
	RowsRemovedByFilter int         `json:"Rows Removed by Filter,omitempty"`
	IndexCond           string      `json:"Index Cond,omitempty"`
	HashCond            string      `json:"Hash Cond,omitempty"`
	JoinFilter          string      `json:"Join Filter,omitempty"`
	SortKey             interface{} `json:"Sort Key,omitempty"` // string or []string
	Plans               []PlanNode  `json:"Plans,omitempty"`
}

type Plan struct {
	Plan          PlanNode `json:"Plan"`
	PlanningTime  float64  `json:"Planning Time,omitempty"`
	ExecutionTime float64  `json:"Execution Time,omitempty"`
}

// explainPlanLine formats a single line for a plan node (matches TypeScript explainPlanLine)
func explainPlanLine(node map[string]interface{}) string {
	var line strings.Builder

	// Index Name
	if indexName, ok := getString(node, "Index Name"); ok && indexName != "" {
		line.WriteString(fmt.Sprintf(" using %s", indexName))
	}

	// Relation Name or CTE Name
	relationName, hasRelation := getString(node, "Relation Name")
	cteName, hasCTE := getString(node, "CTE Name")
	if hasRelation && relationName != "" {
		line.WriteString(fmt.Sprintf(" on %s", relationName))
		// Alias
		if alias, ok := getString(node, "Alias"); ok && alias != "" && alias != relationName {
			line.WriteString(fmt.Sprintf(" %s", alias))
		}
	} else if hasCTE && cteName != "" {
		line.WriteString(fmt.Sprintf(" on %s", cteName))
		// Alias
		if alias, ok := getString(node, "Alias"); ok && alias != "" && alias != cteName {
			line.WriteString(fmt.Sprintf(" %s", alias))
		}
	}

	// Cost and rows
	costStart := 0.0
	if val, ok := getFloat64(node, "Startup Cost"); ok {
		costStart = val
	}
	costEnd := 0.0
	if val, ok := getFloat64(node, "Total Cost"); ok {
		costEnd = val
	}
	rows := 0
	if val, ok := getInt(node, "Plan Rows"); ok {
		rows = val
	}
	width := 0
	if val, ok := getInt(node, "Plan Width"); ok {
		width = val
	}
	line.WriteString(fmt.Sprintf("  (cost=%.2f..%.2f rows=%d width=%d)", costStart, costEnd, rows, width))

	// Actual time if available
	if actualStart, ok1 := getFloat64(node, "Actual Startup Time"); ok1 {
		if actualEnd, ok2 := getFloat64(node, "Actual Total Time"); ok2 {
			actualRows := 0
			if val, ok := getInt(node, "Actual Rows"); ok {
				actualRows = val
			}
			loops := 1
			if val, ok := getInt(node, "Actual Loops"); ok {
				loops = val
			}
			line.WriteString(fmt.Sprintf(" (actual time=%.3f..%.3f rows=%d loops=%d)", actualStart, actualEnd, actualRows, loops))
		}
	}

	return line.String()
}

// Helper functions to safely extract values from map[string]interface{}
func getString(m map[string]interface{}, key string) (string, bool) {
	val, ok := m[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

func getFloat64(m map[string]interface{}, key string) (float64, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

func getInt(m map[string]interface{}, key string) (int, bool) {
	val, ok := m[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	}
	return 0, false
}

func FormatExplainPlanAsText(planJSON any) (string, error) {
	var output []string

	var formatNode func(node map[string]interface{}, level int, isLast bool, prefix string)
	formatNode = func(node map[string]interface{}, level int, isLast bool, prefix string) {
		indent := level
		spaces := strings.Repeat("  ", indent)
		line := spaces

		// Add tree structure characters
		if indent > 0 {
			if isLast {
				line = prefix + "└─ "
			} else {
				line = prefix + "├─ "
			}
		}

		// Subplan Name
		if subplanName, ok := getString(node, "Subplan Name"); ok && subplanName != "" {
			output = append(output, fmt.Sprintf("%s%s", spaces, subplanName))
		}

		// Node type and relation
		nodeType, _ := getString(node, "Node Type")
		line += nodeType
		line += explainPlanLine(node)

		output = append(output, line)

		// Filter condition
		if filter, ok := getString(node, "Filter"); ok && filter != "" {
			filterPrefix := prefix
			if isLast {
				filterPrefix += "   "
			} else {
				filterPrefix += "│  "
			}
			output = append(output, filterPrefix+fmt.Sprintf("Filter: %s", filter))
			if rowsRemoved, ok := getInt(node, "Rows Removed by Filter"); ok {
				output = append(output, filterPrefix+fmt.Sprintf("Rows Removed by Filter: %d", rowsRemoved))
			}
		}

		// Index condition
		if indexCond, ok := getString(node, "Index Cond"); ok && indexCond != "" {
			condPrefix := prefix
			if isLast {
				condPrefix += "   "
			} else {
				condPrefix += "│  "
			}
			output = append(output, condPrefix+fmt.Sprintf("Index Cond: %s", indexCond))
		}

		// Hash condition
		if hashCond, ok := getString(node, "Hash Cond"); ok && hashCond != "" {
			hashPrefix := prefix
			if isLast {
				hashPrefix += "   "
			} else {
				hashPrefix += "│  "
			}
			output = append(output, hashPrefix+fmt.Sprintf("Hash Cond: %s", hashCond))
		}

		// Join Filter
		if joinFilter, ok := getString(node, "Join Filter"); ok && joinFilter != "" {
			joinPrefix := prefix
			if isLast {
				joinPrefix += "   "
			} else {
				joinPrefix += "│  "
			}
			output = append(output, joinPrefix+fmt.Sprintf("Join Filter: %s", joinFilter))
		}

		// Sort Key
		if sortKey, ok := node["Sort Key"]; ok && sortKey != nil {
			sortPrefix := prefix
			if isLast {
				sortPrefix += "   "
			} else {
				sortPrefix += "│  "
			}
			var sortKeys string
			switch v := sortKey.(type) {
			case []interface{}:
				var parts []string
				for _, item := range v {
					parts = append(parts, fmt.Sprintf("%v", item))
				}
				sortKeys = strings.Join(parts, ", ")
			case []string:
				sortKeys = strings.Join(v, ", ")
			default:
				sortKeys = fmt.Sprintf("%v", v)
			}
			output = append(output, sortPrefix+fmt.Sprintf("Sort Key: %s", sortKeys))
		}

		// Process child plans
		if plans, ok := node["Plans"].([]interface{}); ok && len(plans) > 0 {
			childPrefix := prefix
			if isLast {
				childPrefix += "   "
			} else {
				childPrefix += "│  "
			}
			for idx, child := range plans {
				if childMap, ok := child.(map[string]interface{}); ok {
					childIsLast := idx == len(plans)-1
					formatNode(childMap, indent+1, childIsLast, childPrefix)
				}
			}
		}
	}

	// Handle array of plans
	if planArray, ok := planJSON.([]interface{}); ok {
		for _, planItem := range planArray {
			if planMap, ok := planItem.(map[string]interface{}); ok {
				if planNode, ok := planMap["Plan"].(map[string]interface{}); ok {
					formatNode(planNode, 0, true, "")
				}
				if planningTime, ok := getFloat64(planMap, "Planning Time"); ok {
					output = append(output, fmt.Sprintf("Planning Time: %.3f ms", planningTime))
				}
				if executionTime, ok := getFloat64(planMap, "Execution Time"); ok {
					output = append(output, fmt.Sprintf("Execution Time: %.3f ms", executionTime))
				}
				output = append(output, "")
			}
		}
	} else if planMap, ok := planJSON.(map[string]interface{}); ok {
		// Handle single plan
		if planNode, ok := planMap["Plan"].(map[string]interface{}); ok {
			formatNode(planNode, 0, true, "")

			// Add planning and execution time
			if planningTime, ok := getFloat64(planMap, "Planning Time"); ok {
				output = append(output, fmt.Sprintf("Planning Time: %.3f ms", planningTime))
			}
			if executionTime, ok := getFloat64(planMap, "Execution Time"); ok {
				output = append(output, fmt.Sprintf("Execution Time: %.3f ms", executionTime))
			}
		}
	} else {
		return "", fmt.Errorf("unsupported plan type: %T", planJSON)
	}

	return strings.Join(output, "\n"), nil
}
