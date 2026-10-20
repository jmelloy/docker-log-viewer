package sqlutil

import (
	"strings"
	"testing"

	"docker-log-parser/pkg/logs"
)

func TestExtractQuotedSQLQuery(t *testing.T) {
	line := `Oct 19 23:14:06.052 DBG sql query done sql.query="SELECT \"live_agent_certs\".\"id\" FROM \"live_agent_certs\" WHERE \"status\" = $1" duration_ms=1.25 db.vars=["active"] request_id=request-1 error=<nil>`
	entry := logs.ParseLogLine(line)
	want := `SELECT "live_agent_certs"."id" FROM "live_agent_certs" WHERE "status" = $1`
	if entry.Fields["sql.query"] != want {
		t.Errorf("sql.query = %q, want %q", entry.Fields["sql.query"], want)
	}
	if entry.Message != "sql query done" || entry.Fields["duration"] != "1.25" {
		t.Errorf("SQL message or normalized duration changed: %+v", entry)
	}
	if entry.Raw != line || entry.Fields["error"] != "<nil>" {
		t.Fatal("raw log or trailing fields were changed")
	}
	queries := ExtractSQLQueries([]logs.ContainerMessage{{ContainerID: "backend", Entry: entry}})
	if len(queries) != 1 {
		t.Fatalf("got %d SQL queries, want 1", len(queries))
	}
	if strings.TrimSpace(queries[0].Query) != want || queries[0].DurationMS != 1.25 || queries[0].ContainerID != "backend" || queries[0].Variables != `["active"]` || queries[0].LogRequestID != "request-1" {
		t.Fatalf("unexpected extracted query: %+v", queries[0])
	}
}
