package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/google/uuid"
	agentmanagementv1 "github.com/randco/randco-microservices/proto/agent/management/v1"
	gamev1 "github.com/randco/randco-microservices/proto/game/v1"
	walletpb "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/services/service-ticket/internal/models"
	"github.com/randco/randco-microservices/services/service-ticket/internal/repositories"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TicketService defines the interface for ticket business logic
type TicketService interface {
	IssueTicket(ctx context.Context, req IssueTicketRequest) (*models.Ticket, error)
	GetTicket(ctx context.Context, id uuid.UUID) (*models.Ticket, error)
	GetTicketBySerial(ctx context.Context, serialNumber string) (*models.Ticket, error)
	ValidateTicket(ctx context.Context, req ValidateTicketRequest) (*ValidateTicketResponse, error)
	CancelTicket(ctx context.Context, req CancelTicketRequest) (*models.Ticket, error)
	ListTickets(ctx context.Context, filter models.TicketFilter, page, limit int) ([]*models.Ticket, int64, error)
	GetAllTicketsForDraw(ctx context.Context, drawID uuid.UUID, status string) ([]*models.Ticket, error)
	CheckWinning(ctx context.Context, ticketID uuid.UUID, winningNumbers []int32) (*WinningCheckResult, error)
	UpdateTicketsDrawId(ctx context.Context, gameScheduleID uuid.UUID, drawID uuid.UUID) (int64, error)
	MarkTicketAsPaid(ctx context.Context, req MarkTicketAsPaidRequest) (*MarkTicketAsPaidResponse, error)
	UpdateTicketStatus(ctx context.Context, req UpdateTicketStatusRequest) (*UpdateTicketStatusResponse, error)
	UpdateTicketStatuses(ctx context.Context, req *UpdateTicketStatusesRequest) (*UpdateTicketStatusesResponse, error)
	GetDailyMetrics(ctx context.Context, date string, includeComparison bool) (*DailyMetricsResult, error)
	GetMonthlyMetrics(ctx context.Context, months int32) ([]*MonthlyMetricsResult, error)
	GetTopPerformingAgents(ctx context.Context, period string, date string, limit int32) (*TopPerformingAgentsResult, error)
}

// ticketService implements TicketService interface
type ticketService struct {
	db                    *sql.DB
	ticketRepo            repositories.TicketRepository
	walletClient          walletpb.WalletServiceClient
	agentManagementClient agentmanagementv1.AgentManagementServiceClient
	gameClient            gamev1.GameServiceClient
	redisClient           *redis.Client
	securityConfig        SecurityConfig
}

// SecurityConfig holds configuration for security features
type SecurityConfig struct {
	SecretKey     string
	SerialPrefix  string
	QRCodeBaseURL string
	BarcodePrefix string
}

// ServiceConfig holds overall service configuration
type ServiceConfig struct {
	Security        SecurityConfig
	GameServiceConn *grpc.ClientConn // For game service integration
}

// NewTicketService creates a new instance of TicketService
func NewTicketService(
	db *sql.DB,
	ticketRepo repositories.TicketRepository,
	walletClient walletpb.WalletServiceClient,
	agentManagementClient agentmanagementv1.AgentManagementServiceClient,
	gameClient gamev1.GameServiceClient,
	redisClient *redis.Client,
	config *ServiceConfig,
) TicketService {
	return &ticketService{
		db:                    db,
		ticketRepo:            ticketRepo,
		walletClient:          walletClient,
		agentManagementClient: agentManagementClient,
		gameClient:            gameClient,
		redisClient:           redisClient,
		securityConfig:        config.Security,
	}
}

// Request/Response types

// IssueTicketRequest represents a request to issue a ticket
type IssueTicketRequest struct {
	// Game information
	GameCode       string     `json:"game_code"`
	GameScheduleID *uuid.UUID `json:"game_schedule_id,omitempty"`
	DrawNumber     int32      `json:"draw_number"`

	// Number selections
	SelectedNumbers []int32 `json:"selected_numbers"`
	BankerNumbers   []int32 `json:"banker_numbers,omitempty"`
	OpposedNumbers  []int32 `json:"opposed_numbers,omitempty"`

	// Bet lines
	BetLines []models.BetLine `json:"bet_lines"`

	// Issuer information
	IssuerType    string                `json:"issuer_type"`
	IssuerID      string                `json:"issuer_id"`
	IssuerDetails *models.IssuerDetails `json:"issuer_details,omitempty"`

	// Customer information (optional)
	CustomerPhone *string `json:"customer_phone,omitempty"`
	CustomerName  *string `json:"customer_name,omitempty"`
	CustomerEmail *string `json:"customer_email,omitempty"`

	// Payment
	PaymentMethod string  `json:"payment_method"`
	PaymentRef    *string `json:"payment_ref,omitempty"`
}

// ValidateTicketRequest represents a request to validate a ticket
type ValidateTicketRequest struct {
	TicketID         *uuid.UUID `json:"ticket_id,omitempty"`
	SerialNumber     *string    `json:"serial_number,omitempty"`
	ValidationMethod string     `json:"validation_method"`
	ValidatedByType  string     `json:"validated_by_type"`
	ValidatedByID    string     `json:"validated_by_id"`
	TerminalID       *string    `json:"terminal_id,omitempty"`
	IPAddress        *string    `json:"ip_address,omitempty"`
	UserAgent        *string    `json:"user_agent,omitempty"`
}

// ValidateTicketResponse represents the result of ticket validation
type ValidateTicketResponse struct {
	IsValid          bool           `json:"is_valid"`
	Ticket           *models.Ticket `json:"ticket,omitempty"`
	Message          string         `json:"message"`
	ValidationResult string         `json:"validation_result"`
}

// CancelTicketRequest represents a request to cancel a ticket
type CancelTicketRequest struct {
	TicketID        uuid.UUID `json:"ticket_id"`
	Reason          string    `json:"reason"`
	CancelledByType string    `json:"cancelled_by_type"`
	CancelledByID   string    `json:"cancelled_by_id"`
	RefundMethod    *string   `json:"refund_method,omitempty"`
}

// WinningCheckResult represents the result of checking if a ticket won
type WinningCheckResult struct {
	IsWinning     bool   `json:"is_winning"`
	Matches       int32  `json:"matches"`
	WinningAmount int64  `json:"winning_amount"` // in pesewas
	PrizeTier     string `json:"prize_tier"`
}

// MarkTicketAsPaidRequest represents a request to mark a ticket as paid
type MarkTicketAsPaidRequest struct {
	TicketID         uuid.UUID `json:"ticket_id"`
	PaidAmount       int64     `json:"paid_amount"` // in pesewas
	PaymentReference string    `json:"payment_reference"`
	PaidBy           string    `json:"paid_by"`
	DrawID           uuid.UUID `json:"draw_id"`
}

// MarkTicketAsPaidResponse represents the result of marking a ticket as paid
type MarkTicketAsPaidResponse struct {
	Success       bool                `json:"success"`
	Message       string              `json:"message"`
	PayoutDetails *TicketPayoutDetail `json:"payout_details,omitempty"`
}

// TicketPayoutDetail contains payout information for a ticket
type TicketPayoutDetail struct {
	TicketID         uuid.UUID `json:"ticket_id"`
	PaidAmount       int64     `json:"paid_amount"`
	PaymentReference string    `json:"payment_reference"`
	PaidBy           string    `json:"paid_by"`
	PaidAt           time.Time `json:"paid_at"`
}

// UpdateTicketStatusRequest represents a request to update a ticket's status
type UpdateTicketStatusRequest struct {
	TicketID      uuid.UUID `json:"ticket_id"`
	Status        string    `json:"status"` // "won" or "lost"
	WinningAmount int64     `json:"winning_amount"`
	Matches       int32     `json:"matches"`
	PrizeTier     string    `json:"prize_tier"`
	DrawID        uuid.UUID `json:"draw_id"`
}

// UpdateTicketStatusResponse represents the result of updating a ticket's status
type UpdateTicketStatusResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Ticket  *models.Ticket `json:"ticket,omitempty"`
}

