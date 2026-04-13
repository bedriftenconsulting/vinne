package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GameApprovalReader defines read operations for game approvals
type GameApprovalReader interface {
	GetByGameID(ctx context.Context, gameID uuid.UUID) (*models.GameApproval, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.GameApproval, error)
}

// GameApprovalWriter defines write operations for game approvals
type GameApprovalWriter interface {
	Create(ctx context.Context, approval *models.GameApproval) error
	Update(ctx context.Context, approval *models.GameApproval) error
}

// GameApprovalWorkflow defines workflow operations for game approvals
type GameApprovalWorkflow interface {
	SubmitForApproval(ctx context.Context, gameID, submittedBy uuid.UUID, notes string) error
	FirstApprove(ctx context.Context, gameID, approvedBy uuid.UUID, notes string) error
	SecondApprove(ctx context.Context, gameID, approvedBy uuid.UUID, notes string) error
	Reject(ctx context.Context, gameID, rejectedBy uuid.UUID, reason string) error
}

// GameApprovalQuery defines query operations for game approvals
type GameApprovalQuery interface {
	GetPendingApprovals(ctx context.Context) ([]*models.GameApproval, error)
	GetPendingFirstApprovals(ctx context.Context) ([]*models.GameApproval, error)
	GetPendingSecondApprovals(ctx context.Context) ([]*models.GameApproval, error)
}

// GameApprovalRepository combines all game approval interfaces
type GameApprovalRepository interface {
	GameApprovalReader
	GameApprovalWriter
	GameApprovalWorkflow
	GameApprovalQuery
}

// gameApprovalRepository implements GameApprovalRepository interface
type gameApprovalRepository struct {
	db *sql.DB
}

// NewGameApprovalRepository creates a new instance of GameApprovalRepository
func NewGameApprovalRepository(db *sql.DB) GameApprovalRepository {
	return &gameApprovalRepository{
		db: db,
	}
}

// Create creates a new game approval request
func (r *gameApprovalRepository) Create(ctx context.Context, approval *models.GameApproval) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("game.id", approval.GameID.String()),
		attribute.String("approval.stage", string(approval.ApprovalStage)),
	)

	query := `
		INSERT INTO game_approvals (
			id, game_id, approval_stage, approved_by, rejected_by,
			approval_date, rejection_date, notes, reason,
			first_approved_by, first_approval_date, first_approval_notes,
			second_approved_by, second_approval_date, second_approval_notes,
			approval_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING created_at, updated_at`

	approval.ID = uuid.New()
	if approval.ApprovalStage == "" {
		approval.ApprovalStage = models.ApprovalStageSubmitted
	}

	err := r.db.QueryRowContext(ctx, query,
		approval.ID, approval.GameID, approval.ApprovalStage, approval.ApprovedBy, approval.RejectedBy,
		approval.ApprovalDate, approval.RejectionDate, approval.Notes, approval.Reason,
		approval.FirstApprovedBy, approval.FirstApprovalDate, approval.FirstApprovalNotes,
		approval.SecondApprovedBy, approval.SecondApprovalDate, approval.SecondApprovalNotes,
		approval.ApprovalCount,
	).Scan(&approval.CreatedAt, &approval.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create game approval")
		return fmt.Errorf("failed to create game approval: %w", err)
	}

	span.SetAttributes(attribute.String("approval.id", approval.ID.String()))
	return nil
}

