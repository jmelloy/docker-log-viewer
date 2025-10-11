package store

const schema = `
-- Requests table stores GraphQL/API requests
CREATE TABLE IF NOT EXISTS requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    request_data TEXT NOT NULL,
    bearer_token TEXT,
    dev_id TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Executions table stores each execution of a request
CREATE TABLE IF NOT EXISTS executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_id INTEGER NOT NULL,
    request_id_header TEXT NOT NULL,
    status_code INTEGER,
    duration_ms INTEGER,
    response_body TEXT,
    error TEXT,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (request_id) REFERENCES requests(id) ON DELETE CASCADE
);

-- Log entries associated with each execution
CREATE TABLE IF NOT EXISTS execution_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id INTEGER NOT NULL,
    container_id TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    level TEXT,
    message TEXT,
    raw_log TEXT,
    fields TEXT,
    FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE
);

-- SQL queries extracted from logs
CREATE TABLE IF NOT EXISTS sql_queries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id INTEGER NOT NULL,
    query TEXT NOT NULL,
    normalized_query TEXT NOT NULL,
    duration_ms REAL,
    table_name TEXT,
    operation TEXT,
    rows INTEGER,
    FOREIGN KEY (execution_id) REFERENCES executions(id) ON DELETE CASCADE
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_executions_request_id ON executions(request_id);
CREATE INDEX IF NOT EXISTS idx_execution_logs_execution_id ON execution_logs(execution_id);
CREATE INDEX IF NOT EXISTS idx_sql_queries_execution_id ON sql_queries(execution_id);
CREATE INDEX IF NOT EXISTS idx_requests_created_at ON requests(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_executions_executed_at ON executions(executed_at DESC);
`