// IssueTicket issues a new lottery ticket
func (s *ticketService) IssueTicket(ctx context.Context, req IssueTicketRequest) (*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.issue")
	defer span.End()

	span.SetAttributes(
		attribute.String("ticket.game_code", req.GameCode),
		attribute.Int("ticket.draw_number", int(req.DrawNumber)),
		attribute.String("ticket.issuer_type", req.IssuerType),
	)

	log.Printf("[DEBUG] IssueTicket request started - IssuerID: %s, IssuerType: %s, GameCode: %s, BetLines: %d",
		req.IssuerID, req.IssuerType, req.GameCode, len(req.BetLines))

	// Validate request
	log.Printf("[DEBUG] Validating issue ticket request")
	if err := s.validateIssueRequest(req); err != nil {
		log.Printf("[ERROR] Validation failed: %v", err)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "validation failed")
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	log.Printf("[DEBUG] Request validation successful")

	// Fetch game details from Game Service via game schedule
	var gameName, gameType string
	var scheduledDrawTime *time.Time
	if req.GameScheduleID != nil {
		log.Printf("[DEBUG] Fetching game details from Game Service - ScheduleID: %s", req.GameScheduleID.String())
		span.AddEvent("Fetching game details from Game Service")
		scheduleResp, err := s.gameClient.GetScheduleById(ctx, &gamev1.GetScheduleByIdRequest{
			ScheduleId: req.GameScheduleID.String(),
		})
		if err != nil {
			// Log warning but continue with empty game name
			log.Printf("[WARN] Failed to fetch game schedule from Game Service: %v", err)
			span.AddEvent(fmt.Sprintf("Failed to fetch game schedule: %v", err))
			gameName = ""
			gameType = "lottery" // Default to lottery
		} else if scheduleResp != nil && scheduleResp.Schedule != nil {
			gameName = scheduleResp.Schedule.GameName
			gameType = "lottery" // We can enhance this later if needed from the schedule

			if ts := scheduleResp.Schedule.GetScheduledDraw(); ts != nil {
				t := ts.AsTime()
				scheduledDrawTime = &t
			}

			log.Printf("[DEBUG] Game details fetched successfully - Name: %s, Type: %s", gameName, gameType)
			span.SetAttributes(
				attribute.String("game.name", gameName),
				attribute.String("game.type", gameType),
			)
		}
	} else {
		log.Printf("[DEBUG] No GameScheduleID provided, skipping Game Service lookup")
	}

	// Calculate total amount from bet lines
	var totalAmount int64
	var unitPrice int64
	for i, line := range req.BetLines {
		// Validate each bet line (basic validation)
		if err := line.Validate(); err != nil {
			return nil, fmt.Errorf("invalid bet line %d: %w", i+1, err)
		}

		// Additional validation for PERM bets
		if IsPermBet(line.BetType) {
			if err := s.ValidatePermBet(&line); err != nil {
				return nil, fmt.Errorf("invalid PERM bet line %d: %w", i+1, err)
			}
		}

		// Additional validation for Banker bets
		if IsBankerBet(line.BetType) {
			if err := s.ValidateBankerBet(&line); err != nil {
				return nil, fmt.Errorf("invalid Banker bet line %d: %w", i+1, err)
			}
		}

		// Get amount from compact format
		lineAmount := line.TotalAmount

		totalAmount += lineAmount
		if i == 0 {
			unitPrice = lineAmount // Assume first line amount as unit price
		}
	}

	log.Printf("[DEBUG] Calculated ticket amount - Total: %d pesewas (GHS %.2f), Lines: %d, UnitPrice: %d",
		totalAmount, float64(totalAmount)/100, len(req.BetLines), unitPrice)

	// Generate serial number
	log.Printf("[DEBUG] Generating serial number")
	serialNumber, err := s.generateSerialNumber(ctx)
	if err != nil {
		log.Printf("[ERROR] Failed to generate serial number: %v", err)
		span.RecordError(err)
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}
	log.Printf("[DEBUG] Serial number generated successfully: %s", serialNumber)

	// Generate security features
	log.Printf("[DEBUG] Generating security features for serial: %s", serialNumber)
	securityFeatures, err := s.generateSecurityFeatures(ctx, serialNumber, req.GameCode, req.DrawNumber)
	if err != nil {
		log.Printf("[ERROR] Failed to generate security features: %v", err)
		span.RecordError(err)
		return nil, fmt.Errorf("failed to generate security features: %w", err)
	}
	log.Printf("[DEBUG] Security features generated successfully - Hash: %s", securityFeatures.Hash[:16]+"...")

	// Extract selected numbers from bet lines if not provided
	selectedNumbers := req.SelectedNumbers
	if len(selectedNumbers) == 0 && len(req.BetLines) > 0 {
		// For Direct bets, preserve order by using the first bet line's numbers directly
		// For other bet types, collect unique numbers while preserving order of first appearance
		seenNumbers := make(map[int32]bool)
		selectedNumbers = make([]int32, 0)

		for _, line := range req.BetLines {
			numbers := line.SelectedNumbers
			for _, num := range numbers {
				if !seenNumbers[num] {
					selectedNumbers = append(selectedNumbers, num)
					seenNumbers[num] = true
				}
			}
		}
	}

	// Extract banker and opposed numbers from bet lines if not provided
	// Preserve order of first appearance
	bankerNumbers := req.BankerNumbers
	if len(bankerNumbers) == 0 && len(req.BetLines) > 0 {
		seenBanker := make(map[int32]bool)
		bankerNumbers = make([]int32, 0)

		for _, line := range req.BetLines {
			for _, num := range line.Banker {
				if !seenBanker[num] {
					bankerNumbers = append(bankerNumbers, num)
					seenBanker[num] = true
				}
			}
		}
	}

	opposedNumbers := req.OpposedNumbers
	if len(opposedNumbers) == 0 && len(req.BetLines) > 0 {
		seenOpposed := make(map[int32]bool)
		opposedNumbers = make([]int32, 0)

		for _, line := range req.BetLines {
			for _, num := range line.Opposed {
				if !seenOpposed[num] {
					opposedNumbers = append(opposedNumbers, num)
					seenOpposed[num] = true
				}
			}
		}
	}

	// Create ticket model
	now := time.Now()
	paymentStatus := "completed" // Default for immediate payment
	ticket := &models.Ticket{
		SerialNumber:   serialNumber,
		GameCode:       req.GameCode,
		GameScheduleID: req.GameScheduleID,
		DrawNumber:     req.DrawNumber,
		GameName:       gameName, // Fetched from Game Service
		GameType:       gameType, // Fetched from Game Service

		SelectedNumbers: selectedNumbers,
		BankerNumbers:   bankerNumbers,
		OpposedNumbers:  opposedNumbers,

		BetLines:      req.BetLines,
		NumberOfLines: int32(len(req.BetLines)),
		UnitPrice:     unitPrice,
		TotalAmount:   totalAmount,

		IssuerType:    req.IssuerType,
		IssuerID:      req.IssuerID,
		IssuerDetails: req.IssuerDetails,

		CustomerPhone: req.CustomerPhone,
		CustomerName:  req.CustomerName,
		CustomerEmail: req.CustomerEmail,

		PaymentMethod: &req.PaymentMethod,
		PaymentRef:    req.PaymentRef,
		PaymentStatus: &paymentStatus,

		SecurityHash: securityFeatures.Hash,
		SecurityFeatures: &models.SecurityFeatures{
			QRCode:           securityFeatures.QRCode,
			Barcode:          securityFeatures.Barcode,
			VerificationCode: securityFeatures.VerificationCode,
		},

		Status:        string(models.TicketStatusIssued),
		IsWinning:     false,
		WinningAmount: 0,
		Matches:       0,

		// Draw information
		// Persist scheduled draw datetime so admin views can show Draw Time.
		DrawDate: scheduledDrawTime,
		DrawTime: scheduledDrawTime,

		IssuedAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	log.Printf("[DEBUG] Ticket model created - Serial: %s, TotalAmount: %d, Status: %s",
		serialNumber, totalAmount, ticket.Status)

	// Validate business rules
	log.Printf("[DEBUG] Validating business rules")
	if err := ticket.ValidateBusinessRules(); err != nil {
		log.Printf("[ERROR] Business rules validation failed: %v", err)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "business rules validation failed")
		return nil, fmt.Errorf("business rules validation failed: %w", err)
	}
	log.Printf("[DEBUG] Business rules validation successful")

	// If issuer is a retailer, debit their stake wallet BEFORE creating the ticket
	if req.IssuerType == "retailer" {
		log.Printf("[DEBUG] Issuer is retailer - debiting stake wallet. IssuerID: %s, Amount: %d",
			req.IssuerID, totalAmount)
		span.AddEvent("Debiting retailer wallet")

		// Call wallet service to debit retailer's stake wallet with retry logic
		debitReq := &walletpb.DebitRetailerWalletRequest{
			RetailerId: req.IssuerID,
			WalletType: walletpb.WalletType_RETAILER_STAKE,
			Amount:     float64(totalAmount), // Amount in pesewas
			Reference:  serialNumber,
			Reason:     fmt.Sprintf("Ticket purchase - %s", serialNumber),
		}

		var debitResp *walletpb.DebitRetailerWalletResponse
		err := retryWithExponentialBackoff(ctx, 3, func() error {
			resp, err := s.walletClient.DebitRetailerWallet(ctx, debitReq)
			if err != nil {
				// Check error type - don't retry business logic errors
				if st, ok := status.FromError(err); ok {
					switch st.Code() {
					case codes.FailedPrecondition, // Insufficient funds
						codes.InvalidArgument, // Invalid input
						codes.NotFound:        // Wallet not found
						// These are permanent errors - don't retry
						return &permanentError{err}
					case codes.Unavailable, // Service unavailable
						codes.DeadlineExceeded, // Timeout
						codes.Internal:         // Internal error (might be transient)
						// These are transient errors - retry
						span.AddEvent(fmt.Sprintf("Transient error, will retry: %s", st.Message()))
						return err
					default:
						// Unknown error code - don't retry to be safe
						return &permanentError{err}
					}
				}
				// Non-gRPC error - don't retry
				return &permanentError{err}
			}
			debitResp = resp
			return nil
		})

		if err != nil {
			log.Printf("[ERROR] Wallet debit failed after retries: %v", err)
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "wallet debit failed")

			// Check if it's a gRPC error with message
			if st, ok := status.FromError(err); ok {
				return nil, fmt.Errorf("failed to debit wallet: %s", st.Message())
			}
			return nil, fmt.Errorf("failed to debit retailer wallet: %w", err)
		}

		if !debitResp.Success {
			log.Printf("[ERROR] Wallet debit unsuccessful - Message: %s", debitResp.Message)
			err := fmt.Errorf("wallet debit unsuccessful: %s", debitResp.Message)
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "wallet debit unsuccessful")
			return nil, err
		}

		log.Printf("[DEBUG] Wallet debit successful - TransactionID: %s, NewBalance: %.0f",
			debitResp.TransactionId, debitResp.NewBalance)

		span.SetAttributes(
			attribute.String("wallet.transaction_id", debitResp.TransactionId),
			attribute.Float64("wallet.debited_amount", debitResp.DebitedAmount),
			attribute.Float64("wallet.new_balance", debitResp.NewBalance),
		)

		span.AddEvent("Retailer wallet debited successfully")
	}

	// If issuer is a player, debit their wallet BEFORE creating the ticket
	if req.IssuerType == "player" {
		span.AddEvent("Debiting player wallet")

		debitReq := &walletpb.DebitPlayerWalletRequest{
			PlayerId:       req.IssuerID,
			Amount:         float64(totalAmount),
			Reference:      serialNumber,
			Reason:         fmt.Sprintf("Ticket purchase - %s", serialNumber),
			IdempotencyKey: serialNumber,
		}

		var debitResp *walletpb.DebitPlayerWalletResponse
		err := retryWithExponentialBackoff(ctx, 3, func() error {
			resp, err := s.walletClient.DebitPlayerWallet(ctx, debitReq)
			if err != nil {
				// Check error type - don't retry business logic errors
				if st, ok := status.FromError(err); ok {
					switch st.Code() {
					case codes.FailedPrecondition, // Insufficient funds
						codes.InvalidArgument, // Invalid input
						codes.NotFound:        // Wallet not found
						return &permanentError{err}
					case codes.Unavailable, // Service unavailable
						codes.DeadlineExceeded, // Timeout
						codes.Internal:         // Internal error (might be transient)
						span.AddEvent(fmt.Sprintf("Transient error, will retry: %s", st.Message()))
						return err
					default:
						return &permanentError{err}
					}
				}
				return &permanentError{err}
			}
			debitResp = resp
			return nil
		})

		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "wallet debit failed")

			if st, ok := status.FromError(err); ok {
				return nil, fmt.Errorf("failed to debit wallet: %s", st.Message())
			}
			return nil, fmt.Errorf("failed to debit player wallet: %w", err)
		}

		if !debitResp.Success {
			err := fmt.Errorf("wallet debit unsuccessful: %s", debitResp.Message)
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "wallet debit unsuccessful")
			return nil, err
		}

		span.SetAttributes(
			attribute.String("wallet.transaction_id", debitResp.TransactionId),
			attribute.Float64("wallet.debited_amount", debitResp.DebitedAmount),
			attribute.Float64("wallet.new_balance", debitResp.NewBalance),
		)

		span.AddEvent("Player wallet debited successfully")
	}

	// Begin transaction
	log.Printf("[DEBUG] Beginning database transaction for ticket creation")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to begin transaction: %v", err)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Ignore error as it's expected after successful commit
	}()

	// Create ticket in database
	log.Printf("[DEBUG] Creating ticket in database - Serial: %s", serialNumber)
	if err := s.ticketRepo.CreateWithTx(ctx, tx, ticket); err != nil {
		log.Printf("[ERROR] Failed to create ticket in database: %v", err)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to create ticket")
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}
	log.Printf("[DEBUG] Ticket created in database successfully - ID: %s", ticket.ID.String())

	// Commit transaction
	log.Printf("[DEBUG] Committing database transaction")
	if err := tx.Commit(); err != nil {
		log.Printf("[ERROR] Failed to commit transaction: %v", err)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	log.Printf("[DEBUG] Transaction committed successfully")

	// TODO: Publish ticket.issued event to Kafka
	// event := events.NewTicketIssuedEvent(ticket)
	// s.eventBus.Publish(ctx, "ticket.events", event)

	span.SetAttributes(
		attribute.String("ticket.id", ticket.ID.String()),
		attribute.String("ticket.serial_number", ticket.SerialNumber),
	)

	log.Printf("[DEBUG] Ticket issued successfully - ID: %s, Serial: %s, Amount: %d pesewas, GameName: %s",
		ticket.ID.String(), serialNumber, totalAmount, gameName)

	return ticket, nil
}

