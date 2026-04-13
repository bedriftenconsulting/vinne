-- +goose Up
-- +goose StatementBegin

-- Authentication-only tables for Agent Auth Service
-- This service handles AUTHENTICATION ONLY for agents and retailers
-- NO business logic, NO commission, NO territories, NO device management

-- Create agents authentication table (basic auth data only)
CREATE TABLE IF NOT EXISTS agents_auth (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_code VARCHAR(20) UNIQUE NOT NULL, -- AGT-YYYY-XXXXXX format
    phone VARCHAR(20) UNIQUE NOT NULL, -- Primary login method
    email VARCHAR(255) UNIQUE, -- Optional secondary contact
    password_hash TEXT NOT NULL,
    
    -- Authentication status
    is_active BOOLEAN DEFAULT true,
    failed_login_attempts INT DEFAULT 0,
    locked_until TIMESTAMP,
    
    -- Timestamps
    last_login_at TIMESTAMP,
    password_changed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create retailers authentication table (basic auth data only)
CREATE TABLE IF NOT EXISTS retailers_auth (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_code VARCHAR(20) UNIQUE NOT NULL, -- RTL-YYYY-XXXXXXX format
    email VARCHAR(255) UNIQUE, -- Optional for retailers
    phone VARCHAR(20) UNIQUE, -- Optional for retailers
    password_hash TEXT NOT NULL,
    pin_hash TEXT, -- For POS authentication
    
    -- Authentication status
    is_active BOOLEAN DEFAULT true,
    failed_login_attempts INT DEFAULT 0,
    locked_until TIMESTAMP,
    
    -- Timestamps
    last_login_at TIMESTAMP,
    password_changed_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create sessions table
CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    user_type VARCHAR(20) NOT NULL CHECK (user_type IN ('AGENT', 'RETAILER')),
    refresh_token TEXT UNIQUE NOT NULL,
    
    -- Session details
    user_agent TEXT,
    ip_address INET,
    device_id VARCHAR(255), -- For POS/Mobile tracking only
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP NOT NULL,
    last_activity TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create password reset tokens table
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    user_type VARCHAR(20) NOT NULL CHECK (user_type IN ('AGENT', 'RETAILER')),
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create offline authentication tokens table (for POS devices)
CREATE TABLE IF NOT EXISTS offline_auth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    user_type VARCHAR(20) NOT NULL CHECK (user_type IN ('AGENT', 'RETAILER')),
    token TEXT UNIQUE NOT NULL,
    device_imei VARCHAR(50), -- Device identifier only
    
    -- Token validity
    valid_from TIMESTAMP NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMP NOT NULL,
    revoked BOOLEAN DEFAULT false,
    revoked_at TIMESTAMP,
    revoked_reason TEXT,
    
    -- Usage tracking
    last_used_at TIMESTAMP,
    use_count INT DEFAULT 0,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create authentication audit logs table
CREATE TABLE IF NOT EXISTS auth_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID,
    user_type VARCHAR(20),
    action VARCHAR(50) NOT NULL, -- LOGIN, LOGOUT, FAILED_LOGIN, PASSWORD_CHANGE, etc.
    
    -- Details
    ip_address INET,
    user_agent TEXT,
    device_id VARCHAR(255),
    
    -- Result
    success BOOLEAN NOT NULL,
    error_message TEXT,
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for agents_auth
CREATE INDEX idx_agents_auth_phone ON agents_auth(phone);
CREATE INDEX idx_agents_auth_email ON agents_auth(email) WHERE email IS NOT NULL;
CREATE INDEX idx_agents_auth_code ON agents_auth(agent_code);
CREATE INDEX idx_agents_auth_active ON agents_auth(is_active);

-- Create indexes for retailers_auth
CREATE INDEX idx_retailers_auth_email ON retailers_auth(email) WHERE email IS NOT NULL;
CREATE INDEX idx_retailers_auth_phone ON retailers_auth(phone) WHERE phone IS NOT NULL;
CREATE INDEX idx_retailers_auth_code ON retailers_auth(retailer_code);
CREATE INDEX idx_retailers_auth_active ON retailers_auth(is_active);

-- Create indexes for sessions
CREATE INDEX idx_sessions_user ON auth_sessions(user_id, user_type);
CREATE INDEX idx_sessions_token ON auth_sessions(refresh_token);
CREATE INDEX idx_sessions_active ON auth_sessions(is_active, expires_at);

-- Create indexes for password reset tokens
CREATE INDEX idx_reset_tokens_user ON password_reset_tokens(user_id, user_type);
CREATE INDEX idx_reset_tokens_token ON password_reset_tokens(token);

-- Create indexes for offline tokens
CREATE INDEX idx_offline_tokens_user ON offline_auth_tokens(user_id, user_type);
CREATE INDEX idx_offline_tokens_device ON offline_auth_tokens(device_imei);
CREATE INDEX idx_offline_tokens_validity ON offline_auth_tokens(valid_until, revoked);

-- Create indexes for audit logs
CREATE INDEX idx_audit_logs_user ON auth_audit_logs(user_id, user_type);
CREATE INDEX idx_audit_logs_action ON auth_audit_logs(action, created_at);
CREATE INDEX idx_audit_logs_created ON auth_audit_logs(created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop all authentication tables and indexes
DROP TABLE IF EXISTS auth_audit_logs CASCADE;
DROP TABLE IF EXISTS offline_auth_tokens CASCADE;
DROP TABLE IF EXISTS password_reset_tokens CASCADE;
DROP TABLE IF EXISTS auth_sessions CASCADE;
DROP TABLE IF EXISTS retailers_auth CASCADE;
DROP TABLE IF EXISTS agents_auth CASCADE;

-- +goose StatementEnd