package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/randco/randco-microservices/services/service-ticket/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TicketRepository defines the interface for ticket data operations
type TicketRepository interface {
	Create(ctx context.Context, ticket *models.Ticket) error
	CreateWithTx(ctx context.Context, tx *sql.Tx, ticket *models.Ticket) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Ticket, error)
	GetBySerial(ctx context.Context, serialNumber string) (*models.Ticket, error)
	Update(ctx context.Context, ticket *models.Ticket) error
	UpdateWithTx(ctx context.Context, tx *sql.Tx, ticket *models.Ticket) error
	List(ctx context.Context, filter models.TicketFilter, page, limit int) ([]*models.Ticket, int64, error)
	GetByGameAndDraw(ctx context.Context, gameCode string, drawNumber int32) ([]*models.Ticket, error)
	GetByIssuer(ctx context.Context, issuerType, issuerID string, limit int) ([]*models.Ticket, error)
	GetByCustomer(ctx context.Context, customerPhone string, limit int) ([]*models.Ticket, error)
	UpdateDrawIdBySchedule(ctx context.Context, gameScheduleID uuid.UUID, drawID uuid.UUID) (int64, error)
	GetDailyMetrics(ctx context.Context, date string) (*DailyMetrics, error)
	GetMonthlyMetrics(ctx context.Context, months int32) ([]*MonthlyMetrics, error)
	GetRetailerPerformanceByPeriod(ctx context.Context, period string, date string) ([]*RetailerPerformance, error)
	GetTopPerformingAgents(ctx context.Context, period string, date string, limit int32) ([]*AgentPerformanceData, error)
}

// ticketRepository implements TicketRepository interface
type ticketRepository struct {
	db *sql.DB
}

// NewTicketRepository creates a new instance of TicketRepository
func NewTicketRepository(db *sql.DB) TicketRepository {
	return &ticketRepository{
		db: db,
	}
}

// Create creates a new ticket
func (r *ticketRepository) Create(ctx context.Context, ticket *models.Ticket) error {
	return r.CreateWithTx(ctx, nil, ticket)
}

// CreateWithTx creates a new ticket within a transaction
func (r *ticketRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, ticket *models.Ticket) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "tickets"),
		attribute.String("ticket.serial_number", ticket.SerialNumber),
		attribute.String("ticket.game_code", ticket.GameCode),
		attribute.Bool("in_transaction", tx != nil),
	)

	query := `
		INSERT INTO tickets (
			id, serial_number, game_code, game_schedule_id, draw_number, draw_id, game_name, game_type,
			selected_numbers, banker_numbers, opposed_numbers,
			bet_lines, number_of_lines, unit_price, total_amount,
			issuer_type, issuer_id, issuer_details,
			customer_phone, customer_name, customer_email,
			payment_method, payment_ref, payment_status,
			security_hash, security_features,
			status, is_winning, winning_amount, prize_tier, matches,
			draw_date, draw_time, issued_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18,
			$19, $20, $21,
			$22, $23, $24,
			$25, $26,
			$27, $28, $29, $30, $31,
			$32, $33, $34
		)
		RETURNING created_at, updated_at`

	// Generate ID if not provided
	if ticket.ID == uuid.Nil {
		ticket.ID = uuid.New()
	}

	// Set default status if not provided
	if ticket.Status == "" {
		ticket.Status = string(models.TicketStatusIssued)
	}

	// Set issued_at if not provided
	if ticket.IssuedAt.IsZero() {
		ticket.IssuedAt = ticket.CreatedAt
	}

	// Marshal JSONB fields
	betLinesJSON, err := json.Marshal(ticket.BetLines)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal bet lines")
		return fmt.Errorf("failed to marshal bet lines: %w", err)
	}

	var issuerDetailsJSON []byte
	if ticket.IssuerDetails != nil {
		issuerDetailsJSON, err = json.Marshal(ticket.IssuerDetails)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal issuer details")
			return fmt.Errorf("failed to marshal issuer details: %w", err)
		}
	}

	var securityFeaturesJSON []byte
	if ticket.SecurityFeatures != nil {
		securityFeaturesJSON, err = json.Marshal(ticket.SecurityFeatures)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal security features")
			return fmt.Errorf("failed to marshal security features: %w", err)
		}
	}

	// Prepare JSON field values - use nil for empty byte slices to avoid invalid JSON
	var issuerDetailsValue interface{}
	if len(issuerDetailsJSON) > 0 {
		issuerDetailsValue = string(issuerDetailsJSON)
	} else {
		issuerDetailsValue = nil
	}

	var securityFeaturesValue interface{}
	if len(securityFeaturesJSON) > 0 {
		securityFeaturesValue = string(securityFeaturesJSON)
	} else {
		securityFeaturesValue = nil
	}

	// Use transaction if provided, otherwise use the database connection
	var row *sql.Row
	args := []interface{}{
		ticket.ID, ticket.SerialNumber, ticket.GameCode, ticket.GameScheduleID, ticket.DrawNumber, ticket.DrawID,
		ticket.GameName, ticket.GameType,
		pq.Array(ticket.SelectedNumbers), pq.Array(ticket.BankerNumbers), pq.Array(ticket.OpposedNumbers),
		string(betLinesJSON), ticket.NumberOfLines, ticket.UnitPrice, ticket.TotalAmount,
		ticket.IssuerType, ticket.IssuerID, issuerDetailsValue,
		ticket.CustomerPhone, ticket.CustomerName, ticket.CustomerEmail,
		ticket.PaymentMethod, ticket.PaymentRef, ticket.PaymentStatus,
		ticket.SecurityHash, securityFeaturesValue,
		ticket.Status, ticket.IsWinning, ticket.WinningAmount, ticket.PrizeTier, ticket.Matches,
		ticket.DrawDate, ticket.DrawTime, ticket.IssuedAt,
	}

	if tx != nil {
		row = tx.QueryRowContext(ctx, query, args...)
	} else {
		row = r.db.QueryRowContext(ctx, query, args...)
	}

	err = row.Scan(&ticket.CreatedAt, &ticket.UpdatedAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create ticket")
		return fmt.Errorf("failed to create ticket: %w", err)
	}

	span.SetAttributes(attribute.String("ticket.id", ticket.ID.String()))
	return nil
}

