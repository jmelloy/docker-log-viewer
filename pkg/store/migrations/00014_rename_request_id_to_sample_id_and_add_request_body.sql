-- +goose Up
-- +goose StatementBegin
-- SQLite doesn't support ALTER COLUMN or DROP COLUMN easily, so we need to recreate the table
-- Create a new table with the updated schema
UPDATE
    executed_requests
SET
    server_id = NULL
WHERE
    NOT EXISTS (
        SELECT
            1
        FROM
            servers
        WHERE
            servers.id = server_id);

CREATE TABLE executed_requests_new(
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
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (sample_id) REFERENCES sample_queries(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id)
);

-- Copy data from old table to new table
INSERT INTO executed_requests_new(id, sample_id, server_id, request_id_header, status_code, duration_ms, response_body,
    response_headers, error, executed_at, created_at, updated_at, deleted_at)
SELECT
    id,
    request_id,
    server_id,
    request_id_header,
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

-- Rename new table to original name
ALTER TABLE executed_requests_new RENAME TO executed_requests;

-- Recreate indexes
CREATE INDEX idx_executed_requests_sample_id ON executed_requests(sample_id);

CREATE INDEX idx_executed_requests_server_id ON executed_requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON executed_requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON executed_requests(deleted_at);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Recreate the old table structure
CREATE TABLE executed_requests_old(
    id integer PRIMARY KEY AUTOINCREMENT,
    request_id integer NOT NULL,
    server_id integer,
    request_id_header text NOT NULL,
    status_code integer,
    duration_ms integer,
    response_body text,
    response_headers text,
    error text,
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (request_id) REFERENCES sample_queries(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id)
);

-- Copy data back (only rows with sample_id)
INSERT INTO executed_requests_old(id, request_id, server_id, request_id_header, status_code, duration_ms,
    response_body, response_headers, error, executed_at, created_at, updated_at, deleted_at)
SELECT
    id,
    sample_id,
    server_id,
    request_id_header,
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
    sample_id IS NOT NULL;

-- Drop new table
DROP TABLE executed_requests;

-- Rename old table back
ALTER TABLE executed_requests_old RENAME TO executed_requests;

-- Recreate old indexes
CREATE INDEX idx_executed_requests_request_id ON executed_requests(request_id);

CREATE INDEX idx_executed_requests_server_id ON executed_requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON executed_requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON executed_requests(deleted_at);

-- +goose StatementEnd
