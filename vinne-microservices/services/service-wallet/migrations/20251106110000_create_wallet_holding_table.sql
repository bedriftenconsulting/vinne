-- +goose Up
-- +goose StatementBegin

-- Create wallet_holds table
CREATE TYPE withdrawal_status AS ENUM ('ACTIVE', 'RELEASED', 'EXPIRED');

CREATE TABLE IF NOT EXISTS wallet_holds (
    wallet_id UUID PRIMARY KEY  REFERENCES retailer_winning_wallets(id) ON DELETE CASCADE,
    retailer_id UUID NOT NULL,
    placed_by UUID NOT NULL,
    reason TEXT,
    status withdrawal_status NOT NULL DEFAULT 'RELEASED',
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_wallet_holds_retailer_id ON wallet_holds(retailer_id);
CREATE INDEX idx_wallet_holds_placed_by ON wallet_holds(placed_by);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop indexes first
DROP INDEX IF EXISTS idx_wallet_holds_retailer_id;
DROP INDEX IF EXISTS idx_wallet_holds_placed_by;

-- Drop the table
DROP TABLE IF EXISTS wallet_holds;

-- Drop the enum type
DROP TYPE IF EXISTS withdrawal_status;

-- +goose StatementEnd