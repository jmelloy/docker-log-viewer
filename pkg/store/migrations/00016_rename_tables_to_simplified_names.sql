-- +goose Up
-- Rename tables to use simplified, more intuitive names
-- executed_requests => requests
-- execution_logs => request_log_messages
-- sql_queries => request_sql_statements
-- database_urls => databases
-- sample_queries => sample_requests

-- Rename executed_requests to requests
ALTER TABLE executed_requests RENAME TO requests;

-- Rename execution_logs to request_log_messages
ALTER TABLE execution_logs RENAME TO request_log_messages;

-- Rename sql_queries to request_sql_statements
ALTER TABLE sql_queries RENAME TO request_sql_statements;

-- Rename database_urls to databases
ALTER TABLE database_urls RENAME TO databases;

-- Rename sample_queries to sample_requests
ALTER TABLE sample_queries RENAME TO sample_requests;

-- +goose Down
-- Revert the table renames
ALTER TABLE sample_requests RENAME TO sample_queries;
ALTER TABLE databases RENAME TO database_urls;
ALTER TABLE request_sql_statements RENAME TO sql_queries;
ALTER TABLE request_log_messages RENAME TO execution_logs;
ALTER TABLE requests RENAME TO executed_requests;
