-- +goose Up
-- +goose StatementBegin
-- SQLite doesn't support ALTER COLUMN or DROP COLUMN easily, so we need to recreate the tables
-- 1. Rename execution_id to request_id in request_log_messages table
CREATE TABLE request_log_messages_new(
    id integer PRIMARY KEY AUTOINCREMENT,
    request_id integer NOT NULL,
    container_id text NOT NULL,
    timestamp timestamp NOT NULL,
    level text,
    message text,
    raw_log text,
    fields text,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE CASCADE
);

INSERT INTO request_log_messages_new(id, request_id, container_id, timestamp, level, message, raw_log, fields,
    created_at, updated_at, deleted_at)
SELECT
    id,
    execution_id,
    container_id,
    timestamp,
    level,
    message,
    raw_log,
    fields,
    created_at,
    updated_at,
    deleted_at
FROM
    request_log_messages;

DROP TABLE request_log_messages;

ALTER TABLE request_log_messages_new RENAME TO request_log_messages;

CREATE INDEX idx_request_log_messages_request_id ON request_log_messages(request_id);

CREATE INDEX idx_request_log_messages_deleted_at ON request_log_messages(deleted_at);

-- 2. Rename execution_id to request_id in request_sql_statements table
-- Note: The text request_id column (from logs) is being renamed to log_request_id to avoid conflict
CREATE TABLE request_sql_statements_new(
    id integer PRIMARY KEY AUTOINCREMENT,
    request_id integer NOT NULL,
    query text NOT NULL,
    normalized_query text NOT NULL,
    query_hash text,
    duration_ms real,
    table_name text,
    operation text,
    rows integer,
    variables text,
    gql_operation text,
    explain_plan text,
    log_request_id text,
    span_id text,
    trace_id text,
    log_fields text,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE CASCADE
);

-- Copy data: execution_id -> request_id (foreign key), request_id -> log_request_id (text field from logs)
INSERT INTO request_sql_statements_new(id, request_id, query, normalized_query, query_hash, duration_ms, table_name,
    operation, ROWS, variables, gql_operation, explain_plan, log_request_id, span_id, trace_id, log_fields, created_at,
    updated_at, deleted_at)
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
    COALESCE(request_id, '') AS log_request_id,
    span_id,
    trace_id,
    log_fields,
    created_at,
    updated_at,
    deleted_at
FROM
    request_sql_statements;

DROP TABLE request_sql_statements;

ALTER TABLE request_sql_statements_new RENAME TO request_sql_statements;

CREATE INDEX idx_sql_queries_request_id ON request_sql_statements(request_id);

CREATE INDEX idx_sql_queries_deleted_at ON request_sql_statements(deleted_at);

CREATE INDEX idx_sql_queries_query_hash ON request_sql_statements(query_hash);

-- 3. Drop request_id_header column from requests table
CREATE TABLE requests_new(
    id integer PRIMARY KEY AUTOINCREMENT,
    sample_id integer,
    server_id integer,
    request_body text,
    status_code integer,
    duration_ms integer,
    response_body text,
    response_headers text,
    error text,
    is_sync integer DEFAULT 0,
    name text,
    bearer_token_override text,
    dev_id_override text,
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (sample_id) REFERENCES sample_requests(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id)
);

INSERT INTO requests_new(id, sample_id, server_id, request_body, status_code, duration_ms, response_body,
    response_headers, error, is_sync, name, bearer_token_override, dev_id_override, executed_at, created_at,
    updated_at, deleted_at)
SELECT
    id,
    sample_id,
    server_id,
    request_body,
    status_code,
    duration_ms,
    response_body,
    response_headers,
    error,
    is_sync,
    name,
    bearer_token_override,
    dev_id_override,
    executed_at,
    created_at,
    updated_at,
    deleted_at
FROM
    requests;

DROP TABLE requests;

ALTER TABLE requests_new RENAME TO requests;

CREATE INDEX idx_executed_requests_sample_id ON requests(sample_id);

CREATE INDEX idx_executed_requests_server_id ON requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON requests(deleted_at);

