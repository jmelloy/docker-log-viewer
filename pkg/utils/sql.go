package utils

import (
	"regexp"
	"strings"
)

// NormalizeQuery normalizes a SQL query by replacing parameters, strings, and numbers
// with placeholders, making it suitable for grouping similar queries.
func NormalizeQuery(query string) string {
	// Replace parameter placeholders ($1, $2, etc.)
	normalized := regexp.MustCompile(`\$\d+`).ReplaceAllString(query, "?")
	// Replace quoted strings
	normalized = regexp.MustCompile(`'[^']*'`).ReplaceAllString(normalized, "?")
	// Replace numbers
	normalized = regexp.MustCompile(`\b\d+\b`).ReplaceAllString(normalized, "?")
	// Collapse whitespace
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}
