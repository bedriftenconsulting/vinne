-- +goose Up
-- +goose StatementBegin

-- Add payout tracking fields to tickets table
ALTER TABLE tickets
ADD COLUMN IF NOT EXISTS paid_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN IF NOT EXISTS paid_by VARCHAR(255),
ADD COLUMN IF NOT EXISTS payment_reference VARCHAR(255);

-- Create indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_tickets_paid_at ON tickets(paid_at);
CREATE INDEX IF NOT EXISTS idx_tickets_payment_reference ON tickets(payment_reference);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove indexes
DROP INDEX IF EXISTS idx_tickets_payment_reference;
DROP INDEX IF EXISTS idx_tickets_paid_at;

-- Remove payout columns
ALTER TABLE tickets
DROP COLUMN IF EXISTS paid_at,
DROP COLUMN IF EXISTS paid_by,
DROP COLUMN IF EXISTS payment_reference;

-- +goose StatementEnd
