-- +goose Up
-- +goose StatementBegin

-- Agent Management Tables for RANDCO Platform
-- Handles agents, retailers, their relationships, and POS device management


-- Create commission tiers table
CREATE TABLE IF NOT EXISTS commission_tiers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    base_commission_rate DECIMAL(5,4) NOT NULL, -- e.g., 0.30 for 30%
    bonus_commission_rate DECIMAL(5,4) DEFAULT 0,
    sales_threshold DECIMAL(15,2) DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create agents table
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_code VARCHAR(20) UNIQUE NOT NULL, -- AGT-YYYY-XXXXXX format
    business_name VARCHAR(255) NOT NULL,
    registration_number VARCHAR(100),
    tax_id VARCHAR(100),
    
    -- Contact Information
    contact_email VARCHAR(255) UNIQUE NOT NULL,
    contact_phone VARCHAR(20) NOT NULL,
    primary_contact_name VARCHAR(255),
    
    -- Location Information
    physical_address TEXT,
    city VARCHAR(100),
    region VARCHAR(100),
    gps_coordinates POINT,
    
    -- Banking Details
    bank_name VARCHAR(255),
    bank_account_number VARCHAR(50),
    bank_account_name VARCHAR(255),
    
    -- Status and Classification
    status VARCHAR(50) DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED', 'UNDER_REVIEW', 'INACTIVE', 'TERMINATED')),
    onboarding_method VARCHAR(50) DEFAULT 'RAND_LOTTERY_LTD_DIRECT' CHECK (onboarding_method IN ('RAND_LOTTERY_LTD_DIRECT', 'REFERRAL')),
    
    -- Relationships
    commission_tier_id UUID REFERENCES commission_tiers(id),
    
    -- Metadata
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create retailers table
CREATE TABLE IF NOT EXISTS retailers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_code VARCHAR(20) UNIQUE NOT NULL, -- RTL-YYYY-XXXXXXX format
    business_name VARCHAR(255) NOT NULL,
    owner_name VARCHAR(255) NOT NULL,
    
    -- Contact Information
    contact_email VARCHAR(255), -- Optional for retailers
    contact_phone VARCHAR(20) NOT NULL,
    
    -- Location Information
    physical_address TEXT NOT NULL,
    city VARCHAR(100),
    region VARCHAR(100),
    gps_coordinates POINT,
    
    -- Business Details
    business_license VARCHAR(100),
    shop_type VARCHAR(100),
    
    -- Status and Classification
    status VARCHAR(50) DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED', 'UNDER_REVIEW', 'INACTIVE', 'TERMINATED')),
    onboarding_method VARCHAR(50) NOT NULL CHECK (onboarding_method IN ('RAND_LOTTERY_LTD_DIRECT', 'AGENT_ONBOARDED')),
    
    -- Relationships (parent_agent_id is NULL for independent retailers)
    parent_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    
    -- Metadata
    created_by VARCHAR(255) NOT NULL, -- Agent ID or RANDCO admin
    updated_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create agent_retailers relationship table (for explicit relationship tracking)
CREATE TABLE IF NOT EXISTS agent_retailers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    retailer_id UUID NOT NULL REFERENCES retailers(id) ON DELETE CASCADE,
    relationship_type VARCHAR(50) DEFAULT 'MANAGED' CHECK (relationship_type IN ('MANAGED', 'SUPERVISED')),
    assigned_date TIMESTAMP NOT NULL DEFAULT NOW(),
    unassigned_date TIMESTAMP,
    is_active BOOLEAN DEFAULT true,
    assigned_by VARCHAR(255) NOT NULL,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    -- Ensure unique active relationships
    UNIQUE(agent_id, retailer_id, is_active)
);

