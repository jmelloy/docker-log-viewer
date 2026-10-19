package logs

import "testing"

func TestMixedANSIAndPlainFields(t *testing.T) {
	line := "\x1b[32mINF\x1b[0m Processing \x1b[2mrequest_id=\x1b[0mabc123 status=success"
	entry := ParseLogLine(line)
	if entry.Raw != line || entry.Level != "INF" || entry.Message != "Processing" {
		t.Fatalf("incorrect parsed metadata: %+v", entry)
	}
	if entry.Fields["request_id"] != "abc123" || entry.Fields["status"] != "success" {
		t.Fatalf("lost mixed ANSI/plain fields: %v", entry.Fields)
	}
}

func TestSourceFileHeader(t *testing.T) {
	for _, file := range []string{"pkg/service.go:42", "/app/pkg/service.go:42"} {
		for _, source := range []string{file, "\x1b[36m" + file + "\x1b[0m"} {
			t.Run(source, func(t *testing.T) {
				entry := ParseLogLine("INF " + source + " > Processing request_id=abc123")
				if entry.Level != "INF" || entry.File != file || entry.Message != "Processing" || entry.Fields["request_id"] != "abc123" {
					t.Fatalf("incorrect source header parsing: %+v", entry)
				}
			})
		}
	}
}

func TestMessageMetadataIsNotConsumed(t *testing.T) {
	for _, message := range []string{
		"Could not open pkg/service.go:42",
		"The ERROR count is increasing",
	} {
		t.Run(message, func(t *testing.T) {
			entry := ParseLogLine(message)
			if entry.Message != message || entry.File != "" || entry.Level != "" {
				t.Fatalf("message content was consumed as header metadata: %+v", entry)
			}
		})
	}
}
