-- +goose Up
-- Create the draws table
CREATE TABLE draws (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL,
    draw_name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'in_progress', 'completed', 'failed', 'cancelled')),
    scheduled_time TIMESTAMP WITH TIME ZONE NOT NULL,
    executed_time TIMESTAMP WITH TIME ZONE,
    winning_numbers INTEGER[] DEFAULT '{}',
    nla_draw_reference VARCHAR(255), -- Official NLA draw reference number
    draw_location VARCHAR(255), -- Physical location where draw was conducted
    nla_official_signature VARCHAR(255), -- NLA official who supervised the draw
    total_tickets_sold BIGINT DEFAULT 0,
    total_prize_pool BIGINT DEFAULT 0, -- in pesewas
    verification_hash VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create the draw_schedules table
CREATE TABLE draw_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL,
    draw_name VARCHAR(255) NOT NULL,
    scheduled_time TIMESTAMP WITH TIME ZONE NOT NULL,
    frequency VARCHAR(50) NOT NULL DEFAULT 'one_time' CHECK (frequency IN ('one_time', 'daily', 'weekly', 'monthly', 'custom')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by VARCHAR(255) NOT NULL,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create the draw_results table
CREATE TABLE draw_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id UUID NOT NULL REFERENCES draws(id) ON DELETE CASCADE,
    winning_numbers INTEGER[] NOT NULL,
    total_winners BIGINT NOT NULL DEFAULT 0,
    total_prize_paid BIGINT NOT NULL DEFAULT 0, -- in pesewas
    is_published BOOLEAN NOT NULL DEFAULT false,
    published_at TIMESTAMP WITH TIME ZONE,
    verification_hash VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create the prize_distributions table
CREATE TABLE prize_distributions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_result_id UUID NOT NULL REFERENCES draw_results(id) ON DELETE CASCADE,
    tier INTEGER NOT NULL,
    tier_name VARCHAR(100) NOT NULL,
    matches_required INTEGER NOT NULL,
    winners_count BIGINT NOT NULL DEFAULT 0,
    prize_per_winner BIGINT NOT NULL DEFAULT 0, -- in pesewas
    total_prize_amount BIGINT NOT NULL DEFAULT 0, -- in pesewas
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create the draw_validations table for NLA compliance tracking
CREATE TABLE draw_validations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id UUID NOT NULL REFERENCES draws(id) ON DELETE CASCADE,
    nla_reference VARCHAR(255) NOT NULL,
    draw_certificate TEXT, -- Digital certificate from NLA
    witness_signature VARCHAR(255), -- Independent witness signature
    supporting_documents TEXT[], -- Document hashes for audit trail
    validation_status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (validation_status IN ('pending', 'verified', 'rejected')),
    validated_by VARCHAR(255),
    validated_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_draws_game_id ON draws(game_id);
CREATE INDEX idx_draws_status ON draws(status);
CREATE INDEX idx_draws_scheduled_time ON draws(scheduled_time);
CREATE INDEX idx_draws_executed_time ON draws(executed_time);

CREATE INDEX idx_draw_schedules_game_id ON draw_schedules(game_id);
CREATE INDEX idx_draw_schedules_scheduled_time ON draw_schedules(scheduled_time);
CREATE INDEX idx_draw_schedules_is_active ON draw_schedules(is_active);

CREATE INDEX idx_draw_results_draw_id ON draw_results(draw_id);
CREATE INDEX idx_draw_results_is_published ON draw_results(is_published);

CREATE INDEX idx_prize_distributions_draw_result_id ON prize_distributions(draw_result_id);
CREATE INDEX idx_prize_distributions_tier ON prize_distributions(tier);

CREATE INDEX idx_draw_validations_draw_id ON draw_validations(draw_id);
CREATE INDEX idx_draw_validations_status ON draw_validations(validation_status);
CREATE INDEX idx_draw_validations_nla_reference ON draw_validations(nla_reference);

-- Create trigger to automatically update the updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column() RETURNS TRIGGER LANGUAGE plpgsql AS 'BEGIN NEW.updated_at = CURRENT_TIMESTAMP; RETURN NEW; END;';

CREATE TRIGGER update_draws_updated_at
    BEFORE UPDATE ON draws
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add constraints for business rules
ALTER TABLE draws ADD CONSTRAINT chk_execution_time_after_scheduled 
    CHECK (executed_time IS NULL OR executed_time >= scheduled_time);

ALTER TABLE draws ADD CONSTRAINT chk_winning_numbers_valid 
    CHECK (array_length(winning_numbers, 1) IS NULL OR array_length(winning_numbers, 1) BETWEEN 1 AND 10);

ALTER TABLE prize_distributions ADD CONSTRAINT chk_tier_positive 
    CHECK (tier > 0);

ALTER TABLE prize_distributions ADD CONSTRAINT chk_matches_required_positive 
    CHECK (matches_required >= 0);

ALTER TABLE prize_distributions ADD CONSTRAINT chk_winners_count_non_negative 
    CHECK (winners_count >= 0);

ALTER TABLE prize_distributions ADD CONSTRAINT chk_prize_amounts_non_negative 
    CHECK (prize_per_winner >= 0 AND total_prize_amount >= 0);

-- +goose Down
-- Drop tables in reverse order of creation
DROP TRIGGER IF EXISTS update_draws_updated_at ON draws;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS draw_validations;
DROP TABLE IF EXISTS prize_distributions;
DROP TABLE IF EXISTS draw_results;
DROP TABLE IF EXISTS draw_schedules;
DROP TABLE IF EXISTS draws;