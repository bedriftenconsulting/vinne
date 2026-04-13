package repositories

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type ResetToken struct {
	ResetToken string
	AgentID    string
	Token      string
	CreatedAt  time.Time
}

type TokenRepositoryInterface interface {
	StoreResetToken(ctx context.Context, reset_token string, agent_id string, token string, duration time.Duration) error
	GetResetToken(ctx context.Context, reset_token string) (*ResetToken, error)
	DeleteResetToken(ctx context.Context, reset_token string) error
}

type TokenRepository struct {
	redis *redis.Client
}

// NewTokenRepository creates a new token repository
func NewTokenRepository(redis *redis.Client) TokenRepositoryInterface {
	return &TokenRepository{
		redis: redis,
	}
}

// StoreResetToken stores a token with an expiration time
func (r *TokenRepository) StoreResetToken(ctx context.Context, reset_token string, agent_id string, token string, duration time.Duration) error {
	key := "reset_token:" + reset_token
	createdAt := time.Now()

	err := r.redis.HSet(ctx, key, map[string]interface{}{
		"agent_id":   agent_id,
		"token":      token,
		"created_at": createdAt.Unix(),
	}).Err()

	if err != nil {
		return err
	}

	// Set expiration

	return r.redis.Expire(ctx, key, duration).Err()
}

// GetResetToken retrieves a token by agent ID
func (r *TokenRepository) GetResetToken(ctx context.Context, reset_token string) (*ResetToken, error) {
	key := "reset_token:" + reset_token

	data, err := r.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("reset token not found")
	}

	createdAtUnix, err := strconv.ParseInt(data["created_at"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}

	// return r.redis.Get(ctx, key).Result()
	return &ResetToken{
		ResetToken: reset_token,
		AgentID:    data["agent_id"],
		Token:      data["token"],
		CreatedAt:  time.Unix(createdAtUnix, 0),
	}, nil
}

// DeleteResetToken deletes a token by agent ID
func (r *TokenRepository) DeleteResetToken(ctx context.Context, reset_token string) error {
	key := "reset_token:" + reset_token
	return r.redis.Del(ctx, key).Err()
}
