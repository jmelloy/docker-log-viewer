-- +goose Up
-- +goose StatementBegin
-- Add server_id to requests table
ALTER TABLE requests ADD COLUMN server_id INTEGER;

-- Remove url, bearer_token, dev_id from requests as they're now in servers
-- SQLite doesn't support DROP COLUMN, so we need to recreate the table
CREATE TABLE requests_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    server_id INTEGER,
    request_data TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE SET NULL
);

-- Copy data from old table (url, bearer_token, dev_id will be lost - users should migrate manually)
INSERT INTO requests_new (id, name, request_data, created_at, updated_at, deleted_at)
SELECT id, name, request_data, created_at, updated_at, deleted_at FROM requests;

-- Drop old table and rename new one
DROP TABLE requests;
ALTER TABLE requests_new RENAME TO requests;

-- Recreate indexes
CREATE INDEX idx_requests_created_at ON requests(created_at DESC);
CREATE INDEX idx_requests_deleted_at ON requests(deleted_at);
CREATE INDEX idx_requests_server_id ON requests(server_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Recreate old requests table structure
CREATE TABLE requests_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    request_data TEXT NOT NULL,
    bearer_token TEXT,
    dev_id TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

-- Copy data back (server_id will be lost)
INSERT INTO requests_old (id, name, url, request_data, created_at, updated_at, deleted_at)
SELECT id, name, '', request_data, created_at, updated_at, deleted_at FROM requests;

DROP TABLE requests;
ALTER TABLE requests_old RENAME TO requests;

CREATE INDEX idx_requests_created_at ON requests(created_at DESC);
CREATE INDEX idx_requests_deleted_at ON requests(deleted_at);
-- +goose StatementEnd
