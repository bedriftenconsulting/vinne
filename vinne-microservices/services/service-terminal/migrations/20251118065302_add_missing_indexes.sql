-- +goose Up
-- +goose StatementBegin
-- Improve assignment query performance
CREATE INDEX IF NOT EXISTS idx_terminal_assignments_retailer_active
ON terminal_assignments(retailer_id, is_active)
WHERE is_active = true;

-- Improve heartbeat queries
CREATE INDEX IF NOT EXISTS idx_terminal_health_terminal_heartbeat
ON terminal_health(terminal_id, last_heartbeat DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_terminal_assignments_retailer_active;
DROP INDEX IF EXISTS idx_terminal_health_terminal_heartbeat;
-- +goose StatementEnd