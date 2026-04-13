-- +goose Up
-- +goose StatementBegin
-- Add indexes for efficient transaction queries (admin transactions page)

-- Index for filtering by status
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_status
ON wallet_transactions(status);

-- Index for filtering by transaction type
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_type
ON wallet_transactions(transaction_type);

-- Index for date range queries (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_created_at
ON wallet_transactions(created_at DESC);

-- Composite index for common filter combinations
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_status_created_at
ON wallet_transactions(status, created_at DESC);

-- Index for full-text search on transaction_id and reference
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_transaction_id
ON wallet_transactions(transaction_id);

CREATE INDEX IF NOT EXISTS idx_wallet_transactions_reference
ON wallet_transactions(reference)
WHERE reference IS NOT NULL;

-- Composite index for wallet type + created_at (common filter)
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_wallet_type_created_at
ON wallet_transactions(wallet_type, created_at DESC);

-- Index for idempotency key lookups
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_idempotency_key
ON wallet_transactions(idempotency_key)
WHERE idempotency_key IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_wallet_transactions_idempotency_key;
DROP INDEX IF EXISTS idx_wallet_transactions_wallet_type_created_at;
DROP INDEX IF EXISTS idx_wallet_transactions_reference;
DROP INDEX IF EXISTS idx_wallet_transactions_transaction_id;
DROP INDEX IF EXISTS idx_wallet_transactions_status_created_at;
DROP INDEX IF EXISTS idx_wallet_transactions_created_at;
DROP INDEX IF EXISTS idx_wallet_transactions_type;
DROP INDEX IF EXISTS idx_wallet_transactions_status;
-- +goose StatementEnd
