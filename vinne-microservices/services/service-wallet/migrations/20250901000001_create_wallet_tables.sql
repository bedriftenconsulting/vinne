-- +goose Up
-- +goose StatementBegin
-- Agent stake wallets table
-- All monetary values stored as pesewas (100 pesewas = 1 Ghana Cedi)
CREATE TABLE IF NOT EXISTS agent_stake_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    pending_balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    available_balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    currency VARCHAR(3) NOT NULL DEFAULT 'GHS',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    last_transaction_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_stake_wallets_agent_id ON agent_stake_wallets(agent_id);
CREATE INDEX idx_agent_stake_wallets_status ON agent_stake_wallets(status);

-- Retailer stake wallets table
CREATE TABLE IF NOT EXISTS retailer_stake_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_id UUID NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    pending_balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    available_balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    currency VARCHAR(3) NOT NULL DEFAULT 'GHS',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    last_transaction_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retailer_stake_wallets_retailer_id ON retailer_stake_wallets(retailer_id);
CREATE INDEX idx_retailer_stake_wallets_status ON retailer_stake_wallets(status);

-- Retailer winning wallets table
CREATE TABLE IF NOT EXISTS retailer_winning_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_id UUID NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    pending_balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    available_balance BIGINT NOT NULL DEFAULT 0, -- in pesewas
    currency VARCHAR(3) NOT NULL DEFAULT 'GHS',
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
    last_transaction_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retailer_winning_wallets_retailer_id ON retailer_winning_wallets(retailer_id);
CREATE INDEX idx_retailer_winning_wallets_status ON retailer_winning_wallets(status);

-- Wallet transactions table
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id VARCHAR(100) UNIQUE NOT NULL,
    wallet_owner_id UUID NOT NULL,
    wallet_type VARCHAR(50) NOT NULL, -- AGENT_STAKE, RETAILER_STAKE, RETAILER_WINNING
    transaction_type VARCHAR(50) NOT NULL, -- CREDIT, DEBIT, TRANSFER, COMMISSION, PAYOUT
    amount BIGINT NOT NULL, -- in pesewas
    balance_before BIGINT NOT NULL, -- in pesewas
    balance_after BIGINT NOT NULL, -- in pesewas
    reference VARCHAR(255),
    description TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- PENDING, COMPLETED, FAILED, REVERSED
    idempotency_key VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    reversed_at TIMESTAMP
);

CREATE INDEX idx_wallet_transactions_wallet_owner_id ON wallet_transactions(wallet_owner_id);
CREATE INDEX idx_wallet_transactions_wallet_type ON wallet_transactions(wallet_type);
CREATE INDEX idx_wallet_transactions_transaction_type ON wallet_transactions(transaction_type);
CREATE INDEX idx_wallet_transactions_status ON wallet_transactions(status);
CREATE INDEX idx_wallet_transactions_created_at ON wallet_transactions(created_at);
CREATE INDEX idx_wallet_transactions_idempotency_key ON wallet_transactions(idempotency_key);
CREATE UNIQUE INDEX idx_wallet_transactions_idempotency ON wallet_transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Wallet transfers table (for agent to retailer transfers)
CREATE TABLE IF NOT EXISTS wallet_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transfer_id VARCHAR(100) UNIQUE NOT NULL,
    from_wallet_id UUID NOT NULL,
    from_wallet_type VARCHAR(50) NOT NULL,
    to_wallet_id UUID NOT NULL,
    to_wallet_type VARCHAR(50) NOT NULL,
    amount BIGINT NOT NULL, -- in pesewas
    commission_amount BIGINT NOT NULL DEFAULT 0, -- in pesewas
    total_deducted BIGINT NOT NULL, -- in pesewas
    reference VARCHAR(255),
    notes TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    idempotency_key VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    reversed_at TIMESTAMP
);

CREATE INDEX idx_wallet_transfers_from_wallet_id ON wallet_transfers(from_wallet_id);
CREATE INDEX idx_wallet_transfers_to_wallet_id ON wallet_transfers(to_wallet_id);
CREATE INDEX idx_wallet_transfers_status ON wallet_transfers(status);
CREATE INDEX idx_wallet_transfers_created_at ON wallet_transfers(created_at);
CREATE UNIQUE INDEX idx_wallet_transfers_idempotency ON wallet_transfers(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Wallet locks table (for transaction safety)
CREATE TABLE IF NOT EXISTS wallet_locks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID NOT NULL,
    wallet_type VARCHAR(50) NOT NULL,
    lock_reason VARCHAR(255) NOT NULL,
    locked_by VARCHAR(255),
    locked_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    released_at TIMESTAMP
);

CREATE INDEX idx_wallet_locks_wallet_id ON wallet_locks(wallet_id);
CREATE INDEX idx_wallet_locks_wallet_type ON wallet_locks(wallet_type);
CREATE INDEX idx_wallet_locks_expires_at ON wallet_locks(expires_at);

