package handlers

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	"github.com/randco/randco-microservices/services/service-ticket/internal/models"
	"github.com/randco/randco-microservices/services/service-ticket/internal/services"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TicketHandler implements the TicketServiceServer interface
type TicketHandler struct {
	ticketv1.UnimplementedTicketServiceServer
	ticketService services.TicketService
}

// NewTicketHandler creates a new ticket gRPC handler
func NewTicketHandler(ticketService services.TicketService) *TicketHandler {
	return &TicketHandler{
		ticketService: ticketService,
	}
}

// IssueTicket creates a new lottery ticket
func (h *TicketHandler) IssueTicket(ctx context.Context, req *ticketv1.IssueTicketRequest) (*ticketv1.IssueTicketResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.issue_ticket")
	defer span.End()

	span.SetAttributes(
		attribute.String("game_code", req.GameCode),
		attribute.Int("draw_number", int(req.DrawNumber)),
	)

	// Map proto request to service request
	serviceReq := services.IssueTicketRequest{
		GameCode:        req.GameCode,
		DrawNumber:      req.DrawNumber,
		SelectedNumbers: req.SelectedNumbers,
		BetLines:        pbBetLinesToModel(req.BetLines),
		IssuerType:      req.IssuerType,
		IssuerID:        req.IssuerId,
		IssuerDetails:   pbIssuerDetailsToModel(req.IssuerDetails),
		PaymentMethod:   req.PaymentMethod,
	}

	// Handle optional fields
	if req.GameScheduleId != "" {
		scheduleID, err := uuid.Parse(req.GameScheduleId)
		if err == nil {
			serviceReq.GameScheduleID = &scheduleID
		} else {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "invalid game_schedule_id")
			return nil, status.Errorf(3, "invalid game_schedule_id: %v", err) // InvalidArgument
		}
	}
	if req.CustomerPhone != "" {
		serviceReq.CustomerPhone = &req.CustomerPhone
	}
	if req.CustomerEmail != "" {
		serviceReq.CustomerEmail = &req.CustomerEmail
	}

	// Call service layer
	ticket, err := h.ticketService.IssueTicket(ctx, serviceReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to issue ticket: %v", err)
	}

	span.SetStatus(otelcodes.Ok, "ticket issued successfully")
	return &ticketv1.IssueTicketResponse{
		Ticket: modelTicketToPb(ticket),
	}, nil
}

// GetTicket retrieves a ticket by ID
func (h *TicketHandler) GetTicket(ctx context.Context, req *ticketv1.GetTicketRequest) (*ticketv1.GetTicketResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.get_ticket")
	defer span.End()

	ticketID, err := uuid.Parse(req.TicketId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket ID")
		return nil, status.Errorf(3, "invalid ticket ID: %v", err) // InvalidArgument
	}

	span.SetAttributes(attribute.String("ticket_id", ticketID.String()))

	ticket, err := h.ticketService.GetTicket(ctx, ticketID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to get ticket: %v", err)
	}

	span.SetStatus(otelcodes.Ok, "ticket retrieved successfully")
	return &ticketv1.GetTicketResponse{
		Ticket: modelTicketToPb(ticket),
	}, nil
}

// GetTicketBySerial retrieves a ticket by serial number
func (h *TicketHandler) GetTicketBySerial(ctx context.Context, req *ticketv1.GetTicketBySerialRequest) (*ticketv1.GetTicketBySerialResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.get_ticket_by_serial")
	defer span.End()

	span.SetAttributes(attribute.String("serial_number", req.SerialNumber))

	ticket, err := h.ticketService.GetTicketBySerial(ctx, req.SerialNumber)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to get ticket: %v", err)
	}

	span.SetStatus(otelcodes.Ok, "ticket retrieved successfully")
	return &ticketv1.GetTicketBySerialResponse{
		Ticket: modelTicketToPb(ticket),
	}, nil
}

