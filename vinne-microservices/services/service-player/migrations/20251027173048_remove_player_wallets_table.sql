-- +goose Up
-- +goose StatementBegin
DROP TABLE player_wallets;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE player_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID NOT NULL REFERENCES players (id),
    created_at TIMESTAMP NOT NULL DEFAULT current_timestamp,
    updated_at TIMESTAMP NOT NULL DEFAULT current_timestamp
);
-- +goose StatementEnd
