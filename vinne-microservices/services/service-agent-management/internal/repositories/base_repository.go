package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-agent-management/internal/models"
)

// BaseEntity interface that all entities must implement
type BaseEntity interface {
	GetID() uuid.UUID
	SetID(uuid.UUID)
	GetCreatedAt() time.Time
	SetCreatedAt(time.Time)
	GetUpdatedAt() time.Time
	SetUpdatedAt(time.Time)
}

// BaseRepository provides common CRUD operations for entities
type BaseRepository[T BaseEntity] struct {
	db        *sql.DB
	tableName string
	columns   []string
}

// NewBaseRepository creates a new base repository
func NewBaseRepository[T BaseEntity](db *sql.DB, tableName string, columns []string) *BaseRepository[T] {
	return &BaseRepository[T]{
		db:        db,
		tableName: tableName,
		columns:   columns,
	}
}

// Delete soft deletes an entity by ID
func (r *BaseRepository[T]) Delete(ctx context.Context, id uuid.UUID) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET deleted_at = $1, updated_at = $2
		WHERE id = $3 AND deleted_at IS NULL
	`, r.tableName)

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, now, now, id)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdateStatus updates the status of an entity
func (r *BaseRepository[T]) UpdateStatus(ctx context.Context, id uuid.UUID, status models.EntityStatus, updatedBy string) error {
	query := fmt.Sprintf(`
		UPDATE %s
		SET status = $1, updated_by = $2, updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
	`, r.tableName)

	result, err := r.db.ExecContext(ctx, query, status, updatedBy, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// Exists checks if an entity exists by ID
func (r *BaseRepository[T]) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	query := fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s
			WHERE id = $1 AND deleted_at IS NULL
		)
	`, r.tableName)

	var exists bool
	err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return exists, nil
}

// Count counts entities based on a condition
func (r *BaseRepository[T]) CountWithCondition(ctx context.Context, condition string, args ...interface{}) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s
		WHERE deleted_at IS NULL %s
	`, r.tableName, condition)

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count entities: %w", err)
	}

	return count, nil
}

// GetByStatus retrieves entities by status
func (r *BaseRepository[T]) GetByStatusGeneric(ctx context.Context, status models.EntityStatus, scanFunc func(*sql.Rows) (T, error)) ([]T, error) {
	query := fmt.Sprintf(`
		SELECT %s FROM %s
		WHERE status = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`, strings.Join(r.columns, ", "), r.tableName)

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get entities by status: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var entities []T
	for rows.Next() {
		entity, err := scanFunc(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entity: %w", err)
		}
		entities = append(entities, entity)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entities, nil
}

// Transaction helper functions

// ExecInTransaction executes a function within a database transaction
func ExecInTransaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction failed: %v, rollback failed: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Pagination helper

type PaginationParams struct {
	Page     int
	PageSize int
	OrderBy  string
	Order    string // ASC or DESC
}

func (p *PaginationParams) GetOffset() int {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 10
	}
	return (p.Page - 1) * p.PageSize
}

func (p *PaginationParams) GetLimit() int {
	if p.PageSize <= 0 {
		return 10
	}
	if p.PageSize > 100 {
		return 100
	}
	return p.PageSize
}

func (p *PaginationParams) GetOrderClause() string {
	if p.OrderBy == "" {
		p.OrderBy = "created_at"
	}
	if p.Order == "" {
		p.Order = "DESC"
	}
	return fmt.Sprintf("ORDER BY %s %s", p.OrderBy, p.Order)
}

// Query builder helpers

type QueryBuilder struct {
	baseQuery  string
	conditions []string
	args       []interface{}
	argCounter int
}

func NewQueryBuilder(baseQuery string) *QueryBuilder {
	return &QueryBuilder{
		baseQuery:  baseQuery,
		conditions: []string{},
		args:       []interface{}{},
		argCounter: 0,
	}
}

func (qb *QueryBuilder) AddCondition(condition string, arg interface{}) *QueryBuilder {
	qb.argCounter++
	qb.conditions = append(qb.conditions, fmt.Sprintf(condition, qb.argCounter))
	qb.args = append(qb.args, arg)
	return qb
}

func (qb *QueryBuilder) AddConditionIf(shouldAdd bool, condition string, arg interface{}) *QueryBuilder {
	if shouldAdd {
		return qb.AddCondition(condition, arg)
	}
	return qb
}

func (qb *QueryBuilder) Build() (string, []interface{}) {
	if len(qb.conditions) == 0 {
		return qb.baseQuery, qb.args
	}

	whereClause := " WHERE " + strings.Join(qb.conditions, " AND ")
	return qb.baseQuery + whereClause, qb.args
}