// ListTickets retrieves a paginated list of tickets with optional filters
func (h *TicketHandler) ListTickets(ctx context.Context, req *ticketv1.ListTicketsRequest) (*ticketv1.ListTicketsResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.list_tickets")
	defer span.End()

	span.SetAttributes(
		attribute.Int("page", int(req.Page)),
		attribute.Int("page_size", int(req.PageSize)),
	)

	// Convert proto filter to model filter
	filter := models.TicketFilter{}
	if req.Filter != nil {
		if req.Filter.Status != "" {
			filter.Status = &req.Filter.Status
		}
		if req.Filter.GameCode != "" {
			filter.GameCode = &req.Filter.GameCode
		}
		if req.Filter.GameScheduleId != "" {
			filter.GameScheduleID = &req.Filter.GameScheduleId
		}
		if req.Filter.DrawNumber != 0 {
			filter.DrawNumber = &req.Filter.DrawNumber
		}
		if req.Filter.DrawId != "" {
			filter.DrawID = &req.Filter.DrawId
		}
		if req.Filter.IssuerType != "" {
			filter.IssuerType = &req.Filter.IssuerType
		}
		if req.Filter.IssuerId != "" {
			filter.IssuerID = &req.Filter.IssuerId
		}
		// Convert timestamps to RFC3339 strings for filtering
		if req.Filter.StartDate != nil {
			startDate := req.Filter.StartDate.AsTime().Format(time.RFC3339)
			filter.StartDate = &startDate
		}
		if req.Filter.EndDate != nil {
			endDate := req.Filter.EndDate.AsTime().Format(time.RFC3339)
			filter.EndDate = &endDate
		}
	}

	tickets, total, err := h.ticketService.ListTickets(ctx, filter, int(req.Page), int(req.PageSize))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to list tickets: %v", err)
	}

	pbTickets := make([]*ticketv1.Ticket, len(tickets))
	for i, ticket := range tickets {
		pbTickets[i] = modelTicketToPb(ticket)
	}

	span.SetStatus(otelcodes.Ok, "tickets listed successfully")
	return &ticketv1.ListTicketsResponse{
		Tickets:  pbTickets,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// ValidateTicket marks a ticket as validated and checks for winnings
func (h *TicketHandler) ValidateTicket(ctx context.Context, req *ticketv1.ValidateTicketRequest) (*ticketv1.ValidateTicketResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.validate_ticket")
	defer span.End()

	ticketID, err := uuid.Parse(req.TicketId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket ID")
		return nil, status.Errorf(3, "invalid ticket ID: %v", err) // InvalidArgument
	}

	span.SetAttributes(attribute.String("ticket_id", ticketID.String()))

	serviceReq := services.ValidateTicketRequest{
		TicketID:         &ticketID,
		ValidationMethod: "grpc",
		ValidatedByType:  req.ValidatorType,
		ValidatedByID:    req.ValidatorId,
	}

	result, err := h.ticketService.ValidateTicket(ctx, serviceReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to validate ticket: %v", err)
	}

	winningAmount := int64(0)
	if result.Ticket != nil {
		winningAmount = result.Ticket.WinningAmount
	}

	span.SetStatus(otelcodes.Ok, "ticket validated successfully")
	return &ticketv1.ValidateTicketResponse{
		Ticket:        modelTicketToPb(result.Ticket),
		WinningAmount: winningAmount,
	}, nil
}

// CancelTicket cancels a ticket before validation
func (h *TicketHandler) CancelTicket(ctx context.Context, req *ticketv1.CancelTicketRequest) (*ticketv1.CancelTicketResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.cancel_ticket")
	defer span.End()

	ticketID, err := uuid.Parse(req.TicketId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket ID")
		return nil, status.Errorf(3, "invalid ticket ID: %v", err) // InvalidArgument
	}

	span.SetAttributes(attribute.String("ticket_id", ticketID.String()))

	serviceReq := services.CancelTicketRequest{
		TicketID:        ticketID,
		Reason:          req.Reason,
		CancelledByType: "grpc",
		CancelledByID:   req.CancelledBy,
	}

	ticket, err := h.ticketService.CancelTicket(ctx, serviceReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to cancel ticket: %v", err)
	}

	span.SetStatus(otelcodes.Ok, "ticket cancelled successfully")
	return &ticketv1.CancelTicketResponse{
		Ticket: modelTicketToPb(ticket),
	}, nil
}

// VoidTicket voids a ticket (administrative action)
func (h *TicketHandler) VoidTicket(ctx context.Context, req *ticketv1.VoidTicketRequest) (*ticketv1.VoidTicketResponse, error) {
	_, span := otel.Tracer("service-ticket").Start(ctx, "handler.void_ticket")
	defer span.End()

	ticketID, err := uuid.Parse(req.TicketId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket ID")
		return nil, status.Errorf(3, "invalid ticket ID: %v", err) // InvalidArgument
	}

	span.SetAttributes(attribute.String("ticket_id", ticketID.String()))

	// TODO: Implement VoidTicket in service layer
	// For now, return unimplemented error
	return nil, status.Errorf(12, "VoidTicket not yet implemented") // Unimplemented
}

// ReprintTicket generates a reprint of an existing ticket
func (h *TicketHandler) ReprintTicket(ctx context.Context, req *ticketv1.ReprintTicketRequest) (*ticketv1.ReprintTicketResponse, error) {
	_, span := otel.Tracer("service-ticket").Start(ctx, "handler.reprint_ticket")
	defer span.End()

	ticketID, err := uuid.Parse(req.TicketId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket ID")
		return nil, status.Errorf(3, "invalid ticket ID: %v", err) // InvalidArgument
	}

	span.SetAttributes(attribute.String("ticket_id", ticketID.String()))

	// TODO: Implement ReprintTicket in service layer
	// For now, return unimplemented error
	return nil, status.Errorf(12, "ReprintTicket not yet implemented") // Unimplemented
}

// CheckWinnings checks if a ticket has winnings without validating
func (h *TicketHandler) CheckWinnings(ctx context.Context, req *ticketv1.CheckWinningsRequest) (*ticketv1.CheckWinningsResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.check_winnings")
	defer span.End()

	ticketID, err := uuid.Parse(req.TicketId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket ID")
		return nil, status.Errorf(3, "invalid ticket ID: %v", err) // InvalidArgument
	}

	span.SetAttributes(attribute.String("ticket_id", ticketID.String()))

	// TODO: Implement CheckWinnings properly
	// For now, return a basic response
	ticket, err := h.ticketService.GetTicket(ctx, ticketID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to get ticket: %v", err)
	}

	hasWinnings := ticket.WinningAmount > 0
	span.SetStatus(otelcodes.Ok, "winnings checked successfully")
	return &ticketv1.CheckWinningsResponse{
		HasWinnings:   hasWinnings,
		WinningAmount: ticket.WinningAmount,
		Status:        ticket.Status,
	}, nil
}

// Helper functions to map between proto and model types

func pbBetLinesToModel(pbLines []*ticketv1.BetLine) []models.BetLine {
	if pbLines == nil {
		return nil
	}
	lines := make([]models.BetLine, len(pbLines))
	for i, pb := range pbLines {
		lines[i] = models.BetLine{
			LineNumber:           pb.LineNumber,
			BetType:              pb.BetType,
			SelectedNumbers:      pb.SelectedNumbers,
			Banker:               pb.Banker,
			Opposed:              pb.Opposed,
			NumberOfCombinations: pb.NumberOfCombinations,
			AmountPerCombination: pb.AmountPerCombination,
			TotalAmount:          pb.TotalAmount,
		}
	}
	return lines
}

func modelBetLinesToPb(modelLines []models.BetLine) []*ticketv1.BetLine {
	if modelLines == nil {
		return nil
	}
	lines := make([]*ticketv1.BetLine, len(modelLines))
	for i, model := range modelLines {
		lines[i] = &ticketv1.BetLine{
			LineNumber:           model.LineNumber,
			BetType:              model.BetType,
			SelectedNumbers:      model.SelectedNumbers,
			Banker:               model.Banker,
			Opposed:              model.Opposed,
			NumberOfCombinations: model.NumberOfCombinations,
			AmountPerCombination: model.AmountPerCombination,
			TotalAmount:          model.TotalAmount,
		}
	}
	return lines
}

func pbIssuerDetailsToModel(pb *ticketv1.IssuerDetails) *models.IssuerDetails {
	if pb == nil {
		return nil
	}
	details := &models.IssuerDetails{}
	if pb.TerminalId != "" {
		details.TerminalID = &pb.TerminalId
	}
	if pb.RetailerCode != "" {
		details.RetailerCode = &pb.RetailerCode
	}
	if pb.PlayerId != "" {
		details.PlayerID = &pb.PlayerId
	}
	// Note: Location field is stored in proto but not in models.IssuerDetails
	// It's kept for backwards compatibility with existing proto definitions
	return details
}

func modelIssuerDetailsToPb(model *models.IssuerDetails) *ticketv1.IssuerDetails {
	if model == nil {
		return nil
	}
	pb := &ticketv1.IssuerDetails{}
	if model.TerminalID != nil {
		pb.TerminalId = *model.TerminalID
	}
	if model.RetailerCode != nil {
		pb.RetailerCode = *model.RetailerCode
	}
	if model.PlayerID != nil {
		pb.PlayerId = *model.PlayerID
	}
	// Location field not available in models.IssuerDetails
	return pb
}

func modelSecurityFeaturesToPb(model *models.SecurityFeatures) *ticketv1.SecurityFeatures {
	if model == nil {
		return nil
	}
	return &ticketv1.SecurityFeatures{
		QrCode:           model.QRCode,
		Barcode:          model.Barcode,
		VerificationCode: model.VerificationCode,
	}
}

func modelTicketToPb(ticket *models.Ticket) *ticketv1.Ticket {
	if ticket == nil {
		return nil
	}

	pb := &ticketv1.Ticket{
		Id:               ticket.ID.String(),
		SerialNumber:     ticket.SerialNumber,
		GameCode:         ticket.GameCode,
		GameName:         ticket.GameName,
		DrawNumber:       ticket.DrawNumber,
		SelectedNumbers:  ticket.SelectedNumbers,
		BetLines:         modelBetLinesToPb(ticket.BetLines),
		IssuerType:       ticket.IssuerType,
		IssuerId:         ticket.IssuerID,
		IssuerDetails:    modelIssuerDetailsToPb(ticket.IssuerDetails),
		UnitPrice:        ticket.UnitPrice,
		TotalAmount:      ticket.TotalAmount,
		SecurityHash:     ticket.SecurityHash,
		SecurityFeatures: modelSecurityFeaturesToPb(ticket.SecurityFeatures),
		Status:           ticket.Status,
		WinningAmount:    ticket.WinningAmount,
		CreatedAt:        timestamppb.New(ticket.CreatedAt),
		UpdatedAt:        timestamppb.New(ticket.UpdatedAt),
	}

	// Handle optional fields
	if ticket.GameScheduleID != nil {
		pb.GameScheduleId = ticket.GameScheduleID.String()
	}
	if ticket.DrawDate != nil {
		pb.DrawDate = timestamppb.New(*ticket.DrawDate)
	}
	if ticket.CustomerPhone != nil {
		pb.CustomerPhone = *ticket.CustomerPhone
	}
	if ticket.CustomerEmail != nil {
		pb.CustomerEmail = *ticket.CustomerEmail
	}
	if ticket.PaymentMethod != nil {
		pb.PaymentMethod = *ticket.PaymentMethod
	}
	if ticket.ValidatedAt != nil {
		pb.ValidatedAt = timestamppb.New(*ticket.ValidatedAt)
	}
	if ticket.CancelledAt != nil {
		pb.CancelledAt = timestamppb.New(*ticket.CancelledAt)
	}
	if ticket.VoidedAt != nil {
		pb.VoidedAt = timestamppb.New(*ticket.VoidedAt)
	}
	// Note: CancelledReason and VoidedReason are stored in separate tracking tables
	// (TicketCancellation and TicketVoid), not directly on the Ticket model

	return pb
}

// grpcErrorCode maps service errors to gRPC status codes
func grpcErrorCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}

	switch err {
	case sql.ErrNoRows:
		return codes.NotFound
	default:
		// Check error message for common patterns
		errMsg := err.Error()
		if contains(errMsg, "not found") {
			return codes.NotFound
		}
		if contains(errMsg, "invalid") || contains(errMsg, "validation") {
			return codes.InvalidArgument
		}
		if contains(errMsg, "already exists") || contains(errMsg, "duplicate") {
			return codes.AlreadyExists
		}
		if contains(errMsg, "permission") || contains(errMsg, "unauthorized") {
			return codes.PermissionDenied
		}
		return codes.Internal
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr ||
		s[len(s)-len(substr):] == substr ||
		hasSubstr(s, substr)))
}

func hasSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MarkTicketAsPaid marks a winning ticket as paid
func (h *TicketHandler) MarkTicketAsPaid(ctx context.Context, req *ticketv1.MarkTicketAsPaidRequest) (*ticketv1.MarkTicketAsPaidResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.mark_ticket_as_paid")
	defer span.End()

	span.SetAttributes(
		attribute.String("ticket_id", req.TicketId),
		attribute.Int64("paid_amount", req.PaidAmount),
		attribute.String("payment_reference", req.PaymentReference),
		attribute.String("paid_by", req.PaidBy),
	)

	// Parse ticket ID
	ticketID, err := uuid.Parse(req.TicketId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket_id")
		return &ticketv1.MarkTicketAsPaidResponse{
			Success: false,
			Message: "Invalid ticket_id format",
		}, nil
	}

	// Parse draw ID
	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid draw_id")
		return &ticketv1.MarkTicketAsPaidResponse{
			Success: false,
			Message: "Invalid draw_id format",
		}, nil
	}

	// Map to service request
	serviceReq := services.MarkTicketAsPaidRequest{
		TicketID:         ticketID,
		PaidAmount:       req.PaidAmount,
		PaymentReference: req.PaymentReference,
		PaidBy:           req.PaidBy,
		DrawID:           drawID,
	}

	// Call service
	resp, err := h.ticketService.MarkTicketAsPaid(ctx, serviceReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to mark ticket as paid: %v", err)
	}

	// Map payout details to proto if present
	var pbPayoutDetails *ticketv1.TicketPayout
	if resp.PayoutDetails != nil {
		pbPayoutDetails = &ticketv1.TicketPayout{
			TicketId:         resp.PayoutDetails.TicketID.String(),
			PaidAmount:       resp.PayoutDetails.PaidAmount,
			PaymentReference: resp.PayoutDetails.PaymentReference,
			PaidBy:           resp.PayoutDetails.PaidBy,
			PaidAt:           timestamppb.New(resp.PayoutDetails.PaidAt),
		}
	}

	span.SetStatus(otelcodes.Ok, "ticket marked as paid successfully")
	return &ticketv1.MarkTicketAsPaidResponse{
		Success:       resp.Success,
		Message:       resp.Message,
		PayoutDetails: pbPayoutDetails,
	}, nil
}

