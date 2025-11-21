package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"docker-log-parser/pkg/store"
)

// TestMarkdownExportGeneration tests the complete markdown generation flow
func TestMarkdownExportGeneration(t *testing.T) {
	// Create a sample SQL query detail similar to what frontend would receive
	detail := &store.SQLQueryDetail{
		QueryHash:       "abc123def456",
		Query:           "SELECT users.id, users.name FROM users WHERE users.active = true",
		NormalizedQuery: "SELECT users.id, users.name FROM users WHERE users.active = $1",
		Operation:       "SELECT",
		TableName:       "users",
		TotalExecutions: 42,
		AvgDuration:     15.5,
		MinDuration:     5.2,
		MaxDuration:     35.8,
		ExplainPlan:     `[{"Plan":{"Node Type":"Seq Scan","Total Cost":100.50}}]`,
		RelatedExecutions: []store.ExecutionReference{
			{
				ID:          123,
				DisplayName: "GetActiveUsers",
				DurationMS:  15.5,
				ExecutedAt:  time.Now(),
				StatusCode:  200,
			},
		},
	}

	// Simulate markdown generation (similar to frontend logic)
	markdown := simulateMarkdownGeneration(detail)

	// Verify markdown structure
	tests := []struct {
		name     string
		contains string
	}{
		{"has title", "# SQL Query Analysis Report"},
		{"has query info section", "## Query Information"},
		{"has operation", "**Operation:** SELECT"},
		{"has table name", "**Table:** users"},
		{"has executions count", "**Total Executions:** 42"},
		{"has avg duration", "15.50ms"},
		{"has SQL query section", "## SQL Query"},
		{"has normalized section", "## Normalized Query"},
		{"has explain section", "## EXPLAIN Plan"},
		{"has related executions", "## Related Executions"},
		{"has request ID", "req-abc-123"},
		{"has code blocks", "```sql"},
		{"has table header", "| Request ID | Display Name |"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(markdown, tt.contains) {
				t.Errorf("Markdown should contain %q", tt.contains)
			}
		})
	}

	// Verify markdown is not empty
	if len(markdown) < 100 {
		t.Errorf("Markdown is too short: %d bytes", len(markdown))
	}

	// Verify explain plan is formatted
	if !strings.Contains(markdown, "Node Type") {
		t.Error("EXPLAIN plan should be included in markdown")
	}
}

// simulateMarkdownGeneration mimics the frontend markdown generation
func simulateMarkdownGeneration(detail *store.SQLQueryDetail) string {
	var sb strings.Builder

	sb.WriteString("# SQL Query Analysis Report\n\n")
	sb.WriteString("**Generated:** " + time.Now().Format(time.RFC1123) + "\n\n")

	if len(detail.RelatedExecutions) > 0 {
		exec := detail.RelatedExecutions[0]
		sb.WriteString("**Execution Date:** " + exec.ExecutedAt.Format(time.RFC1123) + "\n\n")
	}

	sb.WriteString("## Query Information\n\n")
	sb.WriteString("- **Operation:** " + detail.Operation + "\n")
	sb.WriteString("- **Table:** " + detail.TableName + "\n")
	sb.WriteString(fmt.Sprintf("- **Total Executions:** %d\n", detail.TotalExecutions))
	sb.WriteString(fmt.Sprintf("- **Average Duration:** %.2fms\n", detail.AvgDuration))
	sb.WriteString(fmt.Sprintf("- **Min Duration:** %.2fms\n", detail.MinDuration))
	sb.WriteString(fmt.Sprintf("- **Max Duration:** %.2fms\n\n", detail.MaxDuration))

	sb.WriteString("## SQL Query\n\n")
	sb.WriteString("```sql\n" + detail.Query + "\n```\n\n")

	sb.WriteString("## Normalized Query\n\n")
	sb.WriteString("```sql\n" + detail.NormalizedQuery + "\n```\n\n")

	if detail.ExplainPlan != "" {
		sb.WriteString("## EXPLAIN Plan\n\n")
		// Format the plan
		var planData interface{}
		if err := json.Unmarshal([]byte(detail.ExplainPlan), &planData); err == nil {
			formatted, _ := json.MarshalIndent(planData, "", "  ")
			sb.WriteString("```\n" + string(formatted) + "\n```\n\n")
		} else {
			sb.WriteString("```\n" + detail.ExplainPlan + "\n```\n\n")
		}
	}

	if len(detail.RelatedExecutions) > 0 {
		sb.WriteString("## Related Executions\n\n")
		sb.WriteString("| ID | Display Name | Status | Duration | Executed At |\n")
		sb.WriteString("|----|--------------|--------|----------|-------------|\n")
		for _, exec := range detail.RelatedExecutions {
			sb.WriteString(fmt.Sprintf("| %d | %s | %d | %.2fms | %s |\n",
				exec.ID,
				exec.DisplayName,
				exec.StatusCode,
				exec.DurationMS,
				exec.ExecutedAt.Format(time.RFC1123)))
		}
	}

	return sb.String()
}
