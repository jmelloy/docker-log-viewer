package handlers

import (
	"encoding/json"
	"strings"

	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
)

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// SerializeLogEntry converts a logs.LogEntry into fields for logstore
func SerializeLogEntry(entry *logs.LogEntry) (message string, fields map[string]string) {
	if entry == nil {
		return "", make(map[string]string)
	}

	fields = make(map[string]string)
	message = entry.Message

	// Store the raw log line as a field
	if entry.Raw != "" {
		fields["_raw"] = entry.Raw
	}
	if entry.Timestamp != "" {
		fields["_timestamp"] = entry.Timestamp
	}
	if entry.Level != "" {
		fields["_level"] = entry.Level
	}
	if entry.File != "" {
		fields["_file"] = entry.File
	}

	// Copy all parsed fields
	for k, v := range entry.Fields {
		fields[k] = v
	}

	// Store JSON flag
	if entry.IsJSON {
		fields["_is_json"] = "true"
	}

	return message, fields
}

// DeserializeLogEntry reconstructs a logs.LogEntry from logstore.LogMessage
func DeserializeLogEntry(msg *logstore.LogMessage) *logs.LogEntry {
	if msg == nil {
		return nil
	}

	entry := &logs.LogEntry{
		Message: msg.Message,
		Fields:  make(map[string]string),
	}

	// Extract special fields
	if raw, ok := msg.Fields["_raw"]; ok {
		entry.Raw = raw
	}
	if timestamp, ok := msg.Fields["_timestamp"]; ok {
		entry.Timestamp = timestamp
	}
	if level, ok := msg.Fields["_level"]; ok {
		entry.Level = level
	}
	if file, ok := msg.Fields["_file"]; ok {
		entry.File = file
	}
	if isJSON, ok := msg.Fields["_is_json"]; ok {
		entry.IsJSON = isJSON == "true"
	}

	// Copy non-special fields
	for k, v := range msg.Fields {
		if !strings.HasPrefix(k, "_") {
			entry.Fields[k] = v
		}
	}

	return entry
}
