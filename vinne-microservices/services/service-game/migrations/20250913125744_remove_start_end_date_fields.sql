-- +goose Up
-- Remove start_date and end_date columns from games table if they exist
ALTER TABLE games DROP COLUMN IF EXISTS start_date;
ALTER TABLE games DROP COLUMN IF EXISTS end_date;

-- +goose Down  
-- Re-add start_date and end_date columns to games table
ALTER TABLE games ADD COLUMN IF NOT EXISTS start_date DATE;
ALTER TABLE games ADD COLUMN IF NOT EXISTS end_date DATE;