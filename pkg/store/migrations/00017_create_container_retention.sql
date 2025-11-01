-- +goose Up
CREATE TABLE container_retention (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    container_name TEXT NOT NULL UNIQUE,
    retention_type TEXT NOT NULL CHECK (retention_type IN ('count', 'time')),
    retention_value INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_container_retention_name ON container_retention(container_name);

-- +goose Down
DROP INDEX IF EXISTS idx_container_retention_name;
DROP TABLE IF EXISTS container_retention;
