-- +goose Up
-- +goose StatementBegin
-- Add status field to track schedule state
ALTER TABLE game_schedules ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'SCHEDULED';

-- Add game_name for better visibility
ALTER TABLE game_schedules ADD COLUMN IF NOT EXISTS game_name VARCHAR(255);

-- Add draw_result_id for tracking completed draws
ALTER TABLE game_schedules ADD COLUMN IF NOT EXISTS draw_result_id UUID;

-- Add index for status to improve query performance
CREATE INDEX IF NOT EXISTS idx_game_schedules_status ON game_schedules(status);

-- Add check constraint for valid status values
ALTER TABLE game_schedules ADD CONSTRAINT check_schedule_status
CHECK (status IN ('SCHEDULED', 'IN_PROGRESS', 'COMPLETED', 'CANCELLED', 'FAILED'));

-- Update existing schedules based on their state
UPDATE game_schedules
SET status = CASE
    WHEN scheduled_draw < NOW() THEN 'COMPLETED'
    WHEN is_active = false THEN 'CANCELLED'
    ELSE 'SCHEDULED'
END;

-- Populate game_name from games table
UPDATE game_schedules gs
SET game_name = g.name
FROM games g
WHERE gs.game_id = g.id;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Drop constraints and indexes
ALTER TABLE game_schedules DROP CONSTRAINT IF EXISTS check_schedule_status;
DROP INDEX IF EXISTS idx_game_schedules_status;

-- Drop columns
ALTER TABLE game_schedules DROP COLUMN IF EXISTS draw_result_id;
ALTER TABLE game_schedules DROP COLUMN IF EXISTS game_name;
ALTER TABLE game_schedules DROP COLUMN IF EXISTS status;
-- +goose StatementEnd
