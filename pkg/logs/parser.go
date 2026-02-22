package logs

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type LogEntry struct {
	Raw        string            `json:"raw"`
	Timestamp  string            `json:"timestamp"`
	Level      string            `json:"level"`
	File       string            `json:"file"`
	Message    string            `json:"message"`
	Fields     map[string]string `json:"fields"`
	IsJSON     bool              `json:"isJson"`
	JSONFields map[string]any    `json:"jsonFields,omitempty"`
}

var (
	timestampRegex = regexp.MustCompile(`(\d{1,2}\s+\w+\s+\d{4}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?|\d{4}[-/]\d{2}[-/]\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?|\w+\s+\d+\s+\d+:\d+:\d+(?:\.\d+)?|\[\d{2}:\d{2}:\d{2}\.\d+\]|\d{2}:\d{2}:\d{2}(?:\.\d+)?|\d+[-/]\d+[-/]\d+\s+\d+:\d+:\d+(?:\.\d+)?|\b\d{10,13}\b)`)
	levelRegex     = regexp.MustCompile(`\b(FATAL|DEBUG|INFO|ERROR|DBG|TRC|INF|WARNING|WARN|WRN|ERR)\b`)
	fileRegex      = regexp.MustCompile(`\b([\w/]+\.go:\d+)`)
	ansiRegex      = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHfABCDsuJSTlh]|\x1b\][^\x07]*\x07|\x1b[>=]|\x1b\[?[\d;]*[a-zA-Z]`)
	ansiStartRegex = regexp.MustCompile(`^\x1b\[`)
)

// startsWithANSI checks if a line starts with an ANSI escape code
// This can be used as a heuristic to identify log entry boundaries
func startsWithANSI(s string) bool {
	return ansiStartRegex.MatchString(s)
}

// tryNormalizeJSON attempts to normalize a value that may be double-encoded JSON.
// Returns the normalized JSON string, or empty string if the value is not JSON.
func tryNormalizeJSON(val string) string {
	// Check if it's a JSON-encoded string (starts and ends with quotes)
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		var unquoted string
		if err := json.Unmarshal([]byte(val), &unquoted); err == nil {
			if json.Valid([]byte(unquoted)) {
				var parsedJSON any
				if err := json.Unmarshal([]byte(unquoted), &parsedJSON); err == nil {
					if jsonBytes, err := json.Marshal(parsedJSON); err == nil {
						return string(jsonBytes)
					}
				}
			}
		}
	} else if len(val) >= 2 && (val[0] == '{' || val[0] == '[') && json.Valid([]byte(val)) {
		// If it's already valid JSON (object or array), normalize it
		var parsedJSON any
		if err := json.Unmarshal([]byte(val), &parsedJSON); err == nil {
			if jsonBytes, err := json.Marshal(parsedJSON); err == nil {
				return string(jsonBytes)
			}
		}
	}
	return ""
}

// hasANSICodes checks if a string contains any ANSI escape codes
func hasANSICodes(s string) bool {
	return ansiRegex.MatchString(s)
}

// bracketedPrefixRegex matches common log prefixes like [info], [discovery], [4:56:24 PM]
var bracketedPrefixRegex = regexp.MustCompile(`^\[[\w\s:\.]+\]`)

// IsLikelyNewLogEntry determines if a line is likely the start of a new log entry
// using multiple heuristics including ANSI escape codes, timestamps, and log levels
func IsLikelyNewLogEntry(line string) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}

	if json.Valid([]byte(line)) {
		return true
	}

	// // Check if line starts with ANSI escape code (common for new log entries)
	// if startsWithANSI(line) {
	// 	return true
	// }

	// Check for timestamp at the beginning (after stripping ANSI)
	stripped := stripANSI(line)
	if timestampRegex.MatchString(stripped) {
		// Verify timestamp is at or near the start
		match := timestampRegex.FindStringIndex(stripped)
		if match != nil && match[0] < 5 { // Allow a few characters before timestamp
			return true
		}
	}

	// Check for common log format patterns at the start
	// e.g., [INFO], [ERROR], timestamp patterns, etc.
	trimmed := strings.TrimSpace(stripped)
	if len(trimmed) > 0 {
		// Check for log level at the beginning
		if levelRegex.MatchString(trimmed[:min(20, len(trimmed))]) {
			match := levelRegex.FindStringIndex(trimmed)
			if match != nil && match[0] < 10 {
				return true
			}
		}

		// Check for bracketed prefix at start like [info], [discovery], [4:56:24 PM], etc.
		// This is common in TypeScript/Node.js loggers
		if bracketedPrefixRegex.MatchString(trimmed) {
			return true
		}
	}

	// If line starts with whitespace in the original, it's likely a continuation
	if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
		return false
	}

	return false
}

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

// Block represents a text segment with its associated ANSI codes
type Block struct {
	Text      string
	ANSICodes []string // The ANSI codes applied to this block
	Range     [2]int   // Start and end positions in the original string (including ANSI codes)
}

func (b Block) Equals(other Block) bool {
	return b.Text == other.Text && b.Range[0] == other.Range[0] && b.Range[1] == other.Range[1]
}

// ParseANSIBlocks splits a string into blocks based on ANSI escape codes
func ParseANSIBlocks(s string) []Block {
	// Regex to match ANSI escape codes like ^[[90m, ^[[0m, etc.
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)

	var blocks []Block
	var currentCodes []string
	lastIdx := 0
	textOffset := 0 // Track position in the stripped string (without ANSI codes)

	// Find all ANSI code positions
	matches := ansiRegex.FindAllStringIndex(s, -1)

	for _, match := range matches {
		start, end := match[0], match[1]

		// If there's text before this ANSI code, create a block
		if start > lastIdx {
			text := s[lastIdx:start]
			textLen := len(text)
			blocks = append(blocks, Block{
				Text:      text,
				ANSICodes: append([]string{}, currentCodes...), // Copy current codes
				Range:     [2]int{lastIdx, start},              // Range in original string
			})
			textOffset += textLen
		}

		// Extract the ANSI code
		code := s[start:end]

		// Check if it's a reset code (^[[0m or \x1b[0m)
		if code == "\x1b[0m" {
			currentCodes = []string{}
		} else {
			currentCodes = append(currentCodes, code)
		}

		lastIdx = end
	}

	// Add remaining text after last ANSI code
	if lastIdx < len(s) {
		text := s[lastIdx:]
		blocks = append(blocks, Block{
			Text:      text,
			ANSICodes: append([]string{}, currentCodes...),
			Range:     [2]int{lastIdx, len(s)},
		})
	}

	return blocks
}

// ParseKeyValues extracts key=value pairs from the end of a string.
// Keys match pattern [\w.] and values can be numbers, JSON, or strings.
// Returns a map of key-value pairs.
func ParseKeyValues(s string) (map[string]string, string) {
	result := make(map[string]string)

	pos := 0
	extractedStrings := []string{}
	for pos < len(s) {
		index := findStructuredDataStart(s[pos:])
		if index < 0 || pos+index >= len(s) {
			break
		}

		pos += index

		// Try to match key=
		keyMatch := regexp.MustCompile(`^([\w.]+)=`).FindStringSubmatchIndex(s[pos:])
		if keyMatch == nil {
			panic("keyMatch is nil")
		}

		key := s[pos+keyMatch[2] : pos+keyMatch[3]]
		valueStart := pos + keyMatch[1] // position after '='

		// Extract the value
		value, valueEnd := extractValue(s, valueStart)
		if value != "" {
			result[key] = value
			extractedStrings = append(extractedStrings, s[pos:valueEnd])
			pos = valueEnd
		} else {
			pos = valueStart + 1
		}
	}

	for _, extractedString := range extractedStrings {
		s = strings.Replace(s, extractedString, "", 1)
	}
	return result, s
}

// findStructuredDataStart finds where structured key=value data begins
func findStructuredDataStart(s string) int {
	re := regexp.MustCompile(`[\w.]+=["\{\[]`)
	if match := re.FindStringIndex(s); match != nil {
		return match[0]
	}

	re = regexp.MustCompile(`[\w.]+=`)
	if match := re.FindStringIndex(s); match != nil {
		return match[0]
	}
	return -1
}

// extractValue extracts a value starting at position i
func extractValue(s string, i int) (string, int) {
	if i >= len(s) {
		return "", i
	}

	start := i

	// Handle quoted string - return unquoted value
	if s[i] == '"' {
		i++
		valueStart := i
		for i < len(s) {
			if s[i] == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if s[i] == '"' {
				value := s[valueStart:i]
				i++
				return value, i
			}
			i++
		}
		return s[valueStart:], len(s)
	}

	// Handle JSON object
	if s[i] == '{' {
		return extractBracketed(s, i, '{', '}')
	}

	// Handle JSON array
	if s[i] == '[' {
		return extractBracketed(s, i, '[', ']')
	}

	// Handle unquoted value - stop at whitespace followed by key=
	for i < len(s) {
		if unicode.IsSpace(rune(s[i])) {
			// Look ahead to see if next non-whitespace is a key=
			j := i
			for j < len(s) && unicode.IsSpace(rune(s[j])) {
				j++
			}

			if j < len(s) && regexp.MustCompile(`^[\w.]+=["\{\[]`).MatchString(s[j:]) {
				return strings.TrimSpace(s[start:i]), i
			}
			if j < len(s) && regexp.MustCompile(`^[\w.]+=`).MatchString(s[j:]) {
				return strings.TrimSpace(s[start:i]), i
			}

			if j == len(s) {
				return strings.TrimSpace(s[start:i]), i
			}

			return "", -1
		}
		i++
	}

	return strings.TrimSpace(s[start:i]), i
}

// extractBracketed extracts bracketed content (JSON objects/arrays)
func extractBracketed(s string, i int, open, close byte) (string, int) {
	start := i
	depth := 0
	inString := false

	for i < len(s) {
		c := s[i]

		if inString {
			if c == '\\' && i+1 < len(s) {
				i += 2
				continue
			}
			if c == '"' {
				inString = false
			}
			i++
			continue
		}

		if c == '"' {
			inString = true
		} else if c == open {
			depth++
		} else if c == close {
			depth--
			if depth == 0 {
				i++
				return s[start:i], i
			}
		}
		i++
	}

	return s[start:], len(s)
}

func extractRequestFields(line string) (map[string]string, string) {
	fields := make(map[string]string)
	if strings.HasSuffix(line, "}") {
		// try to extract chunk of json
		for i := len(line) - 1; i >= 0; i-- {
			if line[i] == '{' {
				jsonPart := line[i:]
				if json.Valid([]byte(jsonPart)) {
					var jsonFields map[string]any
					err := json.Unmarshal([]byte(jsonPart), &jsonFields)
					if err != nil {
						return nil, ""
					}
					if req, ok := jsonFields["req"].(map[string]any); ok {
						if id, ok := req["id"]; ok {
							fields["request_id"] = fmt.Sprintf("%v", id)
						}
						if method, ok := req["method"].(string); ok {
							fields["method"] = method
						}
						if headers, ok := req["headers"].(map[string]any); ok {
							if contentLength, ok := headers["content-length"].(string); ok {
								fields["content-length"] = contentLength
							}
							if baggage, ok := headers["baggage"].(string); ok {
								// Parse baggage for sentry-trace_id
								for pair := range strings.SplitSeq(baggage, ",") {
									kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
									if len(kv) == 2 && kv[0] == "sentry-trace_id" {
										fields["trace_id"] = kv[1]
										break
									}
								}
							}
						}
						if url, ok := req["url"].(string); ok {
							fields["path"] = url
						}
					}
					if res, ok := jsonFields["res"].(map[string]any); ok {
						if statusCode, ok := res["statusCode"].(float64); ok {
							fields["status"] = strconv.Itoa(int(statusCode))
						}
					}
					if responseTime, ok := jsonFields["responseTime"].(float64); ok {
						fields["responseTime"] = strconv.FormatFloat(responseTime, 'f', -1, 64)
					}

					for k, data := range jsonFields {
						fields[k] = data.(string)
					}

					return fields, jsonPart
				}
			}
		}
	}
	return nil, ""
}

func parseJSONFields(line string) *LogEntry {
	if !json.Valid([]byte(line)) {
		return nil
	}

	entry := &LogEntry{
		Raw:        line,
		Fields:     make(map[string]string),
		IsJSON:     true,
		JSONFields: make(map[string]any),
	}

	json.Unmarshal([]byte(line), &entry.JSONFields)

	extractedFields := []string{}
	// Try multiple common timestamp field names
	for _, key := range []string{"timestamp", "@timestamp", "time", "ts", "datetime", "date"} {
		if ts, ok := entry.JSONFields[key].(string); ok {
			entry.Timestamp = ts
			tsTime, err := time.Parse(time.RFC3339Nano, ts)
			if err == nil {
				entry.Timestamp = tsTime.Format(time.RFC3339Nano)
			}
			extractedFields = append(extractedFields, key)
			break
		}
	}

	// Try multiple common level field names
	for _, key := range []string{"level", "severity", "log_level", "loglevel", "lvl"} {
		if lvl, ok := entry.JSONFields[key].(string); ok {
			entry.Level = lvl
			if parsedLevel, ok := ParseLevel(lvl); ok {
				entry.Level = parsedLevel
			}
			extractedFields = append(extractedFields, key)
			break
		}
	}

	// Try multiple common message field names
	for _, key := range []string{"message", "msg", "text", "log", "event"} {
		if msg, ok := entry.JSONFields[key].(string); ok {
			entry.Message = msg
			extractedFields = append(extractedFields, key)
			break
		}

	}

	for _, key := range []string{"caller", "source"} {
		if caller, ok := entry.JSONFields[key].(string); ok {
			if _, ok := ParseFile(caller); ok {
				entry.File = caller
				extractedFields = append(extractedFields, key)
				break
			}
		}
	}

	// Populate fields map with all JSON fields (excluding already extracted ones)
	for key, value := range entry.JSONFields {
		// Skip the ones we already extracted into dedicated fields
		if slices.Contains(extractedFields, key) {
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

func parseANSIFields(line string) (*LogEntry, string) {
	entry := &LogEntry{
		Raw:    line,
		Fields: make(map[string]string),
	}

	originalLine := line
	line = stripANSI(line)

	blocks := ParseANSIBlocks(originalLine)
	linesToStrip := []string{}

	nextBlock := Block{}
	for i, block := range blocks {
		block.Text = strings.TrimSpace(block.Text)
		if block.Equals(nextBlock) || block.Text == "" {
			continue
		}

		if strings.HasSuffix(block.Text, "=") && i < len(blocks)-1 {
			nextBlock = blocks[i+1]
			key := block.Text[:len(block.Text)-1]
			value := strings.TrimSpace(nextBlock.Text)
			entry.Fields[key] = value
			linesToStrip = append(linesToStrip, block.Text+value)

			continue
		}

		if ts, ok := ParseTimestamp(block.Text); ok {
			entry.Timestamp = ts.Format(time.RFC3339Nano)
			linesToStrip = append(linesToStrip, block.Text)
			continue
		}

		if lvl, ok := ParseLevel(block.Text); ok {
			entry.Level = lvl
			linesToStrip = append(linesToStrip, block.Text)
			continue
		}

		if file, ok := ParseFile(block.Text); ok {
			entry.File = file
			linesToStrip = append(linesToStrip, block.Text)
			continue
		}

		if block.Text == "[query]" && entry.Fields["type"] == "" {
			entry.Fields["type"] = block.Text[1 : len(block.Text)-1]
			continue
		}
	}

	for _, lineToStrip := range linesToStrip {
		line = strings.Replace(line, lineToStrip, "", 1)
	}
	line = strings.TrimSpace(line)

	return entry, line
}

func ParseLogLine(line string) *LogEntry {
	if strings.TrimSpace(line) == "" {
		return &LogEntry{
			Raw:    line,
			Fields: make(map[string]string),
		}
	}

	// Keep original line with ANSI codes for field boundary detection
	if json.Valid([]byte(line)) {
		return parseJSONFields(line)
	}

	entry, line := parseANSIFields(line)

	if strings.HasSuffix(line, "}") {
		jsonFields, jsonPart := extractRequestFields(line)
		if len(jsonFields) > 0 {
			maps.Copy(entry.Fields, jsonFields)
			if jsonPart != "" {
				line = strings.Replace(line, jsonPart, "", 1)
			}
		}
	}

	if entry.Timestamp == "" {
		if matches := timestampRegex.FindStringSubmatch(line); len(matches) > 1 {
			entry.Timestamp = matches[1]
			tsTime, ok := ParseTimestamp(entry.Timestamp)
			if ok {
				entry.Timestamp = tsTime.Format(time.RFC3339Nano)
			}
			line = strings.Replace(line, matches[1], "", 1)
		}
	}

	if entry.Level == "" {
		match := levelRegex.FindStringIndex(line)
		if match != nil {
			entry.Level = line[match[0]:match[1]]
			parsedLevel, ok := ParseLevel(entry.Level)
			if ok {
				entry.Level = parsedLevel
			}
		}
	}

	if entry.File == "" {
		if matches := fileRegex.FindStringSubmatch(line); len(matches) > 0 {
			if _, ok := ParseFile(matches[1]); ok {
				entry.File = matches[1]
			}
		}
	}

	line = strings.TrimSpace(line)
	entry.Message = line
	// Fall back to regular parsing if ANSI-aware parsing didn't yield results
	if len(entry.Fields) == 0 {
		fields, remaining := ParseKeyValues(line)
		if len(fields) > 0 {
			entry.Message = strings.TrimSpace(remaining)
			maps.Copy(entry.Fields, fields)
		}
	}

	// Check all fields for double-encoded JSON and normalize them
	for key, val := range entry.Fields {
		if val == "" {
			continue
		}
		if normalized := tryNormalizeJSON(val); normalized != "" {
			entry.Fields[key] = normalized
		}
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
		"[19:57:52.076]",
		"[15:04:05.000]",
		// 19:57:52.076536
		"15:04:05.000000",
		"15:04:05.000",
		"15:04:05",
		"15:04PM",
	}

	for _, format := range formats {
		t, err := time.Parse(format, timestampStr)
		if err == nil {
			// For formats without year/date, add current year/date
			switch format {
			case "Jan  2 15:04:05.000000", "Jan _2 15:04:05.000000":
				now := time.Now()
				t = time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
			case "[15:04:05.000]", "15:04:05.000000", "15:04:05.000", "15:04:05", "15:04PM":
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

func ParseLevel(levelStr string) (string, bool) {
	switch strings.TrimSpace(strings.ToUpper(levelStr)) {
	case "ERR", "ERROR", "FATAL":
		return "ERR", true
	case "WRN", "WARN", "WARNING":
		return "WRN", true
	case "INF", "INFO":
		return "INF", true
	case "DBG", "DEBUG":
		return "DBG", true
	case "TRC", "TRACE":
		return "TRC", true
	}
	return "", false
}

func ParseFile(fileStr string) (string, bool) {
	if fileStr == "" {
		return "", false
	}
	matches := fileRegex.FindStringSubmatch(fileStr)
	if len(matches) > 1 {
		return matches[1], true
	}
	return "", false
}

func init() {
	time.Local = time.UTC
}
