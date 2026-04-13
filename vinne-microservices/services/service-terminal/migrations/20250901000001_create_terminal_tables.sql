-- +goose Up
-- +goose StatementBegin
-- Terminals table
CREATE TABLE IF NOT EXISTS terminals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id VARCHAR(20) UNIQUE NOT NULL, -- POS-YYYY-XXXXXX format
    name VARCHAR(255) NOT NULL,
    model VARCHAR(50) NOT NULL, -- ANDROID_POS_V1, ANDROID_POS_V2, WEB_TERMINAL, MOBILE_TERMINAL
    serial_number VARCHAR(100) UNIQUE,
    imei VARCHAR(20) UNIQUE,
    android_version VARCHAR(50),
    app_version VARCHAR(50),
    vendor VARCHAR(100),
    purchase_date TIMESTAMP,
    status VARCHAR(50) NOT NULL DEFAULT 'INACTIVE', -- ACTIVE, INACTIVE, FAULTY, MAINTENANCE, SUSPENDED, DECOMMISSIONED
    retailer_id UUID,
    assignment_date TIMESTAMP,
    last_sync TIMESTAMP,
    last_transaction TIMESTAMP,
    health_status VARCHAR(50) DEFAULT 'OFFLINE', -- HEALTHY, WARNING, CRITICAL, OFFLINE
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_terminals_device_id ON terminals(device_id);
CREATE INDEX idx_terminals_status ON terminals(status);
CREATE INDEX idx_terminals_retailer_id ON terminals(retailer_id);
CREATE INDEX idx_terminals_health_status ON terminals(health_status);
CREATE INDEX idx_terminals_model ON terminals(model);
CREATE INDEX idx_terminals_last_sync ON terminals(last_sync);

-- Terminal assignments table (history of all assignments)
CREATE TABLE IF NOT EXISTS terminal_assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    terminal_id UUID NOT NULL REFERENCES terminals(id),
    retailer_id UUID NOT NULL,
    assigned_by UUID NOT NULL,
    assigned_at TIMESTAMP NOT NULL DEFAULT NOW(),
    unassigned_at TIMESTAMP,
    is_active BOOLEAN NOT NULL DEFAULT true,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_terminal_assignments_terminal_id ON terminal_assignments(terminal_id);
CREATE INDEX idx_terminal_assignments_retailer_id ON terminal_assignments(retailer_id);
CREATE INDEX idx_terminal_assignments_is_active ON terminal_assignments(is_active);
CREATE INDEX idx_terminal_assignments_assigned_at ON terminal_assignments(assigned_at);

-- Ensure only one active assignment per terminal
CREATE UNIQUE INDEX idx_terminal_assignments_active_terminal ON terminal_assignments(terminal_id) 
WHERE is_active = true;

-- Ensure only one active terminal per retailer
CREATE UNIQUE INDEX idx_terminal_assignments_active_retailer ON terminal_assignments(retailer_id) 
WHERE is_active = true;

-- Terminal versions table (for tracking app/firmware versions)
CREATE TABLE IF NOT EXISTS terminal_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    terminal_id UUID NOT NULL REFERENCES terminals(id),
    version_type VARCHAR(50) NOT NULL, -- APP, FIRMWARE, CONFIG
    version_number VARCHAR(50) NOT NULL,
    previous_version VARCHAR(50),
    updated_by VARCHAR(100),
    update_method VARCHAR(50), -- OTA, MANUAL, AUTO
    update_status VARCHAR(50), -- SUCCESS, FAILED, PENDING
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_terminal_versions_terminal_id ON terminal_versions(terminal_id);
CREATE INDEX idx_terminal_versions_version_type ON terminal_versions(version_type);
CREATE INDEX idx_terminal_versions_created_at ON terminal_versions(created_at);

