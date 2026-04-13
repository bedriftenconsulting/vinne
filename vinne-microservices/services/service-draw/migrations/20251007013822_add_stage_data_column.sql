-- +goose Up
-- Add stage_data column to draws table for execution workflow tracking
ALTER TABLE draws ADD COLUMN stage_data JSONB DEFAULT NULL;

-- Add index for stage_data queries
CREATE INDEX idx_draws_stage_data ON draws USING GIN (stage_data);

-- Add comment for documentation
COMMENT ON COLUMN draws.stage_data IS 'JSON column storing draw execution stage data including current_stage, stage_status, preparation_data, number_selection_data, result_calculation_data, and payout_data';

-- +goose Down
-- Remove stage_data column
DROP INDEX IF EXISTS idx_draws_stage_data;
ALTER TABLE draws DROP COLUMN IF EXISTS stage_data;
