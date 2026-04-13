-- +goose Up
-- Add machine_numbers column to draws table
-- Machine numbers are cosmetic identifiers entered after draw completion
-- They are displayed alongside winning numbers but not used in calculations
ALTER TABLE draws ADD COLUMN machine_numbers INTEGER[] DEFAULT '{}';

-- Add comment for documentation
COMMENT ON COLUMN draws.machine_numbers IS 'Cosmetic machine numbers entered after draw completion. Displayed alongside winning numbers but not used in winnings calculations.';

-- +goose Down
-- Remove machine_numbers column
ALTER TABLE draws DROP COLUMN IF EXISTS machine_numbers;
