-- +goose Up
DROP TABLE IF EXISTS webhook_events CASCADE;

CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    transaction_id UUID,
    reference VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL,
    raw_payload JSONB NOT NULL,
    signature_verified BOOLEAN NOT NULL DEFAULT false,
    processed_at TIMESTAMP,
    error_message TEXT,
    received_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_webhook_transaction FOREIGN KEY (transaction_id)
        REFERENCES transactions(id) ON DELETE SET NULL
);

CREATE INDEX idx_webhook_events_provider_txn
    ON webhook_events(provider, ((raw_payload->>'transactionId')::text));

CREATE INDEX idx_webhook_events_reference
    ON webhook_events(reference);

CREATE INDEX idx_webhook_events_transaction_id
    ON webhook_events(transaction_id)
    WHERE transaction_id IS NOT NULL;

CREATE INDEX idx_webhook_events_status_provider
    ON webhook_events(status, provider, received_at DESC);

CREATE INDEX idx_webhook_events_received_at
    ON webhook_events(received_at DESC);

-- +goose StatementBegin
-- ============================================================
-- Comments (safe to run multiple times)
-- ============================================================
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'webhook_events') THEN
        COMMENT ON TABLE webhook_events IS 'Stores webhook events from payment providers for deduplication and audit';
        COMMENT ON COLUMN webhook_events.provider IS 'Payment provider that sent the webhook (orange, mtn, telecel, airteltigo)';
        COMMENT ON COLUMN webhook_events.event_type IS 'Type of webhook event (payment.success, payment.failed, etc.)';
        COMMENT ON COLUMN webhook_events.transaction_id IS 'Links to our transactions table - may be null if transaction not found';
        COMMENT ON COLUMN webhook_events.reference IS 'Our transaction reference from the webhook payload';
        COMMENT ON COLUMN webhook_events.status IS 'Processing status: pending, completed, completed_idempotent, failed';
        COMMENT ON COLUMN webhook_events.raw_payload IS 'Complete raw webhook payload (JSONB) for audit and debugging';
        COMMENT ON COLUMN webhook_events.signature_verified IS 'Whether the webhook signature (HMAC-SHA256) was successfully verified';
        COMMENT ON COLUMN webhook_events.processed_at IS 'Timestamp when webhook was successfully processed (null if still pending or failed)';
        COMMENT ON COLUMN webhook_events.error_message IS 'Error message if webhook processing failed';
        COMMENT ON COLUMN webhook_events.received_at IS 'Timestamp when webhook was received from provider';
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- Drop indexes first
DROP INDEX IF EXISTS idx_webhook_events_received_at;
DROP INDEX IF EXISTS idx_webhook_events_status_provider;
DROP INDEX IF EXISTS idx_webhook_events_transaction_id;
DROP INDEX IF EXISTS idx_webhook_events_reference;
DROP INDEX IF EXISTS idx_webhook_events_provider_txn;

-- Drop table
DROP TABLE IF EXISTS webhook_events;