// GetByID retrieves a ticket by ID
func (r *ticketRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_by_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.String("ticket.id", id.String()),
	)

	query := `
		SELECT
			id, serial_number, game_code, game_schedule_id, draw_number, draw_id, game_name, game_type,
			selected_numbers, banker_numbers, opposed_numbers,
			bet_lines, number_of_lines, unit_price, total_amount,
			issuer_type, issuer_id, issuer_details,
			customer_phone, customer_name, customer_email,
			payment_method, payment_ref, payment_status,
			security_hash, security_features,
			status, is_winning, winning_amount, prize_tier, matches,
			draw_date, draw_time,
			issued_at, validated_at, cancelled_at, paid_at, paid_by, payment_reference, voided_at,
			created_at, updated_at
		FROM tickets
		WHERE id = $1`

	ticket := &models.Ticket{}
	var betLinesJSON, issuerDetailsJSON, securityFeaturesJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&ticket.ID, &ticket.SerialNumber, &ticket.GameCode, &ticket.GameScheduleID, &ticket.DrawNumber, &ticket.DrawID,
		&ticket.GameName, &ticket.GameType,
		pq.Array(&ticket.SelectedNumbers), pq.Array(&ticket.BankerNumbers), pq.Array(&ticket.OpposedNumbers),
		&betLinesJSON, &ticket.NumberOfLines, &ticket.UnitPrice, &ticket.TotalAmount,
		&ticket.IssuerType, &ticket.IssuerID, &issuerDetailsJSON,
		&ticket.CustomerPhone, &ticket.CustomerName, &ticket.CustomerEmail,
		&ticket.PaymentMethod, &ticket.PaymentRef, &ticket.PaymentStatus,
		&ticket.SecurityHash, &securityFeaturesJSON,
		&ticket.Status, &ticket.IsWinning, &ticket.WinningAmount, &ticket.PrizeTier, &ticket.Matches,
		&ticket.DrawDate, &ticket.DrawTime,
		&ticket.IssuedAt, &ticket.ValidatedAt, &ticket.CancelledAt, &ticket.PaidAt, &ticket.PaidBy, &ticket.PaymentReference, &ticket.VoidedAt,
		&ticket.CreatedAt, &ticket.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("ticket.found", false))
			return nil, fmt.Errorf("ticket not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get ticket")
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	// Unmarshal JSONB fields
	if len(betLinesJSON) > 0 {
		if err := json.Unmarshal(betLinesJSON, &ticket.BetLines); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to unmarshal bet lines")
			return nil, fmt.Errorf("failed to unmarshal bet lines: %w", err)
		}
	}

	if len(issuerDetailsJSON) > 0 {
		ticket.IssuerDetails = &models.IssuerDetails{}
		if err := json.Unmarshal(issuerDetailsJSON, ticket.IssuerDetails); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to unmarshal issuer details")
			return nil, fmt.Errorf("failed to unmarshal issuer details: %w", err)
		}
	}

	if len(securityFeaturesJSON) > 0 {
		ticket.SecurityFeatures = &models.SecurityFeatures{}
		if err := json.Unmarshal(securityFeaturesJSON, ticket.SecurityFeatures); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to unmarshal security features")
			return nil, fmt.Errorf("failed to unmarshal security features: %w", err)
		}
	}

	span.SetAttributes(
		attribute.Bool("ticket.found", true),
		attribute.String("ticket.serial_number", ticket.SerialNumber),
	)
	return ticket, nil
}

// GetBySerial retrieves a ticket by serial number
func (r *ticketRepository) GetBySerial(ctx context.Context, serialNumber string) (*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_by_serial")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.String("ticket.serial_number", serialNumber),
	)

	query := `
		SELECT
			id, serial_number, game_code, game_schedule_id, draw_number, draw_id, game_name, game_type,
			selected_numbers, banker_numbers, opposed_numbers,
			bet_lines, number_of_lines, unit_price, total_amount,
			issuer_type, issuer_id, issuer_details,
			customer_phone, customer_name, customer_email,
			payment_method, payment_ref, payment_status,
			security_hash, security_features,
			status, is_winning, winning_amount, prize_tier, matches,
			draw_date, draw_time,
			issued_at, validated_at, cancelled_at, paid_at, paid_by, payment_reference, voided_at,
			created_at, updated_at
		FROM tickets
		WHERE serial_number = $1`

	ticket := &models.Ticket{}
	var betLinesJSON, issuerDetailsJSON, securityFeaturesJSON []byte

	err := r.db.QueryRowContext(ctx, query, serialNumber).Scan(
		&ticket.ID, &ticket.SerialNumber, &ticket.GameCode, &ticket.GameScheduleID, &ticket.DrawNumber, &ticket.DrawID,
		&ticket.GameName, &ticket.GameType,
		pq.Array(&ticket.SelectedNumbers), pq.Array(&ticket.BankerNumbers), pq.Array(&ticket.OpposedNumbers),
		&betLinesJSON, &ticket.NumberOfLines, &ticket.UnitPrice, &ticket.TotalAmount,
		&ticket.IssuerType, &ticket.IssuerID, &issuerDetailsJSON,
		&ticket.CustomerPhone, &ticket.CustomerName, &ticket.CustomerEmail,
		&ticket.PaymentMethod, &ticket.PaymentRef, &ticket.PaymentStatus,
		&ticket.SecurityHash, &securityFeaturesJSON,
		&ticket.Status, &ticket.IsWinning, &ticket.WinningAmount, &ticket.PrizeTier, &ticket.Matches,
		&ticket.DrawDate, &ticket.DrawTime,
		&ticket.IssuedAt, &ticket.ValidatedAt, &ticket.CancelledAt, &ticket.PaidAt, &ticket.PaidBy, &ticket.PaymentReference, &ticket.VoidedAt,
		&ticket.CreatedAt, &ticket.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetAttributes(attribute.Bool("ticket.found", false))
			return nil, fmt.Errorf("ticket not found")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get ticket by serial")
		return nil, fmt.Errorf("failed to get ticket by serial: %w", err)
	}

	// Unmarshal JSONB fields
	if len(betLinesJSON) > 0 {
		if err := json.Unmarshal(betLinesJSON, &ticket.BetLines); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to unmarshal bet lines")
			return nil, fmt.Errorf("failed to unmarshal bet lines: %w", err)
		}
	}

	if len(issuerDetailsJSON) > 0 {
		ticket.IssuerDetails = &models.IssuerDetails{}
		if err := json.Unmarshal(issuerDetailsJSON, ticket.IssuerDetails); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to unmarshal issuer details")
			return nil, fmt.Errorf("failed to unmarshal issuer details: %w", err)
		}
	}

	if len(securityFeaturesJSON) > 0 {
		ticket.SecurityFeatures = &models.SecurityFeatures{}
		if err := json.Unmarshal(securityFeaturesJSON, ticket.SecurityFeatures); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to unmarshal security features")
			return nil, fmt.Errorf("failed to unmarshal security features: %w", err)
		}
	}

	span.SetAttributes(
		attribute.Bool("ticket.found", true),
		attribute.String("ticket.id", ticket.ID.String()),
	)
	return ticket, nil
}

