-- +goose Up
-- +goose StatementBegin
CREATE TABLE executions (
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

CREATE INDEX idx_executions_request_id ON executions(request_id);
CREATE INDEX idx_executions_executed_at ON executions(executed_at DESC);
CREATE INDEX idx_executions_deleted_at ON executions(deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_executions_deleted_at;
DROP INDEX IF EXISTS idx_executions_executed_at;
DROP INDEX IF EXISTS idx_executions_request_id;
DROP TABLE IF EXISTS executions;
-- +goose StatementEnd
