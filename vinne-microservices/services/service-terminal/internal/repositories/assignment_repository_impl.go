package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/services/service-terminal/internal/models"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type assignmentRepositoryImpl struct {
	db     *sqlx.DB
	tracer trace.Tracer
}

func NewTerminalAssignmentRepository(db *sqlx.DB) TerminalAssignmentRepository {
	return &assignmentRepositoryImpl{
		db:     db,
		tracer: otel.Tracer("service-terminal.assignment-repository"),
	}
}

func (r *assignmentRepositoryImpl) CreateAssignment(ctx context.Context, assignment *models.TerminalAssignment) error {
	dbSpan := tracing.TraceDB(ctx, "INSERT", "terminal_assignments").
		SetID(fmt.Sprintf("terminal:%s|retailer:%s", assignment.TerminalID.String(), assignment.RetailerID.String()))
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	// start transaction
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to start transaction: %w", err))
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()

	// check if retailer already has an active terminal
	var count int
	err = tx.GetContext(ctx, &count, `SELECT COUNT(*) FROM terminal_assignments WHERE retailer_id=$1 AND is_active=true`, assignment.RetailerID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to check existing retailer assignments: %w", err))
	}
	if count > 0 {
		return dbSpan.End(fmt.Errorf("retailer %s already has an active terminal", assignment.RetailerID))
	}

	// check if terminal is already assigned
	var terminalCount int
	err = tx.GetContext(ctx, &terminalCount,
		`SELECT COUNT(*) FROM terminal_assignments WHERE terminal_id=$1 AND is_active=true`,
		assignment.TerminalID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to check existing terminal assignments: %w", err))
	}
	if terminalCount > 0 {
		return dbSpan.End(fmt.Errorf("terminal %s is already assigned to another retailer", assignment.TerminalID))
	}

	// create new assignment
	err = tx.QueryRowContext(ctx,
		`INSERT INTO terminal_assignments (terminal_id, retailer_id, assigned_by, assigned_at, unassigned_at, is_active, notes, created_at)
		VALUES ($1, $2, $3, NOW(), NOW(), true, $4, $5)
		RETURNING id`,
		assignment.TerminalID, assignment.RetailerID, assignment.AssignedBy, assignment.Notes, assignment.CreatedAt,
	).Scan(&assignment.ID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to create new assignment: %w", err))

	}

	// update terminal assignment fields
	_, err = tx.ExecContext(ctx,
		`UPDATE terminals
		SET retailer_id = $1, assignment_date = $2, updated_at = $2, status = $3 
		WHERE id = $4`,
		assignment.RetailerID, now, models.TerminalStatusActive, assignment.TerminalID,
	)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to update terminal: %w", err))
	}

	if err = tx.Commit(); err != nil {
		return dbSpan.End(fmt.Errorf("failed to commit transaction: %w", err))
	}

	return dbSpan.End(nil)

}

func (r *assignmentRepositoryImpl) GetAssignmentByID(ctx context.Context, id uuid.UUID) (*models.TerminalAssignment, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_assignments").SetID(id.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, terminal_id, retailer_id, assigned_by, 
				assigned_at, unassigned_at, is_active, notes, created_at
		FROM terminal_assignments
		WHERE id = $1
		`

	var assignment models.TerminalAssignment
	err := r.db.GetContext(ctx, &assignment, query, id)
	if err != nil {
		return nil, dbSpan.End(err)
	}

	return &assignment, dbSpan.End(nil)
}

func (r *assignmentRepositoryImpl) GetActiveAssignmentByTerminalID(ctx context.Context, terminalID uuid.UUID) (*models.TerminalAssignment, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_assignments").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, terminal_id, retailer_id, assigned_by,
				assigned_at, unassigned_at, is_active, notes, created_at
		FROM terminal_assignments
		WHERE terminal_id = $1 AND is_active = true
		LIMIT 1`

	var assignment models.TerminalAssignment
	err := r.db.GetContext(ctx, &assignment, query, terminalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active assignment by terminal ID: %w", err)
	}

	return &assignment, dbSpan.End(nil)
}

