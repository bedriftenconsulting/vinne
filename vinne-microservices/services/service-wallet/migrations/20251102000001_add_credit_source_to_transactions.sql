-- +goose Up
-- Add credit_source column to track how wallet credits originated
ALTER TABLE wallet_transactions
ADD COLUMN credit_source VARCHAR(50) NOT NULL DEFAULT 'admin_direct';

-- Add index for filtering by credit source
CREATE INDEX idx_wallet_transactions_credit_source ON wallet_transactions(credit_source);

-- Add comment for documentation
COMMENT ON COLUMN wallet_transactions.credit_source IS 'Source of wallet credit: admin_direct, mobile_money, bank_transfer, system_adjustment, reversal';

-- +goose Down
DROP INDEX IF EXISTS idx_wallet_transactions_credit_source;
ALTER TABLE wallet_transactions DROP COLUMN IF EXISTS credit_source;
