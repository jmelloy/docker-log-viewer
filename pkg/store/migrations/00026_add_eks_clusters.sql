-- +goose Up
CREATE TABLE eks_clusters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    region TEXT NOT NULL,
    profile TEXT NOT NULL DEFAULT '',
    namespace TEXT NOT NULL DEFAULT '',
    label_selector TEXT NOT NULL DEFAULT '',
    since_minutes INTEGER NOT NULL DEFAULT 15,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME,
    updated_at DATETIME
);

-- +goose Down
DROP TABLE eks_clusters;
