package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/randco/service-payment/internal/events"
	"github.com/randco/service-payment/internal/models"
	"github.com/randco/service-payment/internal/repositories"
)

var tracer = otel.Tracer("payment-service/saga")

// Step represents a single step in a saga
type Step struct {
	Name        string
	Execute     func(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error)
	Compensate  func(ctx context.Context, data map[string]interface{}) error
	Description string
}

// Orchestrator manages saga execution
type Orchestrator struct {
	sagaRepo       repositories.SagaRepository
	eventPublisher events.Publisher
}

// NewOrchestrator creates a new saga orchestrator
func NewOrchestrator(
	sagaRepo repositories.SagaRepository,
	eventPublisher events.Publisher,
) *Orchestrator {
	return &Orchestrator{
		sagaRepo:       sagaRepo,
		eventPublisher: eventPublisher,
	}
}

// Execute executes a saga with the given steps
func (o *Orchestrator) Execute(ctx context.Context, saga *models.Saga, steps []Step) error {
	ctx, span := tracer.Start(ctx, "saga_orchestrator.execute",
		trace.WithAttributes(
			attribute.String("saga_id", saga.SagaID),
			attribute.String("transaction_id", saga.TransactionID.String()),
			attribute.Int("total_steps", len(steps)),
		))
	defer span.End()

	// Publish saga started event
	_ = o.eventPublisher.PublishSagaEvent(ctx, events.NewSagaEvent(
		events.EventTypeSagaStarted,
		saga.SagaID,
		saga.TransactionID.String(),
		string(saga.Status),
		saga.CurrentStep,
		len(steps),
	))

	// Execute each step
	for i, step := range steps {
		if saga.CurrentStep > i {
			// Step already completed (recovery scenario)
			continue
		}

		saga.CurrentStep = i
		span.AddEvent(fmt.Sprintf("executing step %d: %s", i, step.Name))

		// Create saga step record
		sagaStep := models.CreateSagaStep(saga.ID, i, step.Name, "FORWARD")
		if err := o.sagaRepo.CreateStep(ctx, sagaStep); err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to create saga step: %w", err)
		}

		// Mark step as processing
		sagaStep.MarkAsProcessing()
		if err := o.sagaRepo.UpdateStep(ctx, sagaStep); err != nil {
			span.RecordError(err)
		}

		// Execute step
		output, err := step.Execute(ctx, saga.SagaData)
		if err != nil {
			span.RecordError(err)
			span.AddEvent(fmt.Sprintf("step %d failed: %v", i, err))

			// Mark step as failed
			sagaStep.MarkAsFailed(err.Error())
			_ = o.sagaRepo.UpdateStep(ctx, sagaStep)

			// Start compensation
			return o.compensate(ctx, saga, steps, i)
		}

		// Merge output into saga data
		for k, v := range output {
			saga.SagaData[k] = v
		}

		// Mark step as completed
		sagaStep.MarkAsCompleted(output)
		if err := o.sagaRepo.UpdateStep(ctx, sagaStep); err != nil {
			span.RecordError(err)
		}

		// Advance saga step
		saga.AdvanceStep()
		if err := o.sagaRepo.Update(ctx, saga); err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to update saga: %w", err)
		}

		span.AddEvent(fmt.Sprintf("step %d completed", i))
	}

	// All steps completed successfully
	saga.MarkAsCompleted()
	if err := o.sagaRepo.Update(ctx, saga); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to mark saga as completed: %w", err)
	}

	span.SetAttributes(attribute.Bool("success", true))

	// Publish saga completed event
	_ = o.eventPublisher.PublishSagaEvent(ctx, events.NewSagaEvent(
		events.EventTypeSagaCompleted,
		saga.SagaID,
		saga.TransactionID.String(),
		string(saga.Status),
		saga.CurrentStep,
		len(steps),
	))

	return nil
}

