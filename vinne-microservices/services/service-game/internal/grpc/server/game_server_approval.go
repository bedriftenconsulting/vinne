package server

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	pb "github.com/randco/randco-microservices/proto/game/v1"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/services"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GameServerWithApproval extends the minimal server with approval functionality
type GameServerWithApproval struct {
	*GameServerMinimal
	approvalService services.GameApprovalService
}

// NewGameServerWithApproval creates a new game server with approval support
func NewGameServerWithApproval(
	gameService services.GameService,
	approvalService services.GameApprovalService,
	scheduleService services.GameScheduleService,
) *GameServerWithApproval {
	return &GameServerWithApproval{
		GameServerMinimal: NewGameServerMinimal(gameService, scheduleService),
		approvalService:   approvalService,
	}
}

// SubmitForApproval submits a game for approval
func (s *GameServerWithApproval) SubmitForApproval(ctx context.Context, req *pb.SubmitForApprovalRequest) (*pb.SubmitForApprovalResponse, error) {
	log.Printf("[GameServerWithApproval] SubmitForApproval called for game: %s, submitted_by: %s, notes: %s", req.GameId, req.SubmittedBy, req.Notes)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "SubmitForApproval"),
		attribute.String("game.id", req.GameId),
		attribute.String("submitted_by", req.SubmittedBy),
	)

	// Parse UUIDs
	log.Printf("[GameServerWithApproval] Parsing game ID: %s", req.GameId)
	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		log.Printf("[GameServerWithApproval] Failed to parse game ID: %v", err)
		return &pb.SubmitForApprovalResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid game ID: %v", err),
		}, nil
	}

	log.Printf("[GameServerWithApproval] Parsing submitted_by ID: %s", req.SubmittedBy)
	submittedBy, err := uuid.Parse(req.SubmittedBy)
	if err != nil {
		log.Printf("[GameServerWithApproval] Failed to parse submitted_by ID: %v", err)
		return &pb.SubmitForApprovalResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid user ID: %v", err),
		}, nil
	}

	// Submit for approval
	log.Printf("[GameServerWithApproval] Calling approvalService.SubmitForApproval with gameID: %s, submittedBy: %s", gameID, submittedBy)
	if err := s.approvalService.SubmitForApproval(ctx, gameID, submittedBy, req.Notes); err != nil {
		log.Printf("[GameServerWithApproval] Error submitting for approval: %v", err)
		span.RecordError(err)
		return &pb.SubmitForApprovalResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}
	log.Printf("[GameServerWithApproval] Successfully submitted for approval")

	// Get the approval record to return
	log.Printf("[GameServerWithApproval] Getting approval status for game: %s", gameID)
	approval, err := s.approvalService.GetApprovalStatus(ctx, gameID)
	if err != nil {
		log.Printf("[GameServerWithApproval] Error getting approval status: %v", err)
		return &pb.SubmitForApprovalResponse{
			Success: true,
			Message: "Game submitted for approval successfully",
			// Approval is optional in response
		}, nil
	}
	log.Printf("[GameServerWithApproval] Got approval status: %+v", approval)

	pbApproval := convertApprovalToProto(approval)
	log.Printf("[GameServerWithApproval] Converted approval to proto: %+v", pbApproval)

	resp := &pb.SubmitForApprovalResponse{
		Success:  true,
		Message:  "Game submitted for approval successfully",
		Approval: pbApproval,
	}
	log.Printf("[GameServerWithApproval] Returning response: Success=%v, Message=%s", resp.Success, resp.Message)
	return resp, nil
}