// Update updates an existing ticket
func (r *ticketRepository) Update(ctx context.Context, ticket *models.Ticket) error {
	return r.UpdateWithTx(ctx, nil, ticket)
}

// UpdateWithTx updates an existing ticket within a transaction
func (r *ticketRepository) UpdateWithTx(ctx context.Context, tx *sql.Tx, ticket *models.Ticket) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "tickets"),
		attribute.String("ticket.id", ticket.ID.String()),
	)

	query := `
		UPDATE tickets SET
			serial_number = $2, game_code = $3, game_schedule_id = $4, draw_number = $5, draw_id = $6,
			game_name = $7, game_type = $8,
			selected_numbers = $9, banker_numbers = $10, opposed_numbers = $11,
			bet_lines = $12, number_of_lines = $13, unit_price = $14, total_amount = $15,
			issuer_type = $16, issuer_id = $17, issuer_details = $18,
			customer_phone = $19, customer_name = $20, customer_email = $21,
			payment_method = $22, payment_ref = $23, payment_status = $24,
			security_hash = $25, security_features = $26,
			status = $27, is_winning = $28, winning_amount = $29, prize_tier = $30, matches = $31,
			draw_date = $32, draw_time = $33,
			issued_at = $34, validated_at = $35, cancelled_at = $36, paid_at = $37, paid_by = $38, payment_reference = $39, voided_at = $40,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	// Marshal JSONB fields
	betLinesJSON, err := json.Marshal(ticket.BetLines)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal bet lines")
		return fmt.Errorf("failed to marshal bet lines: %w", err)
	}

	var issuerDetailsJSON []byte
	if ticket.IssuerDetails != nil {
		issuerDetailsJSON, err = json.Marshal(ticket.IssuerDetails)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal issuer details")
			return fmt.Errorf("failed to marshal issuer details: %w", err)
		}
	}

	var securityFeaturesJSON []byte
	if ticket.SecurityFeatures != nil {
		securityFeaturesJSON, err = json.Marshal(ticket.SecurityFeatures)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to marshal security features")
			return fmt.Errorf("failed to marshal security features: %w", err)
		}
	}

	// Prepare JSON field values - use nil for empty byte slices to avoid invalid JSON
	var issuerDetailsValue interface{}
	if len(issuerDetailsJSON) > 0 {
		issuerDetailsValue = string(issuerDetailsJSON)
	} else {
		issuerDetailsValue = nil
	}

	var securityFeaturesValue interface{}
	if len(securityFeaturesJSON) > 0 {
		securityFeaturesValue = string(securityFeaturesJSON)
	} else {
		securityFeaturesValue = nil
	}

	args := []interface{}{
		ticket.ID,
		ticket.SerialNumber, ticket.GameCode, ticket.GameScheduleID, ticket.DrawNumber, ticket.DrawID,
		ticket.GameName, ticket.GameType,
		pq.Array(ticket.SelectedNumbers), pq.Array(ticket.BankerNumbers), pq.Array(ticket.OpposedNumbers),
		string(betLinesJSON), ticket.NumberOfLines, ticket.UnitPrice, ticket.TotalAmount,
		ticket.IssuerType, ticket.IssuerID, issuerDetailsValue,
		ticket.CustomerPhone, ticket.CustomerName, ticket.CustomerEmail,
		ticket.PaymentMethod, ticket.PaymentRef, ticket.PaymentStatus,
		ticket.SecurityHash, securityFeaturesValue,
		ticket.Status, ticket.IsWinning, ticket.WinningAmount, ticket.PrizeTier, ticket.Matches,
		ticket.DrawDate, ticket.DrawTime,
		ticket.IssuedAt, ticket.ValidatedAt, ticket.CancelledAt, ticket.PaidAt, ticket.PaidBy, ticket.PaymentReference, ticket.VoidedAt,
	}

	if tx != nil {
		err = tx.QueryRowContext(ctx, query, args...).Scan(&ticket.UpdatedAt)
	} else {
		err = r.db.QueryRowContext(ctx, query, args...).Scan(&ticket.UpdatedAt)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update ticket")
		return fmt.Errorf("failed to update ticket: %w", err)
	}

	return nil
}

// List retrieves tickets with filtering and pagination
func (r *ticketRepository) List(ctx context.Context, filter models.TicketFilter, page, limit int) ([]*models.Ticket, int64, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.list")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.limit", limit),
	)

	// Build WHERE clause
	conditions := []string{}
	args := []interface{}{}
	argCount := 0

	if filter.GameCode != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("game_code = $%d", argCount))
		args = append(args, *filter.GameCode)
	}

	if filter.GameScheduleID != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("game_schedule_id = $%d", argCount))
		args = append(args, *filter.GameScheduleID)
	}

	if filter.DrawNumber != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("draw_number = $%d", argCount))
		args = append(args, *filter.DrawNumber)
	}

	if filter.DrawID != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("draw_id = $%d", argCount))
		args = append(args, *filter.DrawID)
	}

	if filter.IssuerType != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("issuer_type = $%d", argCount))
		args = append(args, *filter.IssuerType)
	}

	if filter.IssuerID != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("issuer_id = $%d", argCount))
		args = append(args, *filter.IssuerID)
	}

	if filter.CustomerPhone != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("customer_phone = $%d", argCount))
		args = append(args, *filter.CustomerPhone)
	}

	if filter.Status != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("status = $%d", argCount))
		args = append(args, *filter.Status)
	}

	if filter.IsWinning != nil {
		argCount++
		conditions = append(conditions, fmt.Sprintf("is_winning = $%d", argCount))
		args = append(args, *filter.IsWinning)
	}

	if filter.StartDate != nil && *filter.StartDate != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("issued_at >= $%d", argCount))
		args = append(args, *filter.StartDate)
	}

	if filter.EndDate != nil && *filter.EndDate != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("issued_at <= $%d", argCount))
		args = append(args, *filter.EndDate)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tickets %s", whereClause)
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to count tickets")
		return nil, 0, fmt.Errorf("failed to count tickets: %w", err)
	}

	// Get paginated results
	offset := (page - 1) * limit
	argCount++
	limitArg := argCount
	argCount++
	offsetArg := argCount

	query := fmt.Sprintf(`
		SELECT
			id, serial_number, game_code, game_schedule_id, draw_number, draw_id, game_name, game_type,
			selected_numbers, banker_numbers, opposed_numbers,
			bet_lines, number_of_lines, unit_price, total_amount,
			issuer_type, issuer_id, issuer_details,
			customer_phone, customer_name, customer_email,
			payment_method, payment_ref, payment_status,
			security_hash, security_features,
			status, is_winning, winning_amount, prize_tier, matches,
			draw_date, draw_time,
			issued_at, validated_at, cancelled_at, paid_at, paid_by, payment_reference, voided_at,
			created_at, updated_at
		FROM tickets %s
		ORDER BY issued_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, limitArg, offsetArg)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list tickets")
		return nil, 0, fmt.Errorf("failed to list tickets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tickets := []*models.Ticket{}
	for rows.Next() {
		ticket := &models.Ticket{}
		var betLinesJSON, issuerDetailsJSON, securityFeaturesJSON []byte

		err := rows.Scan(
			&ticket.ID, &ticket.SerialNumber, &ticket.GameCode, &ticket.GameScheduleID, &ticket.DrawNumber, &ticket.DrawID,
			&ticket.GameName, &ticket.GameType,
			pq.Array(&ticket.SelectedNumbers), pq.Array(&ticket.BankerNumbers), pq.Array(&ticket.OpposedNumbers),
			&betLinesJSON, &ticket.NumberOfLines, &ticket.UnitPrice, &ticket.TotalAmount,
			&ticket.IssuerType, &ticket.IssuerID, &issuerDetailsJSON,
			&ticket.CustomerPhone, &ticket.CustomerName, &ticket.CustomerEmail,
			&ticket.PaymentMethod, &ticket.PaymentRef, &ticket.PaymentStatus,
			&ticket.SecurityHash, &securityFeaturesJSON,
			&ticket.Status, &ticket.IsWinning, &ticket.WinningAmount, &ticket.PrizeTier, &ticket.Matches,
			&ticket.DrawDate, &ticket.DrawTime,
			&ticket.IssuedAt, &ticket.ValidatedAt, &ticket.CancelledAt, &ticket.PaidAt, &ticket.PaidBy, &ticket.PaymentReference, &ticket.VoidedAt,
			&ticket.CreatedAt, &ticket.UpdatedAt,
		)
		if err != nil {
			span.RecordError(err)
			return nil, 0, fmt.Errorf("failed to scan ticket: %w", err)
		}

		// Unmarshal JSONB fields
		if len(betLinesJSON) > 0 {
			if err := json.Unmarshal(betLinesJSON, &ticket.BetLines); err != nil {
				span.RecordError(err)
				return nil, 0, fmt.Errorf("failed to unmarshal bet lines: %w", err)
			}
		}

		if len(issuerDetailsJSON) > 0 {
			ticket.IssuerDetails = &models.IssuerDetails{}
			if err := json.Unmarshal(issuerDetailsJSON, ticket.IssuerDetails); err != nil {
				span.RecordError(err)
				return nil, 0, fmt.Errorf("failed to unmarshal issuer details: %w", err)
			}
		}

		if len(securityFeaturesJSON) > 0 {
			ticket.SecurityFeatures = &models.SecurityFeatures{}
			if err := json.Unmarshal(securityFeaturesJSON, ticket.SecurityFeatures); err != nil {
				span.RecordError(err)
				return nil, 0, fmt.Errorf("failed to unmarshal security features: %w", err)
			}
		}

		tickets = append(tickets, ticket)
	}

	span.SetAttributes(
		attribute.Int64("tickets.count", int64(len(tickets))),
		attribute.Int64("tickets.total", total),
	)

	return tickets, total, nil
}

