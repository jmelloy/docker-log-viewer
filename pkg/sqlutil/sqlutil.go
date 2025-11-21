package sqlutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/sqlexplain"
	"docker-log-parser/pkg/store"
	"docker-log-parser/pkg/utils"
)

// ExtractSQLQueries extracts SQL queries from log messages and returns them as store.SQLQuery objects
func ExtractSQLQueries(logMessages []logs.LogMessage) []store.SQLQuery {
	queries := []store.SQLQuery{}

	for _, msg := range logMessages {
		if msg.Entry == nil || msg.Entry.Message == "" {
			continue
		}

		message := msg.Entry.Message
		// Check for [sql] or [query] format
		if strings.Contains(message, "[sql]") || (msg.Entry.Fields != nil && msg.Entry.Fields["type"] == "query") {
			var sqlText string
			var normalizedQuery string
			var query store.SQLQuery

			// Handle [sql] format
			index := strings.Index(message, "[sql]:")
			if index != -1 {
				message = message[index+6:]
				normalizedQuery = utils.NormalizeQuery(message)
				query = store.SQLQuery{
					Query:           message,
					NormalizedQuery: normalizedQuery,
					QueryHash:       store.ComputeQueryHash(normalizedQuery),
					ContainerID:     msg.ContainerID,
				}
			} else if msg.Entry.Fields != nil && msg.Entry.Fields["type"] == "query" {
				// Handle [query] format - message is the SQL query
				sqlText = message
				normalizedQuery = utils.NormalizeQuery(sqlText)
				query = store.SQLQuery{
					Query:           sqlText,
					NormalizedQuery: normalizedQuery,
					QueryHash:       store.ComputeQueryHash(normalizedQuery),
					ContainerID:     msg.ContainerID,
				}

				// Extract duration and rows from fields
				if duration, ok := msg.Entry.Fields["duration_ms"]; ok {
					if durationVal, err := strconv.ParseFloat(duration, 64); err == nil {
						query.DurationMS = durationVal
					}
				}
				if rows, ok := msg.Entry.Fields["rows"]; ok {
					if rowsVal, err := strconv.Atoi(rows); err == nil {
						query.Rows = rowsVal
					}
				}
			} else {
				continue
			}

			if msg.Entry.Fields != nil {
				// These apply to both [sql] and [query] formats
				if duration, ok := msg.Entry.Fields["duration"]; ok {
					var durationVal float64
					if _, err := strconv.ParseFloat(duration, 64); err == nil {
						durationVal, _ = strconv.ParseFloat(duration, 64)
						query.DurationMS = durationVal
					}
				}
				if table, ok := msg.Entry.Fields["db.table"]; ok {
					query.QueriedTable = table
				}
				if op, ok := msg.Entry.Fields["db.operation"]; ok {
					query.Operation = op
				}
				if rows, ok := msg.Entry.Fields["db.rows"]; ok {
					var rowsVal int
					if _, err := strconv.Atoi(rows); err == nil {
						rowsVal, _ = strconv.Atoi(rows)
						query.Rows = rowsVal
					}
				}
				// Store db.vars as JSON for later use in EXPLAIN
				if vars, ok := msg.Entry.Fields["db.vars"]; ok {
					query.Variables = vars
				}
				// Check both gql.operation and gql.operationName for GraphQL operation
				if gqlOp, ok := msg.Entry.Fields["gql.operation"]; ok {
					query.GraphQLOperation = gqlOp
				} else if gqlOp, ok := msg.Entry.Fields["gql.operationName"]; ok {
					query.GraphQLOperation = gqlOp
				}
				// Extract trace/request/span IDs
				if requestID, ok := msg.Entry.Fields["request_id"]; ok {
					query.LogRequestID = requestID
				}
				if spanID, ok := msg.Entry.Fields["span_id"]; ok {
					query.SpanID = spanID
				}
				if traceID, ok := msg.Entry.Fields["trace_id"]; ok {
					query.TraceID = traceID
				}

				// Store all other log fields as JSON for reference
				otherFields := make(map[string]string)
				excludedFields := map[string]bool{
					"duration":          true,
					"duration_ms":       true,
					"db.table":          true,
					"db.operation":      true,
					"db.rows":           true,
					"db.vars":           true,
					"gql.operation":     true,
					"gql.operationName": true,
					"request_id":        true,
					"span_id":           true,
					"trace_id":          true,
					"type":              true,
				}
				for k, v := range msg.Entry.Fields {
					if !excludedFields[k] {
						otherFields[k] = v
					}
				}
				if len(otherFields) > 0 {
					if fieldsJSON, err := json.Marshal(otherFields); err == nil {
						query.LogFields = string(fieldsJSON)
					}
				}
			}

			queries = append(queries, query)
		}
	}

	return queries
}