// GetByGameID retrieves the latest approval for a game
func (r *gameApprovalRepository) GetByGameID(ctx context.Context, gameID uuid.UUID) (*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.get_by_game_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("game.id", gameID.String()),
	)

	query := `
		SELECT id, game_id, approval_stage, approved_by, rejected_by,
			approval_date, rejection_date, notes, reason,
			first_approved_by, first_approval_date, first_approval_notes,
			second_approved_by, second_approval_date, second_approval_notes,
			approval_count, created_at, updated_at
		FROM game_approvals
		WHERE game_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	approval := &models.GameApproval{}
	err := r.db.QueryRowContext(ctx, query, gameID).Scan(
		&approval.ID, &approval.GameID, &approval.ApprovalStage,
		&approval.ApprovedBy, &approval.RejectedBy,
		&approval.ApprovalDate, &approval.RejectionDate,
		&approval.Notes, &approval.Reason,
		&approval.FirstApprovedBy, &approval.FirstApprovalDate, &approval.FirstApprovalNotes,
		&approval.SecondApprovedBy, &approval.SecondApprovalDate, &approval.SecondApprovalNotes,
		&approval.ApprovalCount, &approval.CreatedAt, &approval.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("approval.found", false))
			return nil, fmt.Errorf("game approval not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game approval")
		return nil, fmt.Errorf("failed to get game approval: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("approval.found", true),
		attribute.String("approval.stage", string(approval.ApprovalStage)),
	)
	return approval, nil
}

// GetByID retrieves a game approval by ID
func (r *gameApprovalRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.get_by_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("approval.id", id.String()),
	)

	query := `
		SELECT id, game_id, approval_stage, approved_by, rejected_by,
			approval_date, rejection_date, notes, reason,
			first_approved_by, first_approval_date, first_approval_notes,
			second_approved_by, second_approval_date, second_approval_notes,
			approval_count, created_at, updated_at
		FROM game_approvals
		WHERE id = $1`

	approval := &models.GameApproval{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&approval.ID, &approval.GameID, &approval.ApprovalStage,
		&approval.ApprovedBy, &approval.RejectedBy,
		&approval.ApprovalDate, &approval.RejectionDate,
		&approval.Notes, &approval.Reason,
		&approval.FirstApprovedBy, &approval.FirstApprovalDate, &approval.FirstApprovalNotes,
		&approval.SecondApprovedBy, &approval.SecondApprovalDate, &approval.SecondApprovalNotes,
		&approval.ApprovalCount, &approval.CreatedAt, &approval.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("game approval not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game approval")
		return nil, fmt.Errorf("failed to get game approval: %w", err)
	}

	return approval, nil
}

// Update updates an existing game approval
func (r *gameApprovalRepository) Update(ctx context.Context, approval *models.GameApproval) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("approval.id", approval.ID.String()),
		attribute.String("approval.stage", string(approval.ApprovalStage)),
	)

	query := `
		UPDATE game_approvals SET
			approval_stage = $2, approved_by = $3, rejected_by = $4,
			approval_date = $5, rejection_date = $6,
			notes = $7, reason = $8,
			first_approved_by = $9, first_approval_date = $10, first_approval_notes = $11,
			second_approved_by = $12, second_approval_date = $13, second_approval_notes = $14,
			approval_count = $15,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := r.db.QueryRowContext(ctx, query,
		approval.ID, approval.ApprovalStage, approval.ApprovedBy, approval.RejectedBy,
		approval.ApprovalDate, approval.RejectionDate,
		approval.Notes, approval.Reason,
		approval.FirstApprovedBy, approval.FirstApprovalDate, approval.FirstApprovalNotes,
		approval.SecondApprovedBy, approval.SecondApprovalDate, approval.SecondApprovalNotes,
		approval.ApprovalCount,
	).Scan(&approval.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update game approval")
		return fmt.Errorf("failed to update game approval: %w", err)
	}

	return nil
}

