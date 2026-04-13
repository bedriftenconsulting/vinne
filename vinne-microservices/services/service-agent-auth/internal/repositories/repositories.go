package repositories

import (
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// Repositories holds all repository instances for authentication
type Repositories struct {
	Auth         AuthRepository
	Session      SessionRepository
	OfflineToken OfflineTokenRepository
}

// NewRepositories creates all repository instances
func NewRepositories(db *sqlx.DB, redis *redis.Client) *Repositories {
	return &Repositories{
		Auth:         NewAuthRepository(db, redis),
		Session:      NewSessionRepository(db.DB, redis),
		OfflineToken: NewOfflineTokenRepository(db),
	}
}
