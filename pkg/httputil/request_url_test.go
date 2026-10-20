package httputil

import "testing"

func TestResolveRequestURL(t *testing.T) {
	for _, tc := range []struct{ endpoint, want string }{
		{"", "https://example.com/graphql"},
		{"/graphql", "https://example.com/graphql"},
		{"/api/search?q=test", "https://example.com/api/search?q=test"},
		{"https://other.example/api", "https://other.example/api"},
		{"  /api  ", "https://example.com/api"},
	} {
		got, err := ResolveRequestURL("https://example.com/graphql", tc.endpoint)
		if err != nil || got != tc.want {
			t.Errorf("endpoint %q: got %q, %v; want %q", tc.endpoint, got, err, tc.want)
		}
	}
	for _, endpoint := range []string{"ftp://example.com/api", "http://", "/bad%zz", "https://user:password@example.com/api"} {
		if _, err := ResolveRequestURL("https://example.com", endpoint); err == nil {
			t.Errorf("accepted invalid endpoint %q", endpoint)
		}
	}
}
