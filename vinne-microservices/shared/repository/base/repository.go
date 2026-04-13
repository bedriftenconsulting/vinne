package base

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/shared/common/errors"
	"github.com/randco/randco-microservices/shared/common/models"
	"github.com/redis/go-redis/v9"
)

// Repository interface defines common database operations
type Repository[T any] interface {
	Create(ctx context.Context, entity *T) error
	GetByID(ctx context.Context, id uuid.UUID) (*T, error)
	Update(ctx context.Context, entity *T) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, opts ListOptions) ([]*T, error)
	Count(ctx context.Context, opts ListOptions) (int64, error)
	WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// ListOptions contains pagination and filtering options
type ListOptions struct {
	Limit          int
	Offset         int
	OrderBy        string
	OrderDir       string // ASC or DESC
	Filters        map[string]interface{}
	IncludeDeleted bool
}

// BaseRepository provides common database operations
type BaseRepository struct {
	db        *sql.DB
	cache     *redis.Client
	tableName string
	cacheTTL  time.Duration
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(db *sql.DB, cache *redis.Client, tableName string) *BaseRepository {
	return &BaseRepository{
		db:        db,
		cache:     cache,
		tableName: tableName,
		cacheTTL:  5 * time.Minute,
	}
}

// WithTransaction executes a function within a database transaction
func (r *BaseRepository) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.NewInternalError("failed to begin transaction", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback() // Best effort rollback on panic
			// Add context to the panic for better debugging
			panic(fmt.Sprintf("transaction panic: %v", p))
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.NewInternalError("failed to rollback transaction", rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.NewInternalError("failed to commit transaction", err)
	}

	return nil
}

// GetDB returns the database connection
func (r *BaseRepository) GetDB() *sql.DB {
	return r.db
}

// GetCache returns the Redis client
func (r *BaseRepository) GetCache() *redis.Client {
	return r.cache
}

// GetTableName returns the table name
func (r *BaseRepository) GetTableName() string {
	return r.tableName
}

// Cache operations

// GetFromCache retrieves an item from cache
func (r *BaseRepository) GetFromCache(ctx context.Context, key string, dest interface{}) error {
	data, err := r.cache.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return errors.NewNotFoundError("cache entry")
		}
		return err
	}

	return json.Unmarshal([]byte(data), dest)
}

// SetInCache stores an item in cache
func (r *BaseRepository) SetInCache(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if ttl == 0 {
		ttl = r.cacheTTL
	}

	return r.cache.Set(ctx, key, data, ttl).Err()
}

// DeleteFromCache removes an item from cache
func (r *BaseRepository) DeleteFromCache(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.cache.Del(ctx, keys...).Err()
}

// CacheKey generates a cache key
func (r *BaseRepository) CacheKey(prefix string, id interface{}) string {
	return fmt.Sprintf("%s:%s:%v", r.tableName, prefix, id)
}

// InvalidateCache invalidates all cache entries for this repository
func (r *BaseRepository) InvalidateCache(ctx context.Context) error {
	pattern := fmt.Sprintf("%s:*", r.tableName)
	iter := r.cache.Scan(ctx, 0, pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return err
	}

	if len(keys) > 0 {
		return r.cache.Del(ctx, keys...).Err()
	}

	return nil
}

// Helper functions for building queries

// BuildSelectQuery builds a SELECT query with filters
func BuildSelectQuery(tableName string, columns []string, opts ListOptions) (string, []interface{}) {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE 1=1",
		columnsToString(columns), tableName)

	var args []interface{}
	argCount := 0

	// Add filters
	for key, value := range opts.Filters {
		argCount++
		query += fmt.Sprintf(" AND %s = $%d", key, argCount)
		args = append(args, value)
	}

	// Add soft delete filter
	if !opts.IncludeDeleted {
		query += " AND deleted_at IS NULL"
	}

	// Add ordering
	if opts.OrderBy != "" {
		orderDir := "ASC"
		if opts.OrderDir == "DESC" {
			orderDir = "DESC"
		}
		query += fmt.Sprintf(" ORDER BY %s %s", opts.OrderBy, orderDir)
	}

	// Add pagination
	if opts.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, opts.Limit)

		if opts.Offset > 0 {
			argCount++
			query += fmt.Sprintf(" OFFSET $%d", argCount)
			args = append(args, opts.Offset)
		}
	}

	return query, args
}

// BuildCountQuery builds a COUNT query with filters
func BuildCountQuery(tableName string, opts ListOptions) (string, []interface{}) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE 1=1", tableName)

	var args []interface{}
	argCount := 0

	// Add filters
	for key, value := range opts.Filters {
		argCount++
		query += fmt.Sprintf(" AND %s = $%d", key, argCount)
		args = append(args, value)
	}

	// Add soft delete filter
	if !opts.IncludeDeleted {
		query += " AND deleted_at IS NULL"
	}

	return query, args
}

func columnsToString(columns []string) string {
	if len(columns) == 0 {
		return "*"
	}
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ", "
		}
		result += col
	}
	return result
}

// QueryExecutor interface for database operations
type QueryExecutor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Define a typed key for context values
type contextKey string

const (
	// TxContextKey is the key for storing transaction in context
	TxContextKey contextKey = "tx"
)

// GetExecutor returns either a transaction or the database connection
func GetExecutor(ctx context.Context, db *sql.DB) QueryExecutor {
	if tx, ok := ctx.Value(TxContextKey).(*sql.Tx); ok && tx != nil {
		return tx
	}
	return db
}

// WithBaseModel interface for entities with BaseModel
type WithBaseModel interface {
	GetBaseModel() *models.BaseModel
}
