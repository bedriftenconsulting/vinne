-- +goose Up
ALTER TABLE draws ADD COLUMN IF NOT EXISTS game_schedule_id UUID;

-- Create an index on game_schedule_id for efficient querying
CREATE INDEX IF NOT EXISTS idx_draws_game_schedule_id ON draws(game_schedule_id);

-- +goose Down
DROP INDEX IF EXISTS idx_draws_game_schedule_id;
ALTER TABLE draws DROP COLUMN IF EXISTS game_schedule_id;