// GetByGameAndDraw retrieves all tickets for a specific game and draw
func (r *ticketRepository) GetByGameAndDraw(ctx context.Context, gameCode string, drawNumber int32) ([]*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_by_game_and_draw")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.String("game_code", gameCode),
		attribute.Int("draw_number", int(drawNumber)),
	)

	query := `
		SELECT
			id, serial_number, game_code, game_schedule_id, draw_number, draw_id, game_name, game_type,
			selected_numbers, banker_numbers, opposed_numbers,
			bet_lines, number_of_lines, unit_price, total_amount,
			issuer_type, issuer_id, issuer_details,
			customer_phone, customer_name, customer_email,
			payment_method, payment_ref, payment_status,
			security_hash, security_features,
			status, is_winning, winning_amount, prize_tier, matches,
			draw_date, draw_time,
			issued_at, validated_at, cancelled_at, paid_at, paid_by, payment_reference, voided_at,
			created_at, updated_at
		FROM tickets
		WHERE game_code = $1 AND draw_number = $2
		ORDER BY issued_at DESC`

	rows, err := r.db.QueryContext(ctx, query, gameCode, drawNumber)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get tickets by game and draw")
		return nil, fmt.Errorf("failed to get tickets by game and draw: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tickets := []*models.Ticket{}
	for rows.Next() {
		ticket := &models.Ticket{}
		var betLinesJSON, issuerDetailsJSON, securityFeaturesJSON []byte

		err := rows.Scan(
			&ticket.ID, &ticket.SerialNumber, &ticket.GameCode, &ticket.GameScheduleID, &ticket.DrawNumber, &ticket.DrawID,
			&ticket.GameName, &ticket.GameType,
			pq.Array(&ticket.SelectedNumbers), pq.Array(&ticket.BankerNumbers), pq.Array(&ticket.OpposedNumbers),
			&betLinesJSON, &ticket.NumberOfLines, &ticket.UnitPrice, &ticket.TotalAmount,
			&ticket.IssuerType, &ticket.IssuerID, &issuerDetailsJSON,
			&ticket.CustomerPhone, &ticket.CustomerName, &ticket.CustomerEmail,
			&ticket.PaymentMethod, &ticket.PaymentRef, &ticket.PaymentStatus,
			&ticket.SecurityHash, &securityFeaturesJSON,
			&ticket.Status, &ticket.IsWinning, &ticket.WinningAmount, &ticket.PrizeTier, &ticket.Matches,
			&ticket.DrawDate, &ticket.DrawTime,
			&ticket.IssuedAt, &ticket.ValidatedAt, &ticket.CancelledAt, &ticket.PaidAt, &ticket.PaidBy, &ticket.PaymentReference, &ticket.VoidedAt,
			&ticket.CreatedAt, &ticket.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}

		// Unmarshal JSONB fields
		if len(betLinesJSON) > 0 {
			if err := json.Unmarshal(betLinesJSON, &ticket.BetLines); err != nil {
				return nil, fmt.Errorf("failed to unmarshal bet lines: %w", err)
			}
		}

		if len(issuerDetailsJSON) > 0 {
			ticket.IssuerDetails = &models.IssuerDetails{}
			if err := json.Unmarshal(issuerDetailsJSON, ticket.IssuerDetails); err != nil {
				return nil, fmt.Errorf("failed to unmarshal issuer details: %w", err)
			}
		}

		if len(securityFeaturesJSON) > 0 {
			ticket.SecurityFeatures = &models.SecurityFeatures{}
			if err := json.Unmarshal(securityFeaturesJSON, ticket.SecurityFeatures); err != nil {
				return nil, fmt.Errorf("failed to unmarshal security features: %w", err)
			}
		}

		tickets = append(tickets, ticket)
	}

	span.SetAttributes(attribute.Int("tickets.count", len(tickets)))
	return tickets, nil
}

