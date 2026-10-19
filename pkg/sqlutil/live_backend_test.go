package sqlutil

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"docker-log-parser/pkg/logs"
)

func TestDNSIDBackendColoredSQL(t *testing.T) {
	// Captured from dnsid-backend; ANSI codes surround keys, not the whole line.
	line := "\x1b[2mOct 19 23:12:06.052\x1b[0m DBG sql query done \x1b[2msql.query=\x1b[0m" + `"SELECT \"live_agent_certs\".\"id\", \"live_agent_certs\".\"hostname\", \"live_agent_certs\".\"activated_at\", \"live_agent_certs\".\"not_after\" FROM \"live_agent_certs\" WHERE \"live_agent_certs\".\"status\" = $1 AND \"live_agent_certs\".\"deleted_at\" IS NULL ORDER BY \"live_agent_certs\".\"hostname\" ASC, \"live_agent_certs\".\"activated_at\" DESC, \"live_agent_certs\".\"id\" ASC"` + " \x1b[2mduration_ms=\x1b[0m1 \x1b[2merror=\x1b[0m<nil>"
	checkBackendSQLLine(t, line)
}

// Run against a fresh docker logs capture without requiring Docker in normal tests.
func TestDNSIDBackendLiveCapture(t *testing.T) {
	path := os.Getenv("DNSID_BACKEND_LOG_FIXTURE")
	if path == "" {
		t.Skip("set DNSID_BACKEND_LOG_FIXTURE to a dnsid-backend docker logs capture")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	count := 0
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "sql.query=") {
			checkBackendSQLLine(t, scanner.Text())
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("capture contained no sql.query logs")
	}
	t.Logf("verified %d live SQL log lines", count)
}

func checkBackendSQLLine(t *testing.T, line string) {
	t.Helper()
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(line, "")
	matches := regexp.MustCompile(`sql.query=("(?:\\.|[^"\\])*"|\S+)`).FindAllStringSubmatch(plain, -1)
	if len(matches) == 0 {
		t.Fatal("SQL fixture does not contain sql.query")
	}
	// Interleaved stdout/stderr can produce duplicate keys; the parser keeps the last.
	want := matches[len(matches)-1][1]
	if strings.HasPrefix(want, `"`) {
		if err := json.Unmarshal([]byte(want), &want); err != nil {
			t.Fatal(err)
		}
	}
	entry := logs.ParseLogLine(line)
	if entry.Fields["sql.query"] != want || entry.Message != "[sql]: "+want {
		t.Fatal("backend parser left SQL quoted/escaped instead of canonical SQL")
	}
	queries := ExtractSQLQueries([]logs.ContainerMessage{{ContainerID: "dnsid-backend", Entry: entry}})
	if len(queries) != 1 || strings.TrimSpace(queries[0].Query) != strings.TrimSpace(want) {
		t.Fatal("SQL extraction did not preserve the decoded query")
	}
	durations := regexp.MustCompile(`duration_ms=(\d+(?:\.\d+)?)`).FindAllStringSubmatch(plain, -1)
	if len(durations) == 0 {
		t.Fatal("fixture missing duration_ms")
	}
	durationText := durations[len(durations)-1][1]
	duration, err := strconv.ParseFloat(durationText, 64)
	if err != nil || queries[0].DurationMS != duration {
		t.Fatalf("duration_ms = %v, want %s", queries[0].DurationMS, durationText)
	}
}
