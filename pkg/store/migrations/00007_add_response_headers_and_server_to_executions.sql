-- +goose Up
-- +goose StatementBegin
ALTER TABLE executions ADD COLUMN response_headers TEXT;
ALTER TABLE executions ADD COLUMN server_id INTEGER;

CREATE INDEX idx_executions_server_id ON executions(server_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite doesn't support DROP COLUMN directly, need to recreate table
CREATE TABLE executions_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id INTEGER NOT NULL,
    request_id_header TEXT NOT NULL,
    status_code INTEGER,
    duration_ms INTEGER,
    response_body TEXT,
    error TEXT,
    executed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,
    FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE CASCADE
);

INSERT INTO executions_old (id, request_id, request_id_header, status_code, duration_ms, response_body, error, executed_at, created_at, updated_at, deleted_at)
SELECT id, request_id, request_id_header, status_code, duration_ms, response_body, error, executed_at, created_at, updated_at, deleted_at FROM executions;

DROP TABLE executions;
ALTER TABLE executions_old RENAME TO executions;

CREATE INDEX idx_executions_request_id ON executions(request_id);
CREATE INDEX idx_executions_executed_at ON executions(executed_at DESC);
CREATE INDEX idx_executions_deleted_at ON executions(deleted_at);
-- +goose StatementEnd
