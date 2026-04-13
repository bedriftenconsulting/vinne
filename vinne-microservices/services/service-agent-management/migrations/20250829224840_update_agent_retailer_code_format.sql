-- +goose Up
-- +goose StatementBegin

-- Update comments to reflect agent and retailer code formats per PRD
COMMENT ON COLUMN agents.agent_code IS '4-digit unique number (e.g., 1001), expandable to 5+ digits';
COMMENT ON COLUMN retailers.retailer_code IS '8-digit code: Agent-managed=[4-digit agent code][4-digit sequence], Independent=0000[4-digit sequence]';

-- Add check constraint to ensure agent codes are numeric
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_agent_code_check;
ALTER TABLE agents ADD CONSTRAINT agents_agent_code_check CHECK (agent_code ~ '^\d+$');

-- Add check constraint to ensure retailer codes are 8 digits
ALTER TABLE retailers DROP CONSTRAINT IF EXISTS retailers_retailer_code_check;
ALTER TABLE retailers ADD CONSTRAINT retailers_retailer_code_check CHECK (retailer_code ~ '^\d{8}$');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove the new constraints
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_agent_code_check;
ALTER TABLE retailers DROP CONSTRAINT IF EXISTS retailers_retailer_code_check;

-- Remove comments
COMMENT ON COLUMN agents.agent_code IS NULL;
COMMENT ON COLUMN retailers.retailer_code IS NULL;

-- +goose StatementEnd
