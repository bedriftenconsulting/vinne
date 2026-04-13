-- +goose Up
-- +goose StatementBegin
ALTER TABLE retailers ALTER COLUMN physical_address DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE retailers ALTER COLUMN physical_address SET NOT NULL;
-- +goose StatementEnd
