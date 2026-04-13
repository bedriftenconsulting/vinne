-- +goose Up
-- +goose StatementBegin
-- Add logo and branding fields to games table
ALTER TABLE games
ADD COLUMN logo_url TEXT,
ADD COLUMN brand_color VARCHAR(7) CHECK (brand_color IS NULL OR brand_color ~ '^#[0-9A-Fa-f]{6}$');

-- Add index for faster lookups when filtering games with logos
CREATE INDEX idx_games_logo_url ON games(logo_url) WHERE logo_url IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN games.logo_url IS 'URL to the game logo/image stored in object storage (Spaces/S3)';
COMMENT ON COLUMN games.brand_color IS 'Brand color in HEX format (e.g., #FF5733) for UI theming';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Remove logo and branding fields
DROP INDEX IF EXISTS idx_games_logo_url;
ALTER TABLE games
DROP COLUMN IF EXISTS logo_url,
DROP COLUMN IF EXISTS brand_color;
-- +goose StatementEnd
