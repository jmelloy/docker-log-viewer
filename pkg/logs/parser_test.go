package logs

import (
	"strings"
	"testing"
)

func TestMultiLineLogParsing(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "Simple single line log",
			input: `Oct  3 21:53:27.208471 INF pkg/observability/logging/logging.go:110 > 2025-10-03T21:53:27Z POST /stream/webhook - 200 (37 bytes) ip=34.198.125.61 latency=1.512547 method=POST path=/stream/webhook request_id=e51c112c status=200 user-agent=Go-http-client/1.1
`,
			expected: 1,
		},
		{
			name: "Multi-line SQL query",
			input: `Oct  3 22:38:50.687733 TRC pkg/repository/externalobject/repository.go:128 > [sql]: SELECT distinct case when previous_ids @> ARRAY[previous_id]::text[] then previous_id else id end,
 		created_at, updated_at, deleted_at, user_id, url, canonical_url, status, og_type, tw_card, site_name, title, description, image, video, audio, icon, workspace_app_id, content_available, ragie_id, layout, app_object_id, previous_ids,
		preview_text, status_icon, status_color, status_text FROM "external_objects" , unnest($1::text[]) previous_id WHERE (external_objects.id IN ($2,$3) or previous_ids @> ARRAY[previous_id]::text[]) AND "external_objects"."deleted_at" IS NULL db.operation=select db.rows=2 db.table=external_objects duration=1.044103 location=["/app/pkg/repository/externalobject/repository.go:128","/app/pkg/repository/externalobject/repository.go:335","/app/pkg/repository/loader/loader.go:240"] request_id=82362443 span_id=078d56e662a3c046 trace_id=ece53be231c1e686d292e6ce4bd8a5ac
`,
			expected: 1,
		},
		{
			name: "Two single-line logs",
			input: `Oct  3 21:53:30.924888 TRC pkg/repository/user/repository.go:108 > [sql]: SELECT * FROM "users" WHERE id = $1 AND "users"."deleted_at" IS NULL LIMIT 1 db.operation=select db.rows=1 db.table=users duration=21.697855
Oct  3 21:53:30.926425 DBG pkg/handlers/graphql.go:234 > Processing GraphQL Operations=[{"operationName":"test"}] request_id=508e6ccc
`,
			expected: 2,
		},
		{
			name: "Multi-line then single-line",
			input: `Oct  3 22:38:50.687733 TRC pkg/repository/externalobject/repository.go:128 > [sql]: SELECT distinct case when previous_ids @> ARRAY[previous_id]::text[] then previous_id else id end,
 		created_at, updated_at FROM "external_objects" db.table=external_objects request_id=82362443
Oct  3 21:53:30.926425 DBG pkg/handlers/graphql.go:234 > Processing GraphQL request_id=508e6ccc
`,
			expected: 2,
		},
		{
			name: "Log without timestamp (standalone)",
			input: `building...
`,
			expected: 1,
		},
		{
			name: "Mixed with non-timestamp lines",
			input: `building...
Oct  3 21:53:26.059834 INF pkg/observability/logging.go:51 > Logger configured
running...
`,
			expected: 3,
		},
		{
			name: "Empty lines should be ignored",
			input: `Oct  3 21:53:26.059834 INF pkg/observability/logging.go:51 > Logger configured

Oct  3 21:53:27.208471 INF pkg/observability/logging/logging.go:110 > POST /stream/webhook
`,
			expected: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bufferedLog strings.Builder
			entries := 0

			lines := strings.Split(tc.input, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}

				hasTimestamp := isTimestampLine(trimmed)

				if hasTimestamp {
					if bufferedLog.Len() > 0 {
						entries++
						bufferedLog.Reset()
					}
					bufferedLog.WriteString(trimmed)
				} else {
					isContinuationLine := bufferedLog.Len() > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t"))

					if isContinuationLine {
						bufferedLog.WriteString("\n")
						bufferedLog.WriteString(trimmed)
					} else {
						if bufferedLog.Len() > 0 {
							entries++
							bufferedLog.Reset()
						}
						entries++
					}
				}
			}

			if bufferedLog.Len() > 0 {
				entries++
			}

			if entries != tc.expected {
				t.Errorf("Expected %d entries, got %d", tc.expected, entries)
			}
		})
	}
}