// GetTicket retrieves a ticket by ID
func (s *ticketService) GetTicket(ctx context.Context, id uuid.UUID) (*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.get")
	defer span.End()

	span.SetAttributes(attribute.String("ticket.id", id.String()))

	ticket, err := s.ticketRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get ticket")
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	return ticket, nil
}

// GetTicketBySerial retrieves a ticket by serial number
func (s *ticketService) GetTicketBySerial(ctx context.Context, serialNumber string) (*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.get_by_serial")
	defer span.End()

	span.SetAttributes(attribute.String("ticket.serial_number", serialNumber))

	ticket, err := s.ticketRepo.GetBySerial(ctx, serialNumber)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get ticket by serial")
		return nil, fmt.Errorf("failed to get ticket by serial: %w", err)
	}

	return ticket, nil
}

// ValidateTicket validates a ticket (marks as validated and checks eligibility)
func (s *ticketService) ValidateTicket(ctx context.Context, req ValidateTicketRequest) (*ValidateTicketResponse, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.validate")
	defer span.End()

	span.SetAttributes(
		attribute.String("validation.method", req.ValidationMethod),
		attribute.String("validated_by.type", req.ValidatedByType),
	)

	// Get ticket by ID or serial number
	var ticket *models.Ticket
	var err error

	if req.TicketID != nil {
		ticket, err = s.ticketRepo.GetByID(ctx, *req.TicketID)
		span.SetAttributes(attribute.String("ticket.id", req.TicketID.String()))
	} else if req.SerialNumber != nil {
		ticket, err = s.ticketRepo.GetBySerial(ctx, *req.SerialNumber)
		span.SetAttributes(attribute.String("ticket.serial_number", *req.SerialNumber))
	} else {
		err := fmt.Errorf("either ticket_id or serial_number must be provided")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid request")
		return &ValidateTicketResponse{
			IsValid:          false,
			Message:          "Invalid validation request",
			ValidationResult: string(models.ValidationResultInvalid),
		}, nil
	}

	if err != nil {
		span.RecordError(err)
		return &ValidateTicketResponse{
			IsValid:          false,
			Message:          "Ticket not found",
			ValidationResult: string(models.ValidationResultInvalid),
		}, nil
	}

	// Check if ticket can be validated
	if !ticket.CanBeValidated() {
		validationResult := s.determineValidationResult(ticket)
		return &ValidateTicketResponse{
			IsValid:          false,
			Ticket:           ticket,
			Message:          s.getValidationMessage(validationResult),
			ValidationResult: validationResult,
		}, nil
	}

	// Check if ticket has expired
	if ticket.IsExpired() {
		return &ValidateTicketResponse{
			IsValid:          false,
			Ticket:           ticket,
			Message:          "Ticket has expired",
			ValidationResult: string(models.ValidationResultExpired),
		}, nil
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Update ticket status to validated
	if err := ticket.SetStatus(models.TicketStatusValidated); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to set ticket status: %w", err)
	}

	if err := s.ticketRepo.UpdateWithTx(ctx, tx, ticket); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Record validation in ticket_validations table
	validationRecord := &models.TicketValidation{
		ID:               uuid.New(),
		TicketID:         ticket.ID,
		ValidatedByType:  req.ValidatedByType,
		ValidatedByID:    req.ValidatedByID,
		ValidationMethod: req.ValidationMethod,
		ValidationResult: string(models.ValidationResultValid),
		TerminalID:       req.TerminalID,
		IPAddress:        req.IPAddress,
		UserAgent:        req.UserAgent,
		ValidatedAt:      time.Now(),
		CreatedAt:        time.Now(),
	}

	// Insert validation record
	if err := s.insertValidationRecord(ctx, tx, validationRecord); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to insert validation record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// TODO: Publish ticket.validated event
	// event := events.NewTicketValidatedEvent(ticket)
	// s.eventBus.Publish(ctx, "ticket.events", event)

	return &ValidateTicketResponse{
		IsValid:          true,
		Ticket:           ticket,
		Message:          "Ticket validated successfully",
		ValidationResult: string(models.ValidationResultValid),
	}, nil
}

// CancelTicket cancels a ticket and initiates refund
func (s *ticketService) CancelTicket(ctx context.Context, req CancelTicketRequest) (*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.cancel")
	defer span.End()

	span.SetAttributes(
		attribute.String("ticket.id", req.TicketID.String()),
		attribute.String("cancelled_by.type", req.CancelledByType),
	)

	// Get ticket
	ticket, err := s.ticketRepo.GetByID(ctx, req.TicketID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "ticket not found")
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	// Check if ticket can be cancelled
	if !ticket.CanBeCancelled() {
		err := fmt.Errorf("ticket with status '%s' cannot be cancelled", ticket.Status)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "ticket cannot be cancelled")
		return nil, err
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Update ticket status
	if err := ticket.SetStatus(models.TicketStatusCancelled); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to set ticket status: %w", err)
	}

	if err := s.ticketRepo.UpdateWithTx(ctx, tx, ticket); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Create cancellation record
	refundStatus := "pending"
	cancellation := &models.TicketCancellation{
		ID:              uuid.New(),
		TicketID:        ticket.ID,
		Reason:          req.Reason,
		CancelledByType: req.CancelledByType,
		CancelledByID:   req.CancelledByID,
		RefundAmount:    ticket.TotalAmount,
		RefundMethod:    req.RefundMethod,
		RefundStatus:    &refundStatus,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Insert cancellation record
	if err := s.insertCancellationRecord(ctx, tx, cancellation); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to insert cancellation record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// TODO: Publish ticket.cancelled event
	// TODO: Trigger refund process via payment service

	return ticket, nil
}

