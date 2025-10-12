-- +goose Up
-- +goose StatementBegin
ALTER TABLE executed_requests ADD COLUMN request_data TEXT;
ALTER TABLE executed_requests ADD COLUMN name TEXT;
ALTER TABLE executed_requests ADD COLUMN server_url TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite doesn't support DROP COLUMN directly, need to recreate table
CREATE TABLE executed_requests_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id INTEGER NOT NULL,
    server_id INTEGER,
    request_id_header TEXT NOT NULL,
    status_code INTEGER,
    duration_ms INTEGER,
    response_body TEXT,
    response_headers TEXT,
    error TEXT,
    executed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,
    FOREIGN KEY (request_id) REFERENCES sample_queries(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE SET NULL
);

INSERT INTO executed_requests_old (id, request_id, server_id, request_id_header, status_code, duration_ms, response_body, response_headers, error, executed_at, created_at, updated_at, deleted_at)
SELECT id, request_id, server_id, request_id_header, status_code, duration_ms, response_body, response_headers, error, executed_at, created_at, updated_at, deleted_at FROM executed_requests;

DROP TABLE executed_requests;
ALTER TABLE executed_requests_old RENAME TO executed_requests;

CREATE INDEX idx_executed_requests_request_id ON executed_requests(request_id);
CREATE INDEX idx_executed_requests_server_id ON executed_requests(server_id);
CREATE INDEX idx_executed_requests_executed_at ON executed_requests(executed_at);
CREATE INDEX idx_executed_requests_deleted_at ON executed_requests(deleted_at);
-- +goose StatementEnd
