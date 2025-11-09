-- +goose Up
-- +goose StatementBegin
-- Add sync flag to requests table to indicate synchronous execution
ALTER TABLE requests ADD COLUMN is_sync BOOLEAN NOT NULL DEFAULT 0;

CREATE INDEX idx_requests_is_sync ON requests(is_sync);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_requests_is_sync;
ALTER TABLE requests DROP COLUMN is_sync;
-- +goose StatementEnd
