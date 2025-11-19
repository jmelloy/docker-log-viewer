package main

import (
	"strings"
	"testing"
	"time"

	"docker-log-parser/pkg/store"
)

func TestFormatSQLForDisplay(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple select",
			input:    "SELECT * FROM users WHERE id = 1",
			expected: "SELECT *\nFROM users\nWHERE id = 1",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "query with join",
			input:    "SELECT u.name FROM users u LEFT JOIN orders o ON u.id = o.user_id WHERE u.active = true",
			expected: "SELECT u.name\nFROM users u\nLEFT JOIN orders o ON u.id = o.user_id\nWHERE u.active = true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSQLForDisplay(tt.input)
			if result != tt.expected {
				t.Errorf("formatSQLForDisplay() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "text shorter than max",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "text equal to max",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "text longer than max",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatExplainPlanForNotion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		hasJSON  bool
		hasError bool
	}{
		{
			name:     "valid JSON",
			input:    `{"Plan": {"Node Type": "Seq Scan"}}`,
			hasJSON:  true,
			hasError: false,
		},
		{
			name:     "invalid JSON",
			input:    "not json",
			hasJSON:  false,
			hasError: false,
		},
		{
			name:     "empty string",
			input:    "",
			hasJSON:  false,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatExplainPlanForNotion(tt.input)
			if result == "" && tt.input != "" {
				t.Errorf("formatExplainPlanForNotion() returned empty for non-empty input")
			}
			if tt.hasJSON {
				// Should be formatted JSON with indentation
				if !strings.Contains(result, "\n") && len(tt.input) > 10 {
					t.Errorf("formatExplainPlanForNotion() should format JSON with newlines")
				}
			}
		})
	}
}

func TestCreateNotionPagePayload(t *testing.T) {
	// Test that we can generate a valid Notion page structure
	detail := &store.SQLQueryDetail{
		QueryHash:       "abc123",
		Query:           "SELECT * FROM users WHERE id = $1",
		NormalizedQuery: "SELECT * FROM users WHERE id = $1",
		Operation:       "SELECT",
		TableName:       "users",
		TotalExecutions: 10,
		AvgDuration:     5.5,
		MinDuration:     1.0,
		MaxDuration:     10.0,
		ExplainPlan:     `{"Plan": {"Node Type": "Seq Scan"}}`,
		RelatedExecutions: []store.ExecutionReference{
			{
				ID:              1,
				DisplayName:     "Test Query",
				RequestIDHeader: "req-123",
				DurationMS:      5.5,
				ExecutedAt:      time.Now(),
				StatusCode:      200,
			},
		},
	}

	// Just verify the helper functions work without panicking
	formatted := formatSQLForDisplay(detail.Query)
	if formatted == "" {
		t.Error("formatSQLForDisplay should not return empty string for valid query")
	}

	explainText := formatExplainPlanForNotion(detail.ExplainPlan)
	if explainText == "" {
		t.Error("formatExplainPlanForNotion should not return empty string for valid plan")
	}

	// Test truncation
	longText := strings.Repeat("a", 3000)
	truncated := truncateText(longText, 2000)
	if len(truncated) > 2000 {
		t.Errorf("truncateText should truncate to max length, got %d", len(truncated))
	}
}