func TestParseLogLine(t *testing.T) {
	testCases := []struct {
		name      string
		input     string
		wantLevel string
		wantMsg   string
		wantField string
	}{
		{
			name:      "Structured log with fields",
			input:     `Oct  3 21:53:27.208471 INF pkg/observability/logging/logging.go:110 > 2025-10-03T21:53:27Z POST /stream/webhook - 200 (37 bytes) ip=34.198.125.61 latency=1.512547 method=POST path=/stream/webhook request_id=e51c112c status=200`,
			wantLevel: "INF",
			wantMsg:   "2025-10-03T21:53:27Z POST /stream/webhook - 200 (37 bytes)",
			wantField: "e51c112c",
		},
		{
			name:      "Log with quoted field value",
			input:     `Oct  3 19:57:52.078096 TRC pkg/repository/workspace/repository.go:145 > [sql]: SELECT * FROM "workspaces" db.error="record not found" db.operation=select`,
			wantLevel: "TRC",
			wantMsg:   "[sql]: SELECT * FROM \"workspaces\"",
			wantField: "record not found",
		},
		{
			name:      "Log with dotted field keys",
			input:     `Oct  3 21:53:30.924888 TRC pkg/repository/user/repository.go:108 > [sql]: SELECT * FROM users db.operation=select db.rows=1 db.table=users duration=21.697855`,
			wantLevel: "TRC",
			wantMsg:   "[sql]: SELECT * FROM users",
			wantField: "users",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry := ParseLogLine(tc.input)

			if entry.Level != tc.wantLevel {
				t.Errorf("Expected level %s, got %s", tc.wantLevel, entry.Level)
			}

			if entry.Message != tc.wantMsg {
				t.Errorf("Expected message %q, got %q", tc.wantMsg, entry.Message)
			}

			if tc.wantField != "" {
				found := false
				for _, v := range entry.Fields {
					if v == tc.wantField {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find field value %q in %v", tc.wantField, entry.Fields)
				}
			}
		})
	}
}

func isTimestampLine(line string) bool {
	if len(line) < 5 {
		return false
	}
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	for _, month := range months {
		if strings.HasPrefix(line, month+" ") {
			return true
		}
	}
	return false
}

func TestMultilineSQLParsing(t *testing.T) {
	input := `Oct  6 18:09:28.984986 TRC pkg/repository/threadrecipient/repository.go:444 > [sql]: 
		SELECT g.id as recipient_id, g.messageable_by, g.workspace_id, g.archived_at, gm.role as member_role
		FROM groups g
		LEFT JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = $1 AND gm.deleted_at IS NULL
		WHERE g.id IN ($2) AND g.deleted_at IS NULL
		UNION
		SELECT w.id as recipient_id, w.messageable_by, w.id, NULL as archived_at, wm.role as member_role
		FROM workspaces w
		LEFT JOIN workspace_members wm ON wm.workspace_id = w.id AND wm.deleted_at IS NULL
		WHERE (wm.user_id = $3 OR (w.id IN ($4) AND wm.user_id = $5)) AND w.deleted_at IS NULL
	 db.operation=select db.rows=0 db.table=restricted_thread_recipients duration=0.546712 location=["/app/pkg/repository/threadrecipient/repository.go:444","/app/pkg/operations/thread/thread.go:638","/app/pkg/operations/thread/send.go:93"] request_id=86ad5b4d span_id=cef3d6b89201c4eb trace_id=959bf87832d58c2088c3bab3514cee63`

	entry := ParseLogLine(input)

	// Should be parsed as TRC level
	if entry.Level != "TRC" {
		t.Errorf("Expected level TRC, got %s", entry.Level)
	}

	// Should contain the SQL UNION statement
	if !strings.Contains(entry.Raw, "UNION") {
		t.Errorf("Expected SQL to contain UNION, raw log: %s", entry.Raw)
	}

	// Should have trace_id field
	if entry.Fields["trace_id"] != "959bf87832d58c2088c3bab3514cee63" {
		t.Errorf("Expected trace_id 959bf87832d58c2088c3bab3514cee63, got %s", entry.Fields["trace_id"])
	}

	// Should have db.table field
	if entry.Fields["db.table"] != "restricted_thread_recipients" {
		t.Errorf("Expected db.table restricted_thread_recipients, got %s", entry.Fields["db.table"])
	}

	// Should have duration field
	if entry.Fields["duration"] != "0.546712" {
		t.Errorf("Expected duration 0.546712, got %s", entry.Fields["duration"])
	}
}

