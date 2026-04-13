-- +goose Up
-- +goose StatementBegin

-- First update the existing values to match the code constants
UPDATE agents SET onboarding_method = 'RAND_LOTTERY_LTD_DIRECT' WHERE onboarding_method = 'RANDCO_DIRECT';
UPDATE agents SET onboarding_method = 'RAND_LOTTERY_LTD_DIRECT' WHERE onboarding_method = 'RAND_LOTTERY_LTD_DIRECT';

-- Drop the old constraint
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_onboarding_method_check;

-- Add the new constraint with correct values matching the code
ALTER TABLE agents ADD CONSTRAINT agents_onboarding_method_check 
CHECK (onboarding_method IN ('RAND_LOTTERY_LTD_DIRECT', 'REFERRAL'));

-- Also fix retailers table if it has the same issue
UPDATE retailers SET onboarding_method = 'RAND_LOTTERY_LTD_DIRECT' WHERE onboarding_method = 'RANDCO_DIRECT';
UPDATE retailers SET onboarding_method = 'AGENT_ONBOARDED' WHERE onboarding_method = 'AGENT_MANAGED';

ALTER TABLE retailers DROP CONSTRAINT IF EXISTS retailers_onboarding_method_check;

ALTER TABLE retailers ADD CONSTRAINT retailers_onboarding_method_check 
CHECK (onboarding_method IN ('RAND_LOTTERY_LTD_DIRECT', 'AGENT_ONBOARDED'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Revert the values
UPDATE agents SET onboarding_method = 'RANDCO_DIRECT' WHERE onboarding_method = 'RAND_LOTTERY_LTD_DIRECT';

-- Revert the constraints
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_onboarding_method_check;
ALTER TABLE agents ADD CONSTRAINT agents_onboarding_method_check 
CHECK (onboarding_method IN ('RANDCO_DIRECT', 'REFERRAL'));

UPDATE retailers SET onboarding_method = 'RANDCO_DIRECT' WHERE onboarding_method = 'RAND_LOTTERY_LTD_DIRECT';
UPDATE retailers SET onboarding_method = 'AGENT_MANAGED' WHERE onboarding_method = 'AGENT_ONBOARDED';

ALTER TABLE retailers DROP CONSTRAINT IF EXISTS retailers_onboarding_method_check;
ALTER TABLE retailers ADD CONSTRAINT retailers_onboarding_method_check 
CHECK (onboarding_method IN ('RANDCO_DIRECT', 'AGENT_MANAGED'));

-- +goose StatementEnd