// UpdateTicketStatus updates a ticket's status after draw processing (won/lost)
func (h *TicketHandler) UpdateTicketStatus(ctx context.Context, req *ticketv1.UpdateTicketStatusRequest) (*ticketv1.UpdateTicketStatusResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.update_ticket_status")
	defer span.End()

	span.SetAttributes(
		attribute.String("ticket_id", req.TicketId),
		attribute.String("status", req.Status),
		attribute.Int64("winning_amount", req.WinningAmount),
		attribute.String("draw_id", req.DrawId),
	)

	// Parse ticket ID
	ticketID, err := uuid.Parse(req.TicketId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket_id")
		return &ticketv1.UpdateTicketStatusResponse{
			Success: false,
			Message: "Invalid ticket_id format",
		}, nil
	}

	// Parse draw ID
	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid draw_id")
		return &ticketv1.UpdateTicketStatusResponse{
			Success: false,
			Message: "Invalid draw_id format",
		}, nil
	}

	// Map to service request
	serviceReq := services.UpdateTicketStatusRequest{
		TicketID:      ticketID,
		Status:        req.Status,
		WinningAmount: req.WinningAmount,
		Matches:       req.Matches,
		PrizeTier:     req.PrizeTier,
		DrawID:        drawID,
	}

	// Call service
	resp, err := h.ticketService.UpdateTicketStatus(ctx, serviceReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to update ticket status: %v", err)
	}

	// Map ticket to proto if present
	var pbTicket *ticketv1.Ticket
	if resp.Ticket != nil {
		pbTicket = modelTicketToPb(resp.Ticket)
	}

	span.SetStatus(otelcodes.Ok, "ticket status updated successfully")
	return &ticketv1.UpdateTicketStatusResponse{
		Success: resp.Success,
		Message: resp.Message,
		Ticket:  pbTicket,
	}, nil
}

