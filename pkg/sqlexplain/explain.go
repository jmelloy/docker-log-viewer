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

func FormatExplainPlanAsText(planJSON any) (string, error) {
	var output []string

	var formatNode func(node PlanNode, indent int, isLast bool, prefix string)
	formatNode = func(node PlanNode, indent int, isLast bool, prefix string) {
		line := ""

		if indent > 0 {
			if isLast {
				line = prefix + "└─ "
			} else {
				line = prefix + "├─ "
			}
		} else {
			line = strings.Repeat("  ", indent)
		}

		line += node.NodeType
		if node.RelationName != "" {
			line += fmt.Sprintf(" on %s", node.RelationName)
			if node.Alias != "" && node.Alias != node.RelationName {
				line += fmt.Sprintf(" %s", node.Alias)
			}
		}

		line += fmt.Sprintf("  (cost=%.2f..%.2f rows=%d width=%d)",
			node.StartupCost, node.TotalCost, node.PlanRows, node.PlanWidth)

		if node.ActualStartupTime > 0 || node.ActualTotalTime > 0 {
			loops := node.ActualLoops
			if loops == 0 {
				loops = 1
			}
			line += fmt.Sprintf(" (actual time=%.3f..%.3f rows=%d loops=%d)",
				node.ActualStartupTime, node.ActualTotalTime, node.ActualRows, loops)
		}

		output = append(output, line)

		childPrefix := prefix
		if isLast {
			childPrefix += "   "
		} else {
			childPrefix += "│  "
		}

		if node.Filter != "" {
			output = append(output, childPrefix+fmt.Sprintf("Filter: %s", node.Filter))
			if node.RowsRemovedByFilter > 0 {
				output = append(output, childPrefix+fmt.Sprintf("Rows Removed by Filter: %d", node.RowsRemovedByFilter))
			}
		}
		if node.IndexCond != "" {
			output = append(output, childPrefix+fmt.Sprintf("Index Cond: %s", node.IndexCond))
		}
		if node.HashCond != "" {
			output = append(output, childPrefix+fmt.Sprintf("Hash Cond: %s", node.HashCond))
		}
		if node.JoinFilter != "" {
			output = append(output, childPrefix+fmt.Sprintf("Join Filter: %s", node.JoinFilter))
		}
		if node.SortKey != nil {
			sortKeys := ""
			switch v := node.SortKey.(type) {
			case []string:
				sortKeys = strings.Join(v, ", ")
			case string:
				sortKeys = v
			}
			output = append(output, childPrefix+fmt.Sprintf("Sort Key: %s", sortKeys))
		}

		for i, child := range node.Plans {
			formatNode(child, indent+1, i == len(node.Plans)-1, childPrefix)
		}
	}

	switch p := planJSON.(type) {
	case []Plan:
		for _, plan := range p {
			formatNode(plan.Plan, 0, true, "")
			if plan.PlanningTime > 0 {
				output = append(output, fmt.Sprintf("Planning Time: %.3f ms", plan.PlanningTime))
			}
			if plan.ExecutionTime > 0 {
				output = append(output, fmt.Sprintf("Execution Time: %.3f ms", plan.ExecutionTime))
			}
			output = append(output, "")
		}
	case Plan:
		formatNode(p.Plan, 0, true, "")
		if p.PlanningTime > 0 {
			output = append(output, fmt.Sprintf("Planning Time: %.3f ms", p.PlanningTime))
		}
		if p.ExecutionTime > 0 {
			output = append(output, fmt.Sprintf("Execution Time: %.3f ms", p.ExecutionTime))
		}
	default:
		return "", fmt.Errorf("unsupported plan type: %T", planJSON)
	}

	return strings.Join(output, "\n"), nil
}
