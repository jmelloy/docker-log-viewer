package main

import (
	"testing"
	"time"

	"docker-log-parser/pkg/logs"
)

// TestTimestampUsageIntegration demonstrates the end-to-end flow
// This shows how the system now uses parsed timestamps instead of time.Now()
func TestTimestampUsageIntegration(t *testing.T) {
	// Simulate a sequence of log messages with timestamps
	testLogs := []struct {
		raw          string
		timestamp    string // Expected RFC3339Nano format after parsing
		expectParsed bool
		expectedHour int
		expectedMin  int
		expectedSec  int
	}{
		{
			raw:          "Oct  3 19:57:52.076536 INFO Starting service",
			timestamp:    "2025-10-03T19:57:52.076536Z",
			expectParsed: true,
			expectedHour: 19,
			expectedMin:  57,
			expectedSec:  52,
		},
		{
			raw:          "  Continuation line without timestamp",
			timestamp:    "",
			expectParsed: false, // Will use interpolation
			expectedHour: 19,
			expectedMin:  57,
			expectedSec:  52,
		},
		{
			raw:          "Oct  3 19:58:10.123456 INFO Request received",
			timestamp:    "2025-10-03T19:58:10.123456Z",
			expectParsed: true,
			expectedHour: 19,
			expectedMin:  58,
			expectedSec:  10,
		},
		{
			raw:          "  Another continuation",
			timestamp:    "",
			expectParsed: false, // Will use updated interpolation
			expectedHour: 19,
			expectedMin:  58,
			expectedSec:  10,
		},
	}

	containerID := "test-container"
	wa := &WebApp{
		lastTimestamps: make(map[string]time.Time),
	}

	for i, testLog := range testLogs {
		entry := logs.ParseLogLine(testLog.raw)

		// Verify the parsed timestamp string matches expectations
		if testLog.expectParsed && entry.Timestamp != testLog.timestamp {
			t.Errorf("Log %d: Expected parsed timestamp '%s', got '%s'",
				i, testLog.timestamp, entry.Timestamp)
		}

		// Simulate what processLogs does
		var logTimestamp time.Time

		if entry.Timestamp != "" {
			if parsedTime, ok := logs.ParseTimestamp(entry.Timestamp); ok {
				logTimestamp = parsedTime
				wa.lastTimestamps[containerID] = parsedTime
			}
		}

		if logTimestamp.IsZero() {
			lastTS, hasLastTS := wa.lastTimestamps[containerID]
			if hasLastTS {
				logTimestamp = lastTS
			} else {
				logTimestamp = time.Now()
			}
		}

		// Verify the final timestamp matches expectations
		if logTimestamp.Hour() != testLog.expectedHour ||
			logTimestamp.Minute() != testLog.expectedMin ||
			logTimestamp.Second() != testLog.expectedSec {
			t.Errorf("Log %d: Expected timestamp %02d:%02d:%02d, got %02d:%02d:%02d",
				i,
				testLog.expectedHour, testLog.expectedMin, testLog.expectedSec,
				logTimestamp.Hour(), logTimestamp.Minute(), logTimestamp.Second())
		}

		t.Logf("Log %d: timestamp=%v, parsed=%v", i, logTimestamp, testLog.expectParsed)
	}
}

// TestMultipleContainersTimestampTracking verifies that timestamps are tracked independently per container
func TestMultipleContainersTimestampTracking(t *testing.T) {
	wa := &WebApp{
		lastTimestamps: make(map[string]time.Time),
	}

	container1 := "container1"
	container2 := "container2"

	// Container 1 logs at 19:57:52
	entry1 := logs.ParseLogLine("Oct  3 19:57:52.000000 INFO Container 1 message")
	ts1, ok1 := logs.ParseTimestamp(entry1.Timestamp)
	if !ok1 {
		t.Fatal("Failed to parse container1 timestamp")
	}
	wa.lastTimestamps[container1] = ts1

	// Container 2 logs at 19:58:00
	entry2 := logs.ParseLogLine("Oct  3 19:58:00.000000 INFO Container 2 message")
	ts2, ok2 := logs.ParseTimestamp(entry2.Timestamp)
	if !ok2 {
		t.Fatal("Failed to parse container2 timestamp")
	}
	wa.lastTimestamps[container2] = ts2

	// Verify each container has its own timestamp
	if wa.lastTimestamps[container1].Second() != 52 {
		t.Errorf("Container1 should have timestamp at :52, got :%02d",
			wa.lastTimestamps[container1].Second())
	}

	if wa.lastTimestamps[container2].Second() != 0 {
		t.Errorf("Container2 should have timestamp at :00, got :%02d",
			wa.lastTimestamps[container2].Second())
	}

	// Container 1 log without timestamp should use :52
	entry3 := logs.ParseLogLine("  Continuation from container 1")
	var logTimestamp time.Time
	if entry3.Timestamp != "" {
		if parsedTime, ok := logs.ParseTimestamp(entry3.Timestamp); ok {
			logTimestamp = parsedTime
		}
	}
	if logTimestamp.IsZero() {
		lastTS, hasLastTS := wa.lastTimestamps[container1]
		if hasLastTS {
			logTimestamp = lastTS
		}
	}

	if logTimestamp.Second() != 52 {
		t.Errorf("Container1 interpolation should use :52, got :%02d",
			logTimestamp.Second())
	}

	// Container 2 log without timestamp should use :00
	entry4 := logs.ParseLogLine("  Continuation from container 2")
	var logTimestamp2 time.Time
	if entry4.Timestamp != "" {
		if parsedTime, ok := logs.ParseTimestamp(entry4.Timestamp); ok {
			logTimestamp2 = parsedTime
		}
	}
	if logTimestamp2.IsZero() {
		lastTS, hasLastTS := wa.lastTimestamps[container2]
		if hasLastTS {
			logTimestamp2 = lastTS
		}
	}

	if logTimestamp2.Second() != 0 {
		t.Errorf("Container2 interpolation should use :00, got :%02d",
			logTimestamp2.Second())
	}
}