// SubmitForApproval submits a game for approval
func (r *gameApprovalRepository) SubmitForApproval(ctx context.Context, gameID, submittedBy uuid.UUID, notes string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.submit")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("game.id", gameID.String()),
		attribute.String("submitted_by", submittedBy.String()),
	)

	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			// Log the error but don't fail the operation
			log.Printf("[WARN] Failed to rollback transaction in SubmitForApproval: %v", err)
		}
	}()

	// Create or update approval record
	approvalQuery := `
		INSERT INTO game_approvals (
			id, game_id, approval_stage, notes, approval_count
		) VALUES ($1, $2, $3, $4, 0)
		ON CONFLICT (game_id) DO UPDATE SET
			approval_stage = EXCLUDED.approval_stage,
			notes = EXCLUDED.notes,
			approval_count = 0,
			first_approved_by = NULL,
			first_approval_date = NULL,
			first_approval_notes = NULL,
			second_approved_by = NULL,
			second_approval_date = NULL,
			second_approval_notes = NULL,
			approved_by = NULL,
			approval_date = NULL,
			rejected_by = NULL,
			rejection_date = NULL,
			reason = NULL,
			updated_at = NOW()`

	_, err = tx.ExecContext(ctx, approvalQuery,
		uuid.New(), gameID, models.ApprovalStageSubmitted, notes,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create/update approval record: %w", err)
	}

	// Update game status to PENDING_APPROVAL
	gameQuery := `UPDATE games SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err = tx.ExecContext(ctx, gameQuery, gameID, "PENDING_APPROVAL")
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update game status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetStatus(codes.Ok, "game submitted for approval")
	return nil
}

// FirstApprove performs the first approval of a game
func (r *gameApprovalRepository) FirstApprove(ctx context.Context, gameID, approvedBy uuid.UUID, notes string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.first_approve")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("game.id", gameID.String()),
		attribute.String("approved_by", approvedBy.String()),
	)

	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			// Log the error but don't fail the operation
			log.Printf("[WARN] Failed to rollback transaction: %v", err)
		}
	}()

	// Check current approval state
	var currentStage string
	var approvalCount int
	checkQuery := `SELECT approval_stage, approval_count FROM game_approvals WHERE game_id = $1`
	err = tx.QueryRowContext(ctx, checkQuery, gameID).Scan(&currentStage, &approvalCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("game approval record not found")
		}
		return fmt.Errorf("failed to check approval state: %w", err)
	}

	if currentStage != string(models.ApprovalStageSubmitted) {
		return fmt.Errorf("game must be in SUBMITTED state for first approval")
	}

	// Update approval record for first approval
	now := time.Now()
	approvalQuery := `
		UPDATE game_approvals SET
			approval_stage = $2,
			first_approved_by = $3,
			first_approval_date = $4,
			first_approval_notes = $5,
			approval_count = 1,
			updated_at = NOW()
		WHERE game_id = $1`

	_, err = tx.ExecContext(ctx, approvalQuery,
		gameID, "FIRST_APPROVED", approvedBy, now, notes,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update approval record: %w", err)
	}

	// Game status remains PENDING_APPROVAL after first approval

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetStatus(codes.Ok, "first approval completed")
	return nil
}

// SecondApprove performs the second approval of a game and activates it
func (r *gameApprovalRepository) SecondApprove(ctx context.Context, gameID, approvedBy uuid.UUID, notes string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.second_approve")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("game.id", gameID.String()),
		attribute.String("approved_by", approvedBy.String()),
	)

	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			// Log the error but don't fail the operation
			log.Printf("[WARN] Failed to rollback transaction: %v", err)
		}
	}()

	// Check current approval state
	var currentStage string
	var approvalCount int
	var firstApprovedBy *uuid.UUID
	checkQuery := `SELECT approval_stage, approval_count, first_approved_by FROM game_approvals WHERE game_id = $1`
	err = tx.QueryRowContext(ctx, checkQuery, gameID).Scan(&currentStage, &approvalCount, &firstApprovedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("game approval record not found")
		}
		return fmt.Errorf("failed to check approval state: %w", err)
	}

	if currentStage != "FIRST_APPROVED" {
		return fmt.Errorf("game must be in FIRST_APPROVED state for second approval")
	}

	// Ensure different approver for second approval
	if firstApprovedBy != nil && *firstApprovedBy == approvedBy {
		return fmt.Errorf("second approval must be done by a different user")
	}

	// Update approval record for second approval
	now := time.Now()
	approvalQuery := `
		UPDATE game_approvals SET
			approval_stage = $2,
			second_approved_by = $3,
			second_approval_date = $4,
			second_approval_notes = $5,
			approved_by = $3,
			approval_date = $4,
			notes = $5,
			approval_count = 2,
			updated_at = NOW()
		WHERE game_id = $1`

	_, err = tx.ExecContext(ctx, approvalQuery,
		gameID, models.ApprovalStageApproved, approvedBy, now, notes,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update approval record: %w", err)
	}

	// Update game status to APPROVED (which can then be activated)
	gameQuery := `UPDATE games SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err = tx.ExecContext(ctx, gameQuery, gameID, "APPROVED")
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update game status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetStatus(codes.Ok, "second approval completed, game approved")
	return nil
}

