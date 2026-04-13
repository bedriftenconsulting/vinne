-- +goose Up
-- +goose StatementBegin
-- Drop the old constraint
ALTER TABLE retailers DROP CONSTRAINT IF EXISTS retailers_retailer_code_check;

-- Add new constraint for 8-digit retailer codes
ALTER TABLE retailers ADD CONSTRAINT retailers_retailer_code_check 
CHECK (retailer_code ~ '^\d{8}$');

-- Update comment on column
COMMENT ON COLUMN retailers.retailer_code IS 'Retailer code format: 8 digits (e.g., 00000001 for independent, 10010001 for agent-managed)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Drop the new constraint
ALTER TABLE retailers DROP CONSTRAINT IF EXISTS retailers_retailer_code_check;

-- Add back the old constraint
ALTER TABLE retailers ADD CONSTRAINT retailers_retailer_code_check 
CHECK (retailer_code ~ '^RTL-\d{4}-\d{7}$');

-- Update comment on column
COMMENT ON COLUMN retailers.retailer_code IS 'Retailer code format: RTL-YYYY-XXXXXXX (e.g., RTL-2025-0000001)';
-- +goose StatementEnd