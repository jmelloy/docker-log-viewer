-- +goose Up
-- +goose StatementBegin
ALTER TABLE sql_queries ADD COLUMN gql_operation TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- SQLite doesn't support DROP COLUMN directly, need to recreate table
CREATE TABLE sql_queries_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id INTEGER NOT NULL,
    query TEXT NOT NULL,
    normalized_query TEXT NOT NULL,
    query_hash TEXT,
    duration_ms REAL,
    table_name TEXT,
    operation TEXT,
    rows INTEGER,
    variables TEXT,
    explain_plan TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP,
    FOREIGN KEY (execution_id) REFERENCES executed_requests(id) ON DELETE CASCADE
);

INSERT INTO sql_queries_old (id, execution_id, query, normalized_query, query_hash, duration_ms, table_name, operation, rows, variables, explain_plan, created_at, updated_at, deleted_at)
SELECT id, execution_id, query, normalized_query, query_hash, duration_ms, table_name, operation, rows, variables, explain_plan, created_at, updated_at, deleted_at FROM sql_queries;

DROP TABLE sql_queries;
ALTER TABLE sql_queries_old RENAME TO sql_queries;

CREATE INDEX idx_sql_queries_execution_id ON sql_queries(execution_id);
CREATE INDEX idx_sql_queries_query_hash ON sql_queries(query_hash);
CREATE INDEX idx_sql_queries_deleted_at ON sql_queries(deleted_at);
-- +goose StatementEnd
