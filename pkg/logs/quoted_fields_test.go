package logs

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestQuotedFieldEscapes(t *testing.T) {
	for _, want := range []string{
		`SELECT "id" FROM "live_agent_certs" WHERE "status" = $1`,
		"SELECT 1\nFROM users\tWHERE id = $1",
		`SELECT E'\\path'`,
	} {
		t.Run(want, func(t *testing.T) {
			entry := ParseLogLine("sql query done sql.query=" + strconv.Quote(want) + " duration_ms=0 error=<nil>")
			if got := entry.Fields["sql.query"]; got != want {
				t.Fatalf("sql.query = %q, want %q", got, want)
			}
			if entry.Fields["duration_ms"] != "0" || entry.Fields["error"] != "<nil>" {
				t.Fatalf("lost trailing fields: %v", entry.Fields)
			}
		})
	}
}

func TestNormalizeStructuredSQLLogs(t *testing.T) {
	query := `SELECT "id" FROM "live_agent_certs" WHERE status = $1`
	jsonLine, err := json.Marshal(map[string]any{
		"msg": "sql query done", "sql.query": query, "duration_ms": 0,
		"db.vars": []string{"active"}, "request_id": "request-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"sql query done sql.query=" + strconv.Quote(query) + ` duration_ms=0 db.vars=["active"] request_id=request-1`,
		"\x1b[32m" + "sql query done sql.query=" + strconv.Quote(query) + " duration_ms=0" + "\x1b[0m" + ` db.vars=["active"] request_id=request-1`,
		string(jsonLine),
	} {
		t.Run(line, func(t *testing.T) {
			entry := ParseLogLine(line)
			if entry.Message != "[sql]: "+query || entry.Fields["duration"] != "0" {
				t.Fatalf("SQL not normalized: %+v", entry)
			}
			if entry.Raw != line || entry.Fields["db.vars"] != `["active"]` || entry.Fields["request_id"] != "request-1" {
				t.Fatalf("raw log, parameters, or request ID lost: %+v", entry)
			}
		})
	}
	entry := ParseLogLine("sql.query=" + strconv.Quote(query) + " duration=5 duration_ms=0")
	if entry.Fields["duration"] != "5" {
		t.Fatal("existing canonical duration was overwritten")
	}
}

func TestQuotedFieldInvalidEscapePreserved(t *testing.T) {
	fields, _ := ParseKeyValues(`path="C:\query"`)
	if fields["path"] != `C:\query` {
		t.Fatalf("invalid escape changed: %q", fields["path"])
	}
}