// ApproveGame approves a game (handles both first and second approval)
func (s *GameServerWithApproval) ApproveGame(ctx context.Context, req *pb.ApproveGameRequest) (*pb.ApproveGameResponse, error) {
	log.Printf("[GameServer] ApproveGame called for game: %s", req.GameId)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "ApproveGame"),
		attribute.String("game.id", req.GameId),
		attribute.String("approved_by", req.ApprovedBy),
	)

	// Parse UUIDs
	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		return &pb.ApproveGameResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid game ID: %v", err),
		}, nil
	}

	approvedBy, err := uuid.Parse(req.ApprovedBy)
	if err != nil {
		return &pb.ApproveGameResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid user ID: %v", err),
		}, nil
	}

	// Approve the game (service layer determines if it's first or second approval)
	if err := s.approvalService.ApproveGame(ctx, gameID, approvedBy, req.Notes); err != nil {
		log.Printf("[GameServer] Error approving game: %v", err)
		span.RecordError(err)
		return &pb.ApproveGameResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Get the updated approval record
	approval, err := s.approvalService.GetApprovalStatus(ctx, gameID)
	if err != nil {
		log.Printf("[GameServer] Error getting approval status: %v", err)
		return &pb.ApproveGameResponse{
			Success:       true,
			Message:       "Game approved successfully",
			ApprovalStage: "",
			// Approval is optional in response
		}, nil
	}

	return &pb.ApproveGameResponse{
		Success:       true,
		Message:       "Game approved successfully",
		ApprovalStage: string(approval.ApprovalStage),
		Approval:      convertApprovalToProto(approval),
	}, nil
}

// RejectGame rejects a game
func (s *GameServerWithApproval) RejectGame(ctx context.Context, req *pb.RejectGameRequest) (*pb.RejectGameResponse, error) {
	log.Printf("[GameServer] RejectGame called for game: %s", req.GameId)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "RejectGame"),
		attribute.String("game.id", req.GameId),
		attribute.String("rejected_by", req.RejectedBy),
	)

	// Parse UUIDs
	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		return &pb.RejectGameResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid game ID: %v", err),
		}, nil
	}

	rejectedBy, err := uuid.Parse(req.RejectedBy)
	if err != nil {
		return &pb.RejectGameResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid user ID: %v", err),
		}, nil
	}

	// Reject the game
	if err := s.approvalService.RejectGame(ctx, gameID, rejectedBy, req.Reason); err != nil {
		log.Printf("[GameServer] Error rejecting game: %v", err)
		span.RecordError(err)
		return &pb.RejectGameResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Get the updated approval record
	approval, err := s.approvalService.GetApprovalStatus(ctx, gameID)
	if err != nil {
		log.Printf("[GameServer] Error getting approval status: %v", err)
		return &pb.RejectGameResponse{
			Success: true,
			Message: "Game rejected successfully",
			// Approval is optional in response
		}, nil
	}

	return &pb.RejectGameResponse{
		Success:  true,
		Message:  "Game rejected successfully",
		Approval: convertApprovalToProto(approval),
	}, nil
}

// GetApprovalStatus gets the approval status for a game
func (s *GameServerWithApproval) GetApprovalStatus(ctx context.Context, req *pb.GetApprovalStatusRequest) (*pb.GetApprovalStatusResponse, error) {
	log.Printf("[GameServer] GetApprovalStatus called for game: %s", req.GameId)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GetApprovalStatus"),
		attribute.String("game.id", req.GameId),
	)

	// Parse UUID
	gameID, err := uuid.Parse(req.GameId)
	if err != nil {
		return &pb.GetApprovalStatusResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid game ID: %v", err),
		}, nil
	}

	// Get approval status
	approval, err := s.approvalService.GetApprovalStatus(ctx, gameID)
	if err != nil {
		log.Printf("[GameServer] Error getting approval status: %v", err)
		span.RecordError(err)
		return &pb.GetApprovalStatusResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.GetApprovalStatusResponse{
		Success:  true,
		Message:  "Approval status retrieved successfully",
		Approval: convertApprovalToProto(approval),
	}, nil
}