// compensate executes compensation for all completed steps in reverse order
func (o *Orchestrator) compensate(ctx context.Context, saga *models.Saga, steps []Step, failedStep int) error {
	ctx, span := tracer.Start(ctx, "saga_orchestrator.compensate",
		trace.WithAttributes(
			attribute.String("saga_id", saga.SagaID),
			attribute.Int("failed_step", failedStep),
		))
	defer span.End()

	saga.MarkAsCompensating()
	if err := o.sagaRepo.Update(ctx, saga); err != nil {
		span.RecordError(err)
	}

	span.AddEvent("starting compensation")

	// Store compensation data
	saga.CompensationData["failed_step"] = failedStep
	saga.CompensationData["failed_at"] = time.Now().Format(time.RFC3339)

	// Compensate completed steps in reverse order
	for i := failedStep - 1; i >= 0; i-- {
		step := steps[i]
		if step.Compensate == nil {
			// No compensation needed for this step
			continue
		}

		span.AddEvent(fmt.Sprintf("compensating step %d: %s", i, step.Name))

		// Create compensation step record
		compensationStep := models.CreateSagaStep(saga.ID, i, step.Name, "COMPENSATION")
		if err := o.sagaRepo.CreateStep(ctx, compensationStep); err != nil {
			span.RecordError(err)
		}

		compensationStep.MarkAsProcessing()
		_ = o.sagaRepo.UpdateStep(ctx, compensationStep)

		// Execute compensation
		err := step.Compensate(ctx, saga.SagaData)
		if err != nil {
			// Compensation failed - this is critical
			span.RecordError(err)
			span.AddEvent(fmt.Sprintf("compensation failed for step %d: %v", i, err))

			compensationStep.MarkAsFailed(err.Error())
			_ = o.sagaRepo.UpdateStep(ctx, compensationStep)

			// Mark saga as failed (compensation failed)
			saga.MarkAsFailed(fmt.Sprintf("compensation failed at step %d: %v", i, err))
			_ = o.sagaRepo.Update(ctx, saga)

			// Publish saga failed event
			_ = o.eventPublisher.PublishSagaEvent(ctx, events.NewSagaEvent(
				events.EventTypeSagaFailed,
				saga.SagaID,
				saga.TransactionID.String(),
				string(saga.Status),
				saga.CurrentStep,
				len(steps),
			))

			return fmt.Errorf("compensation failed at step %d: %w", i, err)
		}

		compensationStep.MarkAsCompleted(nil)
		_ = o.sagaRepo.UpdateStep(ctx, compensationStep)

		span.AddEvent(fmt.Sprintf("step %d compensated", i))
	}

	// All compensations completed
	saga.MarkAsCompensated()
	if err := o.sagaRepo.Update(ctx, saga); err != nil {
		span.RecordError(err)
	}

	span.SetAttributes(attribute.Bool("compensated", true))

	// Publish saga compensated event
	_ = o.eventPublisher.PublishSagaEvent(ctx, events.NewSagaEvent(
		events.EventTypeSagaCompensated,
		saga.SagaID,
		saga.TransactionID.String(),
		string(saga.Status),
		saga.CurrentStep,
		len(steps),
	))

	return fmt.Errorf("saga compensated after failure at step %d", failedStep)
}

// Resume resumes a saga from where it left off (for recovery)
func (o *Orchestrator) Resume(ctx context.Context, sagaID string, steps []Step) error {
	ctx, span := tracer.Start(ctx, "saga_orchestrator.resume",
		trace.WithAttributes(attribute.String("saga_id", sagaID)))
	defer span.End()

	// Load saga
	saga, err := o.sagaRepo.GetBySagaID(ctx, sagaID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to load saga: %w", err)
	}

	// Check if saga can be resumed
	if saga.IsTerminal() {
		return fmt.Errorf("saga is in terminal state: %s", saga.Status)
	}

	span.SetAttributes(
		attribute.Int("current_step", saga.CurrentStep),
		attribute.String("status", string(saga.Status)),
	)

	// Resume execution from current step
	return o.Execute(ctx, saga, steps)
}

// GetSaga retrieves a saga by ID
func (o *Orchestrator) GetSaga(ctx context.Context, sagaID string) (*models.Saga, error) {
	return o.sagaRepo.GetBySagaID(ctx, sagaID)
}

// GetSagaByTransactionID retrieves a saga by transaction ID
func (o *Orchestrator) GetSagaByTransactionID(ctx context.Context, transactionID uuid.UUID) (*models.Saga, error) {
	return o.sagaRepo.GetByTransactionID(ctx, transactionID)
}
