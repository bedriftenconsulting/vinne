-- +goose Up
-- +goose StatementBegin
-- Make selected_numbers nullable since individual numbers are stored in bet_lines
-- This allows tickets where numbers are only in bet_lines (e.g., from POS)
ALTER TABLE tickets ALTER COLUMN selected_numbers DROP NOT NULL;
ALTER TABLE tickets ALTER COLUMN selected_numbers SET DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert selected_numbers to NOT NULL (will fail if there are NULL values)
ALTER TABLE tickets ALTER COLUMN selected_numbers DROP DEFAULT;
ALTER TABLE tickets ALTER COLUMN selected_numbers SET NOT NULL;
-- +goose StatementEnd
