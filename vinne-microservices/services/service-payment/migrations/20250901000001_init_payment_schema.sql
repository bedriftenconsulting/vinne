-- +goose Up
-- +goose StatementBegin

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Transaction status enum
CREATE TYPE transaction_status AS ENUM (
    'PENDING',
    'PROCESSING',
    'SUCCESS',
    'FAILED',
    'VERIFYING',
    'DUPLICATE'
);

-- Transaction type enum
CREATE TYPE transaction_type AS ENUM (
    'DEPOSIT',           -- Mobile Money -> Stake Wallet
    'WITHDRAWAL',        -- Winning Wallet -> Mobile Money
    'BANK_TRANSFER'      -- Wallet -> Bank Account
);

-- Saga status enum
CREATE TYPE saga_status AS ENUM (
    'STARTED',
    'PAYMENT_RESERVED',
    'WALLET_DEBITED',
    'PROVIDER_PROCESSING',
    'COMPLETED',
    'COMPENSATING',
    'COMPENSATED',
    'FAILED'
);

-- ============================================================
-- Transactions Table
-- Stores all payment transactions
-- ============================================================
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Transaction identification
    reference VARCHAR(100) UNIQUE NOT NULL,
    provider_transaction_id VARCHAR(255),

    -- Transaction details
    type transaction_type NOT NULL,
    status transaction_status NOT NULL DEFAULT 'PENDING',
    amount BIGINT NOT NULL CHECK (amount > 0),  -- Amount in pesewas
    currency VARCHAR(3) NOT NULL DEFAULT 'GHS',
    narration TEXT,

    -- Provider information
    provider_name VARCHAR(50) NOT NULL,  -- Orange, MTN, Telecel, etc.

    -- Source information (for debits)
    source_type VARCHAR(50),             -- WALLET, BANK_ACCOUNT
    source_identifier VARCHAR(100),      -- Wallet number or account number
    source_name VARCHAR(255),            -- Account holder name

    -- Destination information (for credits)
    destination_type VARCHAR(50),        -- WALLET, BANK_ACCOUNT
    destination_identifier VARCHAR(100), -- Wallet number or account number
    destination_name VARCHAR(255),       -- Beneficiary name

    -- User information
    user_id UUID NOT NULL,               -- Reference to user service
    customer_remarks TEXT,

    -- Metadata
    metadata JSONB,
    provider_data JSONB,

    -- Error tracking
    error_message TEXT,
    error_code VARCHAR(50),
    retry_count INT DEFAULT 0,
    last_retry_at TIMESTAMP WITH TIME ZONE,

    -- Timestamps
    requested_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for transactions table
CREATE INDEX idx_transactions_reference ON transactions(reference);
CREATE INDEX idx_transactions_provider_tx_id ON transactions(provider_transaction_id);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_status ON transactions(status);
CREATE INDEX idx_transactions_type ON transactions(type);
CREATE INDEX idx_transactions_created_at ON transactions(created_at DESC);
CREATE INDEX idx_transactions_status_created ON transactions(status, created_at DESC);

-- ============================================================
-- Sagas Table
-- Tracks distributed transaction coordination
-- ============================================================
CREATE TABLE sagas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Saga identification
    saga_id VARCHAR(100) UNIQUE NOT NULL,
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,

    -- Saga state
    status saga_status NOT NULL DEFAULT 'STARTED',
    current_step INT NOT NULL DEFAULT 0,
    total_steps INT NOT NULL,

    -- Saga data
    saga_data JSONB NOT NULL,           -- Stores saga state and context
    compensation_data JSONB,            -- Data needed for compensation

    -- Error tracking
    error_message TEXT,
    retry_count INT DEFAULT 0,
    max_retries INT DEFAULT 3,

    -- Timestamps
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    last_updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for sagas table
CREATE INDEX idx_sagas_saga_id ON sagas(saga_id);
CREATE INDEX idx_sagas_transaction_id ON sagas(transaction_id);
CREATE INDEX idx_sagas_status ON sagas(status);
CREATE INDEX idx_sagas_created_at ON sagas(created_at DESC);

