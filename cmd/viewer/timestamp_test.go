package main

import (
	"docker-log-parser/pkg/logs"
	"docker-log-parser/pkg/logstore"
	"strings"
	"testing"
	"time"
)

func TestTimestampInterpolation(t *testing.T) {
	// Create a minimal WebApp for testing
	wa := &WebApp{
		lastTimestamps: make(map[string]time.Time),
	}

	containerID := "test-container-123"

	// Test 1: First log with a timestamp - should parse and store it
	entry1 := &logs.LogEntry{
		Raw:       "Oct  3 19:57:52.076536 INFO Test message 1",
		Timestamp: "Oct  3 19:57:52.076536",
		Message:   "Test message 1",
		Level:     "INFO",
		Fields:    make(map[string]string),
	}

	msg1 := logs.LogMessage{
		ContainerID: containerID,
		Timestamp:   time.Now(), // This is what docker.go sets
		Entry:       entry1,
	}

	// Simulate what processLogs does
	var logTimestamp1 time.Time
	if msg1.Entry != nil && msg1.Entry.Timestamp != "" {
		if parsedTime, ok := logs.ParseTimestamp(msg1.Entry.Timestamp); ok {
			logTimestamp1 = parsedTime
			wa.lastTimestamps[containerID] = parsedTime
		}
	}

	if logTimestamp1.IsZero() {
		t.Fatal("Expected first log to have a parsed timestamp")
	}

	if logTimestamp1.Hour() != 19 || logTimestamp1.Minute() != 57 || logTimestamp1.Second() != 52 {
		t.Errorf("Expected timestamp 19:57:52, got %02d:%02d:%02d",
			logTimestamp1.Hour(), logTimestamp1.Minute(), logTimestamp1.Second())
	}

	// Verify last timestamp was stored
	if lastTS, ok := wa.lastTimestamps[containerID]; !ok || lastTS != logTimestamp1 {
		t.Error("Expected last timestamp to be stored for container")
	}

	// Test 2: Log without timestamp - should interpolate using last timestamp
	entry2 := &logs.LogEntry{
		Raw:       "Continuation line without timestamp",
		Timestamp: "", // No timestamp
		Message:   "Continuation line without timestamp",
		Fields:    make(map[string]string),
	}

	msg2 := logs.LogMessage{
		ContainerID: containerID,
		Timestamp:   time.Now(),
		Entry:       entry2,
	}

	var logTimestamp2 time.Time
	if msg2.Entry != nil && msg2.Entry.Timestamp != "" {
		if parsedTime, ok := logs.ParseTimestamp(msg2.Entry.Timestamp); ok {
			logTimestamp2 = parsedTime
			wa.lastTimestamps[containerID] = parsedTime
		}
	}

	if logTimestamp2.IsZero() {
		lastTS, hasLastTS := wa.lastTimestamps[containerID]
		if hasLastTS {
			logTimestamp2 = lastTS
		} else {
			logTimestamp2 = msg2.Timestamp
		}
	}

	// Should use interpolated timestamp (same as first log)
	if logTimestamp2 != logTimestamp1 {
		t.Errorf("Expected interpolated timestamp to match first timestamp, got %v instead of %v",
			logTimestamp2, logTimestamp1)
	}

	// Test 3: New container without any previous timestamp - should use time.Now()
	newContainerID := "new-container-456"
	entry3 := &logs.LogEntry{
		Raw:       "Log without timestamp",
		Timestamp: "",
		Message:   "Log without timestamp",
		Fields:    make(map[string]string),
	}

	now := time.Now()
	msg3 := logs.LogMessage{
		ContainerID: newContainerID,
		Timestamp:   now,
		Entry:       entry3,
	}

	var logTimestamp3 time.Time
	if msg3.Entry != nil && msg3.Entry.Timestamp != "" {
		if parsedTime, ok := logs.ParseTimestamp(msg3.Entry.Timestamp); ok {
			logTimestamp3 = parsedTime
			wa.lastTimestamps[newContainerID] = parsedTime
		}
	}

	if logTimestamp3.IsZero() {
		lastTS, hasLastTS := wa.lastTimestamps[newContainerID]
		if hasLastTS {
			logTimestamp3 = lastTS
		} else {
			logTimestamp3 = msg3.Timestamp
		}
	}

	// Should use time.Now() since no previous timestamp exists
	if logTimestamp3 != now {
		t.Errorf("Expected to use time.Now() for new container without timestamp history")
	}

	// Test 4: Log with new timestamp - should update last timestamp
	entry4 := &logs.LogEntry{
		Raw:       "Oct  3 19:58:00.123456 INFO New timestamp",
		Timestamp: "Oct  3 19:58:00.123456",
		Message:   "New timestamp",
		Level:     "INFO",
		Fields:    make(map[string]string),
	}

	msg4 := logs.LogMessage{
		ContainerID: containerID,
		Timestamp:   time.Now(),
		Entry:       entry4,
	}

	var logTimestamp4 time.Time
	if msg4.Entry != nil && msg4.Entry.Timestamp != "" {
		if parsedTime, ok := logs.ParseTimestamp(msg4.Entry.Timestamp); ok {
			logTimestamp4 = parsedTime
			wa.lastTimestamps[containerID] = parsedTime
		}
	}

	if logTimestamp4.IsZero() {
		t.Fatal("Expected fourth log to have a parsed timestamp")
	}

	if logTimestamp4.Hour() != 19 || logTimestamp4.Minute() != 58 || logTimestamp4.Second() != 0 {
		t.Errorf("Expected timestamp 19:58:00, got %02d:%02d:%02d",
			logTimestamp4.Hour(), logTimestamp4.Minute(), logTimestamp4.Second())
	}

	// Verify last timestamp was updated
	if lastTS, ok := wa.lastTimestamps[containerID]; !ok || lastTS != logTimestamp4 {
		t.Error("Expected last timestamp to be updated for container")
	}

	// Test 5: Another log without timestamp - should use updated last timestamp
	entry5 := &logs.LogEntry{
		Raw:       "Another continuation",
		Timestamp: "",
		Message:   "Another continuation",
		Fields:    make(map[string]string),
	}

	msg5 := logs.LogMessage{
		ContainerID: containerID,
		Timestamp:   time.Now(),
		Entry:       entry5,
	}

	var logTimestamp5 time.Time
	if msg5.Entry != nil && msg5.Entry.Timestamp != "" {
		if parsedTime, ok := logs.ParseTimestamp(msg5.Entry.Timestamp); ok {
			logTimestamp5 = parsedTime
			wa.lastTimestamps[containerID] = parsedTime
		}
	}

	if logTimestamp5.IsZero() {
		lastTS, hasLastTS := wa.lastTimestamps[containerID]
		if hasLastTS {
			logTimestamp5 = lastTS
		} else {
			logTimestamp5 = msg5.Timestamp
		}
	}

	// Should use the updated interpolated timestamp
	if logTimestamp5 != logTimestamp4 {
		t.Errorf("Expected interpolated timestamp to match latest timestamp, got %v instead of %v",
			logTimestamp5, logTimestamp4)
	}
}

