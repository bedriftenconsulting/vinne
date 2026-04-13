-- +goose Up
-- +goose StatementBegin
-- Tickets table - Core ticket records
CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    serial_number VARCHAR(100) NOT NULL UNIQUE,

    -- Game and draw information
    game_code VARCHAR(50) NOT NULL,
    game_schedule_id UUID, -- References scheduled game/draw from game service
    draw_number INTEGER NOT NULL,
    game_name VARCHAR(255) NOT NULL,
    game_type VARCHAR(50) NOT NULL,

    -- Number selections
    selected_numbers INTEGER[] NOT NULL,
    banker_numbers INTEGER[] DEFAULT '{}', -- For perm bets
    opposed_numbers INTEGER[] DEFAULT '{}', -- For perm bets

    -- Bet lines and pricing
    bet_lines JSONB NOT NULL, -- Array of bet lines with type, numbers, amount
    number_of_lines INTEGER NOT NULL DEFAULT 1,
    unit_price BIGINT NOT NULL, -- Price per line in pesewas
    total_amount BIGINT NOT NULL, -- Total ticket cost in pesewas

    -- Issuer information
    issuer_type VARCHAR(50) NOT NULL, -- pos, web, mobile_app, ussd, telegram, whatsapp
    issuer_id VARCHAR(100) NOT NULL, -- Agent code, player ID, bot user, etc.
    issuer_details JSONB, -- Full issuer context

    -- Customer information (optional, depends on issuer type)
    customer_phone VARCHAR(20),
    customer_name VARCHAR(255),
    customer_email VARCHAR(255),

    -- Payment information
    payment_method VARCHAR(50), -- cash, mobile_money, wallet, bank_transfer
    payment_ref VARCHAR(255),
    payment_status VARCHAR(50) DEFAULT 'completed',

    -- Security features
    security_hash VARCHAR(255) NOT NULL, -- SHA-256 signature
    security_features JSONB, -- QR code, barcode, verification code, etc.

    -- Status and lifecycle
    status VARCHAR(50) NOT NULL DEFAULT 'issued', -- issued, validated, won, paid, cancelled, expired, void
    is_winning BOOLEAN NOT NULL DEFAULT false,
    winning_amount BIGINT DEFAULT 0, -- in pesewas
    prize_tier VARCHAR(50),
    matches INTEGER DEFAULT 0,

    -- Draw information
    draw_date TIMESTAMP,
    draw_time TIME,

    -- Timestamps
    issued_at TIMESTAMP NOT NULL DEFAULT NOW(),
    validated_at TIMESTAMP,
    cancelled_at TIMESTAMP,
    paid_at TIMESTAMP,
    voided_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for tickets
CREATE INDEX idx_tickets_serial_number ON tickets(serial_number);
CREATE INDEX idx_tickets_game_code ON tickets(game_code);
CREATE INDEX idx_tickets_game_schedule_id ON tickets(game_schedule_id);
CREATE INDEX idx_tickets_draw_number ON tickets(draw_number);
CREATE INDEX idx_tickets_customer_phone ON tickets(customer_phone);
CREATE INDEX idx_tickets_issuer_type ON tickets(issuer_type);
CREATE INDEX idx_tickets_issuer_id ON tickets(issuer_id);
CREATE INDEX idx_tickets_status ON tickets(status);
CREATE INDEX idx_tickets_is_winning ON tickets(is_winning);
CREATE INDEX idx_tickets_issued_at ON tickets(issued_at);
CREATE INDEX idx_tickets_draw_date ON tickets(draw_date);
CREATE INDEX idx_tickets_game_draw ON tickets(game_code, draw_number);
CREATE INDEX idx_tickets_customer_status ON tickets(customer_phone, status);
CREATE INDEX idx_tickets_security_hash ON tickets(security_hash);