// GetPendingApprovals gets pending approvals based on type
func (s *GameServerWithApproval) GetPendingApprovals(ctx context.Context, req *pb.GetPendingApprovalsRequest) (*pb.GetPendingApprovalsResponse, error) {
	log.Printf("[GameServer] GetPendingApprovals called with type: %s", req.ApprovalType)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GetPendingApprovals"),
		attribute.String("approval.type", req.ApprovalType),
		attribute.Int64("pagination.page", int64(req.Page)),
		attribute.Int64("pagination.per_page", int64(req.PerPage)),
	)

	// Default pagination values
	page := int(req.Page)
	perPage := int(req.PerPage)
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	var approvals []*models.GameApproval
	var err error

	// Get approvals based on type
	switch req.ApprovalType {
	case "first":
		approvals, err = s.approvalService.GetPendingFirstApprovals(ctx)
	case "second":
		approvals, err = s.approvalService.GetPendingSecondApprovals(ctx)
	case "all", "":
		approvals, err = s.approvalService.GetPendingApprovals(ctx)
	default:
		return &pb.GetPendingApprovalsResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid approval type: %s", req.ApprovalType),
		}, nil
	}

	if err != nil {
		log.Printf("[GameServer] Error getting pending approvals: %v", err)
		span.RecordError(err)
		return &pb.GetPendingApprovalsResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Apply pagination
	total := len(approvals)
	start := (page - 1) * perPage
	end := start + perPage

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	paginatedApprovals := approvals[start:end]

	// Convert to protobuf
	pbApprovals := make([]*pb.GameApproval, 0, len(paginatedApprovals))
	for _, approval := range paginatedApprovals {
		pbApprovals = append(pbApprovals, convertApprovalToProto(approval))
	}

	return &pb.GetPendingApprovalsResponse{
		Success:   true,
		Message:   "Pending approvals retrieved successfully",
		Approvals: pbApprovals,
		Total:     int32(total),
		Page:      req.Page,
		PerPage:   req.PerPage,
	}, nil
}

// convertApprovalToProto converts a domain model approval to protobuf
func convertApprovalToProto(approval *models.GameApproval) *pb.GameApproval {
	if approval == nil {
		return nil
	}

	pbApproval := &pb.GameApproval{
		Id:            approval.ID.String(),
		GameId:        approval.GameID.String(),
		ApprovalStage: string(approval.ApprovalStage),
		ApprovalCount: int32(approval.ApprovalCount),
		CreatedAt:     timestamppb.New(approval.CreatedAt),
		UpdatedAt:     timestamppb.New(approval.UpdatedAt),
	}

	// When submitted, the approvedBy field contains the submitter
	if approval.ApprovalStage == models.ApprovalStageSubmitted && approval.ApprovedBy != nil {
		pbApproval.SubmittedBy = approval.ApprovedBy.String()
		if approval.ApprovalDate != nil {
			pbApproval.SubmittedDate = timestamppb.New(*approval.ApprovalDate)
		}
		if approval.Notes != nil {
			pbApproval.SubmissionNotes = *approval.Notes
		}
	}

	// Optional fields for first approval
	if approval.FirstApprovedBy != nil {
		pbApproval.FirstApprovedBy = approval.FirstApprovedBy.String()
	}
	if approval.FirstApprovalDate != nil {
		pbApproval.FirstApprovalDate = timestamppb.New(*approval.FirstApprovalDate)
	}
	if approval.FirstApprovalNotes != nil {
		pbApproval.FirstApprovalNotes = *approval.FirstApprovalNotes
	}

	// Optional fields for second approval
	if approval.SecondApprovedBy != nil {
		pbApproval.SecondApprovedBy = approval.SecondApprovedBy.String()
	}
	if approval.SecondApprovalDate != nil {
		pbApproval.SecondApprovalDate = timestamppb.New(*approval.SecondApprovalDate)
	}
	if approval.SecondApprovalNotes != nil {
		pbApproval.SecondApprovalNotes = *approval.SecondApprovalNotes
	}

	// Optional fields for rejection
	if approval.RejectedBy != nil {
		pbApproval.RejectedBy = approval.RejectedBy.String()
	}
	if approval.RejectionDate != nil {
		pbApproval.RejectedDate = timestamppb.New(*approval.RejectionDate)
	}
	if approval.Reason != nil {
		pbApproval.RejectionReason = *approval.Reason
	}

	return pbApproval
}
