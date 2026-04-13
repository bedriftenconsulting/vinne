package models

import (
	"time"

	"github.com/google/uuid"
)

// SagaStatus represents the status of a saga
type SagaStatus string

const (
	SagaStatusStarted            SagaStatus = "STARTED"
	SagaStatusPaymentReserved    SagaStatus = "PAYMENT_RESERVED"
	SagaStatusWalletDebited      SagaStatus = "WALLET_DEBITED"
	SagaStatusProviderProcessing SagaStatus = "PROVIDER_PROCESSING"
	SagaStatusCompleted          SagaStatus = "COMPLETED"
	SagaStatusCompensating       SagaStatus = "COMPENSATING"
	SagaStatusCompensated        SagaStatus = "COMPENSATED"
	SagaStatusFailed             SagaStatus = "FAILED"
)

// Saga represents a distributed transaction saga
type Saga struct {
	ID               uuid.UUID              `db:"id"`
	SagaID           string                 `db:"saga_id"`
	TransactionID    uuid.UUID              `db:"transaction_id"`
	Status           SagaStatus             `db:"status"`
	CurrentStep      int                    `db:"current_step"`
	TotalSteps       int                    `db:"total_steps"`
	SagaData         map[string]interface{} `db:"saga_data"`
	CompensationData map[string]interface{} `db:"compensation_data"`
	ErrorMessage     string                 `db:"error_message"`
	RetryCount       int                    `db:"retry_count"`
	MaxRetries       int                    `db:"max_retries"`
	StartedAt        time.Time              `db:"started_at"`
	CompletedAt      *time.Time             `db:"completed_at"`
	UpdatedAt        time.Time              `db:"updated_at"`
	CreatedAt        time.Time              `db:"created_at"`
}

// SagaStep represents a step within a saga
type SagaStep struct {
	ID           uuid.UUID              `db:"id"`
	SagaID       uuid.UUID              `db:"saga_id"`
	StepNumber   int                    `db:"step_number"`
	StepName     string                 `db:"step_name"`
	StepType     string                 `db:"step_type"` // FORWARD, COMPENSATION
	Status       string                 `db:"status"`    // PENDING, PROCESSING, COMPLETED, FAILED
	InputData    map[string]interface{} `db:"input_data"`
	OutputData   map[string]interface{} `db:"output_data"`
	ErrorMessage string                 `db:"error_message"`
	StartedAt    *time.Time             `db:"started_at"`
	CompletedAt  *time.Time             `db:"completed_at"`
	CreatedAt    time.Time              `db:"created_at"`
}

// IsTerminal checks if the saga is in a terminal state
func (s *Saga) IsTerminal() bool {
	return s.Status == SagaStatusCompleted ||
		s.Status == SagaStatusCompensated ||
		s.Status == SagaStatusFailed
}

// CanRetry checks if the saga can be retried
func (s *Saga) CanRetry() bool {
	return !s.IsTerminal() && s.RetryCount < s.MaxRetries
}

// AdvanceStep advances the saga to the next step
func (s *Saga) AdvanceStep() {
	s.CurrentStep++
	s.UpdatedAt = time.Now()
}

// MarkAsCompleted marks the saga as completed
func (s *Saga) MarkAsCompleted() {
	s.Status = SagaStatusCompleted
	now := time.Now()
	s.CompletedAt = &now
	s.UpdatedAt = now
}

// MarkAsFailed marks the saga as failed
func (s *Saga) MarkAsFailed(errorMsg string) {
	s.Status = SagaStatusFailed
	s.ErrorMessage = errorMsg
	now := time.Now()
	s.CompletedAt = &now
	s.UpdatedAt = now
}

// MarkAsCompensating marks the saga as compensating
func (s *Saga) MarkAsCompensating() {
	s.Status = SagaStatusCompensating
	s.UpdatedAt = time.Now()
}

// MarkAsCompensated marks the saga as compensated
func (s *Saga) MarkAsCompensated() {
	s.Status = SagaStatusCompensated
	now := time.Now()
	s.CompletedAt = &now
	s.UpdatedAt = now
}

// UpdateStatus updates the saga status
func (s *Saga) UpdateStatus(status SagaStatus) {
	s.Status = status
	s.UpdatedAt = time.Now()
}

// IncrementRetry increments the retry count
func (s *Saga) IncrementRetry() {
	s.RetryCount++
	s.UpdatedAt = time.Now()
}

// CreateSaga creates a new saga instance
func CreateSaga(sagaID string, transactionID uuid.UUID, totalSteps int, maxRetries int) *Saga {
	now := time.Now()
	return &Saga{
		ID:               uuid.New(),
		SagaID:           sagaID,
		TransactionID:    transactionID,
		Status:           SagaStatusStarted,
		CurrentStep:      0,
		TotalSteps:       totalSteps,
		SagaData:         make(map[string]interface{}),
		CompensationData: make(map[string]interface{}),
		MaxRetries:       maxRetries,
		StartedAt:        now,
		UpdatedAt:        now,
		CreatedAt:        now,
	}
}

// CreateSagaStep creates a new saga step instance
func CreateSagaStep(sagaID uuid.UUID, stepNumber int, stepName string, stepType string) *SagaStep {
	now := time.Now()
	return &SagaStep{
		ID:         uuid.New(),
		SagaID:     sagaID,
		StepNumber: stepNumber,
		StepName:   stepName,
		StepType:   stepType,
		Status:     "PENDING",
		InputData:  make(map[string]interface{}),
		OutputData: make(map[string]interface{}),
		CreatedAt:  now,
	}
}

// MarkAsProcessing marks the step as processing
func (s *SagaStep) MarkAsProcessing() {
	s.Status = "PROCESSING"
	now := time.Now()
	s.StartedAt = &now
}

// MarkAsCompleted marks the step as completed
func (s *SagaStep) MarkAsCompleted(outputData map[string]interface{}) {
	s.Status = "COMPLETED"
	s.OutputData = outputData
	now := time.Now()
	s.CompletedAt = &now
}

// MarkAsFailed marks the step as failed
func (s *SagaStep) MarkAsFailed(errorMsg string) {
	s.Status = "FAILED"
	s.ErrorMessage = errorMsg
	now := time.Now()
	s.CompletedAt = &now
}
