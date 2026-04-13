-- +goose Up
-- +goose StatementBegin
-- Password reset audit logs
CREATE TABLE password_reset_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents_auth(id),
    reset_token VARCHAR(255) NOT NULL,
    request_ip VARCHAR(45) NOT NULL,
    user_agent TEXT,
    channel VARCHAR(10) NOT NULL, -- 'email' or 'sms'
    status VARCHAR(20) NOT NULL, -- 'requested', 'otp_sent', 'validated', 'completed', 'expired', 'failed'
    otp_attempts INTEGER DEFAULT 0,
    completed_at TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_password_reset_logs_agent_id ON password_reset_logs(agent_id);
CREATE INDEX idx_password_reset_logs_reset_token ON password_reset_logs(reset_token);
CREATE INDEX idx_password_reset_logs_status ON password_reset_logs(status);
CREATE INDEX idx_password_reset_logs_created_at ON password_reset_logs(created_at);

-- Password history to prevent reuse
CREATE TABLE agent_password_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents_auth(id),
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_password_history_agent_id ON agent_password_history(agent_id);
CREATE INDEX idx_agent_password_history_created_at ON agent_password_history(created_at);

ALTER TABLE agents_auth
ADD COLUMN last_password_reset TIMESTAMP,
ADD COLUMN password_reset_count INTEGER DEFAULT 0,
ADD COLUMN password_updated_at TIMESTAMP NOT NULL DEFAULT NOW();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS password_reset_logs CASCADE;
DROP TABLE IF EXISTS agent_password_history CASCADE;

DROP COLUMN IF EXISTS last_password_reset,
DROP COLUMN IF EXISTS password_reset_count,
DROP COLUMN IF EXISTS password_updated_at;
-- +goose StatementEnd