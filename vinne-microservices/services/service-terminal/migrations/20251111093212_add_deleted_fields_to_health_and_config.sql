-- +goose Up
-- +goose StatementBegin
ALTER TABLE terminal_health
ADD COLUMN deleted_at TIMESTAMP;

ALTER TABLE terminal_configs
ADD COLUMN deleted_at TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE terminal_health
DROP COLUMN deleted_at;

ALTER TABLE terminal_configs
DROP COLUMN deleted_at;
-- +goose StatementEnd
