-- +goose Up
-- +goose StatementBegin
-- Change monetary columns from BIGINT to DECIMAL to handle float values properly
ALTER TABLE games 
    ALTER COLUMN min_stake_amount TYPE DECIMAL(10,2) USING min_stake_amount::decimal / 100,
    ALTER COLUMN max_stake_amount TYPE DECIMAL(10,2) USING max_stake_amount::decimal / 100;

-- Add base_price column as DECIMAL
ALTER TABLE games ADD COLUMN IF NOT EXISTS base_price DECIMAL(10,2) NOT NULL DEFAULT 1.00;

-- Drop old constraints
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_min_stake;
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_max_stake;
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_stake_range;

-- Add new constraints with decimal values
ALTER TABLE games ADD CONSTRAINT chk_games_min_stake CHECK (min_stake_amount >= 0.50); -- minimum ₵0.50
ALTER TABLE games ADD CONSTRAINT chk_games_max_stake CHECK (max_stake_amount <= 200000.00); -- maximum ₵200,000
ALTER TABLE games ADD CONSTRAINT chk_games_stake_range CHECK (min_stake_amount <= max_stake_amount);
ALTER TABLE games ADD CONSTRAINT chk_games_base_price CHECK (base_price >= 0.50); -- minimum ₵0.50
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Revert monetary columns back to BIGINT
ALTER TABLE games 
    ALTER COLUMN min_stake_amount TYPE BIGINT USING (min_stake_amount * 100)::bigint,
    ALTER COLUMN max_stake_amount TYPE BIGINT USING (max_stake_amount * 100)::bigint;

-- Remove base_price column
ALTER TABLE games DROP COLUMN IF EXISTS base_price;

-- Drop decimal constraints
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_min_stake;
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_max_stake;
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_stake_range;
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_base_price;

-- Re-add old constraints
ALTER TABLE games ADD CONSTRAINT chk_games_min_stake CHECK (min_stake_amount >= 50); -- minimum ₵0.50 in pesewas
ALTER TABLE games ADD CONSTRAINT chk_games_max_stake CHECK (max_stake_amount <= 20000000); -- maximum ₵200,000 in pesewas
ALTER TABLE games ADD CONSTRAINT chk_games_stake_range CHECK (min_stake_amount <= max_stake_amount);
-- +goose StatementEnd