func (r *assignmentRepositoryImpl) GetActiveAssignmentByRetailerID(ctx context.Context, retailerID uuid.UUID) (*models.TerminalAssignment, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_assignments").SetID(retailerID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, terminal_id, retailer_id, assigned_by,
				assigned_at, unassigned_at, is_active, notes, created_at
		FROM terminal_assignments
		WHERE retailer_id = $1 AND is_active = true
		LIMIT 1`

	var assignment models.TerminalAssignment
	err := r.db.GetContext(ctx, &assignment, query, retailerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active assignment by retailer ID: %w", err)
	}

	return &assignment, dbSpan.End(nil)
}

func (r *assignmentRepositoryImpl) UnassignTerminal(ctx context.Context, terminalID uuid.UUID, unassignedBy uuid.UUID, notes string) error {
	dbSpan := tracing.TraceDB(ctx, "UPDATE", "terminal_assignments").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to start transaction: %w", err))
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()

	// deactivate active assignment for terminal

	res, err := tx.ExecContext(ctx, `
		UPDATE terminal_assignments
		SET is_active = false, unassigned_at = $1, notes = $2
		WHERE terminal_id = $3 AND is_active = true
	`, now, notes, terminalID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to deactivate terminal: %w", err))
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to get affected rows: %w", err))
	}

	if rowsAffected == 0 {
		return dbSpan.End(fmt.Errorf("terminal %s has no active assignment to unassign", terminalID))
	}

	// update terminal record
	_, err = tx.ExecContext(ctx, `
		UPDATE terminals
		SET retailer_id = NULL, assignment_date = NULL, status = $1
		WHERE id = $2`,
		models.TerminalStatusInactive, terminalID)
	if err != nil {
		return dbSpan.End(fmt.Errorf("failed to update terminal: %w", err))
	}

	// commit transaction
	if err = tx.Commit(); err != nil {
		return dbSpan.End(fmt.Errorf("failed to unassign terminal: %w", err))
	}

	return dbSpan.End(nil)
}

func (r *assignmentRepositoryImpl) GetAssignmentHistory(ctx context.Context, terminalID uuid.UUID) ([]*models.TerminalAssignment, error) {
	dbSpan := tracing.TraceDB(ctx, "SELECT", "terminal_assignments").SetID(terminalID.String())
	ctx = dbSpan.Context()
	defer func() { _ = dbSpan.End(nil) }()

	query := `
		SELECT id, terminal_id, retailer_id, assigned_by, assigned_at, unassigned_at, 
			is_active, notes, created_at
		FROM terminal_assignments
		WHERE terminal_id = $1
		ORDER BY assigned_at DESC`

	var assignments []*models.TerminalAssignment
	err := r.db.SelectContext(ctx, &assignments, query, terminalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment history: %w", err)
	}

	return assignments, dbSpan.End(nil)
}

func (r *assignmentRepositoryImpl) ListAssignments(ctx context.Context, filters AssignmentFilters) ([]*models.TerminalAssignment, int64, error) {
	query := `SELECT * FROM terminal_assignments WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	// Apply filters
	if filters.TerminalID != nil {
		query += fmt.Sprintf(" AND terminal_id = $%d", argIndex)
		args = append(args, filters.TerminalID)
		argIndex++
	}

	if filters.RetailerID != nil {
		query += fmt.Sprintf(" AND retailer_id = $%d", argIndex)
		args = append(args, filters.RetailerID)
		argIndex++
	}

	if filters.IsActive != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIndex)
		args = append(args, filters.IsActive)
		argIndex++
	}

	if filters.AssignedBy != nil {
		query += fmt.Sprintf(" AND assigned_by = $%d", argIndex)
		args = append(args, filters.AssignedBy)
		argIndex++
	}

	if filters.DateFrom != nil {
		query += fmt.Sprintf(" AND assigned_at >= $%d", argIndex)
		args = append(args, filters.DateFrom)
		argIndex++
	}

	if filters.DateTo != nil {
		query += fmt.Sprintf(" AND unassigned_at <= $%d", argIndex)
		args = append(args, filters.DateTo)
		argIndex++
	}

	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS count_query"
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count assignments: %w", err)
	}

	query += " ORDER BY created_at DESC"

	// Apply pagination
	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filters.Limit)
		argIndex++
	}
	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filters.Offset)
		argIndex++
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list assignments: %w", err)
	}
	defer rows.Close()

	var assignments []*models.TerminalAssignment
	for rows.Next() {
		var assignment models.TerminalAssignment
		err := rows.Scan(
			&assignment.ID, &assignment.TerminalID, &assignment.RetailerID,
			&assignment.AssignedBy, &assignment.AssignedAt, &assignment.UnassignedAt, &assignment.IsActive,
			&assignment.Notes, &assignment.CreatedAt, &assignment.UpdatedAt, &assignment.DeletedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan assignments: %w", err)

		}
		assignments = append(assignments, &assignment)

	}
	return assignments, total, nil

}
