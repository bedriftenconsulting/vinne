package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/randco/service-payment/internal/models"
)

// SagaRepository defines the interface for saga data access
type SagaRepository interface {
	Create(ctx context.Context, saga *models.Saga) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Saga, error)
	GetBySagaID(ctx context.Context, sagaID string) (*models.Saga, error)
	GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*models.Saga, error)
	Update(ctx context.Context, saga *models.Saga) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.SagaStatus) error
	GetIncompleteSagas(ctx context.Context, limit int) ([]*models.Saga, error)

	// Saga steps
	CreateStep(ctx context.Context, step *models.SagaStep) error
	GetStepsBySagaID(ctx context.Context, sagaID uuid.UUID) ([]*models.SagaStep, error)
	UpdateStep(ctx context.Context, step *models.SagaStep) error
}

// sagaRepository implements SagaRepository
type sagaRepository struct {
	db *sqlx.DB
}

// NewSagaRepository creates a new saga repository
func NewSagaRepository(db *sqlx.DB) SagaRepository {
	return &sagaRepository{db: db}
}

// Create creates a new saga
func (r *sagaRepository) Create(ctx context.Context, saga *models.Saga) error {
	ctx, span := tracer.Start(ctx, "saga_repository.create",
		trace.WithAttributes(
			attribute.String("saga_id", saga.SagaID),
			attribute.String("transaction_id", saga.TransactionID.String()),
		))
	defer span.End()

	query := `
		INSERT INTO sagas (
			id, saga_id, transaction_id, status, current_step, total_steps,
			saga_data, compensation_data, max_retries,
			started_at, updated_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`

	sagaDataJSON, err := json.Marshal(saga.SagaData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal saga data: %w", err)
	}

	compensationDataJSON, err := json.Marshal(saga.CompensationData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal compensation data: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		saga.ID, saga.SagaID, saga.TransactionID, saga.Status,
		saga.CurrentStep, saga.TotalSteps, sagaDataJSON, compensationDataJSON,
		saga.MaxRetries, saga.StartedAt, saga.UpdatedAt, saga.CreatedAt,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create saga: %w", err)
	}

	return nil
}

// GetByID retrieves a saga by ID
func (r *sagaRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Saga, error) {
	ctx, span := tracer.Start(ctx, "saga_repository.get_by_id",
		trace.WithAttributes(attribute.String("id", id.String())))
	defer span.End()

	query := `
		SELECT
			id, saga_id, transaction_id, status, current_step, total_steps,
			saga_data, compensation_data, error_message, retry_count, max_retries,
			started_at, completed_at, updated_at, created_at
		FROM sagas
		WHERE id = $1
	`

	saga := &models.Saga{}
	var sagaDataJSON, compensationDataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&saga.ID, &saga.SagaID, &saga.TransactionID, &saga.Status,
		&saga.CurrentStep, &saga.TotalSteps, &sagaDataJSON, &compensationDataJSON,
		&saga.ErrorMessage, &saga.RetryCount, &saga.MaxRetries,
		&saga.StartedAt, &saga.CompletedAt, &saga.UpdatedAt, &saga.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("found", false))
			return nil, fmt.Errorf("saga not found")
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get saga: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(sagaDataJSON, &saga.SagaData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal saga data: %w", err)
	}

	if err := json.Unmarshal(compensationDataJSON, &saga.CompensationData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal compensation data: %w", err)
	}

	span.SetAttributes(attribute.Bool("found", true))
	return saga, nil
}

// GetBySagaID retrieves a saga by saga ID
func (r *sagaRepository) GetBySagaID(ctx context.Context, sagaID string) (*models.Saga, error) {
	ctx, span := tracer.Start(ctx, "saga_repository.get_by_saga_id",
		trace.WithAttributes(attribute.String("saga_id", sagaID)))
	defer span.End()

	query := `
		SELECT
			id, saga_id, transaction_id, status, current_step, total_steps,
			saga_data, compensation_data, error_message, retry_count, max_retries,
			started_at, completed_at, updated_at, created_at
		FROM sagas
		WHERE saga_id = $1
	`

	saga := &models.Saga{}
	var sagaDataJSON, compensationDataJSON []byte

	err := r.db.QueryRowContext(ctx, query, sagaID).Scan(
		&saga.ID, &saga.SagaID, &saga.TransactionID, &saga.Status,
		&saga.CurrentStep, &saga.TotalSteps, &sagaDataJSON, &compensationDataJSON,
		&saga.ErrorMessage, &saga.RetryCount, &saga.MaxRetries,
		&saga.StartedAt, &saga.CompletedAt, &saga.UpdatedAt, &saga.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("found", false))
			return nil, fmt.Errorf("saga not found")
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get saga: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(sagaDataJSON, &saga.SagaData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal saga data: %w", err)
	}

	if err := json.Unmarshal(compensationDataJSON, &saga.CompensationData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal compensation data: %w", err)
	}

	span.SetAttributes(attribute.Bool("found", true))
	return saga, nil
}

