-- +goose Up
-- +goose StatementBegin
-- Migrate existing games with approval statuses to simplified workflow
-- This migration removes the approval workflow requirement

-- Convert PENDING_APPROVAL games back to DRAFT (not yet fully approved)
UPDATE games
SET status = 'DRAFT', updated_at = NOW()
WHERE status = 'PENDING_APPROVAL';

-- Convert APPROVED games back to DRAFT (approved but not activated, allow activation)
UPDATE games
SET status = 'DRAFT', updated_at = NOW()
WHERE status = 'APPROVED';

-- Update column comment to reflect new status values
COMMENT ON COLUMN games.status IS 'Game status: DRAFT, ACTIVE, SUSPENDED, TERMINATED (approval statuses removed)';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Reverse migration - restore approval statuses
-- Note: This is a best-effort reversal and may not perfectly restore the original state

-- Restore DRAFT games created within last 24 hours as PENDING_APPROVAL
-- (assuming they were recently migrated)
UPDATE games
SET status = 'PENDING_APPROVAL', updated_at = NOW()
WHERE status = 'DRAFT'
  AND created_at > NOW() - INTERVAL '24 hours'
  AND created_at < (SELECT MAX(created_at) FROM games WHERE status = 'ACTIVE');

-- Restore column comment
COMMENT ON COLUMN games.status IS 'Game status: DRAFT, PENDING_APPROVAL, APPROVED, ACTIVE, SUSPENDED, TERMINATED';

-- +goose StatementEnd
