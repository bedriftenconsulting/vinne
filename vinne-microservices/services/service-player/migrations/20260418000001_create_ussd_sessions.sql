-- +goose Up
-- +goose StatementBegin

-- USSD sessions table
-- Stores every USSD interaction from mNotify callback
-- msisdn = phone number, sequence_id = mNotify's unique session ID
CREATE TABLE ussd_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    msisdn          VARCHAR(20) NOT NULL,           -- phone number e.g. 233541509394
    sequence_id     VARCHAR(100) NOT NULL,           -- mNotify sequenceID
    player_id       UUID REFERENCES players(id),     -- linked player (null if not registered)
    session_state   VARCHAR(50) NOT NULL DEFAULT 'STARTED', -- STARTED, IN_PROGRESS, COMPLETED, ABANDONED
    current_menu    VARCHAR(100),                    -- which menu step user is on
    user_input      TEXT,                            -- last input from user (data field)
    full_input_log  JSONB DEFAULT '[]',              -- array of all inputs in session
    raw_request     JSONB,                           -- full raw request from mNotify
    started_at      TIMESTAMP NOT NULL DEFAULT now(),
    last_activity   TIMESTAMP NOT NULL DEFAULT now(),
    completed_at    TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_ussd_sessions_msisdn      ON ussd_sessions (msisdn);
CREATE INDEX idx_ussd_sessions_sequence_id ON ussd_sessions (sequence_id);
CREATE INDEX idx_ussd_sessions_player_id   ON ussd_sessions (player_id);
CREATE INDEX idx_ussd_sessions_state       ON ussd_sessions (session_state);
CREATE INDEX idx_ussd_sessions_started     ON ussd_sessions (started_at);

-- USSD registrations table
-- Tracks players who registered or interacted via USSD
CREATE TABLE ussd_registrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    msisdn          VARCHAR(20) NOT NULL,
    player_id       UUID REFERENCES players(id),
    session_id      UUID REFERENCES ussd_sessions(id),
    first_name      VARCHAR(100),
    last_name       VARCHAR(100),
    pin             VARCHAR(255),                    -- hashed PIN for USSD auth
    status          VARCHAR(20) DEFAULT 'PENDING',   -- PENDING, COMPLETED, FAILED
    created_at      TIMESTAMP NOT NULL DEFAULT now(),
    updated_at      TIMESTAMP NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_ussd_registrations_msisdn ON ussd_registrations (msisdn);
CREATE INDEX idx_ussd_registrations_player        ON ussd_registrations (player_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS ussd_registrations CASCADE;
DROP TABLE IF EXISTS ussd_sessions CASCADE;
-- +goose StatementEnd
