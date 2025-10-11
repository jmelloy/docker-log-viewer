-- +goose Up
-- +goose StatementBegin
CREATE TABLE execution_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id INTEGER NOT NULL,
    container_id TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    level TEXT,
    message TEXT,
    raw_log TEXT,
    fields TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,
    FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE
);

CREATE INDEX idx_execution_logs_execution_id ON execution_logs(execution_id);
CREATE INDEX idx_execution_logs_deleted_at ON execution_logs(deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_execution_logs_deleted_at;
DROP INDEX IF EXISTS idx_execution_logs_execution_id;
DROP TABLE IF EXISTS execution_logs;
-- +goose StatementEnd