// Reject rejects a game
func (r *gameApprovalRepository) Reject(ctx context.Context, gameID, rejectedBy uuid.UUID, reason string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.reject")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("game.id", gameID.String()),
		attribute.String("rejected_by", rejectedBy.String()),
	)

	// Start transaction to update both approval and game status
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			// Log the error but don't fail the operation
			log.Printf("[WARN] Failed to rollback transaction: %v", err)
		}
	}()

	// Update or create approval record
	now := time.Now()
	approvalQuery := `
		INSERT INTO game_approvals (
			id, game_id, approval_stage, rejected_by, rejection_date, reason
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (game_id) DO UPDATE SET
			approval_stage = EXCLUDED.approval_stage,
			rejected_by = EXCLUDED.rejected_by,
			rejection_date = EXCLUDED.rejection_date,
			reason = EXCLUDED.reason,
			updated_at = NOW()`

	_, err = tx.ExecContext(ctx, approvalQuery,
		uuid.New(), gameID, models.ApprovalStageRejected, rejectedBy, now, reason,
	)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update approval record: %w", err)
	}

	// Update game status back to DRAFT
	gameQuery := `UPDATE games SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err = tx.ExecContext(ctx, gameQuery, gameID, "DRAFT")
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update game status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetStatus(codes.Ok, "game rejected successfully")
	return nil
}

// GetPendingApprovals retrieves all pending game approvals
func (r *gameApprovalRepository) GetPendingApprovals(ctx context.Context) ([]*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.get_pending")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("filter.stage", "SUBMITTED,REVIEWED"),
	)

	query := `
		SELECT id, game_id, approval_stage, approved_by, rejected_by,
			approval_date, rejection_date, notes, reason,
			first_approved_by, first_approval_date, first_approval_notes,
			second_approved_by, second_approval_date, second_approval_notes,
			approval_count, created_at, updated_at
		FROM game_approvals
		WHERE approval_stage IN ('SUBMITTED', 'FIRST_APPROVED')
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get pending approvals")
		return nil, fmt.Errorf("failed to get pending approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var approvals []*models.GameApproval
	for rows.Next() {
		approval := &models.GameApproval{}
		err := rows.Scan(
			&approval.ID, &approval.GameID, &approval.ApprovalStage,
			&approval.ApprovedBy, &approval.RejectedBy,
			&approval.ApprovalDate, &approval.RejectionDate,
			&approval.Notes, &approval.Reason,
			&approval.FirstApprovedBy, &approval.FirstApprovalDate, &approval.FirstApprovalNotes,
			&approval.SecondApprovedBy, &approval.SecondApprovalDate, &approval.SecondApprovalNotes,
			&approval.ApprovalCount, &approval.CreatedAt, &approval.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan approval: %w", err)
		}
		approvals = append(approvals, approval)
	}

	span.SetAttributes(attribute.Int("approvals.pending_count", len(approvals)))
	return approvals, nil
}

// GetPendingFirstApprovals retrieves games pending first approval
func (r *gameApprovalRepository) GetPendingFirstApprovals(ctx context.Context) ([]*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.get_pending_first")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("filter.stage", "SUBMITTED"),
	)

	query := `
		SELECT id, game_id, approval_stage, approved_by, rejected_by,
			approval_date, rejection_date, notes, reason,
			first_approved_by, first_approval_date, first_approval_notes,
			second_approved_by, second_approval_date, second_approval_notes,
			approval_count, created_at, updated_at
		FROM game_approvals
		WHERE approval_stage = 'SUBMITTED'
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get pending first approvals")
		return nil, fmt.Errorf("failed to get pending first approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var approvals []*models.GameApproval
	for rows.Next() {
		approval := &models.GameApproval{}
		err := rows.Scan(
			&approval.ID, &approval.GameID, &approval.ApprovalStage,
			&approval.ApprovedBy, &approval.RejectedBy,
			&approval.ApprovalDate, &approval.RejectionDate,
			&approval.Notes, &approval.Reason,
			&approval.FirstApprovedBy, &approval.FirstApprovalDate, &approval.FirstApprovalNotes,
			&approval.SecondApprovedBy, &approval.SecondApprovalDate, &approval.SecondApprovalNotes,
			&approval.ApprovalCount, &approval.CreatedAt, &approval.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan approval: %w", err)
		}
		approvals = append(approvals, approval)
	}

	span.SetAttributes(attribute.Int("approvals.pending_first_count", len(approvals)))
	return approvals, nil
}

// GetPendingSecondApprovals retrieves games pending second approval
func (r *gameApprovalRepository) GetPendingSecondApprovals(ctx context.Context) ([]*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "db.game_approval.get_pending_second")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "game_approvals"),
		attribute.String("filter.stage", "FIRST_APPROVED"),
	)

	query := `
		SELECT id, game_id, approval_stage, approved_by, rejected_by,
			approval_date, rejection_date, notes, reason,
			first_approved_by, first_approval_date, first_approval_notes,
			second_approved_by, second_approval_date, second_approval_notes,
			approval_count, created_at, updated_at
		FROM game_approvals
		WHERE approval_stage = 'FIRST_APPROVED'
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get pending second approvals")
		return nil, fmt.Errorf("failed to get pending second approvals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var approvals []*models.GameApproval
	for rows.Next() {
		approval := &models.GameApproval{}
		err := rows.Scan(
			&approval.ID, &approval.GameID, &approval.ApprovalStage,
			&approval.ApprovedBy, &approval.RejectedBy,
			&approval.ApprovalDate, &approval.RejectionDate,
			&approval.Notes, &approval.Reason,
			&approval.FirstApprovedBy, &approval.FirstApprovalDate, &approval.FirstApprovalNotes,
			&approval.SecondApprovedBy, &approval.SecondApprovalDate, &approval.SecondApprovalNotes,
			&approval.ApprovalCount, &approval.CreatedAt, &approval.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan approval: %w", err)
		}
		approvals = append(approvals, approval)
	}

	span.SetAttributes(attribute.Int("approvals.pending_second_count", len(approvals)))
	return approvals, nil
}
