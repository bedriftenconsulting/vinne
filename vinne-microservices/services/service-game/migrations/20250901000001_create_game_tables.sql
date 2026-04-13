-- +goose Up
-- +goose StatementBegin
-- Games table
-- All monetary values stored as pesewas (100 pesewas = 1 Ghana Cedi)
CREATE TABLE IF NOT EXISTS games (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- GAME_TYPE_5_90, GAME_TYPE_6_49, GAME_TYPE_CUSTOM
    organizer VARCHAR(50) NOT NULL, -- ORGANIZER_NLA, ORGANIZER_RAND_LOTTERY
    min_ticket_price BIGINT NOT NULL DEFAULT 50, -- minimum ₵0.50 in pesewas
    max_ticket_price BIGINT NOT NULL DEFAULT 20000, -- configurable max in pesewas
    status VARCHAR(50) NOT NULL DEFAULT 'DRAFT', -- DRAFT, PENDING_APPROVAL, APPROVED, ACTIVE, SUSPENDED, TERMINATED
    description TEXT,
    start_date TIMESTAMP,
    end_date TIMESTAMP,
    draw_time TIMESTAMP,
    version VARCHAR(50) NOT NULL DEFAULT '1.0',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_games_type ON games(type);
CREATE INDEX idx_games_organizer ON games(organizer);
CREATE INDEX idx_games_status ON games(status);
CREATE INDEX idx_games_name ON games(name);
CREATE INDEX idx_games_start_date ON games(start_date);
CREATE INDEX idx_games_draw_time ON games(draw_time);

-- Game rules table
CREATE TABLE IF NOT EXISTS game_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    numbers_to_pick INTEGER NOT NULL, -- e.g., 5 for 5/90
    total_numbers INTEGER NOT NULL, -- e.g., 90 for 5/90
    min_selections INTEGER NOT NULL DEFAULT 1,
    max_selections INTEGER NOT NULL DEFAULT 10,
    allow_quick_pick BOOLEAN NOT NULL DEFAULT true,
    special_rules TEXT,
    effective_from TIMESTAMP NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_game_rules_game_id ON game_rules(game_id);
CREATE INDEX idx_game_rules_effective_from ON game_rules(effective_from);
CREATE INDEX idx_game_rules_effective_to ON game_rules(effective_to);

-- Prize structures table
CREATE TABLE IF NOT EXISTS prize_structures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    total_prize_pool BIGINT NOT NULL DEFAULT 0, -- in pesewas
    house_edge_percentage DECIMAL(5,2) NOT NULL DEFAULT 30.00, -- e.g., 30.00%
    effective_from TIMESTAMP NOT NULL DEFAULT NOW(),
    effective_to TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prize_structures_game_id ON prize_structures(game_id);
CREATE INDEX idx_prize_structures_effective_from ON prize_structures(effective_from);

-- Prize tiers table
CREATE TABLE IF NOT EXISTS prize_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prize_structure_id UUID NOT NULL REFERENCES prize_structures(id) ON DELETE CASCADE,
    tier_number INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL, -- e.g., "First Prize", "Second Prize"
    matches_required INTEGER NOT NULL,
    prize_amount BIGINT NOT NULL DEFAULT 0, -- in pesewas
    prize_percentage DECIMAL(5,2) NOT NULL DEFAULT 0.00, -- percentage of prize pool
    estimated_winners BIGINT NOT NULL DEFAULT 0,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prize_tiers_structure_id ON prize_tiers(prize_structure_id);
CREATE INDEX idx_prize_tiers_tier_number ON prize_tiers(tier_number);
CREATE UNIQUE INDEX idx_prize_tiers_structure_tier ON prize_tiers(prize_structure_id, tier_number);

-- Game versions table (for audit trail)
CREATE TABLE IF NOT EXISTS game_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    changes JSONB,
    changed_by UUID NOT NULL,
    change_reason TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_game_versions_game_id ON game_versions(game_id);
CREATE INDEX idx_game_versions_version ON game_versions(version);
CREATE INDEX idx_game_versions_created_at ON game_versions(created_at);

-- Game approvals table
CREATE TABLE IF NOT EXISTS game_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    approval_stage VARCHAR(50) NOT NULL, -- SUBMITTED, REVIEWED, APPROVED, REJECTED
    approved_by UUID,
    rejected_by UUID,
    approval_date TIMESTAMP,
    rejection_date TIMESTAMP,
    notes TEXT,
    reason TEXT, -- for rejection
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_game_approvals_game_id ON game_approvals(game_id);
CREATE INDEX idx_game_approvals_stage ON game_approvals(approval_stage);
CREATE INDEX idx_game_approvals_approval_date ON game_approvals(approval_date);

-- Game schedules table
CREATE TABLE IF NOT EXISTS game_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    scheduled_start TIMESTAMP NOT NULL,
    scheduled_end TIMESTAMP NOT NULL,
    scheduled_draw TIMESTAMP NOT NULL,
    frequency VARCHAR(50) NOT NULL, -- DAILY, WEEKLY, MONTHLY, CUSTOM
    is_active BOOLEAN NOT NULL DEFAULT true,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_game_schedules_game_id ON game_schedules(game_id);
CREATE INDEX idx_game_schedules_scheduled_start ON game_schedules(scheduled_start);
CREATE INDEX idx_game_schedules_scheduled_draw ON game_schedules(scheduled_draw);
CREATE INDEX idx_game_schedules_frequency ON game_schedules(frequency);
CREATE INDEX idx_game_schedules_is_active ON game_schedules(is_active);

-- Game audit table
CREATE TABLE IF NOT EXISTS game_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL,
    action VARCHAR(100) NOT NULL,
    action_by UUID NOT NULL,
    old_value JSONB,
    new_value JSONB,
    reason TEXT,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_game_audit_game_id ON game_audit(game_id);
CREATE INDEX idx_game_audit_action ON game_audit(action);
CREATE INDEX idx_game_audit_action_by ON game_audit(action_by);
CREATE INDEX idx_game_audit_created_at ON game_audit(created_at);

-- Create update trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column() RETURNS TRIGGER LANGUAGE plpgsql AS 'BEGIN NEW.updated_at = NOW(); RETURN NEW; END;';

-- Create triggers for updated_at columns
CREATE TRIGGER update_games_updated_at BEFORE UPDATE ON games
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_game_rules_updated_at BEFORE UPDATE ON game_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_prize_structures_updated_at BEFORE UPDATE ON prize_structures
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_prize_tiers_updated_at BEFORE UPDATE ON prize_tiers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_game_approvals_updated_at BEFORE UPDATE ON game_approvals
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_game_schedules_updated_at BEFORE UPDATE ON game_schedules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Check constraints for business rules
ALTER TABLE games ADD CONSTRAINT chk_games_min_price CHECK (min_ticket_price >= 50); -- minimum ₵0.50
ALTER TABLE games ADD CONSTRAINT chk_games_max_price CHECK (max_ticket_price <= 20000000); -- maximum ₵200,000
ALTER TABLE games ADD CONSTRAINT chk_games_price_range CHECK (min_ticket_price <= max_ticket_price);
ALTER TABLE prize_structures ADD CONSTRAINT chk_prize_pool_positive CHECK (total_prize_pool >= 0);
ALTER TABLE prize_tiers ADD CONSTRAINT chk_tier_number_positive CHECK (tier_number > 0);
ALTER TABLE prize_tiers ADD CONSTRAINT chk_matches_positive CHECK (matches_required >= 0);
ALTER TABLE game_rules ADD CONSTRAINT chk_numbers_to_pick_positive CHECK (numbers_to_pick > 0);
ALTER TABLE game_rules ADD CONSTRAINT chk_total_numbers_positive CHECK (total_numbers > 0);
ALTER TABLE game_rules ADD CONSTRAINT chk_pick_less_than_total CHECK (numbers_to_pick <= total_numbers);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_game_schedules_updated_at ON game_schedules;
DROP TRIGGER IF EXISTS update_game_approvals_updated_at ON game_approvals;
DROP TRIGGER IF EXISTS update_prize_tiers_updated_at ON prize_tiers;
DROP TRIGGER IF EXISTS update_prize_structures_updated_at ON prize_structures;
DROP TRIGGER IF EXISTS update_game_rules_updated_at ON game_rules;
DROP TRIGGER IF EXISTS update_games_updated_at ON games;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS game_audit CASCADE;
DROP TABLE IF EXISTS game_schedules CASCADE;
DROP TABLE IF EXISTS game_approvals CASCADE;
DROP TABLE IF EXISTS game_versions CASCADE;
DROP TABLE IF EXISTS prize_tiers CASCADE;
DROP TABLE IF EXISTS prize_structures CASCADE;
DROP TABLE IF EXISTS game_rules CASCADE;
DROP TABLE IF EXISTS games CASCADE;
-- +goose StatementEnd