-- Agent commission rates table
CREATE TABLE IF NOT EXISTS agent_commission_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    rate INTEGER NOT NULL, -- basis points (e.g., 3000 = 30%, 100 = 1%)
    effective_from TIMESTAMP NOT NULL,
    effective_to TIMESTAMP,
    notes TEXT,
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_commission_rates_agent_id ON agent_commission_rates(agent_id);
CREATE INDEX idx_agent_commission_rates_effective_from ON agent_commission_rates(effective_from);
CREATE INDEX idx_agent_commission_rates_effective_to ON agent_commission_rates(effective_to);

-- Commission transactions table
CREATE TABLE IF NOT EXISTS commission_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    commission_id VARCHAR(100) UNIQUE NOT NULL, -- COM-YYYYMM-AGENTCODE-XXX
    transaction_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    original_amount BIGINT NOT NULL, -- in pesewas
    gross_amount BIGINT NOT NULL, -- in pesewas
    commission_amount BIGINT NOT NULL, -- in pesewas
    commission_rate INTEGER NOT NULL, -- basis points (e.g., 3000 = 30%)
    commission_type VARCHAR(50) NOT NULL, -- DEPOSIT, TRANSFER, STAKE, PAYOUT, BONUS
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- PENDING, CREDITED, REVERSED
    reference VARCHAR(255),
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    credited_at TIMESTAMP,
    reversed_at TIMESTAMP
);

CREATE INDEX idx_commission_transactions_agent_id ON commission_transactions(agent_id);
CREATE INDEX idx_commission_transactions_transaction_id ON commission_transactions(transaction_id);
CREATE INDEX idx_commission_transactions_commission_type ON commission_transactions(commission_type);
CREATE INDEX idx_commission_transactions_status ON commission_transactions(status);
CREATE INDEX idx_commission_transactions_created_at ON commission_transactions(created_at);

-- Commission calculations table (for audit trail)
CREATE TABLE IF NOT EXISTS commission_calculations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    transaction_id UUID,
    transaction_type VARCHAR(50) NOT NULL,
    calculation_type VARCHAR(50),
    input_amount BIGINT NOT NULL, -- in pesewas
    rate_basis_points INTEGER NOT NULL, -- basis points
    commission_rate INTEGER, -- duplicate for compatibility
    gross_amount BIGINT NOT NULL, -- in pesewas
    commission_amount BIGINT NOT NULL, -- in pesewas
    net_amount BIGINT NOT NULL, -- in pesewas
    formula_used VARCHAR(255),
    metadata JSONB,
    calculated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_commission_calculations_agent_id ON commission_calculations(agent_id);
CREATE INDEX idx_commission_calculations_transaction_id ON commission_calculations(transaction_id);
CREATE INDEX idx_commission_calculations_created_at ON commission_calculations(created_at);

-- Commission audit table
CREATE TABLE IF NOT EXISTS commission_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    action VARCHAR(100) NOT NULL,
    action_by UUID NOT NULL,
    old_value JSONB,
    new_value JSONB,
    reason TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_commission_audit_agent_id ON commission_audit(agent_id);
CREATE INDEX idx_commission_audit_action ON commission_audit(action);
CREATE INDEX idx_commission_audit_created_at ON commission_audit(created_at);

-- Create update trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column() RETURNS TRIGGER LANGUAGE plpgsql AS 'BEGIN NEW.updated_at = NOW(); RETURN NEW; END;';

-- Create triggers for updated_at columns
CREATE TRIGGER update_agent_stake_wallets_updated_at BEFORE UPDATE ON agent_stake_wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_retailer_stake_wallets_updated_at BEFORE UPDATE ON retailer_stake_wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_retailer_winning_wallets_updated_at BEFORE UPDATE ON retailer_winning_wallets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agent_commission_rates_updated_at BEFORE UPDATE ON agent_commission_rates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_agent_commission_rates_updated_at ON agent_commission_rates;
DROP TRIGGER IF EXISTS update_retailer_winning_wallets_updated_at ON retailer_winning_wallets;
DROP TRIGGER IF EXISTS update_retailer_stake_wallets_updated_at ON retailer_stake_wallets;
DROP TRIGGER IF EXISTS update_agent_stake_wallets_updated_at ON agent_stake_wallets;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS commission_audit CASCADE;
DROP TABLE IF EXISTS commission_calculations CASCADE;
DROP TABLE IF EXISTS commission_transactions CASCADE;
DROP TABLE IF EXISTS agent_commission_rates CASCADE;
DROP TABLE IF EXISTS wallet_locks CASCADE;
DROP TABLE IF EXISTS wallet_transfers CASCADE;
DROP TABLE IF EXISTS wallet_transactions CASCADE;
DROP TABLE IF EXISTS retailer_winning_wallets CASCADE;
DROP TABLE IF EXISTS retailer_stake_wallets CASCADE;
DROP TABLE IF EXISTS agent_stake_wallets CASCADE;
-- +goose StatementEnd