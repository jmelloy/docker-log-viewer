package main

import (
	"testing"
)

func TestSubstituteVariables(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		variables map[string]string
		expected  string
	}{
		{
			name:  "Simple variable substitution",
			query: "SELECT * FROM users WHERE id = $1",
			variables: map[string]string{
				"1": "123",
			},
			expected: "SELECT * FROM users WHERE id = 123",
		},
		{
			name:  "String variable substitution",
			query: "SELECT * FROM users WHERE name = $1",
			variables: map[string]string{
				"1": "John Doe",
			},
			expected: "SELECT * FROM users WHERE name = 'John Doe'",
		},
		{
			name:  "Multiple variables",
			query: "SELECT * FROM users WHERE id = $1 AND name = $2",
			variables: map[string]string{
				"1": "123",
				"2": "Alice",
			},
			expected: "SELECT * FROM users WHERE id = 123 AND name = 'Alice'",
		},
		{
			name:  "String with quotes",
			query: "SELECT * FROM users WHERE name = $1",
			variables: map[string]string{
				"1": "O'Brien",
			},
			expected: "SELECT * FROM users WHERE name = 'O''Brien'",
		},
		{
			name:  "No variables",
			query: "SELECT * FROM users",
			variables: map[string]string{},
			expected: "SELECT * FROM users",
		},
		{
			name:  "Missing variable",
			query: "SELECT * FROM users WHERE id = $1 AND name = $2",
			variables: map[string]string{
				"1": "123",
			},
			expected: "SELECT * FROM users WHERE id = 123 AND name = $2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteVariables(tt.query, tt.variables)
			if result != tt.expected {
				t.Errorf("Expected: %s\nGot: %s", tt.expected, result)
			}
		})
	}
}
