-- +goose Up
-- +goose StatementBegin

-- Add columns for dual approval tracking
ALTER TABLE game_approvals
    ADD COLUMN IF NOT EXISTS first_approved_by UUID,
    ADD COLUMN IF NOT EXISTS first_approval_date TIMESTAMP,
    ADD COLUMN IF NOT EXISTS first_approval_notes TEXT,
    ADD COLUMN IF NOT EXISTS second_approved_by UUID,
    ADD COLUMN IF NOT EXISTS second_approval_date TIMESTAMP,
    ADD COLUMN IF NOT EXISTS second_approval_notes TEXT,
    ADD COLUMN IF NOT EXISTS approval_count INTEGER DEFAULT 0;

-- Add approval stage for first approval
ALTER TABLE game_approvals
    DROP CONSTRAINT IF EXISTS game_approvals_approval_stage_check;

-- Note: Removed direct pg_catalog update as it requires superuser privileges
-- The approval_stage column type modification should be done through ALTER TABLE if needed

-- Add index for pending approvals query performance
CREATE INDEX IF NOT EXISTS idx_game_approvals_pending 
    ON game_approvals(approval_stage) 
    WHERE approval_stage IN ('SUBMITTED', 'FIRST_APPROVED');

-- Add unique constraint to prevent multiple approval records for same game
ALTER TABLE game_approvals 
    DROP CONSTRAINT IF EXISTS unique_game_approval,
    ADD CONSTRAINT unique_game_approval UNIQUE(game_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove the dual approval columns
ALTER TABLE game_approvals
    DROP COLUMN IF EXISTS first_approved_by,
    DROP COLUMN IF EXISTS first_approval_date,
    DROP COLUMN IF EXISTS first_approval_notes,
    DROP COLUMN IF EXISTS second_approved_by,
    DROP COLUMN IF EXISTS second_approval_date,
    DROP COLUMN IF EXISTS second_approval_notes,
    DROP COLUMN IF EXISTS approval_count;

-- Remove the new index
DROP INDEX IF EXISTS idx_game_approvals_pending;

-- Remove unique constraint
ALTER TABLE game_approvals 
    DROP CONSTRAINT IF EXISTS unique_game_approval;

-- +goose StatementEnd