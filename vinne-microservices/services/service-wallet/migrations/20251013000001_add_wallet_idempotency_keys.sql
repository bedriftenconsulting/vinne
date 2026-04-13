-- +goose Up
-- +goose StatementBegin

-- Wallet Idempotency Keys Table
-- Tracks idempotency keys to prevent duplicate transactions
-- Keys are stored with transaction references for deduplication
CREATE TABLE IF NOT EXISTS wallet_idempotency_keys (
    idempotency_key VARCHAR(255) PRIMARY KEY,
    transaction_id UUID NOT NULL,
    wallet_owner_id UUID NOT NULL,
    wallet_type VARCHAR(50) NOT NULL,
    operation_type VARCHAR(50) NOT NULL, -- 'credit', 'debit'
    amount BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Index for cleanup queries (TTL-based)
CREATE INDEX IF NOT EXISTS idx_wallet_idempotency_created_at ON wallet_idempotency_keys(created_at);

-- Index for wallet owner lookup
CREATE INDEX IF NOT EXISTS idx_wallet_idempotency_owner ON wallet_idempotency_keys(wallet_owner_id);

-- Add idempotency_key column to wallet_transactions if not exists
ALTER TABLE wallet_transactions
ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255);

-- Create index on idempotency_key for quick lookups
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_idempotency_key
ON wallet_transactions(idempotency_key)
WHERE idempotency_key IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_wallet_transactions_idempotency_key;

ALTER TABLE wallet_transactions
DROP COLUMN IF EXISTS idempotency_key;

DROP INDEX IF EXISTS idx_wallet_idempotency_owner;
DROP INDEX IF EXISTS idx_wallet_idempotency_created_at;
DROP TABLE IF EXISTS wallet_idempotency_keys;

-- +goose StatementEnd
