-- +goose Up
-- +goose StatementBegin
-- Remove request_id column entirely and ensure server_id is populated
-- SQLite doesn't support DROP COLUMN, so we need to recreate the table
CREATE TABLE executed_requests_new(
    id integer PRIMARY KEY AUTOINCREMENT,
    name text,
    server_url text,
    server_id integer NOT NULL,
    request_id_header text NOT NULL,
    request_data text,
    status_code integer,
    duration_ms integer,
    response_body text,
    response_headers text,
    error text,
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);

-- Copy data from old table, ensuring server_id is populated
INSERT INTO executed_requests_new(id, name, server_url, server_id, request_id_header, request_data,
    status_code, duration_ms, response_body, response_headers, error, executed_at, created_at, updated_at, deleted_at)
SELECT
    id,
    name,
    server_url,
    COALESCE(server_id, (SELECT id FROM servers LIMIT 1)),
    request_id_header,
    request_data,
    status_code,
    duration_ms,
    response_body,
    response_headers,
    error,
    executed_at,
    created_at,
    updated_at,
    deleted_at
FROM
    executed_requests;

-- Drop old table
DROP TABLE executed_requests;

-- Rename new table
ALTER TABLE executed_requests_new RENAME TO executed_requests;

-- Recreate indexes
CREATE INDEX idx_executed_requests_server_id ON executed_requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON executed_requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON executed_requests(deleted_at);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Recreate table with request_id column
CREATE TABLE executed_requests_old(
    id integer PRIMARY KEY AUTOINCREMENT,
    name text,
    server_url text,
    request_id integer,
    server_id integer,
    request_id_header text NOT NULL,
    request_data text,
    status_code integer,
    duration_ms integer,
    response_body text,
    response_headers text,
    error text,
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (request_id) REFERENCES sample_queries(id) ON DELETE SET NULL
);

-- Copy data back
INSERT INTO executed_requests_old(id, name, server_url, server_id, request_id_header, request_data,
    status_code, duration_ms, response_body, response_headers, error, executed_at, created_at, updated_at, deleted_at)
SELECT
    id,
    name,
    server_url,
    server_id,
    request_id_header,
    request_data,
    status_code,
    duration_ms,
    response_body,
    response_headers,
    error,
    executed_at,
    created_at,
    updated_at,
    deleted_at
FROM
    executed_requests;

DROP TABLE executed_requests;

ALTER TABLE executed_requests_old RENAME TO executed_requests;

CREATE INDEX idx_executed_requests_server_id ON executed_requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON executed_requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON executed_requests(deleted_at);

-- +goose StatementEnd
