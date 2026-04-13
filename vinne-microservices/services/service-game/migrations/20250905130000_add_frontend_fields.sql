-- +goose Up
-- Add missing fields needed by frontend form

-- Add game code field (unique identifier for games)
ALTER TABLE games ADD COLUMN code VARCHAR(50) UNIQUE;

-- Add game format and category fields (separate from type)
ALTER TABLE games ADD COLUMN game_format VARCHAR(50);
ALTER TABLE games ADD COLUMN game_category VARCHAR(50);

-- Add max tickets per player field
ALTER TABLE games ADD COLUMN max_tickets_per_player INTEGER DEFAULT 10;

-- Add draw frequency fields
ALTER TABLE games ADD COLUMN draw_frequency VARCHAR(50);
ALTER TABLE games ADD COLUMN draw_days JSONB; -- array of days for weekly/bi_weekly
ALTER TABLE games ADD COLUMN weekly_schedule BOOLEAN DEFAULT FALSE;

-- Add separate start and end time fields
ALTER TABLE games ADD COLUMN start_time VARCHAR(5); -- HH:MM format
ALTER TABLE games ADD COLUMN end_time VARCHAR(5); -- HH:MM format

-- Update min/max ticket price to min/max stake amount for consistency
ALTER TABLE games RENAME COLUMN min_ticket_price TO min_stake_amount;
ALTER TABLE games RENAME COLUMN max_ticket_price TO max_stake_amount;

-- Add constraint for max_tickets_per_player
ALTER TABLE games ADD CONSTRAINT chk_max_tickets_per_player CHECK (max_tickets_per_player >= 1 AND max_tickets_per_player <= 1000);

-- Update existing price constraints with new column names
ALTER TABLE games DROP CONSTRAINT chk_games_min_price;
ALTER TABLE games DROP CONSTRAINT chk_games_max_price;
ALTER TABLE games DROP CONSTRAINT chk_games_price_range;

ALTER TABLE games ADD CONSTRAINT chk_games_min_stake CHECK (min_stake_amount >= 50); -- minimum ₵0.50
ALTER TABLE games ADD CONSTRAINT chk_games_max_stake CHECK (max_stake_amount <= 20000000); -- maximum ₵200,000
ALTER TABLE games ADD CONSTRAINT chk_games_stake_range CHECK (min_stake_amount <= max_stake_amount);

-- Add game rules fields that frontend needs
ALTER TABLE game_rules ADD COLUMN number_range_min INTEGER;
ALTER TABLE game_rules ADD COLUMN number_range_max INTEGER;
ALTER TABLE game_rules ADD COLUMN selection_count INTEGER;

-- Map existing fields to new ones for compatibility
UPDATE game_rules SET number_range_min = 1 WHERE number_range_min IS NULL;
UPDATE game_rules SET number_range_max = total_numbers WHERE number_range_max IS NULL;
UPDATE game_rules SET selection_count = numbers_to_pick WHERE selection_count IS NULL;

-- Add constraints for new fields
ALTER TABLE game_rules ADD CONSTRAINT chk_number_range_min_positive CHECK (number_range_min >= 1);
ALTER TABLE game_rules ADD CONSTRAINT chk_number_range_max_positive CHECK (number_range_max >= 1);
ALTER TABLE game_rules ADD CONSTRAINT chk_selection_count_positive CHECK (selection_count >= 1);
ALTER TABLE game_rules ADD CONSTRAINT chk_range_valid CHECK (number_range_min <= number_range_max);
ALTER TABLE game_rules ADD CONSTRAINT chk_selection_valid CHECK (selection_count <= (number_range_max - number_range_min + 1));

-- Create indexes for new fields
CREATE INDEX idx_games_code ON games(code);
CREATE INDEX idx_games_game_format ON games(game_format);
CREATE INDEX idx_games_game_category ON games(game_category);
CREATE INDEX idx_games_draw_frequency ON games(draw_frequency);

-- +goose Down
-- Drop new indexes
DROP INDEX IF EXISTS idx_games_draw_frequency;
DROP INDEX IF EXISTS idx_games_game_category;
DROP INDEX IF EXISTS idx_games_game_format;
DROP INDEX IF EXISTS idx_games_code;

-- Drop constraints for game_rules new fields
ALTER TABLE game_rules DROP CONSTRAINT IF EXISTS chk_selection_valid;
ALTER TABLE game_rules DROP CONSTRAINT IF EXISTS chk_range_valid;
ALTER TABLE game_rules DROP CONSTRAINT IF EXISTS chk_selection_count_positive;
ALTER TABLE game_rules DROP CONSTRAINT IF EXISTS chk_number_range_max_positive;
ALTER TABLE game_rules DROP CONSTRAINT IF EXISTS chk_number_range_min_positive;

-- Remove game_rules new columns
ALTER TABLE game_rules DROP COLUMN IF EXISTS selection_count;
ALTER TABLE game_rules DROP COLUMN IF EXISTS number_range_max;
ALTER TABLE game_rules DROP COLUMN IF EXISTS number_range_min;

-- Remove games new columns
ALTER TABLE games DROP COLUMN IF EXISTS end_time;
ALTER TABLE games DROP COLUMN IF EXISTS start_time;
ALTER TABLE games DROP COLUMN IF EXISTS weekly_schedule;
ALTER TABLE games DROP COLUMN IF EXISTS draw_days;
ALTER TABLE games DROP COLUMN IF EXISTS draw_frequency;
ALTER TABLE games DROP COLUMN IF EXISTS max_tickets_per_player;
ALTER TABLE games DROP COLUMN IF EXISTS game_category;
ALTER TABLE games DROP COLUMN IF EXISTS game_format;
ALTER TABLE games DROP COLUMN IF EXISTS code;

-- Restore original column names and constraints
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_stake_range;
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_max_stake;
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_games_min_stake;
ALTER TABLE games DROP CONSTRAINT IF EXISTS chk_max_tickets_per_player;

ALTER TABLE games RENAME COLUMN max_stake_amount TO max_ticket_price;
ALTER TABLE games RENAME COLUMN min_stake_amount TO min_ticket_price;

ALTER TABLE games ADD CONSTRAINT chk_games_min_price CHECK (min_ticket_price >= 50);
ALTER TABLE games ADD CONSTRAINT chk_games_max_price CHECK (max_ticket_price <= 20000000);
ALTER TABLE games ADD CONSTRAINT chk_games_price_range CHECK (min_ticket_price <= max_ticket_price);