-- +goose Up
-- +goose StatementBegin
ALTER TABLE sql_queries ADD COLUMN query_hash TEXT;
ALTER TABLE sql_queries ADD COLUMN explain_plan TEXT;

CREATE INDEX idx_sql_queries_query_hash ON sql_queries(query_hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite doesn't support DROP COLUMN directly, need to recreate table
CREATE TABLE sql_queries_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id INTEGER NOT NULL,
    query TEXT NOT NULL,
    normalized_query TEXT NOT NULL,
    duration_ms REAL,
    table_name TEXT,
    operation TEXT,
    rows INTEGER,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,
    FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE
);

INSERT INTO sql_queries_old (id, execution_id, query, normalized_query, duration_ms, table_name, operation, rows, created_at, updated_at, deleted_at)
SELECT id, execution_id, query, normalized_query, duration_ms, table_name, operation, rows, created_at, updated_at, deleted_at FROM sql_queries;

DROP TABLE sql_queries;
ALTER TABLE sql_queries_old RENAME TO sql_queries;

CREATE INDEX idx_sql_queries_execution_id ON sql_queries(execution_id);
CREATE INDEX idx_sql_queries_deleted_at ON sql_queries(deleted_at);
-- +goose StatementEnd