// UpdateTicketStatuses updates multiple tickets' statuses in a batch operation
func (h *TicketHandler) UpdateTicketStatuses(ctx context.Context, req *ticketv1.UpdateTicketStatusesRequest) (*ticketv1.UpdateTicketStatusesResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.update_ticket_statuses_batch")
	defer span.End()

	span.SetAttributes(
		attribute.Int("batch_size", len(req.Updates)),
		attribute.String("draw_id", req.DrawId),
	)

	// Convert protobuf updates to service layer format
	serviceUpdates := make([]services.TicketStatusUpdate, 0, len(req.Updates))
	for _, update := range req.Updates {
		serviceUpdates = append(serviceUpdates, services.TicketStatusUpdate{
			TicketID:      update.TicketId,
			Status:        update.Status,
			WinningAmount: update.WinningAmount,
			Matches:       update.Matches,
			PrizeTier:     update.PrizeTier,
		})
	}

	// Create service request
	serviceReq := &services.UpdateTicketStatusesRequest{
		Updates: serviceUpdates,
		DrawID:  req.DrawId,
	}

	// Call service
	resp, err := h.ticketService.UpdateTicketStatuses(ctx, serviceReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to update ticket statuses: %v", err)
	}

	// Convert service results to protobuf format
	pbResults := make([]*ticketv1.TicketUpdateResult, 0, len(resp.Results))
	for _, result := range resp.Results {
		pbResults = append(pbResults, &ticketv1.TicketUpdateResult{
			TicketId: result.TicketID,
			Success:  result.Success,
			Message:  result.Message,
		})
	}

	span.SetAttributes(
		attribute.Int64("successful_updates", resp.Successful),
		attribute.Int64("failed_updates", resp.Failed),
	)

	if resp.Failed > 0 {
		span.SetStatus(otelcodes.Error, resp.Message)
	} else {
		span.SetStatus(otelcodes.Ok, "batch update completed successfully")
	}

	return &ticketv1.UpdateTicketStatusesResponse{
		TotalRequested: resp.TotalRequested,
		Successful:     resp.Successful,
		Failed:         resp.Failed,
		Results:        pbResults,
		Message:        resp.Message,
	}, nil
}

