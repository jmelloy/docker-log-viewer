-- +goose Up
-- +goose StatementBegin
-- Add sync flag to requests table to indicate synchronous execution
ALTER TABLE requests
    ADD COLUMN name TEXT;

-- +goose StatementEnd
-- +goose Down
-- +goose StatementBegin
ALTER TABLE requests
    DROP COLUMN name;

-- +goose StatementEnd