CREATE INDEX idx_executed_requests_is_sync ON requests(is_sync);

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
-- Revert the changes
-- Revert request_log_messages: request_id back to execution_id
CREATE TABLE request_log_messages_old(
    id integer PRIMARY KEY AUTOINCREMENT,
    execution_id integer NOT NULL,
    container_id text NOT NULL,
    timestamp timestamp NOT NULL,
    level text,
    message text,
    raw_log text,
    fields text,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (execution_id) REFERENCES requests(id) ON DELETE CASCADE
);

INSERT INTO request_log_messages_old(id, execution_id, container_id, timestamp, level, message, raw_log, fields,
    created_at, updated_at, deleted_at)
SELECT
    id,
    request_id,
    container_id,
    timestamp,
    level,
    message,
    raw_log,
    fields,
    created_at,
    updated_at,
    deleted_at
FROM
    request_log_messages;

DROP TABLE request_log_messages;

ALTER TABLE request_log_messages_old RENAME TO request_log_messages;

CREATE INDEX idx_execution_logs_execution_id ON request_log_messages(execution_id);

CREATE INDEX idx_execution_logs_deleted_at ON request_log_messages(deleted_at);

-- Revert request_sql_statements: request_id back to execution_id
CREATE TABLE request_sql_statements_old(
    id integer PRIMARY KEY AUTOINCREMENT,
    execution_id integer NOT NULL,
    query text NOT NULL,
    normalized_query text NOT NULL,
    query_hash text,
    duration_ms real,
    table_name text,
    operation text,
    rows integer,
    variables text,
    gql_operation text,
    explain_plan text,
    request_id text,
    span_id text,
    trace_id text,
    log_fields text,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (execution_id) REFERENCES requests(id) ON DELETE CASCADE
);

INSERT INTO request_sql_statements_old(id, execution_id, query, normalized_query, query_hash, duration_ms, table_name,
    operation, ROWS, variables, gql_operation, explain_plan, request_id, span_id, trace_id, log_fields, created_at,
    updated_at, deleted_at)
SELECT
    id,
    request_id,
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
    log_request_id,
    span_id,
    trace_id,
    log_fields,
    created_at,
    updated_at,
    deleted_at
FROM
    request_sql_statements;

DROP TABLE request_sql_statements;

ALTER TABLE request_sql_statements_old RENAME TO request_sql_statements;

CREATE INDEX idx_sql_queries_execution_id ON request_sql_statements(execution_id);

CREATE INDEX idx_sql_queries_deleted_at ON request_sql_statements(deleted_at);

CREATE INDEX idx_sql_queries_query_hash ON request_sql_statements(query_hash);

-- Revert requests: add back request_id_header
CREATE TABLE requests_old(
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
    is_sync integer DEFAULT 0,
    name text,
    bearer_token_override text,
    dev_id_override text,
    executed_at timestamp NOT NULL,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    deleted_at timestamp,
    FOREIGN KEY (sample_id) REFERENCES sample_requests(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id)
);

INSERT INTO requests_old(id, sample_id, server_id, request_id_header, request_body, status_code, duration_ms,
    response_body, response_headers, error, is_sync, name, bearer_token_override, dev_id_override, executed_at,
    created_at, updated_at, deleted_at)
SELECT
    id,
    sample_id,
    server_id,
    '',
    request_body,
    status_code,
    duration_ms,
    response_body,
    response_headers,
    error,
    is_sync,
    name,
    bearer_token_override,
    dev_id_override,
    executed_at,
    created_at,
    updated_at,
    deleted_at
FROM
    requests;

DROP TABLE requests;

ALTER TABLE requests_old RENAME TO requests;

CREATE INDEX idx_executed_requests_sample_id ON requests(sample_id);

CREATE INDEX idx_executed_requests_server_id ON requests(server_id);

CREATE INDEX idx_executed_requests_executed_at ON requests(executed_at DESC);

CREATE INDEX idx_executed_requests_deleted_at ON requests(deleted_at);

CREATE INDEX idx_executed_requests_is_sync ON requests(is_sync);

-- +goose StatementEnd
