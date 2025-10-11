package logs

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

// mockReadCloser simulates Docker log stream with multiplexing headers
type mockReadCloser struct {
	data []byte
	pos  int
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}

func TestStreamLogsMultilineSQL(t *testing.T) {
	// Use raw sample data (without ANSI codes)
	sampleData := []byte(`Oct  6 18:09:28.978363 TRC pkg/repository/app/repository.go:213 > [sql]: SELECT "workspace_apps"."id" FROM "workspace_apps"
Oct  6 18:09:28.980126 TRC pkg/repository/threadparticipant/repository.go:108 > [sql]: SELECT * FROM "thread_participants"
Oct  6 18:09:28.980648 TRC pkg/repository/thread/repository.go:167 > [sql]: SELECT * FROM "threads"
Oct  6 18:09:28.983430 TRC pkg/repository/thread/repository.go:345 > [sql]: SELECT recipient_user_id FROM thread_recipients
Oct  6 18:09:28.984222 TRC pkg/repository/group/repository.go:163 > [sql]: SELECT * FROM "groups"
Oct  6 18:09:28.984986 TRC pkg/repository/threadrecipient/repository.go:444 > [sql]: 
		SELECT g.id as recipient_id, g.messageable_by, g.workspace_id, g.archived_at, gm.role as member_role
		FROM groups g
		LEFT JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = $1 AND gm.deleted_at IS NULL
		WHERE g.id IN ($2) AND g.deleted_at IS NULL
		UNION
		SELECT w.id as recipient_id, w.messageable_by, w.id, NULL as archived_at, wm.role as member_role
		FROM workspaces w
		LEFT JOIN workspace_members wm ON wm.workspace_id = w.id AND wm.deleted_at IS NULL
		WHERE (wm.user_id = $3 OR (w.id IN ($4) AND wm.user_id = $5)) AND w.deleted_at IS NULL
	 db.operation=select db.rows=0 db.table=restricted_thread_recipients duration=0.546712 location=["/app/pkg/repository/threadrecipient/repository.go:444","/app/pkg/operations/thread/thread.go:638","/app/pkg/operations/thread/send.go:93"] request_id=86ad5b4d span_id=cef3d6b89201c4eb trace_id=959bf87832d58c2088c3bab3514cee63
Oct  6 18:09:28.988044 TRC pkg/repository/threadparticipant/repository.go:260 > [sql]: INSERT INTO "thread_participants"


2025/10/06 18:09:28 /app/pkg/repository/message/repository.go:158 record not found
[0.490ms] [rows:0] SELECT * FROM "messages"
Oct  6 18:09:28.988742 TRC pkg/database/database.go:35 > [sql]: SAVEPOINT sp0x105c2a0
Oct  6 18:09:28.988871 TRC pkg/repository/dbksuid/repository.go:17 > [sql]: SELECT ksuid_micros()
Oct  6 18:09:28.990179 TRC pkg/repository/thread/repository.go:622 > [sql]: INSERT INTO "messages"
Oct  6 18:09:28.991025 TRC pkg/repository/thread/repository.go:920 > [sql]: SELECT * FROM "threads"
Oct  6 18:09:28.992255 TRC pkg/repository/message/repository.go:232 > [sql]: SELECT * FROM "message_metadata"
Oct  6 18:09:29.215538 TRC pkg/repository/threadrecipient/repository.go:395 > [sql]: SELECT min(tr.id) FROM thread_recipients
Oct  6 18:09:29.226800 TRC pkg/repository/app/repository.go:269 > [sql]: SELECT DISTINCT workspace_apps.* FROM "workspace_apps"
Oct  6 19:24:05.450809 TRC pkg/repository/threadparticipant/repository.go:425 > [sql]: 
			UPDATE thread_participants tp
			SET
				last_read_id = last_message_id,
				last_seen_id = last_message_id,
				pending_notify_at = NULL,
				unread_at = NULL,
				updated_at = $1,
				remind_at = (
					CASE WHEN tp.remind_at < $2 THEN NULL ELSE tp.remind_at END
				),
				remind_at_entry_id=(
					CASE WHEN tp.remind_at < $3 THEN NULL ELSE tp.remind_at_entry_id END
				)
			FROM threads
			WHERE threads.id = tp.thread_id AND
			tp.id IN ($4) AND
			(tp.unread_at IS NOT NULL OR COALESCE(tp.last_read_id, '') < last_message_id OR tp.remind_at < $5) AND
			tp.deleted_at IS NULL
			 db.operation=update db.rows=0 db.table= duration=4.083263 location=["/app/pkg/repository/threadparticipant/repository.go:425","/app/pkg/operations/thread/threadparticipant.go:57","/app/pkg/handlers/stream.go:173"] request_id=f693a69f span_id=a37fac51bf41c3f9 trace_id=a526c77c76d1e8c68eb8987eca899591
`)

	// Add Docker multiplexing headers (stdout = 1)
	var dockerData bytes.Buffer
	for _, line := range bytes.Split(sampleData, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		// Docker multiplexing: [stream_type, 0, 0, 0, size1, size2, size3, size4, data...]
		header := make([]byte, 8)
		header[0] = 1         // stdout
		size := len(line) + 1 // +1 for newline
		header[4] = byte(size >> 24)
		header[5] = byte(size >> 16)
		header[6] = byte(size >> 8)
		header[7] = byte(size)
		dockerData.Write(header)
		dockerData.Write(line)
		dockerData.WriteByte('\n')
	}

	// Create a mock docker client that returns our sample data
	mockReader := &mockReadCloser{data: dockerData.Bytes()}

	logChan := make(chan LogMessage, 100)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Simulate StreamLogs logic inline
	go func() {
		defer mockReader.Close()
		buf := make([]byte, 8192)
		var leftover []byte
		var bufferedLog bytes.Buffer

		flushLog := func() {
			if bufferedLog.Len() > 0 {
				logText := bufferedLog.String()
				entry := ParseLogLine(logText)
				logChan <- LogMessage{
					ContainerID: "test-container",
					Timestamp:   time.Now(),
					Entry:       entry,
				}
				bufferedLog.Reset()
			}
		}

		for {
			select {
			case <-ctx.Done():
				flushLog()
				close(logChan)
				return
			default:
				n, err := mockReader.Read(buf)
				if n > 0 {
					data := buf[:n]

					// Strip Docker multiplexing headers
					cleanedData := make([]byte, 0, len(data))
					i := 0
					for i < len(data) {
						if i+8 <= len(data) && (data[i] == 0 || data[i] == 1 || data[i] == 2) {
							i += 8
						} else {
							cleanedData = append(cleanedData, data[i])
							i++
						}
					}

					allData := append(leftover, cleanedData...)
					leftover = nil

					lines := bytes.Split(allData, []byte("\n"))
					for i, line := range lines {
						if i == len(lines)-1 && !bytes.HasSuffix(allData, []byte("\n")) {
							leftover = line
							continue
						}

						trimmed := bytes.TrimSpace(line)
						if len(trimmed) == 0 {
							continue
						}

						// Check if line starts with timestamp
						hasTimestamp := false
						for _, month := range []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"} {
							if bytes.HasPrefix(trimmed, []byte(month+" ")) {
								hasTimestamp = true
								break
							}
						}

						if hasTimestamp {
							flushLog()
							bufferedLog.Write(trimmed)
						} else {
							isContinuationLine := bufferedLog.Len() > 0 && (bytes.HasPrefix(line, []byte(" ")) || bytes.HasPrefix(line, []byte("\t")))

							if isContinuationLine {
								bufferedLog.WriteByte('\n')
								bufferedLog.Write(trimmed)
							} else {
								flushLog()
								entry := ParseLogLine(string(trimmed))
								logChan <- LogMessage{
									ContainerID: "test-container",
									Timestamp:   time.Now(),
									Entry:       entry,
								}
							}
						}
					}
				}

				if err == io.EOF {
					flushLog()
					close(logChan)
					return
				}
				if err != nil {
					flushLog()
					close(logChan)
					return
				}
			}
		}
	}()

	// Collect all log messages
	var messages []LogMessage
	for msg := range logChan {
		messages = append(messages, msg)
	}

	// Count SQL statements (same logic as TestSQLStatementCount)
	sqlCount := 0
	inSQL := false
	for _, msg := range messages {
		raw := msg.Entry.Raw

		// Start of new SQL statement
		if bytes.Contains([]byte(raw), []byte("[sql]:")) {
			sqlCount++
			inSQL = true
		} else if !inSQL && (bytes.HasPrefix([]byte(msg.Entry.Message), []byte("SELECT")) ||
			bytes.HasPrefix([]byte(msg.Entry.Message), []byte("INSERT")) ||
			bytes.HasPrefix([]byte(msg.Entry.Message), []byte("UPDATE")) ||
			bytes.HasPrefix([]byte(msg.Entry.Message), []byte("DELETE")) ||
			bytes.HasPrefix([]byte(msg.Entry.Message), []byte("SAVEPOINT"))) {
			sqlCount++
		} else if msg.Entry.Timestamp != "" {
			// New timestamp line, reset
			inSQL = false
		}
	}

	expectedSQL := 15
	if sqlCount != expectedSQL {
		t.Errorf("Expected %d SQL statements, got %d", expectedSQL, sqlCount)
	}

	// Verify multi-line SQL is properly parsed
	foundMultilineSQL := false
	for _, msg := range messages {
		if msg.Entry.Fields["db.table"] == "restricted_thread_recipients" {
			foundMultilineSQL = true
			// Should contain UNION in the raw log
			if !bytes.Contains([]byte(msg.Entry.Raw), []byte("UNION")) {
				t.Errorf("Expected multi-line SQL to contain UNION, got: %s", msg.Entry.Raw)
			}
			// Should have trace_id
			if msg.Entry.Fields["trace_id"] != "959bf87832d58c2088c3bab3514cee63" {
				t.Errorf("Expected trace_id 959bf87832d58c2088c3bab3514cee63, got %s", msg.Entry.Fields["trace_id"])
			}
			// Verify it has duration
			if msg.Entry.Fields["duration"] != "0.546712" {
				t.Errorf("Expected duration 0.546712, got %s", msg.Entry.Fields["duration"])
			}
			break
		}
	}

	if !foundMultilineSQL {
		t.Error("Did not find the multi-line SQL query in parsed messages")
		t.Logf("Total messages: %d", len(messages))
		for i, msg := range messages {
			if bytes.Contains([]byte(msg.Entry.Raw), []byte("UNION")) {
				t.Logf("Message %d contains UNION but table=%s", i, msg.Entry.Fields["db.table"])
				t.Logf("Raw: %s", msg.Entry.Raw)
			}
		}
	}
}
