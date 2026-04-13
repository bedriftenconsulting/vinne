-- +goose Up
-- +goose StatementBegin
ALTER TABLE terminals ADD COLUMN manufacturer VARCHAR(100);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE terminals DROP COLUMN manufacturer;
-- +goose StatementEnd
