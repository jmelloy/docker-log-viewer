package sqlutil

import (
	"testing"

	"docker-log-parser/pkg/logs"
)

func TestExtractSQLQueries(t *testing.T) {
	tests := []struct {
		name     string
		messages []logs.ContainerMessage
		expected int
	}{
		{
			name:     "empty messages",
			messages: []logs.ContainerMessage{},
			expected: 0,
		},
		{
			name: "message with sql format",
			messages: []logs.ContainerMessage{
				{
					Entry: &logs.LogEntry{
						Message: "[sql]: SELECT * FROM users",
						Fields:  map[string]string{},
					},
				},
			},
			expected: 1,
		},
		{
			name: "message with query type",
			messages: []logs.ContainerMessage{
				{
					Entry: &logs.LogEntry{
						Message: "SELECT * FROM users",
						Fields: map[string]string{
							"type": "query",
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "non-SQL message",
			messages: []logs.ContainerMessage{
				{
					Entry: &logs.LogEntry{
						Message: "regular log message",
						Fields:  map[string]string{},
					},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := ExtractSQLQueries(tt.messages)
			if len(queries) != tt.expected {
				t.Errorf("Expected %d queries, got %d", tt.expected, len(queries))
			}
		})
	}
}

func TestInterpolateSQLQuery(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		variables interface{}
		expected  string
	}{
		{
			name:      "nil variables",
			query:     "SELECT * FROM users WHERE id = $1",
			variables: nil,
			expected:  "SELECT * FROM users WHERE id = $1",
		},
		{
			name:      "string variable",
			query:     "SELECT * FROM users WHERE id = $1",
			variables: map[string]string{"1": "123"},
			expected:  "SELECT * FROM users WHERE id = 123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InterpolateSQLQuery(tt.query, tt.variables)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatSQLBasic(t *testing.T) {
	sql := "SELECT * FROM users WHERE id = 1"
	formatted := FormatSQLBasic(sql)

	if formatted == "" {
		t.Error("FormatSQLBasic returned empty string")
	}

	// Should have newlines before SELECT, FROM, WHERE
	if len(formatted) < len(sql) {
		t.Error("Formatted SQL should not be shorter than input")
	}
}

func TestConvertVariablesToMap(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int // number of keys in result
	}{
		{
			name:     "array format",
			input:    []interface{}{"value1", "value2"},
			expected: 2,
		},
		{
			name:     "object format",
			input:    map[string]interface{}{"key1": "value1"},
			expected: 1,
		},
		{
			name:     "already map",
			input:    map[string]string{"key1": "value1"},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertVariablesToMap(tt.input)
			if len(result) != tt.expected {
				t.Errorf("Expected %d keys, got %d", tt.expected, len(result))
			}
		})
	}
}
