-- +goose Up
-- +goose StatementBegin

-- Fix sagas table: rename last_updated_at to updated_at for consistency
-- This fixes the trigger error: record "new" has no field "updated_at"
ALTER TABLE sagas RENAME COLUMN last_updated_at TO updated_at;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Revert: rename updated_at back to last_updated_at
ALTER TABLE sagas RENAME COLUMN updated_at TO last_updated_at;

-- +goose StatementEnd
