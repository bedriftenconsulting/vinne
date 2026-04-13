-- +goose Up
ALTER TABLE draws ADD COLUMN IF NOT EXISTS game_code VARCHAR(20);

-- Create an index on game_code for efficient querying
CREATE INDEX IF NOT EXISTS idx_draws_game_code ON draws(game_code);

-- +goose Down
DROP INDEX IF EXISTS idx_draws_game_code;
ALTER TABLE draws DROP COLUMN IF EXISTS game_code;