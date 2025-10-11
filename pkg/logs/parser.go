package logs

import (
	"encoding/json"
	"regexp"
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
	timestampRegex = regexp.MustCompile(`(\d{1,2}\s+\w+\s+\d{4}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?|\d{4}[-/]\d{2}[-/]\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?|\w+\s+\d+\s+\d+:\d+:\d+(?:\.\d+)?|\d{2}:\d{2}:\d{2}(?:\.\d+)?|\d+[-/]\d+[-/]\d+\s+\d+:\d+:\d+(?:\.\d+)?|\d{10,13})`)
	levelRegex     = regexp.MustCompile(`(FATAL|DEBUG|INFO|WARN|ERROR|DBG|TRC|INF|WRN|ERR)`)
	fileRegex      = regexp.MustCompile(`([\w/]+\.go:\d+)`)
	ansiRegex      = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHfABCDsuJSTlh]|\x1b\][^\x07]*\x07|\x1b[>=]|\x1b\[?[\d;]*[a-zA-Z]`)
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
				entry.Level = lvl
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

func init() {
	time.Local = time.UTC
}
