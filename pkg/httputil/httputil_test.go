package httputil

import (
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == "" {
		t.Error("GenerateRequestID returned empty string")
	}

	if len(id1) != 8 {
		t.Errorf("Expected ID length of 8, got %d", len(id1))
	}

	// IDs should be different
	if id1 == id2 {
		t.Error("GenerateRequestID returned same ID twice")
	}
}

func TestMakeHTTPRequest_InvalidURL(t *testing.T) {
	_, _, _, err := MakeHTTPRequest("://invalid-url", []byte("test"), "test-id", "", "", "")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

func TestCollectLogsForRequest(t *testing.T) {
	// This test is skipped because it requires a real logstore
	// The function is tested indirectly through integration tests
	t.Skip("Requires logstore setup")
}

func TestContainsErrorsKey(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected bool
	}{
		{
			name:     "no errors",
			data:     map[string]interface{}{"data": "value"},
			expected: false,
		},
		{
			name:     "has errors key",
			data:     map[string]interface{}{"errors": []interface{}{map[string]interface{}{"message": "error"}}},
			expected: true,
		},
		{
			name:     "empty errors",
			data:     map[string]interface{}{"errors": []interface{}{}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErrors, _, _ := ContainsErrorsKey(tt.data, "")
			if hasErrors != tt.expected {
				t.Errorf("Expected hasErrors=%v, got %v", tt.expected, hasErrors)
			}
		})
	}
}
