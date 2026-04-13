-- +goose Up
CREATE TABLE IF NOT EXISTS player_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0,
    pending_balance BIGINT NOT NULL DEFAULT 0,
    available_balance BIGINT NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'GHS',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    last_transaction_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
    updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
    CONSTRAINT player_wallet_balance_check CHECK (balance >= 0),
    CONSTRAINT player_wallet_pending_check CHECK (pending_balance >= 0)
);

CREATE INDEX idx_player_wallets_player_id ON player_wallets (player_id);
CREATE INDEX idx_player_wallets_status ON player_wallets (status);

-- +goose Down
DROP TABLE IF EXISTS player_wallets;