-- Ticket payments table (for winning payouts)
CREATE TABLE IF NOT EXISTS ticket_payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    payment_reference VARCHAR(100) NOT NULL UNIQUE,

    -- Winner/Claimant information
    claimant_name VARCHAR(255) NOT NULL,
    claimant_phone VARCHAR(20) NOT NULL,
    claimant_id_type VARCHAR(50), -- national_id, passport, drivers_license, voter_id
    claimant_id_number VARCHAR(100),

    -- Bank information (for larger wins)
    bank_account VARCHAR(100),
    bank_name VARCHAR(255),
    bank_branch VARCHAR(255),

    -- Payment details
    prize_amount BIGINT NOT NULL, -- in pesewas
    payment_method VARCHAR(50), -- cash, mobile_money, bank_transfer, wallet
    payment_status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, approved, paid, rejected
    payment_notes TEXT,

    -- Approval workflow
    approved_by UUID,
    approved_at TIMESTAMP,
    paid_by UUID,
    paid_at TIMESTAMP,
    payment_transaction_ref VARCHAR(255),

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_payments_ticket_id ON ticket_payments(ticket_id);
CREATE INDEX idx_ticket_payments_payment_reference ON ticket_payments(payment_reference);
CREATE INDEX idx_ticket_payments_payment_status ON ticket_payments(payment_status);
CREATE INDEX idx_ticket_payments_claimant_phone ON ticket_payments(claimant_phone);
CREATE INDEX idx_ticket_payments_created_at ON ticket_payments(created_at);
CREATE INDEX idx_ticket_payments_paid_at ON ticket_payments(paid_at);

-- Ticket cancellations table
CREATE TABLE IF NOT EXISTS ticket_cancellations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,

    -- Cancellation details
    reason TEXT NOT NULL,
    cancelled_by_type VARCHAR(50) NOT NULL, -- customer, agent, system, admin
    cancelled_by_id VARCHAR(100) NOT NULL,

    -- Refund information
    refund_amount BIGINT DEFAULT 0, -- in pesewas
    refund_method VARCHAR(50), -- cash, mobile_money, wallet, bank_transfer
    refund_status VARCHAR(50) DEFAULT 'pending', -- pending, approved, processed, rejected
    refund_ref VARCHAR(255),
    refund_notes TEXT,

    -- Approval (if needed)
    approved_by UUID,
    approved_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_cancellations_ticket_id ON ticket_cancellations(ticket_id);
CREATE INDEX idx_ticket_cancellations_refund_status ON ticket_cancellations(refund_status);
CREATE INDEX idx_ticket_cancellations_created_at ON ticket_cancellations(created_at);

-- Ticket voids table (admin operation for fraud/errors)
CREATE TABLE IF NOT EXISTS ticket_voids (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,

    -- Void details
    reason TEXT NOT NULL,
    void_type VARCHAR(50) NOT NULL, -- fraud, error, duplicate, system_error
    authorized_by VARCHAR(255) NOT NULL, -- Admin user who voided
    authorization_notes TEXT,

    voided_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_voids_ticket_id ON ticket_voids(ticket_id);
CREATE INDEX idx_ticket_voids_void_type ON ticket_voids(void_type);
CREATE INDEX idx_ticket_voids_voided_at ON ticket_voids(voided_at);

-- Ticket reprints table (track reprint requests)
CREATE TABLE IF NOT EXISTS ticket_reprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,

    -- Reprint request details
    requested_by_type VARCHAR(50) NOT NULL, -- agent, customer, admin
    requested_by_id VARCHAR(100) NOT NULL,
    reason TEXT,

    -- For POS reprints
    terminal_id VARCHAR(100),
    printer_id VARCHAR(100),

    reprinted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_reprints_ticket_id ON ticket_reprints(ticket_id);
CREATE INDEX idx_ticket_reprints_requested_by_id ON ticket_reprints(requested_by_id);
CREATE INDEX idx_ticket_reprints_reprinted_at ON ticket_reprints(reprinted_at);

-- Ticket validations table (track when tickets are scanned/validated)
CREATE TABLE IF NOT EXISTS ticket_validations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,

    -- Validation details
    validated_by_type VARCHAR(50) NOT NULL, -- agent, system, api
    validated_by_id VARCHAR(100) NOT NULL,
    validation_method VARCHAR(50) NOT NULL, -- qr_scan, barcode_scan, serial_number, api
    validation_result VARCHAR(50) NOT NULL, -- valid, invalid, expired, already_paid, cancelled
    validation_notes TEXT,

    -- Location/context
    terminal_id VARCHAR(100),
    ip_address VARCHAR(45),
    user_agent TEXT,

    validated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_validations_ticket_id ON ticket_validations(ticket_id);
CREATE INDEX idx_ticket_validations_validated_by_id ON ticket_validations(validated_by_id);
CREATE INDEX idx_ticket_validations_validation_result ON ticket_validations(validation_result);
CREATE INDEX idx_ticket_validations_validated_at ON ticket_validations(validated_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ticket_validations;
DROP TABLE IF EXISTS ticket_reprints;
DROP TABLE IF EXISTS ticket_voids;
DROP TABLE IF EXISTS ticket_cancellations;
DROP TABLE IF EXISTS ticket_payments;
DROP TABLE IF EXISTS tickets;
-- +goose StatementEnd