// GetByTransactionID retrieves a saga by transaction ID
func (r *sagaRepository) GetByTransactionID(ctx context.Context, transactionID uuid.UUID) (*models.Saga, error) {
	ctx, span := tracer.Start(ctx, "saga_repository.get_by_transaction_id",
		trace.WithAttributes(attribute.String("transaction_id", transactionID.String())))
	defer span.End()

	query := `
		SELECT
			id, saga_id, transaction_id, status, current_step, total_steps,
			saga_data, compensation_data, error_message, retry_count, max_retries,
			started_at, completed_at, updated_at, created_at
		FROM sagas
		WHERE transaction_id = $1
	`

	saga := &models.Saga{}
	var sagaDataJSON, compensationDataJSON []byte

	err := r.db.QueryRowContext(ctx, query, transactionID).Scan(
		&saga.ID, &saga.SagaID, &saga.TransactionID, &saga.Status,
		&saga.CurrentStep, &saga.TotalSteps, &sagaDataJSON, &compensationDataJSON,
		&saga.ErrorMessage, &saga.RetryCount, &saga.MaxRetries,
		&saga.StartedAt, &saga.CompletedAt, &saga.UpdatedAt, &saga.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("found", false))
			return nil, fmt.Errorf("saga not found")
		}
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get saga: %w", err)
	}

	// Unmarshal JSON fields
	if err := json.Unmarshal(sagaDataJSON, &saga.SagaData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal saga data: %w", err)
	}

	if err := json.Unmarshal(compensationDataJSON, &saga.CompensationData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to unmarshal compensation data: %w", err)
	}

	span.SetAttributes(attribute.Bool("found", true))
	return saga, nil
}

// Update updates a saga
func (r *sagaRepository) Update(ctx context.Context, saga *models.Saga) error {
	ctx, span := tracer.Start(ctx, "saga_repository.update",
		trace.WithAttributes(
			attribute.String("saga_id", saga.SagaID),
			attribute.String("status", string(saga.Status)),
		))
	defer span.End()

	query := `
		UPDATE sagas SET
			status = $1,
			current_step = $2,
			saga_data = $3,
			compensation_data = $4,
			error_message = $5,
			retry_count = $6,
			completed_at = $7,
			updated_at = $8
		WHERE id = $9
	`

	sagaDataJSON, err := json.Marshal(saga.SagaData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal saga data: %w", err)
	}

	compensationDataJSON, err := json.Marshal(saga.CompensationData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal compensation data: %w", err)
	}

	saga.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, query,
		saga.Status, saga.CurrentStep, sagaDataJSON, compensationDataJSON,
		saga.ErrorMessage, saga.RetryCount, saga.CompletedAt, saga.UpdatedAt,
		saga.ID,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update saga: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("saga not found")
		span.RecordError(err)
		return err
	}

	return nil
}

// UpdateStatus updates only the saga status
func (r *sagaRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.SagaStatus) error {
	ctx, span := tracer.Start(ctx, "saga_repository.update_status",
		trace.WithAttributes(
			attribute.String("id", id.String()),
			attribute.String("status", string(status)),
		))
	defer span.End()

	query := `
		UPDATE sagas SET
			status = $1,
			updated_at = $2
		WHERE id = $3
	`

	result, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update saga status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("saga not found")
		span.RecordError(err)
		return err
	}

	return nil
}

