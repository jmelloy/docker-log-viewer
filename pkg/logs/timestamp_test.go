package logs

import (
	"testing"
	"time"
)

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
		checkFunc func(time.Time) bool // Function to validate the parsed time
	}{
		{
			name:      "Oct  3 19:57:52.076536 format",
			input:     "Oct  3 19:57:52.076536",
			wantValid: true,
			checkFunc: func(ts time.Time) bool {
				return ts.Month() == time.October && ts.Day() == 3 &&
					ts.Hour() == 19 && ts.Minute() == 57 && ts.Second() == 52
			},
		},
		{
			name:      "Oct 3 19:57:52.076536 format (single space)",
			input:     "Oct 3 19:57:52.076536",
			wantValid: true,
			checkFunc: func(ts time.Time) bool {
				return ts.Month() == time.October && ts.Day() == 3 &&
					ts.Hour() == 19 && ts.Minute() == 57 && ts.Second() == 52
			},
		},
		{
			name:      "RFC3339 format",
			input:     "2024-10-03T19:57:52.076536Z",
			wantValid: true,
			checkFunc: func(ts time.Time) bool {
				return ts.Year() == 2024 && ts.Month() == time.October && ts.Day() == 3 &&
					ts.Hour() == 19 && ts.Minute() == 57 && ts.Second() == 52
			},
		},
		{
			name:      "Time only format HH:MM:SS.ffffff",
			input:     "19:57:52.076536",
			wantValid: true,
			checkFunc: func(ts time.Time) bool {
				return ts.Hour() == 19 && ts.Minute() == 57 && ts.Second() == 52
			},
		},
		{
			name:      "Time only format [HH:MM:SS.fff]",
			input:     "[19:57:52.076]",
			wantValid: true,
			checkFunc: func(ts time.Time) bool {
				return ts.Hour() == 19 && ts.Minute() == 57 && ts.Second() == 52
			},
		},
		{
			name:      "ISO 8601 format",
			input:     "2024-10-03 19:57:52.076536",
			wantValid: true,
			checkFunc: func(ts time.Time) bool {
				return ts.Year() == 2024 && ts.Month() == time.October && ts.Day() == 3 &&
					ts.Hour() == 19 && ts.Minute() == 57 && ts.Second() == 52
			},
		},
		{
			name:      "Unix timestamp (seconds)",
			input:     "1696361872",
			wantValid: true,
			checkFunc: func(ts time.Time) bool {
				return ts.Unix() == 1696361872
			},
		},
		{
			name:      "Unix timestamp (milliseconds)",
			input:     "1696361872076",
			wantValid: true,
			checkFunc: func(ts time.Time) bool {
				return ts.UnixMilli() == 1696361872076
			},
		},
		{
			name:      "Empty string",
			input:     "",
			wantValid: false,
		},
		{
			name:      "Invalid format",
			input:     "not a timestamp",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseTimestamp(tt.input)
			if ok != tt.wantValid {
				t.Errorf("ParseTimestamp(%q) valid = %v, want %v", tt.input, ok, tt.wantValid)
				return
			}

			if tt.wantValid && tt.checkFunc != nil {
				if !tt.checkFunc(got) {
					t.Errorf("ParseTimestamp(%q) = %v, failed validation check", tt.input, got)
				}
			}
		})
	}
}

func TestParseTimestampPreservesNanoseconds(t *testing.T) {
	input := "Oct  3 19:57:52.076536"
	ts, ok := ParseTimestamp(input)
	if !ok {
		t.Fatalf("ParseTimestamp(%q) failed", input)
	}

	// Check that microseconds are preserved (076536 microseconds = 76536000 nanoseconds)
	if ts.Nanosecond() < 76000000 || ts.Nanosecond() > 77000000 {
		t.Errorf("ParseTimestamp(%q) nanoseconds = %d, want ~76536000", input, ts.Nanosecond())
	}
}