func TestSerializeLogEntry(t *testing.T) {
	entry := &logs.LogEntry{
		Raw:       "Oct  3 19:57:52.076536 INFO Test message",
		Timestamp: "Oct  3 19:57:52.076536",
		Message:   "Test message",
		Level:     "INFO",
		File:      "test.go:123",
		Fields: map[string]string{
			"request_id": "req-123",
			"user_id":    "user-456",
		},
		IsJSON: false,
	}

	message, fields := serializeLogEntry(entry)

	if message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", message)
	}

	expectedFields := map[string]string{
		"_raw":       "Oct  3 19:57:52.076536 INFO Test message",
		"_timestamp": "Oct  3 19:57:52.076536",
		"_level":     "INFO",
		"_file":      "test.go:123",
		"request_id": "req-123",
		"user_id":    "user-456",
	}

	for key, expectedValue := range expectedFields {
		if fields[key] != expectedValue {
			t.Errorf("Expected field %s='%s', got '%s'", key, expectedValue, fields[key])
		}
	}

	if len(fields) != len(expectedFields) {
		t.Errorf("Expected %d fields, got %d", len(expectedFields), len(fields))
	}
}

func TestDeserializeLogEntry(t *testing.T) {
	storeMsg := &logstore.LogMessage{
		Timestamp:   time.Now(),
		ContainerID: "test-container",
		Message:     "Test message",
		Fields: map[string]string{
			"_raw":       "Oct  3 19:57:52.076536 INFO Test message",
			"_timestamp": "Oct  3 19:57:52.076536",
			"_level":     "INFO",
			"_file":      "test.go:123",
			"_is_json":   "true",
			"request_id": "req-123",
			"user_id":    "user-456",
		},
	}

	entry := deserializeLogEntry(storeMsg)

	if entry.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", entry.Message)
	}

	if entry.Raw != "Oct  3 19:57:52.076536 INFO Test message" {
		t.Errorf("Expected raw log line, got '%s'", entry.Raw)
	}

	if entry.Timestamp != "Oct  3 19:57:52.076536" {
		t.Errorf("Expected timestamp string, got '%s'", entry.Timestamp)
	}

	if entry.Level != "INFO" {
		t.Errorf("Expected level 'INFO', got '%s'", entry.Level)
	}

	if entry.File != "test.go:123" {
		t.Errorf("Expected file 'test.go:123', got '%s'", entry.File)
	}

	if !entry.IsJSON {
		t.Error("Expected IsJSON to be true")
	}

	if entry.Fields["request_id"] != "req-123" {
		t.Errorf("Expected request_id='req-123', got '%s'", entry.Fields["request_id"])
	}

	if entry.Fields["user_id"] != "user-456" {
		t.Errorf("Expected user_id='user-456', got '%s'", entry.Fields["user_id"])
	}

	// Should not include special fields (those starting with _)
	for key := range entry.Fields {
		if strings.HasPrefix(key, "_") {
			t.Errorf("Field '%s' should not be in entry.Fields", key)
		}
	}
}