// GetIncompleteSagas retrieves incomplete sagas for recovery
func (r *sagaRepository) GetIncompleteSagas(ctx context.Context, limit int) ([]*models.Saga, error) {
	ctx, span := tracer.Start(ctx, "saga_repository.get_incomplete_sagas",
		trace.WithAttributes(attribute.Int("limit", limit)))
	defer span.End()

	query := `
		SELECT
			id, saga_id, transaction_id, status, current_step, total_steps,
			saga_data, compensation_data, error_message, retry_count, max_retries,
			started_at, completed_at, updated_at, created_at
		FROM sagas
		WHERE status NOT IN ('COMPLETED', 'COMPENSATED', 'FAILED')
		AND created_at > NOW() - INTERVAL '24 hours'
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get incomplete sagas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sagas := make([]*models.Saga, 0)
	for rows.Next() {
		saga := &models.Saga{}
		var sagaDataJSON, compensationDataJSON []byte

		err := rows.Scan(
			&saga.ID, &saga.SagaID, &saga.TransactionID, &saga.Status,
			&saga.CurrentStep, &saga.TotalSteps, &sagaDataJSON, &compensationDataJSON,
			&saga.ErrorMessage, &saga.RetryCount, &saga.MaxRetries,
			&saga.StartedAt, &saga.CompletedAt, &saga.UpdatedAt, &saga.CreatedAt,
		)

		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan saga: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(sagaDataJSON, &saga.SagaData); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal saga data: %w", err)
		}

		if err := json.Unmarshal(compensationDataJSON, &saga.CompensationData); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal compensation data: %w", err)
		}

		sagas = append(sagas, saga)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating sagas: %w", err)
	}

	span.SetAttributes(attribute.Int("result_count", len(sagas)))
	return sagas, nil
}

// CreateStep creates a new saga step
func (r *sagaRepository) CreateStep(ctx context.Context, step *models.SagaStep) error {
	ctx, span := tracer.Start(ctx, "saga_repository.create_step",
		trace.WithAttributes(
			attribute.String("saga_id", step.SagaID.String()),
			attribute.Int("step_number", step.StepNumber),
			attribute.String("step_name", step.StepName),
		))
	defer span.End()

	query := `
		INSERT INTO saga_steps (
			id, saga_id, step_number, step_name, step_type, status,
			input_data, output_data, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		)
	`

	inputDataJSON, err := json.Marshal(step.InputData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal input data: %w", err)
	}

	outputDataJSON, err := json.Marshal(step.OutputData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal output data: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		step.ID, step.SagaID, step.StepNumber, step.StepName, step.StepType,
		step.Status, inputDataJSON, outputDataJSON, step.CreatedAt,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create saga step: %w", err)
	}

	return nil
}

// GetStepsBySagaID retrieves all steps for a saga
func (r *sagaRepository) GetStepsBySagaID(ctx context.Context, sagaID uuid.UUID) ([]*models.SagaStep, error) {
	ctx, span := tracer.Start(ctx, "saga_repository.get_steps_by_saga_id",
		trace.WithAttributes(attribute.String("saga_id", sagaID.String())))
	defer span.End()

	query := `
		SELECT
			id, saga_id, step_number, step_name, step_type, status,
			input_data, output_data, error_message,
			started_at, completed_at, created_at
		FROM saga_steps
		WHERE saga_id = $1
		ORDER BY step_number ASC
	`

	rows, err := r.db.QueryContext(ctx, query, sagaID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get saga steps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	steps := make([]*models.SagaStep, 0)
	for rows.Next() {
		step := &models.SagaStep{}
		var inputDataJSON, outputDataJSON []byte

		err := rows.Scan(
			&step.ID, &step.SagaID, &step.StepNumber, &step.StepName, &step.StepType,
			&step.Status, &inputDataJSON, &outputDataJSON, &step.ErrorMessage,
			&step.StartedAt, &step.CompletedAt, &step.CreatedAt,
		)

		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan saga step: %w", err)
		}

		// Unmarshal JSON fields
		if err := json.Unmarshal(inputDataJSON, &step.InputData); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal input data: %w", err)
		}

		if err := json.Unmarshal(outputDataJSON, &step.OutputData); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to unmarshal output data: %w", err)
		}

		steps = append(steps, step)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating saga steps: %w", err)
	}

	span.SetAttributes(attribute.Int("result_count", len(steps)))
	return steps, nil
}

// UpdateStep updates a saga step
func (r *sagaRepository) UpdateStep(ctx context.Context, step *models.SagaStep) error {
	ctx, span := tracer.Start(ctx, "saga_repository.update_step",
		trace.WithAttributes(
			attribute.String("step_id", step.ID.String()),
			attribute.String("status", step.Status),
		))
	defer span.End()

	query := `
		UPDATE saga_steps SET
			status = $1,
			output_data = $2,
			error_message = $3,
			started_at = $4,
			completed_at = $5
		WHERE id = $6
	`

	outputDataJSON, err := json.Marshal(step.OutputData)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal output data: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query,
		step.Status, outputDataJSON, step.ErrorMessage,
		step.StartedAt, step.CompletedAt, step.ID,
	)

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update saga step: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		err := fmt.Errorf("saga step not found")
		span.RecordError(err)
		return err
	}

	return nil
}
