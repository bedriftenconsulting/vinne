-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS draw_payout_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    draw_id UUID NOT NULL REFERENCES draws(id) ON DELETE CASCADE,
    ticket_id UUID NOT NULL,
    serial_number VARCHAR(100) NOT NULL,
    retailer_id UUID NOT NULL,

    -- Amount tracking
    winning_amount BIGINT NOT NULL, -- pesewas

    -- Payout status
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed, skipped
    payout_type VARCHAR(50) NOT NULL, -- auto, manual
    requires_approval BOOLEAN NOT NULL DEFAULT false,

    -- Processing tracking
    wallet_transaction_id VARCHAR(255), -- From wallet service
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT,

    -- Completion tracking
    wallet_credited_at TIMESTAMP WITH TIME ZONE,
    ticket_marked_paid_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,

    -- Manual approval (for big wins)
    approved_by VARCHAR(255),
    approved_at TIMESTAMP WITH TIME ZONE,
    rejection_reason TEXT,

    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT unique_draw_ticket_payout UNIQUE (draw_id, ticket_id)
);

-- Indexes for performance
CREATE INDEX idx_draw_payout_records_draw_id ON draw_payout_records(draw_id);
CREATE INDEX idx_draw_payout_records_status ON draw_payout_records(status);
CREATE INDEX idx_draw_payout_records_idempotency ON draw_payout_records(idempotency_key);
CREATE INDEX idx_draw_payout_records_retailer ON draw_payout_records(retailer_id);
CREATE INDEX idx_draw_payout_records_draw_status ON draw_payout_records(draw_id, status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS draw_payout_records CASCADE;
-- +goose StatementEnd