-- ============================================================
-- Saga Steps Table
-- Tracks individual steps within a saga
-- ============================================================
CREATE TABLE saga_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Saga reference
    saga_id UUID NOT NULL REFERENCES sagas(id) ON DELETE CASCADE,

    -- Step information
    step_number INT NOT NULL,
    step_name VARCHAR(100) NOT NULL,
    step_type VARCHAR(50) NOT NULL,     -- FORWARD, COMPENSATION
    status VARCHAR(50) NOT NULL,        -- PENDING, PROCESSING, COMPLETED, FAILED

    -- Step data
    input_data JSONB,
    output_data JSONB,
    error_message TEXT,

    -- Timestamps
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Constraints
    UNIQUE (saga_id, step_number, step_type)
);

-- Create indexes for saga_steps table
CREATE INDEX idx_saga_steps_saga_id ON saga_steps(saga_id);
CREATE INDEX idx_saga_steps_status ON saga_steps(status);

-- ============================================================
-- Idempotency Records Table
-- Prevents duplicate transactions (Level 2 idempotency)
-- ============================================================
CREATE TABLE idempotency_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Idempotency key (client reference)
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,

    -- Request details
    request_hash VARCHAR(64) NOT NULL,  -- SHA-256 hash of request body
    endpoint VARCHAR(255) NOT NULL,
    http_method VARCHAR(10) NOT NULL,

    -- Response details
    status_code INT,
    response_body JSONB,

    -- Transaction reference
    transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL,

    -- Lock management
    is_locked BOOLEAN NOT NULL DEFAULT false,
    locked_at TIMESTAMP WITH TIME ZONE,
    lock_expires_at TIMESTAMP WITH TIME ZONE,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL  -- TTL for cleanup
);

-- Create indexes for idempotency_records table
CREATE INDEX idx_idempotency_key ON idempotency_records(idempotency_key);
CREATE INDEX idx_idempotency_expires_at ON idempotency_records(expires_at);
CREATE INDEX idx_idempotency_locked ON idempotency_records(is_locked, lock_expires_at);

-- ============================================================
-- Provider Configurations Table
-- Stores provider-specific settings and credentials
-- ============================================================
CREATE TABLE provider_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Provider identification
    provider_name VARCHAR(50) UNIQUE NOT NULL,
    provider_type VARCHAR(50) NOT NULL,  -- AGGREGATOR, MOBILE_MONEY, BANK_TRANSFER

    -- Configuration
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    is_test_mode BOOLEAN NOT NULL DEFAULT false,
    priority INT NOT NULL DEFAULT 0,     -- For provider selection

    -- Credentials (encrypted in production)
    credentials JSONB NOT NULL,

    -- Limits
    min_amount BIGINT NOT NULL,
    max_amount BIGINT NOT NULL,
    daily_limit BIGINT,
    monthly_limit BIGINT,

    -- API configuration
    base_url VARCHAR(255) NOT NULL,
    timeout_seconds INT NOT NULL DEFAULT 30,
    retry_attempts INT NOT NULL DEFAULT 3,
    retry_delay_seconds INT NOT NULL DEFAULT 2,

    -- Circuit breaker configuration
    circuit_breaker_threshold INT NOT NULL DEFAULT 5,
    circuit_breaker_timeout_seconds INT NOT NULL DEFAULT 60,

    -- Supported operations
    supported_operations TEXT[] NOT NULL,
    supported_currencies TEXT[] NOT NULL DEFAULT ARRAY['GHS'],

    -- Metadata
    metadata JSONB,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for provider_configurations table
CREATE INDEX idx_provider_name ON provider_configurations(provider_name);
CREATE INDEX idx_provider_enabled ON provider_configurations(is_enabled);
CREATE INDEX idx_provider_priority ON provider_configurations(priority DESC);

