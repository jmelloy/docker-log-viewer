package httputil

import (
	"testing"
)

func TestValidateHeaders(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers map[string]string
		valid   bool
	}{
		{"empty", nil, true},
		{"custom", map[string]string{"X-User-ID": "user-1", "Authorization": "Bearer token", "X-Empty": ""}, true},
		{"blank name", map[string]string{"": "value"}, false},
		{"invalid name", map[string]string{"Bad Header": "value"}, false},
		{"unicode name", map[string]string{"X-☃": "value"}, false},
		{"newline", map[string]string{"X-Test": "value\r\nInjected: yes"}, false},
		{"control", map[string]string{"X-Test": "\x00"}, false},
		{"correlation", map[string]string{"x-request-id": "override"}, false},
		{"transport", map[string]string{"Content-Length": "1"}, false},
		{"host", map[string]string{"Host": "elsewhere"}, false},
		{"duplicate case", map[string]string{"X-Test": "1", "x-test": "2"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateHeaders(tc.headers); (err == nil) != tc.valid {
				t.Fatalf("ValidateHeaders = %v, valid = %v", err, tc.valid)
			}
		})
	}
}

func TestMergeHeaders(t *testing.T) {
	server := map[string]string{"X-Test": "server", "X-Server": "kept"}
	headers := MergeHeaders(server, map[string]string{"x-test": "sample"}, map[string]string{"X-TEST": "execution"})
	if headers["X-Test"] != "execution" || headers["X-Server"] != "kept" || len(headers) != 2 {
		t.Fatalf("unexpected headers: %v", headers)
	}
	if server["X-Test"] != "server" {
		t.Fatal("server defaults mutated")
	}
}
