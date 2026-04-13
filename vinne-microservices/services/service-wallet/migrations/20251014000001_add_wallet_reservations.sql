-- +goose Up
-- Create wallet_reservations table for two-phase commit pattern
CREATE TABLE IF NOT EXISTS wallet_reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reservation_id VARCHAR(255) UNIQUE NOT NULL,
    wallet_owner_id UUID NOT NULL,
    wallet_type VARCHAR(50) NOT NULL CHECK (wallet_type IN ('AGENT_STAKE', 'RETAILER_STAKE', 'RETAILER_WINNING')),
    amount BIGINT NOT NULL CHECK (amount > 0),
    reference VARCHAR(255) NOT NULL,
    reason TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'COMMITTED', 'RELEASED', 'EXPIRED')),
    transaction_id UUID,
    idempotency_key VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    committed_at TIMESTAMP,
    released_at TIMESTAMP
);

-- Create indexes for wallet_reservations
CREATE INDEX idx_wallet_reservations_wallet_owner ON wallet_reservations(wallet_owner_id, wallet_type);
CREATE INDEX idx_wallet_reservations_reference ON wallet_reservations(reference);
CREATE INDEX idx_wallet_reservations_status ON wallet_reservations(status);
CREATE INDEX idx_wallet_reservations_expires_at ON wallet_reservations(expires_at) WHERE status = 'ACTIVE';
CREATE UNIQUE INDEX idx_wallet_reservations_idempotency_key ON wallet_reservations(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Add comment
COMMENT ON TABLE wallet_reservations IS 'Temporary fund reservations for two-phase commit pattern in external transactions';

-- +goose Down
DROP TABLE IF EXISTS wallet_reservations;
