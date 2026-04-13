-- +goose Up
-- +goose StatementBegin

-- Make email field optional for agents
-- This allows agents to be created without email addresses for cases where email is not available

-- Drop the existing UNIQUE constraint on contact_email first (to recreate it as unique only for non-NULL values)
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_contact_email_key;

-- Make the email field nullable (remove NOT NULL constraint)
ALTER TABLE agents ALTER COLUMN contact_email DROP NOT NULL;

-- Add a partial unique index that only enforces uniqueness for non-NULL email values
-- This prevents duplicate emails while allowing multiple NULL values
-- Note: Removed CONCURRENTLY to allow running inside transaction during tests
CREATE UNIQUE INDEX IF NOT EXISTS agents_contact_email_unique_partial 
ON agents (contact_email) WHERE contact_email IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Reverse the changes - make email required again
-- Note: This rollback will fail if there are agents with NULL emails in the database

-- Drop the partial unique index
DROP INDEX IF EXISTS agents_contact_email_unique_partial;

-- Make the email field required again (add NOT NULL constraint)
-- This will fail if there are any NULL email values in the database
ALTER TABLE agents ALTER COLUMN contact_email SET NOT NULL;

-- Add back the simple unique constraint
ALTER TABLE agents ADD CONSTRAINT agents_contact_email_key UNIQUE (contact_email);

-- +goose StatementEnd