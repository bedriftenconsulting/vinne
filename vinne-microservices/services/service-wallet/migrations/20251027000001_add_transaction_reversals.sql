-- +goose Up
-- +goose StatementBegin

-- Transaction reversal audit table
CREATE TABLE IF NOT EXISTS transaction_reversals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_transaction_id UUID NOT NULL REFERENCES wallet_transactions(id),
    reversal_transaction_id UUID REFERENCES wallet_transactions(id),
    original_amount BIGINT NOT NULL, -- in pesewas
    wallet_owner_id UUID NOT NULL,
    wallet_type VARCHAR(50) NOT NULL,
    reason TEXT NOT NULL,
    reversed_by UUID NOT NULL, -- Admin user ID
    reversed_by_name VARCHAR(255),
    reversed_by_email VARCHAR(255),
    reversed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transaction_reversals_original_tx ON transaction_reversals(original_transaction_id);
CREATE INDEX idx_transaction_reversals_reversal_tx ON transaction_reversals(reversal_transaction_id);
CREATE INDEX idx_transaction_reversals_wallet_owner ON transaction_reversals(wallet_owner_id);
CREATE INDEX idx_transaction_reversals_reversed_by ON transaction_reversals(reversed_by);
CREATE INDEX idx_transaction_reversals_reversed_at ON transaction_reversals(reversed_at);

-- Add reversal reference to transactions
ALTER TABLE wallet_transactions
ADD COLUMN IF NOT EXISTS reversed_by_transaction_id UUID REFERENCES wallet_transactions(id),
ADD COLUMN IF NOT EXISTS reversal_reason TEXT;

CREATE INDEX idx_wallet_transactions_reversed_by_tx ON wallet_transactions(reversed_by_transaction_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_wallet_transactions_reversed_by_tx;
ALTER TABLE wallet_transactions DROP COLUMN IF EXISTS reversal_reason;
ALTER TABLE wallet_transactions DROP COLUMN IF EXISTS reversed_by_transaction_id;
DROP TABLE IF EXISTS transaction_reversals CASCADE;
-- +goose StatementEnd
