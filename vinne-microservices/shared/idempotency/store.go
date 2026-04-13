package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/randco/randco-microservices/shared/common/errors"
	"github.com/redis/go-redis/v9"
)

type IdempotencyStore interface {
	CheckAndCreate(ctx context.Context, key string, requestHash string) (*IdempotencyRecord, error)
	MarkCompleted(ctx context.Context, key string, response any) error
	MarkFailed(ctx context.Context, key string, err error) error
	GetSharedBulkIdempotencyKey(key string) string
	BuildBulkIdempotencyKey(sharedKey string, index int) string
}

// Store provides idempotency checking for operations using Redis only
type Store struct {
	redis *redis.Client
}

func NewStore(redis *redis.Client) IdempotencyStore {
	return &Store{
		redis: redis,
	}
}

type IdempotencyRecord struct {
	Key         string          `json:"key"`
	Status      string          `json:"status"` // pending, completed, failed
	Response    json.RawMessage `json:"response,omitempty"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
	RequestHash string          `json:"request_hash"`
}

const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	TTL             = 86400 // 24 hours in seconds
)

// CheckAndCreate checks if an operation has been processed and creates a record if not
func (s *Store) CheckAndCreate(ctx context.Context, key string, requestHash string) (*IdempotencyRecord, error) {
	redisKey := fmt.Sprintf("idemp:%s", key)

	exists, err := s.redis.Exists(ctx, redisKey).Result()
	if err != nil {
		return nil, errors.NewInternalError("failed to check idempotency in Redis", err)
	}

	if exists > 0 {
		slog.Info("Idempotency record already exists", "key", key)
		data, err := s.redis.Get(ctx, redisKey).Result()
		if err != nil {
			return nil, errors.NewInternalError("failed to get idempotency record from Redis", err)
		}

		var record IdempotencyRecord
		if err := json.Unmarshal([]byte(data), &record); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal idempotency record", err)
		}

		if record.RequestHash != requestHash {
			return nil, errors.NewConflictError("different request with same idempotency key")
		}

		return &record, nil
	}

	newRecord := &IdempotencyRecord{
		Key:         key,
		Status:      StatusPending,
		RequestHash: requestHash,
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(TTL),
	}

	if err := s.setRecord(ctx, newRecord, TTL); err != nil {
		return nil, errors.NewInternalError("failed to create idempotency record", err)
	}

	return newRecord, nil
}

// MarkCompleted marks an operation as completed with the response
func (s *Store) MarkCompleted(ctx context.Context, key string, response any) error {
	responseData, err := json.Marshal(response)
	if err != nil {
		return errors.NewInternalError("failed to marshal response", err)
	}

	redisKey := fmt.Sprintf("idemp:%s", key)

	data, err := s.redis.Get(ctx, redisKey).Result()
	if err != nil {
		return errors.NewInternalError("failed to get idempotency record", err)
	}

	var record IdempotencyRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return errors.NewInternalError("failed to unmarshal idempotency record", err)
	}

	record.Status = StatusCompleted
	record.Response = responseData

	ttl := time.Until(record.ExpiresAt)
	if err := s.setRecord(ctx, &record, ttl); err != nil {
		return errors.NewInternalError("failed to update idempotency record", err)
	}

	return nil
}

// MarkFailed marks an operation as failed with the error
func (s *Store) MarkFailed(ctx context.Context, key string, err error) error {
	redisKey := fmt.Sprintf("idemp:%s", key)

	data, getErr := s.redis.Get(ctx, redisKey).Result()
	if getErr != nil {
		return errors.NewInternalError("failed to get idempotency record", getErr)
	}

	var record IdempotencyRecord
	if unmarshalErr := json.Unmarshal([]byte(data), &record); unmarshalErr != nil {
		return errors.NewInternalError("failed to unmarshal idempotency record", unmarshalErr)
	}

	record.Status = StatusFailed
	record.Error = err.Error()

	ttl := time.Until(record.ExpiresAt)
	if setErr := s.setRecord(ctx, &record, ttl); setErr != nil {
		return errors.NewInternalError("failed to update idempotency record", setErr)
	}

	return nil
}

func (s *Store) setRecord(ctx context.Context, record *IdempotencyRecord, ttl time.Duration) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	redisKey := fmt.Sprintf("idemp:%s", record.Key)
	return s.redis.Set(ctx, redisKey, data, ttl).Err()
}

// GenerateKey generates an idempotency key from request parameters
func GenerateKey(service, operation string, params ...string) string {
	key := fmt.Sprintf("%s:%s", service, operation)
	for _, param := range params {
		key += ":" + param
	}
	return key
}

func (s *Store) GetSharedBulkIdempotencyKey(key string) string {
	if key == "" {
		return ""
	}

	parts := strings.SplitN(key, "_", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// BuildBulkIdempotencyKey constructs a unique idempotency key for each item in a bulk request
// The format is "bulk_<sharedKey>_<index>"
func (s *Store) BuildBulkIdempotencyKey(sharedKey string, index int) string {
	return fmt.Sprintf("bulk_%s_%d", sharedKey, index)
}
