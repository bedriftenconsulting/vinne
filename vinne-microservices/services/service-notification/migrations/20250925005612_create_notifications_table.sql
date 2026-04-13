-- +goose Up
-- +goose StatementBegin
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE,
    type VARCHAR(20) NOT NULL, -- 'email', 'sms' , 'push'
    subject VARCHAR(500), -- for emails
    content TEXT NOT NULL,
    status VARCHAR(50) NOT NULL, -- 'queued', 'sent', 'delivered', 'failed', 'bounced'
    provider VARCHAR(50),
    provider_message_id VARCHAR(255),
    provider_response JSONB,
    variables JSONB,
    template_id VARCHAR(150),
    retry_count INTEGER DEFAULT 0,
    scheduled_for TIMESTAMP,
    sent_at TIMESTAMP,
    delivered_at TIMESTAMP,
    failed_at TIMESTAMP,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_idempotency ON notifications(idempotency_key);
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_created_at ON notifications(created_at);
CREATE INDEX idx_notifications_scheduled ON notifications(scheduled_for) WHERE scheduled_for IS NOT NULL;

CREATE TABLE notification_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID REFERENCES notifications(id),
    event_type VARCHAR(50) NOT NULL, -- 'queued', 'sent', 'delivered', 'failed', 'bounced', 'clicked'
    event_data JSONB,
    occurred_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_notification ON notification_events(notification_id);
CREATE INDEX idx_events_type ON notification_events(event_type);

CREATE TABLE recipients (
    id SERIAL PRIMARY KEY,
    notification_id UUID REFERENCES notifications(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL, -- 'to', 'cc', 'bcc
    address VARCHAR(255) NOT NULL, -- email, phone number, device token
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(notification_id, address)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_events;
DROP TABLE IF EXISTS recipients;
DROP TABLE IF EXISTS notifications;
-- +goose StatementEnd
