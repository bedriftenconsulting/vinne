package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GameApprovalService defines the interface for game approval business logic
type GameApprovalService interface {
	SubmitForApproval(ctx context.Context, gameID, submittedBy uuid.UUID, notes string) error
	ApproveGame(ctx context.Context, gameID, approvedBy uuid.UUID, notes string) error
	RejectGame(ctx context.Context, gameID, rejectedBy uuid.UUID, reason string) error
	GetApprovalStatus(ctx context.Context, gameID uuid.UUID) (*models.GameApproval, error)
	GetPendingApprovals(ctx context.Context) ([]*models.GameApproval, error)
	GetPendingFirstApprovals(ctx context.Context) ([]*models.GameApproval, error)
	GetPendingSecondApprovals(ctx context.Context) ([]*models.GameApproval, error)
}

// gameApprovalService implements GameApprovalService interface
type gameApprovalService struct {
	approvalRepo repositories.GameApprovalRepository
	gameRepo     repositories.GameRepository
	rulesRepo    repositories.GameRulesRepository
	prizeRepo    repositories.PrizeStructureRepository
}

// NewGameApprovalService creates a new instance of GameApprovalService
func NewGameApprovalService(
	approvalRepo repositories.GameApprovalRepository,
	gameRepo repositories.GameRepository,
	rulesRepo repositories.GameRulesRepository,
	prizeRepo repositories.PrizeStructureRepository,
) GameApprovalService {
	return &gameApprovalService{
		approvalRepo: approvalRepo,
		gameRepo:     gameRepo,
		rulesRepo:    rulesRepo,
		prizeRepo:    prizeRepo,
	}
}

// SubmitForApproval submits a game for approval
func (s *gameApprovalService) SubmitForApproval(ctx context.Context, gameID, submittedBy uuid.UUID, notes string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_approval.submit")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", gameID.String()))

	// Verify game exists
	game, err := s.gameRepo.GetByID(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return fmt.Errorf("game not found: %w", err)
	}

	// Validate game is in draft status
	if game.Status != "DRAFT" {
		err := fmt.Errorf("game must be in DRAFT status to submit for approval, current status: %s", game.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game status")
		return err
	}

	// TODO: For development, temporarily skip rules and prize structure validation
	// In production, these validations should be enabled

	// // Validate game has rules
	// _, err = s.rulesRepo.GetByGameID(ctx, gameID)
	// if err != nil {
	// 	err := fmt.Errorf("game must have rules defined before submission")
	// 	span.RecordError(err)
	// 	span.SetStatus(codes.Error, "missing game rules")
	// 	return err
	// }

	// // Validate game has prize structure
	// _, err = s.prizeRepo.GetByGameID(ctx, gameID)
	// if err != nil {
	// 	err := fmt.Errorf("game must have prize structure defined before submission")
	// 	span.RecordError(err)
	// 	span.SetStatus(codes.Error, "missing prize structure")
	// 	return err
	// }

	// Submit for approval using the repository method that handles status update
	if err := s.approvalRepo.SubmitForApproval(ctx, gameID, submittedBy, notes); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to submit for approval")
		return fmt.Errorf("failed to submit for approval: %w", err)
	}

	span.SetStatus(codes.Ok, "game submitted for approval")
	return nil
}

