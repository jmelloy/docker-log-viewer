-- +goose Up
-- +goose StatementBegin
CREATE TABLE database_urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    connection_string TEXT NOT NULL,
    database_type TEXT NOT NULL DEFAULT 'postgresql',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_database_urls_deleted_at ON database_urls(deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_database_urls_deleted_at;
DROP TABLE IF EXISTS database_urls;
-- +goose StatementEnd
