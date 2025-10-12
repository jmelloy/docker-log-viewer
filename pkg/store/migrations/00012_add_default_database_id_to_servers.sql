-- +goose Up
-- +goose StatementBegin
ALTER TABLE servers ADD COLUMN default_database_id INTEGER;

CREATE INDEX idx_servers_default_database_id ON servers(default_database_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_servers_default_database_id;

-- SQLite doesn't support DROP COLUMN directly in older versions
-- Create a new table without the column, copy data, drop old, rename new
CREATE TABLE servers_backup (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    bearer_token TEXT,
    dev_id TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

INSERT INTO servers_backup SELECT id, name, url, bearer_token, dev_id, created_at, updated_at, deleted_at FROM servers;
DROP TABLE servers;
ALTER TABLE servers_backup RENAME TO servers;

CREATE INDEX idx_servers_deleted_at ON servers(deleted_at);
-- +goose StatementEnd
