-- +goose Up
-- +goose StatementBegin

-- Add commission_percentage column to agents table
ALTER TABLE agents 
ADD COLUMN commission_percentage DECIMAL(5,2) DEFAULT 30.00 CHECK (commission_percentage >= 0 AND commission_percentage <= 100);

-- Update existing agents with commission percentage from their tier
UPDATE agents a
SET commission_percentage = COALESCE(
    (SELECT ct.base_commission_rate * 100 
     FROM commission_tiers ct 
     WHERE ct.id = a.commission_tier_id),
    30.00
);

-- Make commission_percentage NOT NULL after setting values
ALTER TABLE agents 
ALTER COLUMN commission_percentage SET NOT NULL;

-- Drop foreign key constraint and commission_tier_id column
ALTER TABLE agents 
DROP CONSTRAINT IF EXISTS agents_commission_tier_id_fkey;

ALTER TABLE agents 
DROP COLUMN IF EXISTS commission_tier_id;

-- Drop commission_tiers table as it's no longer needed
DROP TABLE IF EXISTS commission_tiers CASCADE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Recreate commission_tiers table
CREATE TABLE IF NOT EXISTS commission_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    base_commission_rate DECIMAL(5,4) NOT NULL,
    bonus_commission_rate DECIMAL(5,4) DEFAULT 0,
    sales_threshold DECIMAL(15,2) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Insert a default commission tier
INSERT INTO commission_tiers (name, description, base_commission_rate, bonus_commission_rate)
VALUES ('Default', 'Default commission tier', 0.30, 0);

-- Add commission_tier_id column back to agents
ALTER TABLE agents 
ADD COLUMN commission_tier_id UUID REFERENCES commission_tiers(id);

-- Set all agents to use the default tier
UPDATE agents 
SET commission_tier_id = (SELECT id FROM commission_tiers WHERE name = 'Default' LIMIT 1);

-- Drop commission_percentage column
ALTER TABLE agents 
DROP COLUMN IF EXISTS commission_percentage;

-- +goose StatementEnd