// ListTickets retrieves tickets with filtering and pagination
func (s *ticketService) ListTickets(ctx context.Context, filter models.TicketFilter, page, limit int) ([]*models.Ticket, int64, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.list")
	defer span.End()

	span.SetAttributes(
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.limit", limit),
	)

	// Validate pagination
	if page < 1 {
		page = 1
	}
	// Cap limit at 100 for general API/UI calls to prevent performance issues
	if limit < 1 {
		limit = 20 // Default page size
	} else if limit > 100 {
		limit = 100 // Max page size for general listing
	}

	tickets, total, err := s.ticketRepo.List(ctx, filter, page, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to list tickets")
		return nil, 0, fmt.Errorf("failed to list tickets: %w", err)
	}

	span.SetAttributes(
		attribute.Int("tickets.count", len(tickets)),
		attribute.Int64("tickets.total", total),
	)

	return tickets, total, nil
}

// GetAllTicketsForDraw retrieves all tickets for a specific draw without pagination limits.
// This method is specifically designed for internal service-to-service calls (e.g., draw processing)
// where we need to process ALL tickets for winner calculation.
func (s *ticketService) GetAllTicketsForDraw(ctx context.Context, drawID uuid.UUID, status string) ([]*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.get_all_for_draw")
	defer span.End()

	span.SetAttributes(
		attribute.String("draw.id", drawID.String()),
		attribute.String("ticket.status", status),
	)

	// Build filter for the specific draw
	drawIDStr := drawID.String()
	filter := models.TicketFilter{
		DrawID: &drawIDStr,
	}
	if status != "" {
		filter.Status = &status
	}

	// Use a high page size (10000) to get all tickets in one call
	// This is safe because:
	// 1. We're filtering by draw_id, which naturally limits the result set
	// 2. This is an internal service-to-service call, not exposed to external APIs
	// 3. We need ALL tickets for accurate winner calculation
	const bulkLimit = 10000

	tickets, total, err := s.ticketRepo.List(ctx, filter, 1, bulkLimit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get tickets for draw")
		return nil, fmt.Errorf("failed to get tickets for draw: %w", err)
	}

	// Log warning if we hit the limit (indicating there might be more tickets)
	if total > int64(bulkLimit) {
		fmt.Printf("[WARN] Draw %s has %d tickets, but only fetched %d (limit reached). Consider pagination or increasing limit.\n",
			drawID.String(), total, bulkLimit)
	}

	span.SetAttributes(
		attribute.Int("tickets.count", len(tickets)),
		attribute.Int64("tickets.total", total),
	)

	return tickets, nil
}

// CheckWinning checks if a ticket is a winner against winning numbers
func (s *ticketService) CheckWinning(ctx context.Context, ticketID uuid.UUID, winningNumbers []int32) (*WinningCheckResult, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.check_winning")
	defer span.End()

	span.SetAttributes(attribute.String("ticket.id", ticketID.String()))

	// Get ticket
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	// Check matches
	matches := s.countMatches(ticket.SelectedNumbers, winningNumbers)

	// TODO: Get prize structure from game service to determine winning amount and tier
	// For now, use simple logic
	var isWinning bool
	var winningAmount int64
	var prizeTier string

	if matches >= 3 {
		isWinning = true
		// Simple prize calculation (should be fetched from game service)
		switch matches {
		case 3:
			prizeTier = "tier_5"
			winningAmount = ticket.TotalAmount * 10
		case 4:
			prizeTier = "tier_4"
			winningAmount = ticket.TotalAmount * 50
		case 5:
			prizeTier = "tier_3"
			winningAmount = ticket.TotalAmount * 500
		}
	}

	result := &WinningCheckResult{
		IsWinning:     isWinning,
		Matches:       matches,
		WinningAmount: winningAmount,
		PrizeTier:     prizeTier,
	}

	// Update ticket if winning
	if isWinning {
		ticket.IsWinning = true
		ticket.WinningAmount = winningAmount
		ticket.Matches = matches
		ticket.PrizeTier = &prizeTier
		if err := ticket.SetStatus(models.TicketStatusWon); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to set winning status: %w", err)
		}
		if err := s.ticketRepo.Update(ctx, ticket); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to update ticket: %w", err)
		}
	}

	return result, nil
}

// Private helper methods

// validateIssueRequest validates the ticket issue request
func (s *ticketService) validateIssueRequest(req IssueTicketRequest) error {
	if req.GameCode == "" {
		return fmt.Errorf("game code is required")
	}
	if req.GameScheduleID == nil {
		return fmt.Errorf("game schedule ID is required")
	}
	if req.DrawNumber < 1 {
		return fmt.Errorf("draw number must be positive")
	}
	if len(req.BetLines) == 0 {
		return fmt.Errorf("at least one bet line is required")
	}
	if req.IssuerType == "" {
		return fmt.Errorf("issuer type is required")
	}
	if req.IssuerID == "" {
		return fmt.Errorf("issuer ID is required")
	}
	if req.PaymentMethod == "" {
		return fmt.Errorf("payment method is required")
	}
	return nil
}

// generateSerialNumber generates a unique serial number for the ticket with retry logic
func (s *ticketService) generateSerialNumber(ctx context.Context) (string, error) {
	_, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.generate_serial")
	defer span.End()

	prefix := s.securityConfig.SerialPrefix
	if prefix == "" {
		prefix = "TKT"
	}

	// Retry up to 5 times to generate a unique serial number
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Generate random 8-digit number
		max := big.NewInt(99999999)
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			span.RecordError(err)
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}

		serialNumber := fmt.Sprintf("%s-%08d", prefix, n.Int64())

		// Check if serial number already exists
		_, err = s.ticketRepo.GetBySerial(ctx, serialNumber)
		if err != nil {
			// If not found (error), the serial number is unique
			if err.Error() == "ticket not found" {
				span.SetAttributes(
					attribute.String("serial.number", serialNumber),
					attribute.Int("attempts", attempt+1),
				)
				return serialNumber, nil
			}
			// Other errors should be returned
			span.RecordError(err)
			return "", fmt.Errorf("failed to check serial number existence: %w", err)
		}

		// Serial number exists, retry
		span.AddEvent(fmt.Sprintf("Serial number collision detected on attempt %d, retrying...", attempt+1))
	}

	// Failed to generate unique serial after max retries
	err := fmt.Errorf("failed to generate unique serial number after %d attempts", maxRetries)
	span.RecordError(err)
	span.SetStatus(otelcodes.Error, "serial generation failed")
	return "", err
}

// SecurityFeaturesGenerated holds generated security features
type SecurityFeaturesGenerated struct {
	Hash             string
	QRCode           string
	Barcode          string
	VerificationCode string
}

// generateSecurityFeatures generates security features for the ticket
func (s *ticketService) generateSecurityFeatures(ctx context.Context, serialNumber, gameCode string, drawNumber int32) (*SecurityFeaturesGenerated, error) {
	_, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.generate_security")
	defer span.End()

	// Generate security hash (SHA-256 of ticket data + secret)
	hashData := fmt.Sprintf("%s:%s:%d:%s:%d",
		serialNumber, gameCode, drawNumber, s.securityConfig.SecretKey, time.Now().Unix())
	hash := sha256.Sum256([]byte(hashData))
	securityHash := hex.EncodeToString(hash[:])

	// Generate QR code data (URL that can be scanned to verify ticket)
	qrCodeBaseURL := s.securityConfig.QRCodeBaseURL
	if qrCodeBaseURL == "" {
		qrCodeBaseURL = "https://verify.randlottery.com"
	}
	qrCode := fmt.Sprintf("%s/ticket/%s", qrCodeBaseURL, serialNumber)

	// Generate barcode (Code 128 format)
	barcodePrefix := s.securityConfig.BarcodePrefix
	if barcodePrefix == "" {
		barcodePrefix = "RLT"
	}
	barcode := fmt.Sprintf("%s%s", barcodePrefix, serialNumber)

	// Generate verification code (6-digit PIN)
	max := big.NewInt(999999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return nil, fmt.Errorf("failed to generate verification code: %w", err)
	}
	verificationCode := fmt.Sprintf("%06d", n.Int64())

	return &SecurityFeaturesGenerated{
		Hash:             securityHash,
		QRCode:           qrCode,
		Barcode:          barcode,
		VerificationCode: verificationCode,
	}, nil
}

