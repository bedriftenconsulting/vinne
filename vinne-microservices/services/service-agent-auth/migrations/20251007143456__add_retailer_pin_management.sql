-- +goose Up
-- +goose StatementBegin


-- Creates table for PIN change audit logs
CREATE TABLE IF NOT EXISTS retailer_pin_change_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_id UUID NOT NULL REFERENCES retailers_auth(id),
    retailer_code VARCHAR(20) NOT NULL,
    changed_by UUID NOT NULL,
    change_reason VARCHAR(100),
    device_imei VARCHAR(50),
    ip_address VARCHAR(45),
    user_agent TEXT,
    success BOOLEAN NOT NULL DEFAULT TRUE,
    failure_reason VARCHAR(255),
    sessions_invalidated INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for retailer_pin_change_logs
CREATE INDEX idx_pin_change_logs_retailer_id ON retailer_pin_change_logs(retailer_id);
CREATE INDEX idx_pin_change_logs_created_at ON retailer_pin_change_logs(created_at);
CREATE INDEX idx_pin_change_logs_success ON retailer_pin_change_logs(success);


-- PIN management fields to retailers_auth table
ALTER TABLE retailers_auth
ADD COLUMN pin_updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
ADD COLUMN pin_change_count INTEGER DEFAULT 0,
ADD COLUMN last_pin_change TIMESTAMP,
ADD COLUMN next_pin_change_allowed TIMESTAMP;

-- Trigger to update pin_updated_at
CREATE OR REPLACE FUNCTION update_pin_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.pin_hash IS DISTINCT FROM OLD.pin_hash THEN
        NEW.pin_updated_at = NOW();
        NEW.pin_change_count = OLD.pin_change_count + 1;
        NEW.last_pin_change = NOW();
        NEW.next_pin_change_allowed = NOW() + INTERVAL '24 hours';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_pin_updated_at
BEFORE UPDATE ON retailers_auth
FOR EACH ROW
EXECUTE FUNCTION update_pin_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop all authentication tables and indexes

-- Drop the trigger and its function first (must be done before dropping columns or tables)
DROP TRIGGER IF EXISTS trigger_update_pin_updated_at ON retailers_auth;
DROP FUNCTION IF EXISTS update_pin_updated_at();

-- Remove the added columns from retailers_auth
ALTER TABLE retailers_auth
DROP COLUMN IF EXISTS pin_updated_at,
DROP COLUMN IF EXISTS pin_change_count,
DROP COLUMN IF EXISTS last_pin_change,
DROP COLUMN IF EXISTS next_pin_change_allowed;

-- Drop indexes created for retailer_pin_change_logs
DROP INDEX IF EXISTS idx_pin_change_logs_retailer_id;
DROP INDEX IF EXISTS idx_pin_change_logs_created_at;
DROP INDEX IF EXISTS idx_pin_change_logs_success;

-- Finally, drop the retailer_pin_change_logs table
DROP TABLE IF EXISTS retailer_pin_change_logs;

-- +goose StatementEnd