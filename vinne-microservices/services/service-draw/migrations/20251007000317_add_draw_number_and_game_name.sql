-- +goose Up
-- Add draw_number and game_name columns to draws table
ALTER TABLE draws ADD COLUMN draw_number INTEGER;

-- +goose StatementBegin
ALTER TABLE draws ADD COLUMN game_name VARCHAR(255);
-- +goose StatementEnd

-- +goose StatementBegin
-- Update existing draws with sequential draw numbers per game
WITH numbered_draws AS (
    SELECT
        id,
        game_id,
        ROW_NUMBER() OVER (PARTITION BY game_id ORDER BY scheduled_time, created_at) as rn
    FROM draws
)
UPDATE draws d
SET draw_number = nd.rn
FROM numbered_draws nd
WHERE d.id = nd.id;
-- +goose StatementEnd

-- Make draw_number NOT NULL after backfilling
ALTER TABLE draws ALTER COLUMN draw_number SET NOT NULL;

-- Create a unique index to ensure draw numbers are sequential per game
CREATE UNIQUE INDEX idx_draws_game_id_draw_number ON draws(game_id, draw_number);

-- Create an index on game_name for faster queries
CREATE INDEX idx_draws_game_name ON draws(game_name);

-- +goose Down
-- Drop indexes and column
DROP INDEX IF EXISTS idx_draws_game_name;

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_draws_game_id_draw_number;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE draws DROP COLUMN IF EXISTS game_name;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE draws DROP COLUMN IF EXISTS draw_number;
-- +goose StatementEnd