// countMatches counts how many numbers match between ticket and winning numbers
func (s *ticketService) countMatches(ticketNumbers, winningNumbers []int32) int32 {
	matches := int32(0)
	winningSet := make(map[int32]bool)
	for _, num := range winningNumbers {
		winningSet[num] = true
	}
	for _, num := range ticketNumbers {
		if winningSet[num] {
			matches++
		}
	}
	return matches
}

// determineValidationResult determines the validation result based on ticket status
func (s *ticketService) determineValidationResult(ticket *models.Ticket) string {
	if ticket.VoidedAt != nil {
		return string(models.ValidationResultVoid)
	}
	if ticket.CancelledAt != nil {
		return string(models.ValidationResultCancelled)
	}
	if ticket.PaidAt != nil {
		return string(models.ValidationResultAlreadyPaid)
	}
	if ticket.IsExpired() {
		return string(models.ValidationResultExpired)
	}
	if ticket.Status == string(models.TicketStatusValidated) {
		return string(models.ValidationResultAlreadyValidated)
	}
	return string(models.ValidationResultInvalid)
}

// getValidationMessage returns a user-friendly validation message
func (s *ticketService) getValidationMessage(validationResult string) string {
	messages := map[string]string{
		string(models.ValidationResultValid):            "Ticket is valid",
		string(models.ValidationResultInvalid):          "Ticket is invalid",
		string(models.ValidationResultExpired):          "Ticket has expired",
		string(models.ValidationResultAlreadyPaid):      "Ticket has already been paid",
		string(models.ValidationResultCancelled):        "Ticket has been cancelled",
		string(models.ValidationResultVoid):             "Ticket has been voided",
		string(models.ValidationResultAlreadyValidated): "Ticket has already been validated",
	}
	if msg, ok := messages[validationResult]; ok {
		return msg
	}
	return "Unknown validation result"
}

// insertValidationRecord inserts a validation record into the database
func (s *ticketService) insertValidationRecord(ctx context.Context, tx *sql.Tx, record *models.TicketValidation) error {
	query := `
		INSERT INTO ticket_validations (
			id, ticket_id, validated_by_type, validated_by_id,
			validation_method, validation_result, validation_notes,
			terminal_id, ip_address, user_agent, validated_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := tx.ExecContext(ctx, query,
		record.ID, record.TicketID, record.ValidatedByType, record.ValidatedByID,
		record.ValidationMethod, record.ValidationResult, record.ValidationNotes,
		record.TerminalID, record.IPAddress, record.UserAgent,
		record.ValidatedAt, record.CreatedAt,
	)
	return err
}

// insertCancellationRecord inserts a cancellation record into the database
func (s *ticketService) insertCancellationRecord(ctx context.Context, tx *sql.Tx, record *models.TicketCancellation) error {
	query := `
		INSERT INTO ticket_cancellations (
			id, ticket_id, reason, cancelled_by_type, cancelled_by_id,
			refund_amount, refund_method, refund_status, refund_ref, refund_notes,
			approved_by, approved_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := tx.ExecContext(ctx, query,
		record.ID, record.TicketID, record.Reason, record.CancelledByType, record.CancelledByID,
		record.RefundAmount, record.RefundMethod, record.RefundStatus, record.RefundRef, record.RefundNotes,
		record.ApprovedBy, record.ApprovedAt, record.CreatedAt, record.UpdatedAt,
	)
	return err
}

// Retry helpers

// permanentError wraps an error to indicate it should not be retried
type permanentError struct {
	err error
}

func (e *permanentError) Error() string {
	return e.err.Error()
}

func (e *permanentError) Unwrap() error {
	return e.err
}

// retryWithExponentialBackoff retries a function with exponential backoff
// maxRetries: maximum number of retry attempts
// fn: function to retry
func retryWithExponentialBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if it's a permanent error
		var permErr *permanentError
		if errors, ok := err.(interface{ Unwrap() error }); ok {
			if errors.Unwrap() != nil {
				permErr, _ = err.(*permanentError)
			}
		}

		if permErr != nil {
			// Don't retry permanent errors
			return permErr.err
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt == maxRetries {
			break
		}

		// Exponential backoff: 100ms * 2^attempt (100ms, 200ms, 400ms)
		backoff := time.Duration(100*(1<<uint(attempt))) * time.Millisecond

		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}

// MarkTicketAsPaid marks a winning ticket as paid
func (s *ticketService) MarkTicketAsPaid(ctx context.Context, req MarkTicketAsPaidRequest) (*MarkTicketAsPaidResponse, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.mark_as_paid")
	defer span.End()

	span.SetAttributes(
		attribute.String("ticket.id", req.TicketID.String()),
		attribute.Int64("paid.amount", req.PaidAmount),
		attribute.String("payment.reference", req.PaymentReference),
		attribute.String("paid.by", req.PaidBy),
		attribute.String("draw.id", req.DrawID.String()),
	)

	// Validate request
	if req.TicketID == uuid.Nil {
		err := fmt.Errorf("ticket_id is required")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid request")
		return &MarkTicketAsPaidResponse{
			Success: false,
			Message: "ticket_id is required",
		}, nil
	}

	if req.PaidAmount <= 0 {
		err := fmt.Errorf("paid_amount must be positive")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid request")
		return &MarkTicketAsPaidResponse{
			Success: false,
			Message: "paid_amount must be positive",
		}, nil
	}

	if req.PaymentReference == "" {
		err := fmt.Errorf("payment_reference is required")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid request")
		return &MarkTicketAsPaidResponse{
			Success: false,
			Message: "payment_reference is required",
		}, nil
	}

	// Get ticket
	ticket, err := s.ticketRepo.GetByID(ctx, req.TicketID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "ticket not found")
		return &MarkTicketAsPaidResponse{
			Success: false,
			Message: fmt.Sprintf("ticket not found: %v", err),
		}, nil
	}

	// Validate ticket is in "won" status
	if ticket.Status != string(models.TicketStatusWon) {
		err := fmt.Errorf("ticket status is '%s', expected 'won'", ticket.Status)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket status")
		return &MarkTicketAsPaidResponse{
			Success: false,
			Message: fmt.Sprintf("ticket must be in 'won' status, current status: %s", ticket.Status),
		}, nil
	}

	// Validate paid amount matches winning amount
	if req.PaidAmount != ticket.WinningAmount {
		err := fmt.Errorf("paid amount (%d) does not match winning amount (%d)", req.PaidAmount, ticket.WinningAmount)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "amount mismatch")
		return &MarkTicketAsPaidResponse{
			Success: false,
			Message: fmt.Sprintf("paid amount (%d pesewas) does not match winning amount (%d pesewas)", req.PaidAmount, ticket.WinningAmount),
		}, nil
	}

	// Check if already paid
	if ticket.PaidAt != nil {
		span.AddEvent("ticket already paid")
		return &MarkTicketAsPaidResponse{
			Success: false,
			Message: "ticket has already been paid",
		}, nil
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Update ticket with payout information
	now := time.Now()
	ticket.PaidAt = &now
	ticket.PaidBy = &req.PaidBy
	ticket.PaymentReference = &req.PaymentReference

	// Set status to paid
	if err := ticket.SetStatus(models.TicketStatusPaid); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to set status")
		return nil, fmt.Errorf("failed to set ticket status to paid: %w", err)
	}

	// Update ticket in database
	if err := s.ticketRepo.UpdateWithTx(ctx, tx, ticket); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to update ticket")
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// TODO: Publish ticket.paid event to Kafka
	// event := events.NewTicketPaidEvent(ticket, req.DrawID)
	// s.eventBus.Publish(ctx, "ticket.events", event)

	span.SetStatus(otelcodes.Ok, "ticket marked as paid")

	return &MarkTicketAsPaidResponse{
		Success: true,
		Message: "ticket successfully marked as paid",
		PayoutDetails: &TicketPayoutDetail{
			TicketID:         ticket.ID,
			PaidAmount:       req.PaidAmount,
			PaymentReference: req.PaymentReference,
			PaidBy:           req.PaidBy,
			PaidAt:           now,
		},
	}, nil
}