// GetByIssuer retrieves tickets for a specific issuer
func (r *ticketRepository) GetByIssuer(ctx context.Context, issuerType, issuerID string, limit int) ([]*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_by_issuer")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.String("issuer_type", issuerType),
		attribute.String("issuer_id", issuerID),
		attribute.Int("limit", limit),
	)

	query := `
		SELECT
			id, serial_number, game_code, game_schedule_id, draw_number, draw_id, game_name, game_type,
			selected_numbers, banker_numbers, opposed_numbers,
			bet_lines, number_of_lines, unit_price, total_amount,
			issuer_type, issuer_id, issuer_details,
			customer_phone, customer_name, customer_email,
			payment_method, payment_ref, payment_status,
			security_hash, security_features,
			status, is_winning, winning_amount, prize_tier, matches,
			draw_date, draw_time,
			issued_at, validated_at, cancelled_at, paid_at, paid_by, payment_reference, voided_at,
			created_at, updated_at
		FROM tickets
		WHERE issuer_type = $1 AND issuer_id = $2
		ORDER BY issued_at DESC
		LIMIT $3`

	rows, err := r.db.QueryContext(ctx, query, issuerType, issuerID, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get tickets by issuer")
		return nil, fmt.Errorf("failed to get tickets by issuer: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tickets := []*models.Ticket{}
	for rows.Next() {
		ticket := &models.Ticket{}
		var betLinesJSON, issuerDetailsJSON, securityFeaturesJSON []byte

		err := rows.Scan(
			&ticket.ID, &ticket.SerialNumber, &ticket.GameCode, &ticket.GameScheduleID, &ticket.DrawNumber, &ticket.DrawID,
			&ticket.GameName, &ticket.GameType,
			pq.Array(&ticket.SelectedNumbers), pq.Array(&ticket.BankerNumbers), pq.Array(&ticket.OpposedNumbers),
			&betLinesJSON, &ticket.NumberOfLines, &ticket.UnitPrice, &ticket.TotalAmount,
			&ticket.IssuerType, &ticket.IssuerID, &issuerDetailsJSON,
			&ticket.CustomerPhone, &ticket.CustomerName, &ticket.CustomerEmail,
			&ticket.PaymentMethod, &ticket.PaymentRef, &ticket.PaymentStatus,
			&ticket.SecurityHash, &securityFeaturesJSON,
			&ticket.Status, &ticket.IsWinning, &ticket.WinningAmount, &ticket.PrizeTier, &ticket.Matches,
			&ticket.DrawDate, &ticket.DrawTime,
			&ticket.IssuedAt, &ticket.ValidatedAt, &ticket.CancelledAt, &ticket.PaidAt, &ticket.PaidBy, &ticket.PaymentReference, &ticket.VoidedAt,
			&ticket.CreatedAt, &ticket.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}

		// Unmarshal JSONB fields
		if len(betLinesJSON) > 0 {
			if err := json.Unmarshal(betLinesJSON, &ticket.BetLines); err != nil {
				return nil, fmt.Errorf("failed to unmarshal bet lines: %w", err)
			}
		}

		if len(issuerDetailsJSON) > 0 {
			ticket.IssuerDetails = &models.IssuerDetails{}
			if err := json.Unmarshal(issuerDetailsJSON, ticket.IssuerDetails); err != nil {
				return nil, fmt.Errorf("failed to unmarshal issuer details: %w", err)
			}
		}

		if len(securityFeaturesJSON) > 0 {
			ticket.SecurityFeatures = &models.SecurityFeatures{}
			if err := json.Unmarshal(securityFeaturesJSON, ticket.SecurityFeatures); err != nil {
				return nil, fmt.Errorf("failed to unmarshal security features: %w", err)
			}
		}

		tickets = append(tickets, ticket)
	}

	span.SetAttributes(attribute.Int("tickets.count", len(tickets)))
	return tickets, nil
}

// GetByCustomer retrieves tickets for a specific customer
func (r *ticketRepository) GetByCustomer(ctx context.Context, customerPhone string, limit int) ([]*models.Ticket, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_by_customer")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.String("customer_phone", customerPhone),
		attribute.Int("limit", limit),
	)

	query := `
		SELECT
			id, serial_number, game_code, game_schedule_id, draw_number, draw_id, game_name, game_type,
			selected_numbers, banker_numbers, opposed_numbers,
			bet_lines, number_of_lines, unit_price, total_amount,
			issuer_type, issuer_id, issuer_details,
			customer_phone, customer_name, customer_email,
			payment_method, payment_ref, payment_status,
			security_hash, security_features,
			status, is_winning, winning_amount, prize_tier, matches,
			draw_date, draw_time,
			issued_at, validated_at, cancelled_at, paid_at, paid_by, payment_reference, voided_at,
			created_at, updated_at
		FROM tickets
		WHERE customer_phone = $1
		ORDER BY issued_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, customerPhone, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get tickets by customer")
		return nil, fmt.Errorf("failed to get tickets by customer: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tickets := []*models.Ticket{}
	for rows.Next() {
		ticket := &models.Ticket{}
		var betLinesJSON, issuerDetailsJSON, securityFeaturesJSON []byte

		err := rows.Scan(
			&ticket.ID, &ticket.SerialNumber, &ticket.GameCode, &ticket.GameScheduleID, &ticket.DrawNumber, &ticket.DrawID,
			&ticket.GameName, &ticket.GameType,
			pq.Array(&ticket.SelectedNumbers), pq.Array(&ticket.BankerNumbers), pq.Array(&ticket.OpposedNumbers),
			&betLinesJSON, &ticket.NumberOfLines, &ticket.UnitPrice, &ticket.TotalAmount,
			&ticket.IssuerType, &ticket.IssuerID, &issuerDetailsJSON,
			&ticket.CustomerPhone, &ticket.CustomerName, &ticket.CustomerEmail,
			&ticket.PaymentMethod, &ticket.PaymentRef, &ticket.PaymentStatus,
			&ticket.SecurityHash, &securityFeaturesJSON,
			&ticket.Status, &ticket.IsWinning, &ticket.WinningAmount, &ticket.PrizeTier, &ticket.Matches,
			&ticket.DrawDate, &ticket.DrawTime,
			&ticket.IssuedAt, &ticket.ValidatedAt, &ticket.CancelledAt, &ticket.PaidAt, &ticket.PaidBy, &ticket.PaymentReference, &ticket.VoidedAt,
			&ticket.CreatedAt, &ticket.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}

		// Unmarshal JSONB fields
		if len(betLinesJSON) > 0 {
			if err := json.Unmarshal(betLinesJSON, &ticket.BetLines); err != nil {
				return nil, fmt.Errorf("failed to unmarshal bet lines: %w", err)
			}
		}

		if len(issuerDetailsJSON) > 0 {
			ticket.IssuerDetails = &models.IssuerDetails{}
			if err := json.Unmarshal(issuerDetailsJSON, ticket.IssuerDetails); err != nil {
				return nil, fmt.Errorf("failed to unmarshal issuer details: %w", err)
			}
		}

		if len(securityFeaturesJSON) > 0 {
			ticket.SecurityFeatures = &models.SecurityFeatures{}
			if err := json.Unmarshal(securityFeaturesJSON, ticket.SecurityFeatures); err != nil {
				return nil, fmt.Errorf("failed to unmarshal security features: %w", err)
			}
		}

		tickets = append(tickets, ticket)
	}

	span.SetAttributes(attribute.Int("tickets.count", len(tickets)))
	return tickets, nil
}

// UpdateDrawIdBySchedule updates draw_id for all tickets with a specific game_schedule_id
func (r *ticketRepository) UpdateDrawIdBySchedule(ctx context.Context, gameScheduleID uuid.UUID, drawID uuid.UUID) (int64, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.update_draw_id_by_schedule")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "tickets"),
		attribute.String("game_schedule_id", gameScheduleID.String()),
		attribute.String("draw_id", drawID.String()),
	)

	query := `
		UPDATE tickets
		SET draw_id = $1, updated_at = NOW()
		WHERE game_schedule_id = $2`

	result, err := r.db.ExecContext(ctx, query, drawID, gameScheduleID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update tickets with draw_id")
		return 0, fmt.Errorf("failed to update tickets with draw_id: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get rows affected")
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	span.SetAttributes(attribute.Int64("rows_affected", rowsAffected))
	return rowsAffected, nil
}

// DailyMetrics represents daily metrics data
type DailyMetrics struct {
	// Today's metrics
	TodayRevenue   int64
	TodayTickets   int64
	TodayPayouts   int64
	WinningTickets int64
	CheckedTickets int64

	// NEW: Additional today's metrics
	PaidTicketsCount           int64 // Number of winning tickets paid today
	PaidTicketsAmount          int64 // Total amount paid for winning tickets today
	UnpaidWinningTicketsCount  int64 // Number of unpaid winning tickets (cumulative)
	UnpaidWinningTicketsAmount int64 // Total unpaid winning amounts (cumulative)

	// Yesterday's metrics (for comparison)
	YesterdayRevenue int64
	YesterdayTickets int64
	YesterdayPayouts int64

	// NEW: Additional yesterday's metrics
	YesterdayPaidTicketsCount    int64
	YesterdayPaidTicketsAmount   int64
	YesterdayUnpaidTicketsCount  int64
	YesterdayUnpaidTicketsAmount int64
}

// GetDailyMetrics retrieves daily metrics for a specific date
func (r *ticketRepository) GetDailyMetrics(ctx context.Context, date string) (*DailyMetrics, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_daily_metrics")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.String("metrics.date", date),
	)

	// Optimized single query to get all metrics at once
	query := `
		WITH today_stats AS (
			SELECT
				-- Revenue: sum of ticket amounts created today
				COALESCE(SUM(total_amount), 0) as revenue,

				-- Ticket count: tickets created today
				COUNT(*) as ticket_count,

				-- Payouts: sum of winning amounts paid today (based on paid_at date)
				COALESCE(SUM(CASE
					WHEN is_winning = true
						AND status = 'paid'
						AND DATE(paid_at) = $1::date
					THEN winning_amount
					ELSE 0
				END), 0) as payouts,

				-- Win rate components
				COUNT(CASE
					WHEN is_winning = true
						AND status IN ('won', 'paid')
						AND draw_id IS NOT NULL
					THEN 1
				END) as winning_tickets,

				COUNT(CASE
					WHEN draw_id IS NOT NULL
					THEN 1
				END) as checked_tickets
			FROM tickets
			WHERE DATE(created_at) = $1::date
				AND status NOT IN ('cancelled', 'void')
		),
		yesterday_stats AS (
			SELECT
				COALESCE(SUM(total_amount), 0) as revenue,
				COUNT(*) as ticket_count,
				COALESCE(SUM(CASE
					WHEN is_winning = true
						AND status = 'paid'
						AND DATE(paid_at) = ($1::date - INTERVAL '1 day')::date
					THEN winning_amount
					ELSE 0
				END), 0) as payouts
			FROM tickets
			WHERE DATE(created_at) = ($1::date - INTERVAL '1 day')::date
				AND status NOT IN ('cancelled', 'void')
		),
		paid_unpaid_today AS (
			SELECT
				-- Paid tickets: tickets paid on target date
				COUNT(CASE
					WHEN is_winning = true
						AND status = 'paid'
						AND DATE(paid_at) = $1::date
					THEN 1
				END) as paid_count,

				COALESCE(SUM(CASE
					WHEN is_winning = true
						AND status = 'paid'
						AND DATE(paid_at) = $1::date
					THEN winning_amount
					ELSE 0
				END), 0) as paid_amount,

				-- Unpaid winning tickets: cumulative as of target date
				COUNT(CASE
					WHEN is_winning = true
						AND status IN ('won')
						AND paid_at IS NULL
						AND DATE(created_at) <= $1::date
					THEN 1
				END) as unpaid_count,

				COALESCE(SUM(CASE
					WHEN is_winning = true
						AND status IN ('won')
						AND paid_at IS NULL
						AND DATE(created_at) <= $1::date
					THEN winning_amount
					ELSE 0
				END), 0) as unpaid_amount
			FROM tickets
			WHERE status NOT IN ('cancelled', 'void')
		),
		paid_unpaid_yesterday AS (
			SELECT
				COUNT(CASE
					WHEN is_winning = true
						AND status = 'paid'
						AND DATE(paid_at) = ($1::date - INTERVAL '1 day')::date
					THEN 1
				END) as paid_count,

				COALESCE(SUM(CASE
					WHEN is_winning = true
						AND status = 'paid'
						AND DATE(paid_at) = ($1::date - INTERVAL '1 day')::date
					THEN winning_amount
					ELSE 0
				END), 0) as paid_amount,

				COUNT(CASE
					WHEN is_winning = true
						AND status IN ('won')
						AND paid_at IS NULL
						AND DATE(created_at) <= ($1::date - INTERVAL '1 day')::date
					THEN 1
				END) as unpaid_count,

				COALESCE(SUM(CASE
					WHEN is_winning = true
						AND status IN ('won')
						AND paid_at IS NULL
						AND DATE(created_at) <= ($1::date - INTERVAL '1 day')::date
					THEN winning_amount
					ELSE 0
				END), 0) as unpaid_amount
			FROM tickets
			WHERE status NOT IN ('cancelled', 'void')
		)
		SELECT
			-- Today's metrics
			t.revenue as today_revenue,
			t.ticket_count as today_tickets,
			t.payouts as today_payouts,
			t.winning_tickets,
			t.checked_tickets,

			-- NEW: Today's paid/unpaid metrics
			pt.paid_count as today_paid_count,
			pt.paid_amount as today_paid_amount,
			pt.unpaid_count as today_unpaid_count,
			pt.unpaid_amount as today_unpaid_amount,

			-- Yesterday's metrics
			y.revenue as yesterday_revenue,
			y.ticket_count as yesterday_tickets,
			y.payouts as yesterday_payouts,

			-- NEW: Yesterday's paid/unpaid metrics
			py.paid_count as yesterday_paid_count,
			py.paid_amount as yesterday_paid_amount,
			py.unpaid_count as yesterday_unpaid_count,
			py.unpaid_amount as yesterday_unpaid_amount
		FROM today_stats t, yesterday_stats y, paid_unpaid_today pt, paid_unpaid_yesterday py`

	metrics := &DailyMetrics{}
	err := r.db.QueryRowContext(ctx, query, date).Scan(
		&metrics.TodayRevenue,
		&metrics.TodayTickets,
		&metrics.TodayPayouts,
		&metrics.WinningTickets,
		&metrics.CheckedTickets,
		// NEW: Today's paid/unpaid metrics
		&metrics.PaidTicketsCount,
		&metrics.PaidTicketsAmount,
		&metrics.UnpaidWinningTicketsCount,
		&metrics.UnpaidWinningTicketsAmount,
		&metrics.YesterdayRevenue,
		&metrics.YesterdayTickets,
		&metrics.YesterdayPayouts,
		// NEW: Yesterday's paid/unpaid metrics
		&metrics.YesterdayPaidTicketsCount,
		&metrics.YesterdayPaidTicketsAmount,
		&metrics.YesterdayUnpaidTicketsCount,
		&metrics.YesterdayUnpaidTicketsAmount,
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get daily metrics")
		return nil, fmt.Errorf("failed to get daily metrics: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("metrics.today_revenue", metrics.TodayRevenue),
		attribute.Int64("metrics.today_tickets", metrics.TodayTickets),
		attribute.Int64("metrics.today_payouts", metrics.TodayPayouts),
		attribute.Int64("metrics.winning_tickets", metrics.WinningTickets),
	)

	return metrics, nil
}

// MonthlyMetrics represents monthly aggregated metrics
type MonthlyMetrics struct {
	Month      string  // YYYY-MM format
	Year       int32   // Year (e.g., 2025)
	Revenue    int64   // Revenue in pesewas
	RevenueGHS float64 // Revenue in GHS (for convenience)
	Tickets    int64   // Number of tickets sold
	Payouts    int64   // Payouts in pesewas
	PayoutsGHS float64 // Payouts in GHS (for convenience)
}

// GetMonthlyMetrics retrieves monthly aggregated metrics for the last N months
func (r *ticketRepository) GetMonthlyMetrics(ctx context.Context, months int32) ([]*MonthlyMetrics, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_monthly_metrics")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.Int("months", int(months)),
	)

	// Default to 6 months if not specified or invalid
	if months <= 0 {
		months = 6
	}

	// Query to aggregate monthly metrics
	query := `
		WITH month_series AS (
			-- Generate series of months
			SELECT
				TO_CHAR(d, 'YYYY-MM') as month,
				EXTRACT(YEAR FROM d)::int as year
			FROM generate_series(
				DATE_TRUNC('month', NOW()) - ($1::int - 1 || ' months')::interval,
				DATE_TRUNC('month', NOW()),
				'1 month'::interval
			) d
		),
		monthly_stats AS (
			SELECT
				TO_CHAR(DATE_TRUNC('month', created_at), 'YYYY-MM') as month,
				EXTRACT(YEAR FROM DATE_TRUNC('month', created_at))::int as year,

				-- Revenue: sum of ticket amounts created in the month
				COALESCE(SUM(total_amount), 0) as revenue,

				-- Ticket count: tickets created in the month
				COUNT(*) as ticket_count,

				-- Payouts: sum of winning amounts paid in the month (based on paid_at date)
				COALESCE(SUM(CASE
					WHEN is_winning = true
						AND status = 'paid'
						AND DATE_TRUNC('month', paid_at) = DATE_TRUNC('month', created_at)
					THEN winning_amount
					ELSE 0
				END), 0) as payouts
			FROM tickets
			WHERE DATE_TRUNC('month', created_at) >= DATE_TRUNC('month', NOW()) - ($1::int - 1 || ' months')::interval
				AND DATE_TRUNC('month', created_at) <= DATE_TRUNC('month', NOW())
				AND status NOT IN ('cancelled', 'void')
			GROUP BY DATE_TRUNC('month', created_at), EXTRACT(YEAR FROM DATE_TRUNC('month', created_at))
		)
		SELECT
			ms.month,
			ms.year,
			COALESCE(s.revenue, 0) as revenue,
			COALESCE(s.revenue, 0)::float / 100.0 as revenue_ghs,
			COALESCE(s.ticket_count, 0) as tickets,
			COALESCE(s.payouts, 0) as payouts,
			COALESCE(s.payouts, 0)::float / 100.0 as payouts_ghs
		FROM month_series ms
		LEFT JOIN monthly_stats s ON ms.month = s.month
		ORDER BY ms.month ASC`

	rows, err := r.db.QueryContext(ctx, query, months)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get monthly metrics")
		return nil, fmt.Errorf("failed to get monthly metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	metrics := []*MonthlyMetrics{}
	for rows.Next() {
		m := &MonthlyMetrics{}
		err := rows.Scan(
			&m.Month,
			&m.Year,
			&m.Revenue,
			&m.RevenueGHS,
			&m.Tickets,
			&m.Payouts,
			&m.PayoutsGHS,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to scan monthly metric")
			return nil, fmt.Errorf("failed to scan monthly metric: %w", err)
		}
		metrics = append(metrics, m)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rows iteration error")
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	span.SetAttributes(attribute.Int("metrics.count", len(metrics)))
	return metrics, nil
}

// RetailerPerformance represents performance metrics for a retailer
type RetailerPerformance struct {
	RetailerID   string  // Retailer code/ID
	TotalRevenue int64   // Revenue in pesewas
	TotalTickets int64   // Number of tickets sold
	RevenueGHS   float64 // Revenue in GHS (for convenience)
}

// GetRetailerPerformanceByPeriod retrieves retailer performance metrics for a given period
// period can be "daily", "monthly", or "yearly"
// date should be in YYYY-MM-DD format
func (r *ticketRepository) GetRetailerPerformanceByPeriod(ctx context.Context, period string, date string) ([]*RetailerPerformance, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_retailer_performance")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "tickets"),
		attribute.String("period", period),
		attribute.String("date", date),
	)

	// Determine date filter based on period
	var dateFilter string
	switch period {
	case "daily":
		dateFilter = "DATE(created_at) = $1::date"
	case "monthly":
		dateFilter = "TO_CHAR(created_at, 'YYYY-MM') = TO_CHAR($1::date, 'YYYY-MM')"
	case "yearly":
		dateFilter = "EXTRACT(YEAR FROM created_at) = EXTRACT(YEAR FROM $1::date)"
	default:
		// Default to monthly
		period = "monthly"
		dateFilter = "TO_CHAR(created_at, 'YYYY-MM') = TO_CHAR($1::date, 'YYYY-MM')"
	}

	// Query to aggregate metrics by retailer
	// Only include POS tickets (issuer_type = 'pos') as these are issued by retailers
	query := fmt.Sprintf(`
		SELECT
			issuer_id as retailer_id,
			COALESCE(SUM(total_amount), 0) as total_revenue,
			COUNT(*) as total_tickets,
			COALESCE(SUM(total_amount), 0)::float / 100.0 as revenue_ghs
		FROM tickets
		WHERE %s
			AND issuer_type = 'pos'
			AND status NOT IN ('cancelled', 'void')
			AND issuer_id IS NOT NULL
			AND issuer_id != ''
		GROUP BY issuer_id
		HAVING COUNT(*) > 0
		ORDER BY total_revenue DESC`, dateFilter)

	rows, err := r.db.QueryContext(ctx, query, date)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get retailer performance")
		return nil, fmt.Errorf("failed to get retailer performance: %w", err)
	}
	defer func() { _ = rows.Close() }()

	performances := []*RetailerPerformance{}
	for rows.Next() {
		p := &RetailerPerformance{}
		err := rows.Scan(
			&p.RetailerID,
			&p.TotalRevenue,
			&p.TotalTickets,
			&p.RevenueGHS,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to scan retailer performance")
			return nil, fmt.Errorf("failed to scan retailer performance: %w", err)
		}
		performances = append(performances, p)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rows iteration error")
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	span.SetAttributes(
		attribute.Int("retailer.count", len(performances)),
		attribute.String("period.used", period),
	)

	return performances, nil
}

// AgentPerformanceData represents aggregated performance metrics for an agent
type AgentPerformanceData struct {
	AgentID       string  // Agent UUID
	AgentCode     string  // Agent code
	AgentName     string  // Agent business name
	TotalRevenue  int64   // Total revenue in pesewas from all retailers
	RevenueGHS    float64 // Revenue in GHS
	TotalTickets  int64   // Total tickets sold by all retailers
	RetailerCount int32   // Number of active retailers under this agent
}

// GetTopPerformingAgents retrieves top performing agents by aggregating sales by issuer_id (agent_id)
// The service layer will enrich this data with agent details via gRPC
func (r *ticketRepository) GetTopPerformingAgents(ctx context.Context, period string, date string, limit int32) ([]*AgentPerformanceData, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-ticket").Start(ctx, "db.ticket.get_top_performing_agents")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("period", period),
		attribute.String("date", date),
		attribute.Int("limit", int(limit)),
	)

	// Default limit
	if limit <= 0 {
		limit = 10
	}

	// Determine date filter based on period
	var dateFilter string
	switch period {
	case "daily":
		dateFilter = "DATE(created_at) = $1::date"
	case "monthly":
		dateFilter = "TO_CHAR(created_at, 'YYYY-MM') = TO_CHAR($1::date, 'YYYY-MM')"
	case "yearly":
		dateFilter = "EXTRACT(YEAR FROM created_at) = EXTRACT(YEAR FROM $1::date)"
	default:
		// Default to monthly
		period = "monthly"
		dateFilter = "TO_CHAR(created_at, 'YYYY-MM') = TO_CHAR($1::date, 'YYYY-MM')"
	}

	// This query aggregates ticket sales by issuer_id (which is the agent_id for agent-issued tickets)
	// The service layer will then call Agent Management Service to get agent details
	query := fmt.Sprintf(`
		SELECT
			issuer_id as agent_id,
			COALESCE(SUM(total_amount), 0) as total_revenue,
			COALESCE(SUM(total_amount), 0)::float / 100.0 as revenue_ghs,
			COUNT(*) as total_tickets
		FROM tickets
		WHERE %s
			AND issuer_type = 'pos'
			AND status NOT IN ('cancelled', 'void')
			AND issuer_id IS NOT NULL
			AND issuer_id != ''
		GROUP BY issuer_id
		HAVING COUNT(*) > 0
		ORDER BY total_revenue DESC
		LIMIT $2`, dateFilter)

	rows, err := r.db.QueryContext(ctx, query, date, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get top performing agents")
		return nil, fmt.Errorf("failed to get top performing agents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	agents := []*AgentPerformanceData{}
	for rows.Next() {
		agent := &AgentPerformanceData{}
		err := rows.Scan(
			&agent.AgentID,
			&agent.TotalRevenue,
			&agent.RevenueGHS,
			&agent.TotalTickets,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to scan agent performance")
			return nil, fmt.Errorf("failed to scan agent performance: %w", err)
		}
		// AgentCode, AgentName, and RetailerCount will be populated by the service layer
		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rows iteration error")
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	span.SetAttributes(
		attribute.Int("agent.count", len(agents)),
		attribute.String("period.used", period),
	)

	return agents, nil
}
