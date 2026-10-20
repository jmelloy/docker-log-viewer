-- +goose Up
ALTER TABLE servers ADD COLUMN headers TEXT;
ALTER TABLE sample_requests ADD COLUMN headers TEXT;
ALTER TABLE requests ADD COLUMN headers TEXT;

-- +goose Down
ALTER TABLE requests DROP COLUMN headers;
ALTER TABLE sample_requests DROP COLUMN headers;
ALTER TABLE servers DROP COLUMN headers;
