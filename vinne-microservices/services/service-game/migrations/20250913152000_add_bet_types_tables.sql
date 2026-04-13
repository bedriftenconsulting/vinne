-- +goose Up
-- Migration to add bet types tables for format-based betting system

-- Create bet types table (global catalog of available bet types)
CREATE TABLE bet_types (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    base_multiplier DECIMAL(10,2) NOT NULL DEFAULT 1.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create game bet types junction table (bet types enabled for specific games with custom multipliers)
CREATE TABLE game_bet_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    bet_type_id VARCHAR(255) NOT NULL REFERENCES bet_types(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    multiplier DECIMAL(10,2) NOT NULL DEFAULT 1.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(game_id, bet_type_id)
);

-- Create indexes for performance
CREATE INDEX idx_game_bet_types_game_id ON game_bet_types(game_id);
CREATE INDEX idx_game_bet_types_bet_type_id ON game_bet_types(bet_type_id);
CREATE INDEX idx_game_bet_types_enabled ON game_bet_types(enabled);

-- Insert standard Ghanaian lottery bet types
INSERT INTO bet_types (id, name, description, base_multiplier) VALUES
-- Direct bet types
('direct_1', '1 Direct', 'Pick 1 number correctly', 80.00),
('direct_2', '2 Direct', 'Pick 2 numbers correctly in exact order', 750.00),
('direct_3', '3 Direct', 'Pick 3 numbers correctly in exact order', 4500.00),
('direct_4', '4 Direct', 'Pick 4 numbers correctly in exact order', 125000.00),
('direct_5', '5 Direct', 'Pick 5 numbers correctly in exact order', 7500000.00),
('direct_6', '6 Direct', 'Pick 6 numbers correctly in exact order', 25000000.00),

-- Permutation bet types
('perm_2', 'Perm 2', 'Pick 2 numbers correctly in any order', 300.00),
('perm_3', 'Perm 3', 'Pick 3 numbers correctly in any order', 1800.00),
('perm_4', 'Perm 4', 'Pick 4 numbers correctly in any order', 20000.00),
('perm_5', 'Perm 5', 'Pick 5 numbers correctly in any order', 1250000.00),
('perm_6', 'Perm 6', 'Pick 6 numbers correctly in any order', 4166666.67),

-- Banker bet types
('banker_against_all', 'Banker Against All', 'One number must be in winning numbers, pick additional numbers', 50.00),
('banker_with_2', 'Banker with 2', 'One banker number plus 2 other numbers', 150.00),
('banker_with_3', 'Banker with 3', 'One banker number plus 3 other numbers', 900.00),

-- Special bet types
('lucky_pick', 'Lucky Pick', 'System generated numbers', 80.00),
('super_6', 'Super 6', 'Pick 6 numbers with bonus multiplier', 50000000.00),
('forecaster', 'Forecaster', 'Predict first two numbers in exact order', 1000.00);

-- +goose Down
-- Remove bet types tables and data
DROP TABLE IF EXISTS game_bet_types;
DROP TABLE IF EXISTS bet_types;