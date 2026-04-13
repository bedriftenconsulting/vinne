-- +goose Up
-- +goose StatementBegin
ALTER TABLE agents ALTER COLUMN contact_email DROP NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agents ALTER COLUMN contact_email SET NOT NULL;
-- +goose StatementEnd