// ApproveGame approves a game
func (s *gameApprovalService) ApproveGame(ctx context.Context, gameID, approvedBy uuid.UUID, notes string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_approval.approve")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", gameID.String()),
		attribute.String("approved_by", approvedBy.String()),
	)

	// Verify game exists
	_, err := s.gameRepo.GetByID(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return fmt.Errorf("game not found: %w", err)
	}

	// Get current approval status
	approval, err := s.approvalRepo.GetByGameID(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "approval record not found")
		return fmt.Errorf("approval record not found: %w", err)
	}

	// Determine which approval to perform based on current state
	switch approval.ApprovalStage {
	case models.ApprovalStageSubmitted:
		// First approval
		if err := s.approvalRepo.FirstApprove(ctx, gameID, approvedBy, notes); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to perform first approval")
			return fmt.Errorf("failed to perform first approval: %w", err)
		}
		span.SetStatus(codes.Ok, "first approval completed")
	case models.ApprovalStageFirstApproved:
		// Second approval
		if err := s.approvalRepo.SecondApprove(ctx, gameID, approvedBy, notes); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to perform second approval")
			return fmt.Errorf("failed to perform second approval: %w", err)
		}
		span.SetStatus(codes.Ok, "second approval completed, game approved")
	default:
		err := fmt.Errorf("game is not in a state that can be approved, current stage: %s", approval.ApprovalStage)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid approval stage")
		return err
	}

	span.SetStatus(codes.Ok, "game approved successfully")
	return nil
}

// RejectGame rejects a game
func (s *gameApprovalService) RejectGame(ctx context.Context, gameID, rejectedBy uuid.UUID, reason string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_approval.reject")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", gameID.String()),
		attribute.String("rejected_by", rejectedBy.String()),
	)

	// Verify game exists
	game, err := s.gameRepo.GetByID(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return fmt.Errorf("game not found: %w", err)
	}

	// Validate game is pending approval
	if game.Status != "PENDING_APPROVAL" {
		err := fmt.Errorf("game must be in PENDING_APPROVAL status to reject, current status: %s", game.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid game status")
		return err
	}

	// Validate rejection reason is provided
	if reason == "" {
		err := fmt.Errorf("rejection reason is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "missing rejection reason")
		return err
	}

	// Reject the game
	if err := s.approvalRepo.Reject(ctx, gameID, rejectedBy, reason); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to reject game")
		return fmt.Errorf("failed to reject game: %w", err)
	}

	span.SetStatus(codes.Ok, "game rejected successfully")
	return nil
}

// GetApprovalStatus gets the approval status for a game
func (s *gameApprovalService) GetApprovalStatus(ctx context.Context, gameID uuid.UUID) (*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_approval.get_status")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", gameID.String()))

	approval, err := s.approvalRepo.GetByGameID(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "approval not found")
		return nil, fmt.Errorf("approval not found: %w", err)
	}

	span.SetAttributes(
		attribute.String("approval.stage", string(approval.ApprovalStage)),
		attribute.String("approval.id", approval.ID.String()),
	)
	return approval, nil
}

// GetPendingApprovals retrieves all pending game approvals
func (s *gameApprovalService) GetPendingApprovals(ctx context.Context) ([]*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_approval.get_pending")
	defer span.End()

	approvals, err := s.approvalRepo.GetPendingApprovals(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get pending approvals")
		return nil, fmt.Errorf("failed to get pending approvals: %w", err)
	}

	span.SetAttributes(attribute.Int("approvals.pending_count", len(approvals)))
	return approvals, nil
}

// GetPendingFirstApprovals retrieves games pending first approval
func (s *gameApprovalService) GetPendingFirstApprovals(ctx context.Context) ([]*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_approval.get_pending_first")
	defer span.End()

	approvals, err := s.approvalRepo.GetPendingFirstApprovals(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get pending first approvals")
		return nil, fmt.Errorf("failed to get pending first approvals: %w", err)
	}

	span.SetAttributes(attribute.Int("approvals.pending_first_count", len(approvals)))
	return approvals, nil
}

// GetPendingSecondApprovals retrieves games pending second approval
func (s *gameApprovalService) GetPendingSecondApprovals(ctx context.Context) ([]*models.GameApproval, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game_approval.get_pending_second")
	defer span.End()

	approvals, err := s.approvalRepo.GetPendingSecondApprovals(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get pending second approvals")
		return nil, fmt.Errorf("failed to get pending second approvals: %w", err)
	}

	span.SetAttributes(attribute.Int("approvals.pending_second_count", len(approvals)))
	return approvals, nil
}
