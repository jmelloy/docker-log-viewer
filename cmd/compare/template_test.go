package main

import (
	"docker-log-parser/pkg/logs"
	"os"
	"testing"
	"time"
)

// TestEmbeddedTemplate verifies that the template is embedded correctly
func TestEmbeddedTemplate(t *testing.T) {
	// Create temporary output file
	tmpfile, err := os.CreateTemp("", "comparison-*.html")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	// Create minimal test data
	result1 := &RequestResult{
		URL:        "http://test1.example.com",
		RequestID:  "test-req-1",
		Duration:   100 * time.Millisecond,
		StatusCode: 200,
		Logs:       []logs.LogMessage{},
		SQLAnalysis: &SQLAnalysis{
			TotalQueries:   0,
			UniqueQueries:  0,
			AvgDuration:    0,
			TotalDuration:  0,
			TablesAccessed: make(map[string]int),
		},
	}

	result2 := &RequestResult{
		URL:        "http://test2.example.com",
		RequestID:  "test-req-2",
		Duration:   120 * time.Millisecond,
		StatusCode: 200,
		Logs:       []logs.LogMessage{},
		SQLAnalysis: &SQLAnalysis{
			TotalQueries:   0,
			UniqueQueries:  0,
			AvgDuration:    0,
			TotalDuration:  0,
			TablesAccessed: make(map[string]int),
		},
	}

	postData := `{"query": "{ test }"}`

	// Generate HTML using embedded template
	err = generateHTML(tmpfile.Name(), result1, result2, postData)
	if err != nil {
		t.Fatalf("Failed to generate HTML: %v", err)
	}

	// Verify the output file was created and has content
	stat, err := os.Stat(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}

	if stat.Size() == 0 {
		t.Fatal("Output file is empty")
	}

	// Read and verify content contains expected HTML elements
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// Check for essential HTML elements
	expectedStrings := []string{
		"<!DOCTYPE html>",
		"URL Comparison Report",
		"http://test1.example.com",
		"http://test2.example.com",
		"test-req-1",
		"test-req-2",
	}

	contentStr := string(content)
	for _, expected := range expectedStrings {
		if !contains(contentStr, expected) {
			t.Errorf("Output HTML missing expected string: %s", expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
