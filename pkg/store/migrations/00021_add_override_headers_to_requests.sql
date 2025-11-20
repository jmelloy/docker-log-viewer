-- +goose Up
-- +goose StatementBegin
ALTER TABLE requests
    ADD COLUMN bearer_token_override TEXT;

ALTER TABLE requests
    ADD COLUMN dev_id_override TEXT;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- SQLite doesn't support DROP COLUMN directly, need to recreate table
CREATE TABLE requests_old(
    id integer PRIMARY KEY AUTOINCREMENT,
    sample_id integer,
    server_id integer,
    request_id_header text NOT NULL,
    request_body text,
    status_code integer,
    duration_ms integer,
    response_body text,
    response_headers text,
    error text,
    is_sync integer DEFAULT 0,
    name text,
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (sample_id) REFERENCES sample_queries(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id)
);

INSERT INTO requests_old(id, sample_id, server_id, request_id_header, request_body, status_code, duration_ms,
    response_body, response_headers, error, is_sync, name, executed_at, created_at, updated_at, deleted_at)
SELECT
    id,
    sample_id,
    server_id,
    request_id_header,
    request_body,
    status_code,
    duration_ms,
    response_body,
    response_headers,
    error,
    is_sync,
    name,
    executed_at,
    created_at,
    updated_at,
    deleted_at
FROM
    requests;

DROP TABLE requests;

ALTER TABLE requests_old RENAME TO requests;

CREATE INDEX idx_executed_requests_sample_id ON requests(sample_id);

CREATE INDEX idx_executed_requests_server_id ON requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON requests(deleted_at);

CREATE INDEX idx_executed_requests_is_sync ON requests(is_sync);

-- +goose StatementEnd