-- ============================================================
-- Provider Health Checks Table
-- Tracks provider availability and health
-- ============================================================
CREATE TABLE provider_health_checks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Provider reference
    provider_name VARCHAR(50) NOT NULL,

    -- Health check results
    is_healthy BOOLEAN NOT NULL,
    response_time_ms INT,
    error_message TEXT,

    -- Circuit breaker state
    circuit_state VARCHAR(20),  -- CLOSED, OPEN, HALF_OPEN
    failure_count INT DEFAULT 0,

    -- Timestamps
    checked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for provider_health_checks table
CREATE INDEX idx_health_provider_name ON provider_health_checks(provider_name);
CREATE INDEX idx_health_checked_at ON provider_health_checks(checked_at DESC);

-- ============================================================
-- Webhook Events Table
-- Stores incoming webhook events from providers
-- ============================================================
CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Provider information
    provider_name VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,

    -- Event data
    payload JSONB NOT NULL,
    headers JSONB,

    -- Processing status
    is_processed BOOLEAN NOT NULL DEFAULT false,
    processed_at TIMESTAMP WITH TIME ZONE,
    processing_error TEXT,

    -- Transaction reference
    transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL,

    -- Timestamps
    received_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for webhook_events table
CREATE INDEX idx_webhook_provider ON webhook_events(provider_name);
CREATE INDEX idx_webhook_processed ON webhook_events(is_processed);
CREATE INDEX idx_webhook_received_at ON webhook_events(received_at DESC);
CREATE INDEX idx_webhook_transaction_id ON webhook_events(transaction_id);

-- ============================================================
-- Audit Log Table
-- Tracks all state changes for compliance
-- ============================================================
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Entity reference
    entity_type VARCHAR(50) NOT NULL,   -- TRANSACTION, SAGA, PROVIDER
    entity_id UUID NOT NULL,

    -- Action details
    action VARCHAR(100) NOT NULL,       -- CREATED, UPDATED, STATUS_CHANGED, etc.
    actor_id UUID,                      -- User or service that performed action
    actor_type VARCHAR(50),             -- USER, SERVICE, SYSTEM

    -- Change tracking
    old_state JSONB,
    new_state JSONB,
    changes JSONB,                      -- Specific field changes

    -- Context
    metadata JSONB,
    ip_address INET,
    user_agent TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for audit_logs table
CREATE INDEX idx_audit_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX idx_audit_actor ON audit_logs(actor_id);
CREATE INDEX idx_audit_created_at ON audit_logs(created_at DESC);

-- ============================================================
-- Functions and Triggers
-- ============================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply updated_at trigger to relevant tables
CREATE TRIGGER update_transactions_updated_at BEFORE UPDATE ON transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sagas_updated_at BEFORE UPDATE ON sagas
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_idempotency_updated_at BEFORE UPDATE ON idempotency_records
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_provider_config_updated_at BEFORE UPDATE ON provider_configurations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to cleanup expired idempotency records
CREATE OR REPLACE FUNCTION cleanup_expired_idempotency_records()
RETURNS void AS $$
BEGIN
    DELETE FROM idempotency_records
    WHERE expires_at < CURRENT_TIMESTAMP;
END;
$$ LANGUAGE plpgsql;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop triggers
DROP TRIGGER IF EXISTS update_transactions_updated_at ON transactions;
DROP TRIGGER IF EXISTS update_sagas_updated_at ON sagas;
DROP TRIGGER IF EXISTS update_idempotency_updated_at ON idempotency_records;
DROP TRIGGER IF EXISTS update_provider_config_updated_at ON provider_configurations;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS cleanup_expired_idempotency_records();

-- Drop tables (in reverse order of dependencies)
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS provider_health_checks;
DROP TABLE IF EXISTS provider_configurations;
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS saga_steps;
DROP TABLE IF EXISTS sagas;
DROP TABLE IF EXISTS transactions;

-- Drop enums
DROP TYPE IF EXISTS saga_status;
DROP TYPE IF EXISTS transaction_type;
DROP TYPE IF EXISTS transaction_status;

-- Note: Not dropping uuid-ossp extension as it's a shared extension
-- and may be used by other databases. The extension can only be dropped
-- by the database superuser.

-- +goose StatementEnd
