-- +goose Up
ALTER TABLE tickets ADD COLUMN draw_id UUID;
CREATE INDEX idx_tickets_draw_id ON tickets(draw_id);

-- Update existing tickets to set draw_id based on draw_number (if possible)
-- This is a placeholder - in production you'd need to look up actual draw IDs

-- +goose Down
DROP INDEX IF EXISTS idx_tickets_draw_id;
ALTER TABLE tickets DROP COLUMN draw_id;
