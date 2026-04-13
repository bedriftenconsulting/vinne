-- Migration 1: Create players table
CREATE TABLE IF NOT EXISTS players (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number VARCHAR(15) UNIQUE NOT NULL,
    email VARCHAR(255),
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    date_of_birth DATE,
    national_id VARCHAR(50),
    mobile_money_phone VARCHAR(15),
    status VARCHAR(20) DEFAULT 'ACTIVE',
    email_verified BOOLEAN DEFAULT FALSE,
    phone_verified BOOLEAN DEFAULT FALSE,
    registration_channel VARCHAR(20),
    terms_accepted BOOLEAN DEFAULT FALSE,
    marketing_consent BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    last_login_at TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_players_phone ON players (phone_number);
CREATE INDEX IF NOT EXISTS idx_players_email ON players (email);
CREATE INDEX IF NOT EXISTS idx_players_status ON players (status);
CREATE INDEX IF NOT EXISTS idx_players_created ON players (created_at);

CREATE TABLE IF NOT EXISTS player_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID NOT NULL REFERENCES players (id),
    device_id VARCHAR(255) NOT NULL,
    refresh_token VARCHAR(512) UNIQUE NOT NULL,
    access_token_jti VARCHAR(255),
    channel VARCHAR(20) NOT NULL,
    device_type VARCHAR(50),
    app_version VARCHAR(50),
    ip_address INET,
    user_agent TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    expires_at TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP NOT NULL DEFAULT now(),
    revoked_at TIMESTAMP,
    revoked_reason VARCHAR(255)
);

CREATE INDEX IF NOT EXISTS idx_sessions_player ON player_sessions (player_id);
CREATE INDEX IF NOT EXISTS idx_sessions_device ON player_sessions (device_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON player_sessions (refresh_token);
CREATE INDEX IF NOT EXISTS idx_sessions_active ON player_sessions (is_active, expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_channel ON player_sessions (channel, created_at DESC);

CREATE TABLE IF NOT EXISTS player_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID NOT NULL REFERENCES players (id),
    device_id VARCHAR(255) NOT NULL,
    device_type VARCHAR(50),
    device_name VARCHAR(255),
    os VARCHAR(50),
    os_version VARCHAR(50),
    app_version VARCHAR(50),
    push_token VARCHAR(512),
    fingerprint VARCHAR(512),
    is_trusted BOOLEAN DEFAULT FALSE,
    is_blocked BOOLEAN DEFAULT FALSE,
    first_seen_at TIMESTAMP NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMP NOT NULL DEFAULT now(),
    trust_score INTEGER DEFAULT 50,
    UNIQUE (player_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_devices_player ON player_devices (player_id);
CREATE INDEX IF NOT EXISTS idx_devices_fingerprint ON player_devices (fingerprint);
CREATE INDEX IF NOT EXISTS idx_devices_trusted ON player_devices (is_trusted);

CREATE TABLE IF NOT EXISTS player_wallets (
    player_id UUID PRIMARY KEY REFERENCES players (id),
    wallet_id UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS player_login_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number VARCHAR(15) NOT NULL,
    player_id UUID,
    device_id VARCHAR(255),
    channel VARCHAR(20) NOT NULL,
    ip_address INET,
    attempt_type VARCHAR(20),
    success BOOLEAN NOT NULL,
    failure_reason VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_login_phone ON player_login_attempts (phone_number, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_player ON player_login_attempts (player_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_ip ON player_login_attempts (ip_address);
CREATE INDEX IF NOT EXISTS idx_login_channel ON player_login_attempts (channel, created_at DESC);

CREATE TABLE IF NOT EXISTS player_channel_analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID NOT NULL REFERENCES players (id),
    channel VARCHAR(20) NOT NULL,
    login_count INTEGER DEFAULT 0,
    last_login_at TIMESTAMP,
    total_session_duration BIGINT DEFAULT 0,
    device_types TEXT [],
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (player_id, channel)
);

CREATE INDEX IF NOT EXISTS idx_channel_analytics_player ON player_channel_analytics (player_id);
CREATE INDEX IF NOT EXISTS idx_channel_analytics_channel ON player_channel_analytics (channel);

CREATE TABLE IF NOT EXISTS player_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID NOT NULL REFERENCES players (id),
    action VARCHAR(100) NOT NULL,
    channel VARCHAR(20),
    performed_by UUID,
    ip_address INET,
    user_agent TEXT,
    old_value JSONB,
    new_value JSONB,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_player ON player_audit_log (player_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON player_audit_log (action);
CREATE INDEX IF NOT EXISTS idx_audit_channel ON player_audit_log (channel);

-- Migration 2: Add OTPs table
CREATE TABLE IF NOT EXISTS otps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number VARCHAR(20) NOT NULL,
    code VARCHAR(10) NOT NULL,
    purpose VARCHAR(50) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    used_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_otps_phone_purpose ON otps (phone_number, purpose);
CREATE INDEX IF NOT EXISTS idx_otps_code ON otps (code);