// UpdateTicketsDrawId updates all tickets for a game schedule with a draw ID
func (h *TicketHandler) UpdateTicketsDrawId(ctx context.Context, req *ticketv1.UpdateTicketsDrawIdRequest) (*ticketv1.UpdateTicketsDrawIdResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.update_tickets_draw_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("game_schedule_id", req.GameScheduleId),
		attribute.String("draw_id", req.DrawId),
	)

	// Parse UUIDs
	scheduleID, err := uuid.Parse(req.GameScheduleId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid game_schedule_id")
		return &ticketv1.UpdateTicketsDrawIdResponse{
			Success: false,
			Message: "Invalid game_schedule_id format",
		}, nil
	}

	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid draw_id")
		return &ticketv1.UpdateTicketsDrawIdResponse{
			Success: false,
			Message: "Invalid draw_id format",
		}, nil
	}

	// Update tickets via service
	count, err := h.ticketService.UpdateTicketsDrawId(ctx, scheduleID, drawID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return &ticketv1.UpdateTicketsDrawIdResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	span.SetAttributes(attribute.Int64("updated_count", count))
	span.SetStatus(otelcodes.Ok, "tickets updated successfully")

	return &ticketv1.UpdateTicketsDrawIdResponse{
		UpdatedCount: count,
		Success:      true,
		Message:      "Tickets updated successfully",
	}, nil
}