-- Terminal configurations table
CREATE TABLE IF NOT EXISTS terminal_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    terminal_id UUID UNIQUE NOT NULL REFERENCES terminals(id),
    transaction_limit INTEGER DEFAULT 10000, -- max per transaction
    daily_limit INTEGER DEFAULT 100000, -- max per day
    offline_mode_enabled BOOLEAN DEFAULT true,
    offline_sync_interval INTEGER DEFAULT 30, -- minutes
    auto_update_enabled BOOLEAN DEFAULT true,
    minimum_app_version VARCHAR(50),
    settings JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_terminal_configs_terminal_id ON terminal_configs(terminal_id);

-- Terminal health table (latest health status)
CREATE TABLE IF NOT EXISTS terminal_health (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    terminal_id UUID UNIQUE NOT NULL REFERENCES terminals(id),
    status VARCHAR(50) NOT NULL DEFAULT 'OFFLINE', -- HEALTHY, WARNING, CRITICAL, OFFLINE
    battery_level INTEGER,
    signal_strength INTEGER,
    storage_available BIGINT, -- bytes
    storage_total BIGINT, -- bytes
    memory_usage INTEGER, -- percentage
    cpu_usage INTEGER, -- percentage
    last_heartbeat TIMESTAMP,
    diagnostics JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_terminal_health_terminal_id ON terminal_health(terminal_id);
CREATE INDEX idx_terminal_health_status ON terminal_health(status);
CREATE INDEX idx_terminal_health_last_heartbeat ON terminal_health(last_heartbeat);

-- Terminal health history table (for tracking health over time)
CREATE TABLE IF NOT EXISTS terminal_health_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    terminal_id UUID NOT NULL REFERENCES terminals(id),
    status VARCHAR(50) NOT NULL,
    battery_level INTEGER,
    signal_strength INTEGER,
    storage_available BIGINT,
    storage_total BIGINT,
    memory_usage INTEGER,
    cpu_usage INTEGER,
    diagnostics JSONB,
    recorded_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_terminal_health_history_terminal_id ON terminal_health_history(terminal_id);
CREATE INDEX idx_terminal_health_history_recorded_at ON terminal_health_history(recorded_at);
CREATE INDEX idx_terminal_health_history_status ON terminal_health_history(status);

-- Terminal audit log table
CREATE TABLE IF NOT EXISTS terminal_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    terminal_id UUID NOT NULL REFERENCES terminals(id),
    action VARCHAR(100) NOT NULL,
    action_by UUID NOT NULL,
    old_value JSONB,
    new_value JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_terminal_audit_logs_terminal_id ON terminal_audit_logs(terminal_id);
CREATE INDEX idx_terminal_audit_logs_action ON terminal_audit_logs(action);
CREATE INDEX idx_terminal_audit_logs_action_by ON terminal_audit_logs(action_by);
CREATE INDEX idx_terminal_audit_logs_created_at ON terminal_audit_logs(created_at);

-- Create update trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column() RETURNS TRIGGER LANGUAGE plpgsql AS 'BEGIN NEW.updated_at = NOW(); RETURN NEW; END;';

-- Create triggers for updated_at columns
CREATE TRIGGER update_terminals_updated_at BEFORE UPDATE ON terminals
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_terminal_configs_updated_at BEFORE UPDATE ON terminal_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_terminal_health_updated_at BEFORE UPDATE ON terminal_health
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_terminal_health_updated_at ON terminal_health;
DROP TRIGGER IF EXISTS update_terminal_configs_updated_at ON terminal_configs;
DROP TRIGGER IF EXISTS update_terminals_updated_at ON terminals;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS terminal_audit_logs CASCADE;
DROP TABLE IF EXISTS terminal_health_history CASCADE;
DROP TABLE IF EXISTS terminal_health CASCADE;
DROP TABLE IF EXISTS terminal_configs CASCADE;
DROP TABLE IF EXISTS terminal_versions CASCADE;
DROP TABLE IF EXISTS terminal_assignments CASCADE;
DROP TABLE IF EXISTS terminals CASCADE;
-- +goose StatementEnd