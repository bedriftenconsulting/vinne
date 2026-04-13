package models

import (
	"time"

	"github.com/google/uuid"
)

type PlayerWalletReference struct {
	PlayerID  uuid.UUID `json:"player_id" db:"player_id"`
	WalletID  uuid.UUID `json:"wallet_id" db:"wallet_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type CreateWalletReferenceRequest struct {
	PlayerID uuid.UUID
	WalletID uuid.UUID
}

type WalletReferenceFilter struct {
	PlayerID uuid.UUID
	WalletID uuid.UUID
	Limit    int
	Offset   int
}