// GetDailyMetrics returns daily metrics for dashboard analytics
func (h *TicketHandler) GetDailyMetrics(ctx context.Context, req *ticketv1.GetDailyMetricsRequest) (*ticketv1.GetDailyMetricsResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.get_daily_metrics")
	defer span.End()

	span.SetAttributes(
		attribute.String("metrics.date", req.Date),
		attribute.Bool("metrics.include_comparison", req.IncludeComparison),
	)

	// Call service layer
	result, err := h.ticketService.GetDailyMetrics(ctx, req.Date, req.IncludeComparison)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to get daily metrics: %v", err)
	}

	// Map service result to proto response
	response := &ticketv1.GetDailyMetricsResponse{
		Date: result.Date,
		GrossRevenue: &ticketv1.RevenueMetric{
			Amount:           result.GrossRevenue,
			ChangePercentage: result.GrossRevenueChange,
			PreviousAmount:   result.PreviousGrossRevenue,
		},
		Tickets: &ticketv1.CountMetric{
			Count:            result.TicketsCount,
			ChangePercentage: result.TicketsChange,
			PreviousCount:    result.PreviousTicketsCount,
		},
		Payouts: &ticketv1.RevenueMetric{
			Amount:           result.PayoutsAmount,
			ChangePercentage: result.PayoutsChange,
			PreviousAmount:   result.PreviousPayoutsAmount,
		},
		WinRate: &ticketv1.WinRateMetric{
			Percentage:     result.WinRatePercentage,
			WinningTickets: result.WinningTickets,
			TotalTickets:   result.TotalTickets,
		},
		// NEW: Additional metrics for enhanced dashboard
		Stakes: &ticketv1.CountMetric{
			Count:            result.StakesCount,
			ChangePercentage: result.StakesChange,
			PreviousCount:    result.PreviousStakesCount,
		},
		StakesAmount: &ticketv1.RevenueMetric{
			Amount:           result.StakesAmount,
			ChangePercentage: result.StakesAmountChange,
			PreviousAmount:   result.PreviousStakesAmount,
		},
		PaidTickets: &ticketv1.CountMetric{
			Count:            result.PaidTicketsCount,
			ChangePercentage: result.PaidTicketsChange,
			PreviousCount:    result.PreviousPaidTicketsCount,
		},
		PaymentsAmount: &ticketv1.RevenueMetric{
			Amount:           result.PaymentsAmount,
			ChangePercentage: result.PaymentsAmountChange,
			PreviousAmount:   result.PreviousPaymentsAmount,
		},
		UnpaidTickets: &ticketv1.CountMetric{
			Count:            result.UnpaidTicketsCount,
			ChangePercentage: result.UnpaidTicketsChange,
			PreviousCount:    result.PreviousUnpaidTicketsCount,
		},
		UnpaidAmount: &ticketv1.RevenueMetric{
			Amount:           result.UnpaidAmount,
			ChangePercentage: result.UnpaidAmountChange,
			PreviousAmount:   result.PreviousUnpaidAmount,
		},
	}

	span.SetAttributes(
		attribute.Int64("metrics.gross_revenue", result.GrossRevenue),
		attribute.Int64("metrics.tickets_count", result.TicketsCount),
		attribute.Int64("metrics.payouts", result.PayoutsAmount),
		attribute.Float64("metrics.win_rate", result.WinRatePercentage),
	)
	span.SetStatus(otelcodes.Ok, "daily metrics retrieved successfully")

	return response, nil
}