-- Create POS devices table
CREATE TABLE IF NOT EXISTS pos_devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_code VARCHAR(20) UNIQUE NOT NULL, -- POS-YYYY-XXXXXX format
    imei VARCHAR(50) UNIQUE NOT NULL,
    serial_number VARCHAR(100),
    model VARCHAR(100),
    manufacturer VARCHAR(100),
    
    -- Assignment Information
    assigned_retailer_id UUID REFERENCES retailers(id) ON DELETE SET NULL,
    assignment_date TIMESTAMP,
    last_sync TIMESTAMP,
    last_transaction TIMESTAMP,
    
    -- Device Status
    status VARCHAR(50) DEFAULT 'AVAILABLE' CHECK (status IN ('AVAILABLE', 'ASSIGNED', 'ACTIVE', 'INACTIVE', 'FAULTY', 'DECOMMISSIONED')),
    software_version VARCHAR(50),
    
    -- Network Information
    network_operator VARCHAR(100),
    sim_card_number VARCHAR(50),
    
    -- Metadata
    assigned_by VARCHAR(255),
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create agent KYC table
CREATE TABLE IF NOT EXISTS agent_kyc (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    
    -- KYC Status
    kyc_status VARCHAR(50) DEFAULT 'PENDING' CHECK (kyc_status IN ('PENDING', 'SUBMITTED', 'UNDER_REVIEW', 'APPROVED', 'REJECTED', 'EXPIRED')),
    
    -- Document Information
    business_registration_cert VARCHAR(255), -- File path/URL
    tax_clearance_cert VARCHAR(255),
    director_id_document VARCHAR(255),
    proof_of_address VARCHAR(255),
    bank_account_verification VARCHAR(255),
    
    -- Review Information
    reviewed_by VARCHAR(255),
    reviewed_at TIMESTAMP,
    rejection_reason TEXT,
    notes TEXT,
    
    -- Expiry Information
    expires_at TIMESTAMP,
    renewal_reminder_sent BOOLEAN DEFAULT false,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create retailer KYC table
CREATE TABLE IF NOT EXISTS retailer_kyc (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_id UUID NOT NULL REFERENCES retailers(id) ON DELETE CASCADE,
    
    -- KYC Status
    kyc_status VARCHAR(50) DEFAULT 'PENDING' CHECK (kyc_status IN ('PENDING', 'SUBMITTED', 'UNDER_REVIEW', 'APPROVED', 'REJECTED', 'EXPIRED')),
    
    -- Document Information
    business_license VARCHAR(255), -- File path/URL
    owner_id_document VARCHAR(255),
    proof_of_address VARCHAR(255),
    shop_photos VARCHAR(255)[],
    
    -- Review Information
    reviewed_by VARCHAR(255),
    reviewed_at TIMESTAMP,
    rejection_reason TEXT,
    notes TEXT,
    
    -- Expiry Information
    expires_at TIMESTAMP,
    renewal_reminder_sent BOOLEAN DEFAULT false,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create performance tracking table for agents
CREATE TABLE IF NOT EXISTS agent_performance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    
    -- Performance Metrics (monthly aggregates)
    period_year INT NOT NULL,
    period_month INT NOT NULL,
    
    total_retailers_active INT DEFAULT 0,
    total_retailers_inactive INT DEFAULT 0,
    total_sales_amount DECIMAL(15,2) DEFAULT 0,
    total_commission_earned DECIMAL(15,2) DEFAULT 0,
    total_transactions INT DEFAULT 0,
    
    -- Calculated at month end
    calculated_at TIMESTAMP,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    UNIQUE(agent_id, period_year, period_month)
);

-- Create performance tracking table for retailers
CREATE TABLE IF NOT EXISTS retailer_performance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_id UUID NOT NULL REFERENCES retailers(id) ON DELETE CASCADE,
    
    -- Performance Metrics (monthly aggregates)
    period_year INT NOT NULL,
    period_month INT NOT NULL,
    
    total_sales_amount DECIMAL(15,2) DEFAULT 0,
    total_commission_earned DECIMAL(15,2) DEFAULT 0,
    total_transactions INT DEFAULT 0,
    avg_transaction_value DECIMAL(10,2) DEFAULT 0,
    
    -- Calculated at month end
    calculated_at TIMESTAMP,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    UNIQUE(retailer_id, period_year, period_month)
);

-- Create indexes for agents
CREATE INDEX idx_agents_code ON agents(agent_code);
CREATE INDEX idx_agents_email ON agents(contact_email);
CREATE INDEX idx_agents_phone ON agents(contact_phone);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_commission_tier ON agents(commission_tier_id);

-- Create indexes for retailers
CREATE INDEX idx_retailers_code ON retailers(retailer_code);
CREATE INDEX idx_retailers_phone ON retailers(contact_phone);
CREATE INDEX idx_retailers_status ON retailers(status);
CREATE INDEX idx_retailers_parent_agent ON retailers(parent_agent_id);
CREATE INDEX idx_retailers_onboarding_method ON retailers(onboarding_method);

-- Create indexes for agent_retailers
CREATE INDEX idx_agent_retailers_agent ON agent_retailers(agent_id);
CREATE INDEX idx_agent_retailers_retailer ON agent_retailers(retailer_id);
CREATE INDEX idx_agent_retailers_active ON agent_retailers(is_active);

-- Create indexes for pos_devices
CREATE INDEX idx_pos_devices_code ON pos_devices(device_code);
CREATE INDEX idx_pos_devices_imei ON pos_devices(imei);
CREATE INDEX idx_pos_devices_status ON pos_devices(status);
CREATE INDEX idx_pos_devices_assigned_retailer ON pos_devices(assigned_retailer_id);

-- Create indexes for KYC tables
CREATE INDEX idx_agent_kyc_agent ON agent_kyc(agent_id);
CREATE INDEX idx_agent_kyc_status ON agent_kyc(kyc_status);
CREATE INDEX idx_retailer_kyc_retailer ON retailer_kyc(retailer_id);
CREATE INDEX idx_retailer_kyc_status ON retailer_kyc(kyc_status);

-- Create indexes for performance tables
CREATE INDEX idx_agent_performance_agent ON agent_performance(agent_id);
CREATE INDEX idx_agent_performance_period ON agent_performance(period_year, period_month);
CREATE INDEX idx_retailer_performance_retailer ON retailer_performance(retailer_id);
CREATE INDEX idx_retailer_performance_period ON retailer_performance(period_year, period_month);

-- Create indexes for commission tiers
CREATE INDEX idx_commission_tiers_active ON commission_tiers(is_active);

-- Insert default commission tier
INSERT INTO commission_tiers (name, description, base_commission_rate, is_active) 
VALUES ('Default 30%', 'Default commission tier with 30% rate', 0.30, true);


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop all agent management tables and indexes
DROP TABLE IF EXISTS retailer_performance CASCADE;
DROP TABLE IF EXISTS agent_performance CASCADE;
DROP TABLE IF EXISTS retailer_kyc CASCADE;
DROP TABLE IF EXISTS agent_kyc CASCADE;
DROP TABLE IF EXISTS pos_devices CASCADE;
DROP TABLE IF EXISTS agent_retailers CASCADE;
DROP TABLE IF EXISTS retailers CASCADE;
DROP TABLE IF EXISTS agents CASCADE;
DROP TABLE IF EXISTS commission_tiers CASCADE;

-- +goose StatementEnd