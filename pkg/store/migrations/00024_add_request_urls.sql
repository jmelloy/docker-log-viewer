-- +goose Up
ALTER TABLE sample_requests ADD COLUMN url_override text;
ALTER TABLE requests ADD COLUMN url text;

-- +goose Down
ALTER TABLE requests DROP COLUMN url;
ALTER TABLE sample_requests DROP COLUMN url_override;
