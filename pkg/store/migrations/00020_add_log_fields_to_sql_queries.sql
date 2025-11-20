-- +goose Up
-- +goose StatementBegin
ALTER TABLE request_sql_statements
    ADD COLUMN request_id TEXT;

ALTER TABLE request_sql_statements
    ADD COLUMN span_id TEXT;

ALTER TABLE request_sql_statements
    ADD COLUMN trace_id TEXT;

ALTER TABLE request_sql_statements
    ADD COLUMN log_fields TEXT;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- SQLite doesn't support DROP COLUMN directly, need to recreate table
CREATE TABLE request_sql_statements_old(
    id integer PRIMARY KEY AUTOINCREMENT,
    execution_id integer NOT NULL,
    query text NOT NULL,
    normalized_query text NOT NULL,
    query_hash text,
    duration_ms real,
    table_name text,
    operation text,
    rows INTEGER,
    variables text,
    gql_operation text,
    explain_plan text,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (execution_id) REFERENCES requests(id) ON DELETE CASCADE
);

INSERT INTO request_sql_statements_old(id, execution_id, query, normalized_query, query_hash, duration_ms, table_name,
    operation, ROWS, variables, gql_operation, explain_plan, created_at, updated_at, deleted_at)
SELECT
    id,
    execution_id,
    query,
    normalized_query,
    query_hash,
    duration_ms,
    table_name,
    operation,
    ROWS,
    variables,
    gql_operation,
    explain_plan,
    created_at,
    updated_at,
    deleted_at
FROM
    request_sql_statements;

DROP TABLE request_sql_statements;

ALTER TABLE request_sql_statements_old RENAME TO request_sql_statements;

CREATE INDEX idx_sql_queries_execution_id ON request_sql_statements(execution_id);

CREATE INDEX idx_sql_queries_query_hash ON request_sql_statements(query_hash);

CREATE INDEX idx_sql_queries_deleted_at ON request_sql_statements(deleted_at);

-- +goose StatementEnd
