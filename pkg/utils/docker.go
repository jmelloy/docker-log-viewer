package utils

import (
	"net/url"
	"os"
	"strings"
)

// IsRunningInDocker checks if the current process is running inside a Docker container
// by checking for the existence of /.dockerenv file
func IsRunningInDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// ReplaceLocalhostWithDockerHost replaces localhost or 127.0.0.1 in URLs with host.docker.internal
// when running inside a Docker container. This allows containers to access services
// running on the host machine.
func ReplaceLocalhostWithDockerHost(urlStr string) string {
	if !IsRunningInDocker() {
		return urlStr
	}

	// Parse the URL
	parsed, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, try simple string replacement as fallback
		urlStr = strings.ReplaceAll(urlStr, "localhost", "host.docker.internal")
		urlStr = strings.ReplaceAll(urlStr, "127.0.0.1", "host.docker.internal")
		return urlStr
	}

	// Replace localhost or 127.0.0.1 in host
	if parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "0.0.0.0" {
		parsed.Host = strings.Replace(parsed.Host, parsed.Hostname(), "host.docker.internal", 1)
	}

	return parsed.String()
}
