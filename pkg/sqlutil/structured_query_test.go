package sqlutil

import (
	"strings"
	"testing"

	"docker-log-parser/pkg/logs"
)

func TestSQLQuerySources(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		fields        map[string]string
		want          string
	}{
		{"structured", "sql query done", map[string]string{"sql.query": "SELECT 1", "duration_ms": "2.5"}, "SELECT 1"},
		{"empty message", "", map[string]string{"sql.query": "SELECT 2"}, "SELECT 2"},
		{"structured wins", "[sql]: SELECT 0", map[string]string{"sql.query": "SELECT 3"}, "SELECT 3"},
		{"legacy", "[sql]: SELECT 4", nil, "SELECT 4"},
		{"query type", "SELECT 5", map[string]string{"type": "query", "duration_ms": "2.5", "rows": "3"}, "SELECT 5"},
		{"ordinary", "sql query done", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queries := ExtractSQLQueries([]logs.ContainerMessage{{Entry: &logs.LogEntry{Message: tc.message, Fields: tc.fields}}})
			if tc.want == "" {
				if len(queries) != 0 {
					t.Fatalf("unexpected queries: %+v", queries)
				}
				return
			}
			if len(queries) != 1 || strings.TrimSpace(queries[0].Query) != tc.want {
				t.Fatalf("unexpected queries: %+v", queries)
			}
			if tc.fields["duration_ms"] != "" && queries[0].DurationMS != 2.5 {
				t.Fatalf("lost duration: %+v", queries[0])
			}
			if tc.fields["rows"] != "" && queries[0].Rows != 3 {
				t.Fatalf("lost rows: %+v", queries[0])
			}
		})
	}
}
