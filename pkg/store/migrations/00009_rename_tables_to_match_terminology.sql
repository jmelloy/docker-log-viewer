-- +goose Up
-- +goose StatementBegin
-- Rename 'requests' table to 'sample_queries' to match UI terminology
ALTER TABLE requests RENAME TO sample_queries;

-- Rename 'executions' table to 'executed_requests' to match UI terminology
ALTER TABLE executions RENAME TO executed_requests;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Rename tables back to original names
ALTER TABLE executed_requests RENAME TO executions;
ALTER TABLE sample_queries RENAME TO requests;
-- +goose StatementEnd
