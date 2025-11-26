-- +goose Up
-- +goose StatementBegin
-- Add back request_id_header column to requests table
-- SQLite doesn't support ALTER TABLE ADD COLUMN with NOT NULL, so we need to recreate the table
CREATE TABLE requests_new(
    id integer PRIMARY KEY AUTOINCREMENT,
    sample_id integer,
    server_id integer,
    request_id_header text NOT NULL DEFAULT '',
    request_body text,
    status_code integer,
    duration_ms integer,
    response_body text,
    response_headers text,
    error text,
    is_sync integer DEFAULT 0,
    name text,
    bearer_token_override text,
    dev_id_override text,
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (sample_id) REFERENCES sample_requests(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id)
);

INSERT INTO requests_new(id, sample_id, server_id, request_id_header, request_body, status_code, duration_ms,
    response_body, response_headers, error, is_sync, name, bearer_token_override, dev_id_override, executed_at,
    created_at, updated_at, deleted_at)
SELECT
    id,
    sample_id,
    server_id,
    '',
    request_body,
    status_code,
    duration_ms,
    response_body,
    response_headers,
    error,
    is_sync,
    name,
    bearer_token_override,
    dev_id_override,
    executed_at,
    created_at,
    updated_at,
    deleted_at
FROM
    requests;

DROP TABLE requests;

ALTER TABLE requests_new RENAME TO requests;

CREATE INDEX idx_executed_requests_sample_id ON requests(sample_id);

CREATE INDEX idx_executed_requests_server_id ON requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON requests(deleted_at);

CREATE INDEX idx_executed_requests_is_sync ON requests(is_sync);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Remove request_id_header column from requests table
CREATE TABLE requests_old(
    id integer PRIMARY KEY AUTOINCREMENT,
    sample_id integer,
    server_id integer,
    request_body text,
    status_code integer,
    duration_ms integer,
    response_body text,
    response_headers text,
    error text,
    is_sync integer DEFAULT 0,
    name text,
    bearer_token_override text,
    dev_id_override text,
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (sample_id) REFERENCES sample_requests(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id)
);

INSERT INTO requests_old(id, sample_id, server_id, request_body, status_code, duration_ms, response_body,
    response_headers, error, is_sync, name, bearer_token_override, dev_id_override, executed_at, created_at,
    updated_at, deleted_at)
SELECT
    id,
    sample_id,
    server_id,
    request_body,
    status_code,
    duration_ms,
    response_body,
    response_headers,
    error,
    is_sync,
    name,
    bearer_token_override,
    dev_id_override,
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
