-- +goose Up
-- Add time fields to games table for start_time, end_time, and draw_time
ALTER TABLE games 
    ADD COLUMN IF NOT EXISTS start_time_str VARCHAR(10),
    ADD COLUMN IF NOT EXISTS end_time_str VARCHAR(10),
    ADD COLUMN IF NOT EXISTS draw_time_str VARCHAR(10);

-- Update existing row with sample time values
UPDATE games 
SET start_time_str = '08:00',
    end_time_str = '17:00',
    draw_time_str = '19:00'
WHERE name = 'National Lotto 5/90';

-- +goose Down
-- Remove time fields from games table
ALTER TABLE games 
    DROP COLUMN IF EXISTS start_time_str,
    DROP COLUMN IF EXISTS end_time_str,
    DROP COLUMN IF EXISTS draw_time_str;