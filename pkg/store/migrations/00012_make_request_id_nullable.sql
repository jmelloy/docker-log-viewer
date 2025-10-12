-- +goose Up
-- +goose StatementBegin
-- Make request_id nullable in executed_requests since we're storing server_url directly
-- SQLite doesn't support ALTER COLUMN, so we need to recreate the table
-- Create new table with nullable request_id
CREATE TABLE executed_requests_new(
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

-- Copy data from old table
INSERT INTO executed_requests_new(id, name, server_url, request_id, server_id, request_id_header, request_data,
    status_code, duration_ms, response_body, response_headers, error, executed_at, created_at, updated_at, deleted_at)
SELECT
    executed_requests.id,
    sample_queries.name,
    servers.url,
    request_id,
    executed_requests.server_id,
    request_id_header,
    sample_queries.request_data,
    status_code,
    duration_ms,
    response_body,
    response_headers,
    error,
    executed_at,
    executed_requests.created_at,
    executed_requests.updated_at,
    executed_requests.deleted_at
FROM
    executed_requests
    JOIN sample_queries ON executed_requests.request_id = sample_queries.id
    JOIN servers ON sample_queries.server_id = servers.id;

-- Drop old table
DROP TABLE executed_requests;

-- Rename new table
ALTER TABLE executed_requests_new RENAME TO executed_requests;

-- Recreate indexes
CREATE INDEX idx_executed_requests_request_id ON executed_requests(request_id);

CREATE INDEX idx_executed_requests_server_id ON executed_requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON executed_requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON executed_requests(deleted_at);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Recreate table with NOT NULL request_id
CREATE TABLE executed_requests_old(
    id integer PRIMARY KEY AUTOINCREMENT,
    name text,
    server_url text,
    request_id integer NOT NULL,
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
    FOREIGN KEY (request_id) REFERENCES sample_queries(id) ON DELETE CASCADE
);

-- Copy data (this will fail if there are NULL request_ids)
INSERT INTO executed_requests_old(id, name, server_url, request_id, server_id, request_id_header, request_data,
    status_code, duration_ms, response_body, response_headers, error, executed_at, created_at, updated_at, deleted_at)
SELECT
    id,
    name,
    server_url,
    request_id,
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
    executed_requests
WHERE
    request_id IS NOT NULL;

DROP TABLE executed_requests;

ALTER TABLE executed_requests_old RENAME TO executed_requests;

CREATE INDEX idx_executed_requests_request_id ON executed_requests(request_id);

CREATE INDEX idx_executed_requests_server_id ON executed_requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON executed_requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON executed_requests(deleted_at);

-- +goose StatementEnd
