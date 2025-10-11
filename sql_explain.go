package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	_ "github.com/lib/pq"
)

type ExplainRequest struct {
	Query     string            `json:"query"`
	Variables map[string]string `json:"variables,omitempty"`
}

type ExplainResponse struct {
	QueryPlan []map[string]interface{} `json:"queryPlan"`
	Error     string                   `json:"error,omitempty"`
	Query     string                   `json:"query"`
}

var db *sql.DB

// InitDB initializes the database connection for EXPLAIN queries
func InitDB() error {
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
			// Quote string values if they're not already numbers
			if _, err := fmt.Sscanf(val, "%f", new(float64)); err != nil {
				// Not a number, quote it
				return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
			}
			return val
		}
		return match
	})
	
	return result
}

// ExplainQuery runs EXPLAIN (ANALYZE, FORMAT JSON) on the given query
func ExplainQuery(req ExplainRequest) ExplainResponse {
	resp := ExplainResponse{
		Query: req.Query,
	}

	if db == nil {
		resp.Error = "Database connection not configured. Set DATABASE_URL environment variable."
		return resp
	}

	query := req.Query
	
	// Substitute variables if provided
	if len(req.Variables) > 0 {
		query = substituteVariables(query, req.Variables)
		resp.Query = query
	}

	// Run EXPLAIN ANALYZE (FORMAT JSON)
	explainQuery := fmt.Sprintf("EXPLAIN (ANALYZE, FORMAT JSON) %s", query)
	
	rows, err := db.Query(explainQuery)
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

// CloseDB closes the database connection
func CloseDB() {
	if db != nil {
		db.Close()
	}
}