// InterpolateSQLQuery replaces placeholder variables in a SQL query with their actual values
func InterpolateSQLQuery(query string, variables interface{}) string {
	if variables == nil {
		return query
	}

	var varsMap map[string]string

	// Handle different variable formats
	switch v := variables.(type) {
	case string:
		if v == "" {
			return query
		}
		// Try to parse as JSON
		var parsed interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err != nil {
			return query
		}
		varsMap = ConvertVariablesToMap(parsed)
	case map[string]string:
		varsMap = v
	case map[string]interface{}:
		varsMap = make(map[string]string)
		for k, val := range v {
			varsMap[k] = fmt.Sprintf("%v", val)
		}
	default:
		// Try to convert to map
		varsMap = ConvertVariablesToMap(v)
	}

	if len(varsMap) == 0 {
		return query
	}

	// Use the same substitution logic as sqlexplain package
	return SubstituteVariables(query, varsMap)
}

// ConvertVariablesToMap converts variables from various formats to map[string]string
func ConvertVariablesToMap(vars interface{}) map[string]string {
	result := make(map[string]string)

	switch v := vars.(type) {
	case []interface{}:
		// Array format: ["value1", "value2"] -> {"1": "value1", "2": "value2"}
		for i, val := range v {
			result[strconv.Itoa(i+1)] = fmt.Sprintf("%v", val)
		}
	case map[string]interface{}:
		// Object format: {"key": "value"} -> {"key": "value"}
		for k, val := range v {
			result[k] = fmt.Sprintf("%v", val)
		}
	case map[string]string:
		return v
	}

	return result
}

// SubstituteVariables replaces $1, $2, etc. with actual values from variables map
func SubstituteVariables(query string, variables map[string]string) string {
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

// FormatSQLForDisplay formats SQL using sql-formatter binary or falls back to basic formatting
func FormatSQLForDisplay(sql string) string {
	if sql == "" {
		return ""
	}

	// Try to find sql-formatter binary
	// First, try in web/node_modules/.bin/sql-formatter (relative to current working directory)
	webDir := "web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		// Try to find web directory relative to executable
		execPath, err := os.Executable()
		if err == nil {
			execDir := filepath.Dir(execPath)
			// Try going up a few directories to find web
			for i := 0; i < 5; i++ {
				candidate := filepath.Join(execDir, webDir)
				if _, err := os.Stat(candidate); err == nil {
					webDir = candidate
					break
				}
				execDir = filepath.Dir(execDir)
			}
		}
	}

	sqlFormatterPath := filepath.Join(webDir, "node_modules", ".bin", "sql-formatter")
	if _, err := os.Stat(sqlFormatterPath); os.IsNotExist(err) {
		// Fallback: try using npx
		return formatSQLWithNpx(sql)
	}

	// Use the sql-formatter binary
	cmd := exec.Command(sqlFormatterPath, "--language", "postgresql")
	cmd.Stdin = strings.NewReader(sql)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// If formatting fails, fall back to basic formatting
		return FormatSQLBasic(sql)
	}

	formatted := strings.TrimSpace(out.String())
	if formatted == "" {
		return FormatSQLBasic(sql)
	}

	return formatted
}

// formatSQLWithNpx tries to format SQL using npx sql-formatter
func formatSQLWithNpx(sql string) string {
	cmd := exec.Command("npx", "--yes", "sql-formatter")
	cmd.Stdin = strings.NewReader(sql)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// If npx fails, fall back to basic formatting
		return FormatSQLBasic(sql)
	}

	formatted := strings.TrimSpace(out.String())
	if formatted == "" {
		return FormatSQLBasic(sql)
	}

	return formatted
}

// FormatSQLBasic applies basic SQL formatting as fallback
func FormatSQLBasic(sql string) string {
	// Add newlines before major keywords
	formatted := regexp.MustCompile(`\s+(SELECT|FROM|WHERE|JOIN|LEFT JOIN|RIGHT JOIN|INNER JOIN|ORDER BY|GROUP BY|HAVING|LIMIT|OFFSET)`).
		ReplaceAllString(sql, "\n$1")
	return strings.TrimSpace(formatted)
}

// FormatExplainPlanForNotion converts JSON explain plan to readable text
func FormatExplainPlanForNotion(explainPlan string) string {
	// Try to parse as JSON and format nicely
	var parsed any
	if err := json.Unmarshal([]byte(explainPlan), &parsed); err == nil {
		// It's valid JSON, format it nicely
		formatted, err := sqlexplain.FormatExplainPlanAsText(parsed)
		if err != nil {
			return explainPlan
		}
		return formatted
	}
	// Not JSON or formatting failed, return as-is
	return explainPlan
}
