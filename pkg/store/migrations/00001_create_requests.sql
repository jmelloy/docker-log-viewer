-- +goose Up
-- +goose StatementBegin
CREATE TABLE requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    request_data TEXT NOT NULL,
    bearer_token TEXT,
    dev_id TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_requests_created_at ON requests(created_at DESC);
CREATE INDEX idx_requests_deleted_at ON requests(deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_requests_deleted_at;
DROP INDEX IF EXISTS idx_requests_created_at;
DROP TABLE IF EXISTS requests;
-- +goose StatementEnd
