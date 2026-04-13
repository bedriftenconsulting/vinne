-- +goose Up
-- +goose StatementBegin
-- Add composite index for efficient schedule processing queries
-- This index optimizes queries that check for schedules due for processing
CREATE INDEX IF NOT EXISTS idx_game_schedules_processing
ON game_schedules(status, is_active, scheduled_end, scheduled_draw)
WHERE status = 'SCHEDULED' AND is_active = true;

-- Add comment for documentation
COMMENT ON INDEX idx_game_schedules_processing IS
'Composite index for efficient scheduler queries checking sales cutoffs and draw times';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_game_schedules_processing;
-- +goose StatementEnd
