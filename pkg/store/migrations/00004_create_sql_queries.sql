-- +goose Up
-- +goose StatementBegin
CREATE TABLE sql_queries (
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

CREATE INDEX idx_sql_queries_execution_id ON sql_queries(execution_id);
CREATE INDEX idx_sql_queries_deleted_at ON sql_queries(deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_sql_queries_deleted_at;
DROP INDEX IF EXISTS idx_sql_queries_execution_id;
DROP TABLE IF EXISTS sql_queries;
-- +goose StatementEnd
