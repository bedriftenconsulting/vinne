-- +goose Up
-- Remove bonus number fields from games table if they exist
ALTER TABLE games DROP COLUMN IF EXISTS bonus_number_enabled;
ALTER TABLE games DROP COLUMN IF EXISTS bonus_range_min;
ALTER TABLE games DROP COLUMN IF EXISTS bonus_range_max;
ALTER TABLE games DROP COLUMN IF EXISTS bonus_count;

-- +goose Down
-- Add bonus number fields back to games table

ALTER TABLE games ADD COLUMN IF NOT EXISTS bonus_number_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE games ADD COLUMN IF NOT EXISTS bonus_range_min INTEGER;
ALTER TABLE games ADD COLUMN IF NOT EXISTS bonus_range_max INTEGER;
ALTER TABLE games ADD COLUMN IF NOT EXISTS bonus_count INTEGER;