func TestSQLStatementCount(t *testing.T) {
	input := `Oct  6 18:09:28.978363 TRC pkg/repository/app/repository.go:213 > [sql]: SELECT "workspace_apps"."id" FROM "workspace_apps"
Oct  6 18:09:28.980126 TRC pkg/repository/threadparticipant/repository.go:108 > [sql]: SELECT * FROM "thread_participants"
Oct  6 18:09:28.980648 TRC pkg/repository/thread/repository.go:167 > [sql]: SELECT * FROM "threads"
Oct  6 18:09:28.983430 TRC pkg/repository/thread/repository.go:345 > [sql]: SELECT recipient_user_id FROM thread_recipients
Oct  6 18:09:28.984222 TRC pkg/repository/group/repository.go:163 > [sql]: SELECT * FROM "groups"
Oct  6 18:09:28.984986 TRC pkg/repository/threadrecipient/repository.go:444 > [sql]: 
		SELECT g.id as recipient_id FROM groups g
		UNION
		SELECT w.id FROM workspaces w
	
Oct  6 18:09:28.988044 TRC pkg/repository/threadparticipant/repository.go:260 > [sql]: INSERT INTO "thread_participants"


2025/10/06 18:09:28 /app/pkg/repository/message/repository.go:158 record not found
[0.490ms] [rows:0] SELECT * FROM "messages"
Oct  6 18:09:28.988742 TRC pkg/database/database.go:35 > [sql]: SAVEPOINT sp0x105c2a0
Oct  6 18:09:28.988871 TRC pkg/repository/dbksuid/repository.go:17 > [sql]: SELECT ksuid_micros()
Oct  6 18:09:28.990179 TRC pkg/repository/thread/repository.go:622 > [sql]: INSERT INTO "messages"
Oct  6 18:09:28.991025 TRC pkg/repository/thread/repository.go:920 > [sql]: SELECT * FROM "threads"
Oct  6 18:09:28.992255 TRC pkg/repository/message/repository.go:232 > [sql]: SELECT * FROM "message_metadata"
Oct  6 18:09:29.215538 TRC pkg/repository/threadrecipient/repository.go:395 > [sql]: SELECT min(tr.id) FROM thread_recipients
Oct  6 18:09:29.226800 TRC pkg/repository/app/repository.go:269 > [sql]: SELECT DISTINCT workspace_apps.* FROM "workspace_apps"`

	sqlCount := 0
	lines := strings.Split(input, "\n")
	inSQL := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Start of new SQL statement
		if strings.Contains(line, "[sql]:") {
			sqlCount++
			inSQL = true
		} else if !inSQL && (strings.HasPrefix(trimmed, "SELECT") || strings.HasPrefix(trimmed, "INSERT") || strings.HasPrefix(trimmed, "UPDATE") || strings.HasPrefix(trimmed, "DELETE") || strings.HasPrefix(trimmed, "SAVEPOINT")) {
			sqlCount++
		} else if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Not a continuation line, reset
			inSQL = false
		}
	}

	expected := 14
	if sqlCount != expected {
		t.Errorf("Expected %d SQL statements, got %d", expected, sqlCount)
	}
}