// UpdateTicketStatus updates a ticket's status after draw processing (won/lost)
func (s *ticketService) UpdateTicketStatus(ctx context.Context, req UpdateTicketStatusRequest) (*UpdateTicketStatusResponse, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.update_status")
	defer span.End()

	span.SetAttributes(
		attribute.String("ticket.id", req.TicketID.String()),
		attribute.String("status", req.Status),
		attribute.Int64("winning.amount", req.WinningAmount),
		attribute.String("draw.id", req.DrawID.String()),
	)

	// Validate request
	if req.TicketID == uuid.Nil {
		err := fmt.Errorf("ticket_id is required")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid request")
		return &UpdateTicketStatusResponse{
			Success: false,
			Message: "ticket_id is required",
		}, nil
	}

	if req.Status != "won" && req.Status != "lost" {
		err := fmt.Errorf("status must be 'won' or 'lost', got: %s", req.Status)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid status")
		return &UpdateTicketStatusResponse{
			Success: false,
			Message: fmt.Sprintf("status must be 'won' or 'lost', got: %s", req.Status),
		}, nil
	}

	if req.DrawID == uuid.Nil {
		err := fmt.Errorf("draw_id is required")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid request")
		return &UpdateTicketStatusResponse{
			Success: false,
			Message: "draw_id is required",
		}, nil
	}

	// Get ticket
	ticket, err := s.ticketRepo.GetByID(ctx, req.TicketID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "ticket not found")
		return &UpdateTicketStatusResponse{
			Success: false,
			Message: fmt.Sprintf("ticket not found: %v", err),
		}, nil
	}

	// Idempotency check: if ticket already has the desired status, return success
	if (req.Status == "won" && ticket.Status == string(models.TicketStatusWon)) ||
		(req.Status == "lost" && ticket.Status == string(models.TicketStatusLost)) {
		span.SetAttributes(attribute.Bool("idempotent", true))
		span.SetStatus(otelcodes.Ok, "ticket already in desired status")
		return &UpdateTicketStatusResponse{
			Success: true,
			Message: fmt.Sprintf("ticket already has status '%s' (idempotent operation)", req.Status),
			Ticket:  ticket,
		}, nil
	}

	// Validate ticket is in "issued" status (or already in the desired final status)
	if ticket.Status != string(models.TicketStatusIssued) {
		err := fmt.Errorf("ticket status is '%s', expected 'issued' or already '%s'", ticket.Status, req.Status)
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid ticket status")
		return &UpdateTicketStatusResponse{
			Success: false,
			Message: fmt.Sprintf("ticket must be in 'issued' status, current status: %s", ticket.Status),
		}, nil
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Update ticket based on status
	if req.Status == "won" {
		ticket.IsWinning = true
		ticket.WinningAmount = req.WinningAmount
		ticket.Matches = req.Matches
		if req.PrizeTier != "" {
			ticket.PrizeTier = &req.PrizeTier
		}

		// Set status to won
		if err := ticket.SetStatus(models.TicketStatusWon); err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "failed to set status")
			return nil, fmt.Errorf("failed to set ticket status to won: %w", err)
		}
	} else {
		// Status is "lost"
		ticket.IsWinning = false
		ticket.WinningAmount = 0
		ticket.Matches = req.Matches

		// Set status to lost
		if err := ticket.SetStatus(models.TicketStatusLost); err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "failed to set status")
			return nil, fmt.Errorf("failed to set ticket status to lost: %w", err)
		}
	}

	// Store draw ID for audit trail
	ticket.DrawID = &req.DrawID

	// Update ticket in database
	if err := s.ticketRepo.UpdateWithTx(ctx, tx, ticket); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to update ticket")
		return nil, fmt.Errorf("failed to update ticket: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// TODO: Publish ticket.status_updated event to Kafka
	// event := events.NewTicketStatusUpdatedEvent(ticket, req.DrawID, req.Status)
	// s.eventBus.Publish(ctx, "ticket.events", event)

	span.SetStatus(otelcodes.Ok, "ticket status updated")

	return &UpdateTicketStatusResponse{
		Success: true,
		Message: fmt.Sprintf("ticket status successfully updated to '%s'", req.Status),
		Ticket:  ticket,
	}, nil
}

// UpdateTicketStatusesRequest represents a batch update request
type UpdateTicketStatusesRequest struct {
	Updates []TicketStatusUpdate
	DrawID  string
}

// TicketStatusUpdate represents a single ticket status update
type TicketStatusUpdate struct {
	TicketID      string
	Status        string
	WinningAmount int64
	Matches       int32
	PrizeTier     string
}

// TicketUpdateResult represents the result of updating a single ticket
type TicketUpdateResult struct {
	TicketID string
	Success  bool
	Message  string
}

// UpdateTicketStatusesResponse contains the batch update results
type UpdateTicketStatusesResponse struct {
	TotalRequested int64
	Successful     int64
	Failed         int64
	Results        []TicketUpdateResult
	Message        string
}

// UpdateTicketStatuses updates multiple tickets' statuses in a batch operation
// This is optimized for draw processing to avoid N+1 query problems
func (s *ticketService) UpdateTicketStatuses(ctx context.Context, req *UpdateTicketStatusesRequest) (*UpdateTicketStatusesResponse, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.update_statuses_batch")
	defer span.End()

	span.SetAttributes(
		attribute.Int("batch_size", len(req.Updates)),
		attribute.String("draw_id", req.DrawID),
	)

	response := &UpdateTicketStatusesResponse{
		TotalRequested: int64(len(req.Updates)),
		Successful:     0,
		Failed:         0,
		Results:        []TicketUpdateResult{},
	}

	// Validate request
	if len(req.Updates) == 0 {
		err := fmt.Errorf("no ticket updates provided")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "empty batch")
		response.Message = "no ticket updates provided"
		return response, nil
	}

	// Begin transaction for atomicity across all updates
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Process each update
	for _, update := range req.Updates {
		// Parse ticket ID
		ticketID, err := uuid.Parse(update.TicketID)
		if err != nil {
			result := TicketUpdateResult{
				TicketID: update.TicketID,
				Success:  false,
				Message:  fmt.Sprintf("invalid ticket ID format: %v", err),
			}
			response.Results = append(response.Results, result)
			response.Failed++
			continue
		}

		// Fetch ticket
		ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
		if err != nil {
			result := TicketUpdateResult{
				TicketID: update.TicketID,
				Success:  false,
				Message:  fmt.Sprintf("ticket not found: %v", err),
			}
			response.Results = append(response.Results, result)
			response.Failed++
			continue
		}

		// Idempotency check: if ticket already has the desired status, skip
		if (update.Status == "won" && ticket.Status == string(models.TicketStatusWon)) ||
			(update.Status == "lost" && ticket.Status == string(models.TicketStatusLost)) {
			response.Successful++
			continue
		}

		// Validate ticket is in "issued" status
		if ticket.Status != string(models.TicketStatusIssued) {
			result := TicketUpdateResult{
				TicketID: update.TicketID,
				Success:  false,
				Message:  fmt.Sprintf("ticket must be in 'issued' status, current status: %s", ticket.Status),
			}
			response.Results = append(response.Results, result)
			response.Failed++
			continue
		}

		// Update ticket based on status
		if update.Status == "won" {
			ticket.IsWinning = true
			ticket.WinningAmount = update.WinningAmount
			ticket.Matches = update.Matches
			if update.PrizeTier != "" {
				ticket.PrizeTier = &update.PrizeTier
			}

			// Set status to won
			if err := ticket.SetStatus(models.TicketStatusWon); err != nil {
				result := TicketUpdateResult{
					TicketID: update.TicketID,
					Success:  false,
					Message:  fmt.Sprintf("failed to set status to won: %v", err),
				}
				response.Results = append(response.Results, result)
				response.Failed++
				continue
			}
		} else {
			// Status is "lost"
			ticket.IsWinning = false
			ticket.WinningAmount = 0
			ticket.Matches = update.Matches

			// Set status to lost
			if err := ticket.SetStatus(models.TicketStatusLost); err != nil {
				result := TicketUpdateResult{
					TicketID: update.TicketID,
					Success:  false,
					Message:  fmt.Sprintf("failed to set status to lost: %v", err),
				}
				response.Results = append(response.Results, result)
				response.Failed++
				continue
			}
		}

		// Store draw ID for audit trail (parse string to UUID)
		drawUUID, err := uuid.Parse(req.DrawID)
		if err != nil {
			result := TicketUpdateResult{
				TicketID: update.TicketID,
				Success:  false,
				Message:  fmt.Sprintf("invalid draw_id format: %v", err),
			}
			response.Results = append(response.Results, result)
			response.Failed++
			continue
		}
		ticket.DrawID = &drawUUID

		// Update ticket in database (within transaction)
		if err := s.ticketRepo.UpdateWithTx(ctx, tx, ticket); err != nil {
			result := TicketUpdateResult{
				TicketID: update.TicketID,
				Success:  false,
				Message:  fmt.Sprintf("failed to update ticket: %v", err),
			}
			response.Results = append(response.Results, result)
			response.Failed++
			continue
		}

		// Success
		response.Successful++
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Set response attributes
	span.SetAttributes(
		attribute.Int64("successful_updates", response.Successful),
		attribute.Int64("failed_updates", response.Failed),
	)

	// TODO: Publish batch ticket.statuses_updated event to Kafka
	// event := events.NewTicketStatusesBatchUpdatedEvent(req.Updates, req.DrawID)
	// s.eventBus.Publish(ctx, "ticket.events", event)

	if response.Failed > 0 {
		response.Message = fmt.Sprintf("batch update completed: %d successful, %d failed", response.Successful, response.Failed)
		span.SetStatus(otelcodes.Error, response.Message)
	} else {
		response.Message = fmt.Sprintf("all %d tickets successfully updated", response.Successful)
		span.SetStatus(otelcodes.Ok, "batch update completed")
	}

	return response, nil
}

// UpdateTicketsDrawId updates all tickets for a game schedule with a draw ID
func (s *ticketService) UpdateTicketsDrawId(ctx context.Context, gameScheduleID uuid.UUID, drawID uuid.UUID) (int64, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.update_draw_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("game_schedule_id", gameScheduleID.String()),
		attribute.String("draw_id", drawID.String()),
	)

	// Update tickets in repository
	count, err := s.ticketRepo.UpdateDrawIdBySchedule(ctx, gameScheduleID, drawID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to update tickets")
		return 0, fmt.Errorf("failed to update tickets with draw_id: %w", err)
	}

	span.SetAttributes(attribute.Int64("updated_count", count))
	return count, nil
}

