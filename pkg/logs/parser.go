package logs

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type LogEntry struct {
	Raw        string                 `json:"raw"`
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	File       string                 `json:"file"`
	Message    string                 `json:"message"`
	Fields     map[string]string      `json:"fields"`
	IsJSON     bool                   `json:"isJson"`
	JSONFields map[string]interface{} `json:"jsonFields,omitempty"`
}

var (
	timestampRegex = regexp.MustCompile(`(\d{1,2}\s+\w+\s+\d{4}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?|\d{4}[-/]\d{2}[-/]\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?|\w+\s+\d+\s+\d+:\d+:\d+(?:\.\d+)?|\[\d{2}:\d{2}:\d{2}\.\d+\]|\d{2}:\d{2}:\d{2}(?:\.\d+)?|\d+[-/]\d+[-/]\d+\s+\d+:\d+:\d+(?:\.\d+)?|\b\d{10,13}\b)`)
	levelRegex     = regexp.MustCompile(`(FATAL|DEBUG|INFO|WARN|ERROR|DBG|TRC|INF|WRN|ERR)`)
	fileRegex      = regexp.MustCompile(`([\w/]+\.go:\d+)`)
	ansiRegex      = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHfABCDsuJSTlh]|\x1b\][^\x07]*\x07|\x1b[>=]|\x1b\[?[\d;]*[a-zA-Z]`)
	sentryRegex    = regexp.MustCompile(`^Sentry Logger \[(log|warn|error|debug)\]:\s*(.*)$`)
	pinoRegex      = regexp.MustCompile(`^\[(\d{2}:\d{2}:\d{2}\.\d+)\]\s+(DEBUG|INFO|WARN|ERROR)\s+\((\d+)\):\s+(.*)$`)
	queryRegex     = regexp.MustCompile(`^\[query\]\s+(.+?)(?:\s+\[took\s+(\d+)\s*ms(?:,\s*(\d+)\s+(row|result)s?\s+affected)?\])?$`)
)

func stripANSI(s string) string {
	cleaned := ansiRegex.ReplaceAllString(s, "")
	cleaned = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, cleaned)
	return cleaned
}

func parseKeyValuePairs(s string) map[string]string {
	fields := make(map[string]string)
	i := 0

	for i < len(s) {
		keyStart := i
		for i < len(s) && (s[i] == '_' || s[i] == '.' || (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= '0' && s[i] <= '9')) {
			i++
		}

		if i >= len(s) || s[i] != '=' {
			i++
			continue
		}

		key := s[keyStart:i]
		if key == "" {
			i++
			continue
		}

		i++
		if i >= len(s) {
			break
		}

		var value string
		if s[i] == '"' {
			i++
			valueStart := i
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					i += 2
				} else {
					i++
				}
			}
			value = s[valueStart:i]
			if i < len(s) {
				i++
			}
		} else if s[i] == '{' {
			depth := 1
			i++
			valueStart := i - 1
			for i < len(s) && depth > 0 {
				if s[i] == '{' {
					depth++
				} else if s[i] == '}' {
					depth--
				}
				i++
			}
			value = s[valueStart:i]
		} else if s[i] == '[' {
			depth := 1
			i++
			valueStart := i - 1
			for i < len(s) && depth > 0 {
				if s[i] == '[' {
					depth++
				} else if s[i] == ']' {
					depth--
				}
				i++
			}
			value = s[valueStart:i]
		} else {
			valueStart := i
			for i < len(s) && s[i] != ' ' {
				i++
			}
			value = s[valueStart:i]
		}

		fields[key] = value

		for i < len(s) && s[i] == ' ' {
			i++
		}
	}

	return fields
}

func ParseLogLine(line string) *LogEntry {
	entry := &LogEntry{
		Raw:    line,
		Fields: make(map[string]string),
	}

	if strings.TrimSpace(line) == "" {
		return entry
	}

	line = stripANSI(line)

	// Try Sentry Logger format
	if matches := sentryRegex.FindStringSubmatch(line); len(matches) >= 3 {
		entry.Level = strings.ToUpper(matches[1])
		entry.Message = matches[2]
		return entry
	}

	// Try Pino format: [HH:MM:SS.mmm] LEVEL (pid): message {json}
	if matches := pinoRegex.FindStringSubmatch(line); len(matches) >= 5 {
		entry.Timestamp = matches[1]
		entry.Level = matches[2]
		entry.Fields["pid"] = matches[3]
		remaining := matches[4]

		// Check if there's JSON at the end
		if idx := strings.Index(remaining, "{"); idx >= 0 {
			entry.Message = strings.TrimSpace(remaining[:idx])
			jsonPart := remaining[idx:]
			if json.Valid([]byte(jsonPart)) {
				entry.IsJSON = true
				entry.JSONFields = make(map[string]interface{})
				json.Unmarshal([]byte(jsonPart), &entry.JSONFields)

				// Extract specific fields from nested JSON
				if req, ok := entry.JSONFields["req"].(map[string]interface{}); ok {
					if method, ok := req["method"].(string); ok {
						entry.Fields["method"] = method
					}
					if headers, ok := req["headers"].(map[string]interface{}); ok {
						if contentLength, ok := headers["content-length"].(string); ok {
							entry.Fields["content-length"] = contentLength
						}
						if baggage, ok := headers["baggage"].(string); ok {
							// Parse baggage for sentry-trace_id
							pairs := strings.Split(baggage, ",")
							for _, pair := range pairs {
								kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
								if len(kv) == 2 && kv[0] == "sentry-trace_id" {
									entry.Fields["trace_id"] = kv[1]
									break
								}
							}
						}
					}
				}
				if res, ok := entry.JSONFields["res"].(map[string]interface{}); ok {
					if statusCode, ok := res["statusCode"].(float64); ok {
						entry.Fields["statusCode"] = strconv.Itoa(int(statusCode))
					}
				}
				if responseTime, ok := entry.JSONFields["responseTime"].(float64); ok {
					entry.Fields["responseTime"] = strconv.FormatFloat(responseTime, 'f', -1, 64)
				}
			}
		} else {
			entry.Message = remaining
		}
		return entry
	}

	// Try query format: [query] statement [took X ms, Y results]
	if matches := queryRegex.FindStringSubmatch(line); len(matches) >= 2 {
		entry.Level = "INFO"
		entry.Message = matches[1]
		entry.Fields["type"] = "query"
		if len(matches) > 2 && matches[2] != "" {
			entry.Fields["duration_ms"] = matches[2]
		}
		if len(matches) > 3 && matches[3] != "" {
			entry.Fields["rows"] = matches[3]
		}
		return entry
	}

	if json.Valid([]byte(line)) {
		entry.IsJSON = true
		entry.JSONFields = make(map[string]interface{})
		json.Unmarshal([]byte(line), &entry.JSONFields)

		// Try multiple common timestamp field names
		for _, key := range []string{"timestamp", "@timestamp", "time", "ts", "datetime", "date"} {
			if ts, ok := entry.JSONFields[key].(string); ok {
				entry.Timestamp = ts
				break
			}
		}

		// Try multiple common level field names
		for _, key := range []string{"level", "severity", "log_level", "loglevel", "lvl"} {
			if lvl, ok := entry.JSONFields[key].(string); ok {
				entry.Level = strings.ToUpper(lvl)
				break
			}
		}

		// Try multiple common message field names
		for _, key := range []string{"message", "msg", "text", "log", "event"} {
			if msg, ok := entry.JSONFields[key].(string); ok {
				entry.Message = msg
				break
			}
		}

		// Populate fields map with all JSON fields (excluding already extracted ones)
		for key, value := range entry.JSONFields {
			// Skip the ones we already extracted into dedicated fields
			if key == "level" || key == "severity" || key == "log_level" || key == "loglevel" || key == "lvl" ||
				key == "message" || key == "msg" || key == "text" || key == "log" || key == "event" ||
				key == "timestamp" || key == "@timestamp" || key == "time" || key == "ts" || key == "datetime" || key == "date" {
				continue
			}

			// Convert value to string for fields map
			switch v := value.(type) {
			case string:
				entry.Fields[key] = v
			case nil:
				entry.Fields[key] = ""
			default:
				// Convert complex types to JSON string
				if jsonBytes, err := json.Marshal(v); err == nil {
					entry.Fields[key] = string(jsonBytes)
				}
			}
		}

		return entry
	}

	remaining := line

	if matches := timestampRegex.FindStringSubmatch(line); len(matches) > 1 {
		entry.Timestamp = matches[1]
		idx := strings.Index(line, matches[1])
		remaining = line[idx+len(matches[1]):]
		remaining = strings.TrimSpace(remaining)
	}

	if matches := levelRegex.FindStringSubmatch(remaining); len(matches) > 0 {
		entry.Level = matches[0]
		remaining = strings.Replace(remaining, matches[0], "", 1)
		remaining = strings.TrimSpace(remaining)
	}

	if matches := fileRegex.FindStringSubmatch(remaining); len(matches) > 1 {
		entry.File = matches[1]
		remaining = strings.Replace(remaining, matches[0], "", 1)
		remaining = strings.TrimSpace(remaining)
	}

	if strings.HasPrefix(remaining, ">") {
		remaining = strings.TrimPrefix(remaining, ">")
		remaining = strings.TrimSpace(remaining)
	}

	fields := parseKeyValuePairs(remaining)
	if len(fields) > 0 {
		firstKey := ""
		for k := range fields {
			idx := strings.Index(remaining, k+"=")
			if idx >= 0 && (firstKey == "" || idx < strings.Index(remaining, firstKey+"=")) {
				firstKey = k
			}
		}
		if firstKey != "" {
			firstFieldIdx := strings.Index(remaining, firstKey+"=")
			if firstFieldIdx > 0 {
				entry.Message = strings.TrimSpace(remaining[:firstFieldIdx])
			}
		}
		entry.Fields = fields
	} else {
		entry.Message = remaining
	}

	return entry
}

func (e *LogEntry) MatchesSearch(query string) bool {
	query = strings.ToLower(query)
	if strings.Contains(strings.ToLower(e.Raw), query) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Message), query) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Level), query) {
		return true
	}
	for k, v := range e.Fields {
		if strings.Contains(strings.ToLower(k), query) || strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}

func (e *LogEntry) FormattedString() string {
	var sb strings.Builder

	if e.Timestamp != "" {
		sb.WriteString("[yellow]")
		sb.WriteString(e.Timestamp)
		sb.WriteString("[white] ")
	}

	if e.Level != "" {
		color := "white"
		switch strings.ToUpper(e.Level) {
		case "ERR", "ERROR", "FATAL":
			color = "red"
		case "WRN", "WARN":
			color = "orange"
		case "INF", "INFO":
			color = "green"
		case "DBG", "DEBUG":
			color = "blue"
		case "TRC", "TRACE":
			color = "gray"
		}
		sb.WriteString("[")
		sb.WriteString(color)
		sb.WriteString("]")
		sb.WriteString(e.Level)
		sb.WriteString("[white] ")
	}

	if e.File != "" {
		sb.WriteString("[cyan]")
		sb.WriteString(e.File)
		sb.WriteString("[white] ")
	}

	if e.Message != "" {
		sb.WriteString(e.Message)
		sb.WriteString(" ")
	}

	if len(e.Fields) > 0 {
		sb.WriteString("[gray]")
		for k, v := range e.Fields {
			sb.WriteString(k)
			sb.WriteString("=")
			if len(v) > 100 {
				sb.WriteString(v[:100])
				sb.WriteString("...")
			} else {
				sb.WriteString(v)
			}
			sb.WriteString(" ")
		}
		sb.WriteString("[white]")
	}

	if e.IsJSON && len(e.JSONFields) > 0 {
		sb.WriteString("[purple]JSON[white] ")
	}

	return sb.String()
}

// ParseTimestamp attempts to parse the timestamp string from a log entry
// Returns the parsed time and true if successful, otherwise returns zero time and false
func ParseTimestamp(timestampStr string) (time.Time, bool) {
	if timestampStr == "" {
		return time.Time{}, false
	}

	// Try various timestamp formats
	formats := []string{
		// Oct  3 19:57:52.076536
		"Jan  2 15:04:05.000000",
		"Jan _2 15:04:05.000000",
		// 2024-10-03T19:57:52.076536Z
		time.RFC3339Nano,
		time.RFC3339,
		// 2024-10-03 19:57:52.076536
		"2006-01-02T15:04:05.000000",
		"2006-01-02 15:04:05.000000",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		// [19:57:52.076]
		"[15:04:05.000]",
		// 19:57:52.076536
		"15:04:05.000000",
		"15:04:05.000",
		"15:04:05",
	}

	for _, format := range formats {
		t, err := time.Parse(format, timestampStr)
		if err == nil {
			// For formats without year/date, add current year/date
			if format == "Jan  2 15:04:05.000000" || format == "Jan _2 15:04:05.000000" {
				now := time.Now()
				t = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
			} else if format == "[15:04:05.000]" || format == "15:04:05.000000" || format == "15:04:05.000" || format == "15:04:05" {
				now := time.Now()
				t = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
			}
			return t, true
		}
	}

	// Try parsing as unix timestamp (10 digits for seconds, 13 digits for milliseconds)
	if len(timestampStr) == 10 || len(timestampStr) == 13 {
		if ts, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
			if len(timestampStr) == 13 {
				// Milliseconds
				return time.Unix(0, ts*1e6), true
			} else {
				// Seconds (length is 10)
				return time.Unix(ts, 0), true
			}
		}
	}

	return time.Time{}, false
}

func init() {
	time.Local = time.UTC
}
