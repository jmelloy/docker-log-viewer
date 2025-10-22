-- +goose Up
-- +goose StatementBegin
ALTER TABLE servers ADD COLUMN experimental_mode TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE servers DROP COLUMN experimental_mode;
-- +goose StatementEnd