// DailyMetricsResult represents the aggregated daily metrics with calculated changes
type DailyMetricsResult struct {
	Date string

	// Revenue metric
	GrossRevenue         int64
	GrossRevenueChange   float64
	PreviousGrossRevenue int64

	// Tickets metric
	TicketsCount         int64
	TicketsChange        float64
	PreviousTicketsCount int64

	// Payouts metric
	PayoutsAmount         int64
	PayoutsChange         float64
	PreviousPayoutsAmount int64

	// Win rate metric
	WinRatePercentage float64
	WinningTickets    int64
	TotalTickets      int64

	// NEW: Stakes metrics (same as tickets/revenue but named differently for clarity)
	StakesCount         int64
	StakesChange        float64
	PreviousStakesCount int64

	StakesAmount         int64
	StakesAmountChange   float64
	PreviousStakesAmount int64

	// NEW: Paid tickets metrics
	PaidTicketsCount         int64
	PaidTicketsChange        float64
	PreviousPaidTicketsCount int64

	PaymentsAmount         int64
	PaymentsAmountChange   float64
	PreviousPaymentsAmount int64

	// NEW: Unpaid tickets metrics
	UnpaidTicketsCount         int64
	UnpaidTicketsChange        float64
	PreviousUnpaidTicketsCount int64

	UnpaidAmount         int64
	UnpaidAmountChange   float64
	PreviousUnpaidAmount int64
}

// GetDailyMetrics retrieves and aggregates daily metrics with Redis caching
func (s *ticketService) GetDailyMetrics(ctx context.Context, date string, includeComparison bool) (*DailyMetricsResult, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.get_daily_metrics")
	defer span.End()

	span.SetAttributes(
		attribute.String("metrics.date", date),
		attribute.Bool("metrics.include_comparison", includeComparison),
	)

	// Validate date format (YYYY-MM-DD)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	} else {
		_, err := time.Parse("2006-01-02", date)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(otelcodes.Error, "invalid date format")
			return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
		}
	}

	// Check if date is in the future
	requestedDate, _ := time.Parse("2006-01-02", date)
	if requestedDate.After(time.Now()) {
		err := fmt.Errorf("cannot retrieve metrics for future dates")
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "future date not allowed")
		return nil, err
	}

	// Try to get from cache first (if Redis is available)
	cacheKey := fmt.Sprintf("daily_metrics:%s", date)
	if s.redisClient != nil {
		_, cacheSpan := trace.SpanFromContext(ctx).TracerProvider().
			Tracer("service-ticket").Start(ctx, "cache.get_daily_metrics")

		cachedData, err := s.redisClient.Get(ctx, cacheKey).Result()
		cacheSpan.End()

		if err == nil {
			// Cache hit - deserialize and return
			var result DailyMetricsResult
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				span.SetAttributes(attribute.Bool("cache.hit", true))
				span.AddEvent("Cache hit for daily metrics")
				return &result, nil
			}
			// If unmarshal fails, continue to database query
			span.AddEvent("Cache data corrupted, fetching from database")
		} else if err != redis.Nil {
			// Log error but continue to database query
			span.AddEvent(fmt.Sprintf("Cache read error: %v", err))
		}
		span.SetAttributes(attribute.Bool("cache.hit", false))
	}

	// Cache miss or Redis unavailable - get from repository
	_, dbSpan := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.get_daily_metrics")

	metrics, err := s.ticketRepo.GetDailyMetrics(ctx, date)
	dbSpan.End()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get metrics from repository")
		return nil, fmt.Errorf("failed to get daily metrics: %w", err)
	}

	// Calculate percentage changes
	result := &DailyMetricsResult{
		Date:                  date,
		GrossRevenue:          metrics.TodayRevenue,
		PreviousGrossRevenue:  metrics.YesterdayRevenue,
		TicketsCount:          metrics.TodayTickets,
		PreviousTicketsCount:  metrics.YesterdayTickets,
		PayoutsAmount:         metrics.TodayPayouts,
		PreviousPayoutsAmount: metrics.YesterdayPayouts,
		WinningTickets:        metrics.WinningTickets,
		TotalTickets:          metrics.CheckedTickets,

		// NEW: Stakes metrics (duplicate of tickets/revenue for UI clarity)
		StakesCount:          metrics.TodayTickets,
		PreviousStakesCount:  metrics.YesterdayTickets,
		StakesAmount:         metrics.TodayRevenue,
		PreviousStakesAmount: metrics.YesterdayRevenue,

		// NEW: Paid tickets metrics
		PaidTicketsCount:         metrics.PaidTicketsCount,
		PreviousPaidTicketsCount: metrics.YesterdayPaidTicketsCount,
		PaymentsAmount:           metrics.PaidTicketsAmount,
		PreviousPaymentsAmount:   metrics.YesterdayPaidTicketsAmount,

		// NEW: Unpaid tickets metrics
		UnpaidTicketsCount:         metrics.UnpaidWinningTicketsCount,
		PreviousUnpaidTicketsCount: metrics.YesterdayUnpaidTicketsCount,
		UnpaidAmount:               metrics.UnpaidWinningTicketsAmount,
		PreviousUnpaidAmount:       metrics.YesterdayUnpaidTicketsAmount,
	}

	// Calculate change percentages (only if comparison requested)
	if includeComparison {
		result.GrossRevenueChange = calculatePercentageChange(metrics.YesterdayRevenue, metrics.TodayRevenue)
		result.TicketsChange = calculatePercentageChange(float64(metrics.YesterdayTickets), float64(metrics.TodayTickets))
		result.PayoutsChange = calculatePercentageChange(float64(metrics.YesterdayPayouts), float64(metrics.TodayPayouts))

		// NEW: Calculate change percentages for new metrics
		result.StakesChange = calculatePercentageChange(float64(metrics.YesterdayTickets), float64(metrics.TodayTickets))
		result.StakesAmountChange = calculatePercentageChange(float64(metrics.YesterdayRevenue), float64(metrics.TodayRevenue))
		result.PaidTicketsChange = calculatePercentageChange(float64(metrics.YesterdayPaidTicketsCount), float64(metrics.PaidTicketsCount))
		result.PaymentsAmountChange = calculatePercentageChange(float64(metrics.YesterdayPaidTicketsAmount), float64(metrics.PaidTicketsAmount))
		result.UnpaidTicketsChange = calculatePercentageChange(float64(metrics.YesterdayUnpaidTicketsCount), float64(metrics.UnpaidWinningTicketsCount))
		result.UnpaidAmountChange = calculatePercentageChange(float64(metrics.YesterdayUnpaidTicketsAmount), float64(metrics.UnpaidWinningTicketsAmount))
	}

	// Calculate win rate percentage
	if metrics.CheckedTickets > 0 {
		result.WinRatePercentage = (float64(metrics.WinningTickets) / float64(metrics.CheckedTickets)) * 100
	} else {
		result.WinRatePercentage = 0
	}

	// Cache the result (if Redis is available)
	if s.redisClient != nil {
		_, cacheSpan := trace.SpanFromContext(ctx).TracerProvider().
			Tracer("service-ticket").Start(ctx, "cache.set_daily_metrics")

		resultJSON, err := json.Marshal(result)
		if err == nil {
			// Set with 5-minute TTL
			ttl := 5 * time.Minute
			if err := s.redisClient.Set(ctx, cacheKey, resultJSON, ttl).Err(); err != nil {
				// Log error but don't fail the request
				span.AddEvent(fmt.Sprintf("Failed to cache result: %v", err))
			} else {
				span.AddEvent("Successfully cached daily metrics")
				span.SetAttributes(attribute.String("cache.ttl", ttl.String()))
			}
		}
		cacheSpan.End()
	}

	span.SetAttributes(
		attribute.Int64("metrics.gross_revenue", result.GrossRevenue),
		attribute.Int64("metrics.tickets_count", result.TicketsCount),
		attribute.Int64("metrics.payouts", result.PayoutsAmount),
		attribute.Float64("metrics.win_rate", result.WinRatePercentage),
	)

	return result, nil
}

// calculatePercentageChange calculates the percentage change between old and new values
func calculatePercentageChange(oldValue interface{}, newValue interface{}) float64 {
	var old, new float64

	// Convert to float64
	switch v := oldValue.(type) {
	case int64:
		old = float64(v)
	case float64:
		old = v
	case int:
		old = float64(v)
	default:
		return 0
	}

	switch v := newValue.(type) {
	case int64:
		new = float64(v)
	case float64:
		new = v
	case int:
		new = float64(v)
	default:
		return 0
	}

	// Avoid division by zero
	if old == 0 {
		if new == 0 {
			return 0
		}
		// If old is 0 and new is not, it's a 100% increase (or we could return infinity)
		return 100
	}

	return ((new - old) / old) * 100
}

// MonthlyMetricsResult represents monthly aggregated metrics for charts
type MonthlyMetricsResult struct {
	Month      string  `json:"month"`       // YYYY-MM format or MMM
	Year       int32   `json:"year"`        // Year (e.g., 2025)
	Revenue    int64   `json:"revenue"`     // Revenue in pesewas
	RevenueGHS float64 `json:"revenue_ghs"` // Revenue in GHS
	Tickets    int64   `json:"tickets"`     // Number of tickets sold
	Payouts    int64   `json:"payouts"`     // Payouts in pesewas
	PayoutsGHS float64 `json:"payouts_ghs"` // Payouts in GHS
}

