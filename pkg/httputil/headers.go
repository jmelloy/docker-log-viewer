package httputil

import (
	"fmt"
	"net/http"
	"strings"
)

// ValidateHeaders rejects malformed headers and headers owned by transport or log correlation.
func ValidateHeaders(headers map[string]string) error {
	seen := make(map[string]bool)
	for name, value := range headers {
		if name == "" {
			return fmt.Errorf("header name is required")
		}
		for _, ch := range name {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", ch)) {
				return fmt.Errorf("invalid header name %q", name)
			}
		}
		key := http.CanonicalHeaderKey(name)
		if seen[key] {
			return fmt.Errorf("duplicate header %q", name)
		}
		seen[key] = true
		switch key {
		case "Host", "Content-Length", "Transfer-Encoding", "Connection", "Trailer", "Upgrade", "Proxy-Connection", "Te", "X-Request-Id":
			return fmt.Errorf("header %q is managed by the application", name)
		}
		for _, ch := range value {
			if ch == 127 || (ch < 32 && ch != '\t') {
				return fmt.Errorf("invalid value for header %q", name)
			}
		}
	}
	return nil
}

// MergeHeaders applies later layers over earlier ones, ignoring header-name casing.
func MergeHeaders(layers ...map[string]string) map[string]string {
	headers := make(map[string]string)
	for _, layer := range layers {
		for name, value := range layer {
			headers[http.CanonicalHeaderKey(name)] = value
		}
	}
	return headers
}
