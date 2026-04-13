-- +goose Up
-- Migration to update games table for format-based betting system

-- Add missing fields needed for format-based betting (most fields already exist from previous migrations)
ALTER TABLE games 
ADD COLUMN IF NOT EXISTS game_type VARCHAR(50), -- Optional for backward compatibility
ADD COLUMN IF NOT EXISTS number_range_min INTEGER,
ADD COLUMN IF NOT EXISTS number_range_max INTEGER,
ADD COLUMN IF NOT EXISTS selection_count INTEGER,
ADD COLUMN IF NOT EXISTS sales_cutoff_minutes INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS base_price DECIMAL(10,2), -- GHS amount
ADD COLUMN IF NOT EXISTS multi_draw_enabled BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS max_draws_advance INTEGER;

-- Update existing data to have default values for new fields only
UPDATE games SET number_range_min = 1 WHERE number_range_min IS NULL;
UPDATE games SET number_range_max = 90 WHERE number_range_max IS NULL;
UPDATE games SET selection_count = 5 WHERE selection_count IS NULL;
UPDATE games SET base_price = 1.00 WHERE base_price IS NULL;
UPDATE games SET multi_draw_enabled = false WHERE multi_draw_enabled IS NULL;
UPDATE games SET sales_cutoff_minutes = 30 WHERE sales_cutoff_minutes IS NULL;

-- Add constraints for the new fields only (skip ones that may already exist)
ALTER TABLE games ADD CONSTRAINT chk_games_number_range_min CHECK (number_range_min >= 1);
ALTER TABLE games ADD CONSTRAINT chk_games_number_range_max CHECK (number_range_max >= number_range_min);
ALTER TABLE games ADD CONSTRAINT chk_games_selection_count CHECK (selection_count >= 1 AND selection_count <= (number_range_max - number_range_min + 1));
ALTER TABLE games ADD CONSTRAINT chk_games_max_draws CHECK (max_draws_advance IS NULL OR max_draws_advance >= 1);
ALTER TABLE games ADD CONSTRAINT chk_games_cutoff_minutes CHECK (sales_cutoff_minutes >= 0);

-- Indexes already exist from previous migrations
-- No additional indexes needed for new fields

-- +goose Down
-- Remove the new format-based betting fields added by this migration only
ALTER TABLE games
DROP CONSTRAINT IF EXISTS chk_games_number_range_min,
DROP CONSTRAINT IF EXISTS chk_games_number_range_max,
DROP CONSTRAINT IF EXISTS chk_games_selection_count,
DROP CONSTRAINT IF EXISTS chk_games_max_draws,
DROP CONSTRAINT IF EXISTS chk_games_cutoff_minutes,
DROP CONSTRAINT IF EXISTS chk_games_base_price;

ALTER TABLE games 
DROP COLUMN IF EXISTS game_type,
DROP COLUMN IF EXISTS number_range_min,
DROP COLUMN IF EXISTS number_range_max,
DROP COLUMN IF EXISTS selection_count,
DROP COLUMN IF EXISTS sales_cutoff_minutes,
DROP COLUMN IF EXISTS base_price,
DROP COLUMN IF EXISTS multi_draw_enabled,
DROP COLUMN IF EXISTS max_draws_advance;