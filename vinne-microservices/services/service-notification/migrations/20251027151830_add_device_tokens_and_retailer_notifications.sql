-- +goose Up
-- +goose StatementBegin

-- Table to store FCM device tokens for retailers
CREATE TABLE device_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_id VARCHAR(50) NOT NULL, -- 8-digit retailer code
    device_id VARCHAR(255) NOT NULL, -- Android device ID
    fcm_token TEXT NOT NULL, -- Firebase Cloud Messaging token
    platform VARCHAR(20) NOT NULL DEFAULT 'android', -- 'android' or 'ios'
    app_version VARCHAR(50), -- App version for debugging
    is_active BOOLEAN NOT NULL DEFAULT true, -- Token validity
    last_used_at TIMESTAMP, -- Track last successful push
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(device_id) -- One token per device
);

CREATE INDEX idx_device_tokens_retailer ON device_tokens(retailer_id);
CREATE INDEX idx_device_tokens_active ON device_tokens(is_active) WHERE is_active = true;
CREATE INDEX idx_device_tokens_device ON device_tokens(device_id);

-- Table to store retailer notification history
CREATE TABLE retailer_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retailer_id VARCHAR(50) NOT NULL, -- 8-digit retailer code
    type VARCHAR(20) NOT NULL, -- 'stake', 'winning', 'commission', 'low_balance', 'general'
    title VARCHAR(500) NOT NULL,
    body TEXT NOT NULL,
    amount BIGINT, -- Amount in pesewas (GHS * 100), nullable
    transaction_id VARCHAR(100), -- Related transaction ID if applicable
    is_read BOOLEAN NOT NULL DEFAULT false,
    read_at TIMESTAMP, -- When user marked as read
    notification_id UUID REFERENCES notifications(id), -- Link to sent notification
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retailer_notif_retailer ON retailer_notifications(retailer_id);
CREATE INDEX idx_retailer_notif_type ON retailer_notifications(type);
CREATE INDEX idx_retailer_notif_unread ON retailer_notifications(retailer_id, is_read) WHERE is_read = false;
CREATE INDEX idx_retailer_notif_created ON retailer_notifications(created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS retailer_notifications;
DROP TABLE IF EXISTS device_tokens;
-- +goose StatementEnd