// GetMonthlyMetrics returns monthly aggregated metrics for charts
func (h *TicketHandler) GetMonthlyMetrics(ctx context.Context, req *ticketv1.GetMonthlyMetricsRequest) (*ticketv1.GetMonthlyMetricsResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.get_monthly_metrics")
	defer span.End()

	span.SetAttributes(attribute.Int("metrics.months", int(req.Months)))

	// Call service layer
	result, err := h.ticketService.GetMonthlyMetrics(ctx, req.Months)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to get monthly metrics: %v", err)
	}

	// Map service result to proto response
	dataPoints := make([]*ticketv1.MonthlyDataPoint, 0, len(result))
	for _, m := range result {
		dataPoints = append(dataPoints, &ticketv1.MonthlyDataPoint{
			Month:      m.Month,
			Year:       m.Year,
			Revenue:    m.Revenue,
			RevenueGhs: m.RevenueGHS,
			Tickets:    m.Tickets,
			Payouts:    m.Payouts,
			PayoutsGhs: m.PayoutsGHS,
		})
	}

	response := &ticketv1.GetMonthlyMetricsResponse{
		Data: dataPoints,
	}

	span.SetAttributes(attribute.Int("metrics.count", len(dataPoints)))
	span.SetStatus(otelcodes.Ok, "monthly metrics retrieved successfully")

	return response, nil
}

// GetTopPerformingAgents returns top performing agents by revenue
func (h *TicketHandler) GetTopPerformingAgents(ctx context.Context, req *ticketv1.GetTopPerformingAgentsRequest) (*ticketv1.GetTopPerformingAgentsResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.get_top_performing_agents")
	defer span.End()

	span.SetAttributes(
		attribute.String("metrics.date", req.Date),
		attribute.String("metrics.period", req.Period),
		attribute.Int("metrics.limit", int(req.Limit)),
	)

	// Call service layer
	result, err := h.ticketService.GetTopPerformingAgents(ctx, req.Period, req.Date, req.Limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to get top performing agents: %v", err)
	}

	// Map service result to proto response
	agents := make([]*ticketv1.AgentPerformance, 0, len(result.Agents))
	for _, a := range result.Agents {
		agents = append(agents, &ticketv1.AgentPerformance{
			AgentId:         a.AgentID,
			AgentCode:       a.AgentCode,
			AgentName:       a.AgentName,
			TotalRevenue:    a.TotalRevenue,
			TotalRevenueGhs: a.TotalRevenueGHS,
			TotalTickets:    a.TotalTickets,
			RetailerCount:   a.RetailerCount,
		})
	}

	response := &ticketv1.GetTopPerformingAgentsResponse{
		Agents: agents,
		Period: result.Period,
		Date:   result.Date,
	}

	span.SetAttributes(
		attribute.Int("agent.count", len(agents)),
		attribute.String("period.used", result.Period),
	)
	span.SetStatus(otelcodes.Ok, "top performing agents retrieved successfully")

	return response, nil
}

// GetAllTicketsForDraw retrieves all tickets for a specific draw without pagination limits.
// This is specifically for internal service-to-service calls (e.g., draw processing).
func (h *TicketHandler) GetAllTicketsForDraw(ctx context.Context, req *ticketv1.GetAllTicketsForDrawRequest) (*ticketv1.GetAllTicketsForDrawResponse, error) {
	ctx, span := otel.Tracer("service-ticket").Start(ctx, "handler.get_all_tickets_for_draw")
	defer span.End()

	// Parse and validate draw ID
	drawID, err := uuid.Parse(req.DrawId)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid draw_id")
		return nil, status.Errorf(codes.InvalidArgument, "invalid draw_id: %v", err)
	}

	span.SetAttributes(
		attribute.String("draw.id", drawID.String()),
		attribute.String("ticket.status", req.Status),
	)

	// Call service layer
	tickets, err := h.ticketService.GetAllTicketsForDraw(ctx, drawID, req.Status)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, status.Errorf(grpcErrorCode(err), "failed to get tickets for draw: %v", err)
	}

	// Map service result to proto response
	pbTickets := make([]*ticketv1.Ticket, 0, len(tickets))
	for _, ticket := range tickets {
		pbTickets = append(pbTickets, modelTicketToPb(ticket))
	}

	response := &ticketv1.GetAllTicketsForDrawResponse{
		Tickets: pbTickets,
		Total:   int64(len(tickets)),
	}

	span.SetAttributes(
		attribute.Int("tickets.count", len(tickets)),
	)
	span.SetStatus(otelcodes.Ok, "all tickets for draw retrieved successfully")

	return response, nil
}