// GetMonthlyMetrics retrieves monthly aggregated metrics with Redis caching
func (s *ticketService) GetMonthlyMetrics(ctx context.Context, months int32) ([]*MonthlyMetricsResult, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.get_monthly_metrics")
	defer span.End()

	span.SetAttributes(attribute.Int("metrics.months", int(months)))

	// Default to 6 months if not specified or invalid
	if months <= 0 {
		months = 6
	}

	// Try to get from cache first (if Redis is available)
	cacheKey := fmt.Sprintf("monthly_metrics:%d", months)
	if s.redisClient != nil {
		_, cacheSpan := trace.SpanFromContext(ctx).TracerProvider().
			Tracer("service-ticket").Start(ctx, "cache.get_monthly_metrics")

		cachedData, err := s.redisClient.Get(ctx, cacheKey).Result()
		cacheSpan.End()

		if err == nil {
			// Cache hit - deserialize and return
			var result []*MonthlyMetricsResult
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				span.SetAttributes(attribute.Bool("cache.hit", true))
				span.AddEvent("Cache hit for monthly metrics")
				return result, nil
			}
			// If unmarshal fails, continue to database query
			span.AddEvent("Cache data corrupted, fetching from database")
		} else if err != redis.Nil {
			// Log error but continue to database query
			span.AddEvent(fmt.Sprintf("Cache read error: %v", err))
		}
		span.SetAttributes(attribute.Bool("cache.hit", false))
	}

	// Cache miss or Redis unavailable - get from repository
	_, dbSpan := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.get_monthly_metrics")

	metrics, err := s.ticketRepo.GetMonthlyMetrics(ctx, months)
	dbSpan.End()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get metrics from repository")
		return nil, fmt.Errorf("failed to get monthly metrics: %w", err)
	}

	// Convert repository metrics to service result
	result := make([]*MonthlyMetricsResult, 0, len(metrics))
	for _, m := range metrics {
		result = append(result, &MonthlyMetricsResult{
			Month:      m.Month,
			Year:       m.Year,
			Revenue:    m.Revenue,
			RevenueGHS: m.RevenueGHS,
			Tickets:    m.Tickets,
			Payouts:    m.Payouts,
			PayoutsGHS: m.PayoutsGHS,
		})
	}

	// Cache the result (if Redis is available)
	if s.redisClient != nil {
		_, cacheSpan := trace.SpanFromContext(ctx).TracerProvider().
			Tracer("service-ticket").Start(ctx, "cache.set_monthly_metrics")

		resultJSON, err := json.Marshal(result)
		if err == nil {
			// Set with 5-minute TTL
			ttl := 5 * time.Minute
			if err := s.redisClient.Set(ctx, cacheKey, resultJSON, ttl).Err(); err != nil {
				// Log error but don't fail the request
				span.AddEvent(fmt.Sprintf("Failed to cache result: %v", err))
			} else {
				span.AddEvent("Successfully cached monthly metrics")
				span.SetAttributes(attribute.String("cache.ttl", ttl.String()))
			}
		}
		cacheSpan.End()
	}

	span.SetAttributes(attribute.Int("metrics.count", len(result)))
	return result, nil
}

// AgentPerformance Represents performance metrics for a single agent
type AgentPerformance struct {
	AgentID         string  `json:"agent_id"`
	AgentCode       string  `json:"agent_code"`
	AgentName       string  `json:"agent_name"`
	TotalRevenue    int64   `json:"total_revenue"`     // in pesewas
	TotalRevenueGHS float64 `json:"total_revenue_ghs"` // in GHS
	TotalTickets    int64   `json:"total_tickets"`
	RetailerCount   int32   `json:"retailer_count"`
}

// TopPerformingAgentsResult represents the result of top performing agents query
type TopPerformingAgentsResult struct {
	Agents []*AgentPerformance `json:"agents"`
	Period string              `json:"period"`
	Date   string              `json:"date"`
}

// GetTopPerformingAgents retrieves top performing agents with Redis caching
func (s *ticketService) GetTopPerformingAgents(ctx context.Context, period string, date string, limit int32) (*TopPerformingAgentsResult, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "service.ticket.get_top_performing_agents")
	defer span.End()

	span.SetAttributes(
		attribute.String("metrics.period", period),
		attribute.String("metrics.date", date),
		attribute.Int("metrics.limit", int(limit)),
	)

	// Default values
	if period == "" {
		period = "monthly"
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	if limit <= 0 {
		limit = 10
	}

	// Validate date format (YYYY-MM-DD)
	_, err := time.Parse("2006-01-02", date)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "invalid date format")
		return nil, fmt.Errorf("invalid date format, expected YYYY-MM-DD: %w", err)
	}

	// Try to get from cache first (if Redis is available)
	cacheKey := fmt.Sprintf("top_agents:%s:%s:%d", period, date, limit)
	if s.redisClient != nil {
		_, cacheSpan := trace.SpanFromContext(ctx).TracerProvider().
			Tracer("service-ticket").Start(ctx, "cache.get_top_performing_agents")

		cachedData, err := s.redisClient.Get(ctx, cacheKey).Result()
		cacheSpan.End()

		if err == nil {
			// Cache hit - deserialize and return
			var result TopPerformingAgentsResult
			if err := json.Unmarshal([]byte(cachedData), &result); err == nil {
				span.SetAttributes(attribute.Bool("cache.hit", true))
				span.AddEvent("Cache hit for top performing agents")
				return &result, nil
			}
			// If unmarshal fails, continue to database query
			span.AddEvent("Cache data corrupted, fetching from database")
		} else if err != redis.Nil {
			// Log error but continue to database query
			span.AddEvent(fmt.Sprintf("Cache read error: %v", err))
		}
		span.SetAttributes(attribute.Bool("cache.hit", false))
	}

	// Cache miss or Redis unavailable - get from repository
	_, dbSpan := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.get_top_performing_agents")

	agents, err := s.ticketRepo.GetTopPerformingAgents(ctx, period, date, limit)
	dbSpan.End()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, "failed to get agents from repository")
		return nil, fmt.Errorf("failed to get top performing agents: %w", err)
	}

	// Convert repository results to service result and enrich with agent details via gRPC
	result := &TopPerformingAgentsResult{
		Agents: make([]*AgentPerformance, 0, len(agents)),
		Period: period,
		Date:   date,
	}

	// Enrich each agent with data from Agent Management Service
	for _, agent := range agents {
		_, enrichSpan := trace.SpanFromContext(ctx).TracerProvider().
			Tracer("service-ticket").Start(ctx, "grpc.enrich_agent_data")
		enrichSpan.SetAttributes(attribute.String("agent.id", agent.AgentID))

		// Get agent details from Agent Management Service
		agentReq := &agentmanagementv1.GetAgentRequest{Id: agent.AgentID}
		agentResp, err := s.agentManagementClient.GetAgent(ctx, agentReq)

		var agentCode, agentName string
		var retailerCount int32

		if err != nil {
			// Log error but continue with partial data
			enrichSpan.AddEvent(fmt.Sprintf("Failed to get agent details: %v", err))
			agentCode = ""
			agentName = fmt.Sprintf("Agent %s", agent.AgentID)
		} else {
			agentCode = agentResp.AgentCode
			agentName = agentResp.Name
		}

		// Get retailer count from Agent Management Service
		retailersReq := &agentmanagementv1.GetAgentRetailersRequest{AgentId: agent.AgentID}
		retailersResp, err := s.agentManagementClient.GetAgentRetailers(ctx, retailersReq)

		if err != nil {
			// Log error but continue with zero count
			enrichSpan.AddEvent(fmt.Sprintf("Failed to get retailer count: %v", err))
			retailerCount = 0
		} else {
			retailerCount = int32(len(retailersResp.Retailers))
		}

		enrichSpan.End()

		// Append enriched agent performance data
		result.Agents = append(result.Agents, &AgentPerformance{
			AgentID:         agent.AgentID,
			AgentCode:       agentCode,
			AgentName:       agentName,
			TotalRevenue:    agent.TotalRevenue,
			TotalRevenueGHS: agent.RevenueGHS,
			TotalTickets:    agent.TotalTickets,
			RetailerCount:   retailerCount,
		})
	}

	// Cache the result (if Redis is available)
	if s.redisClient != nil {
		_, cacheSpan := trace.SpanFromContext(ctx).TracerProvider().
			Tracer("service-ticket").Start(ctx, "cache.set_top_performing_agents")

		resultJSON, err := json.Marshal(result)
		if err == nil {
			// Set with 5-minute TTL
			ttl := 5 * time.Minute
			if err := s.redisClient.Set(ctx, cacheKey, resultJSON, ttl).Err(); err != nil {
				// Log error but don't fail the request
				span.AddEvent(fmt.Sprintf("Failed to cache result: %v", err))
			} else {
				span.AddEvent("Successfully cached top performing agents")
				span.SetAttributes(attribute.String("cache.ttl", ttl.String()))
			}
		}
		cacheSpan.End()
	}

	span.SetAttributes(
		attribute.Int("agent.count", len(result.Agents)),
		attribute.String("period.used", period),
	)
	span.SetStatus(otelcodes.Ok, "top performing agents retrieved successfully")

	return result, nil
}
