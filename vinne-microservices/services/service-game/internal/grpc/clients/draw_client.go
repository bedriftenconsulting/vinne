package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	drawpb "github.com/randco/randco-microservices/proto/draw/v1"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DrawServiceClient defines the interface for communicating with the Draw Service
type DrawServiceClient interface {
	CreateDraw(ctx context.Context, game *models.Game, schedule *models.GameSchedule, ticketClient TicketServiceClient) (uuid.UUID, error)
	Close() error
}

// drawServiceClient implements DrawServiceClient interface
type drawServiceClient struct {
	conn   *grpc.ClientConn
	client drawpb.DrawServiceClient
	tracer trace.Tracer
	addr   string
}

// NewDrawServiceClient creates a new Draw Service gRPC client
func NewDrawServiceClient(addr string) (DrawServiceClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to draw service at %s: %w", addr, err)
	}

	return &drawServiceClient{
		conn:   conn,
		client: drawpb.NewDrawServiceClient(conn),
		tracer: otel.Tracer("service-game"),
		addr:   addr,
	}, nil
}

// CreateDraw creates a new draw record in the Draw Service
func (c *drawServiceClient) CreateDraw(
	ctx context.Context,
	game *models.Game,
	schedule *models.GameSchedule,
	ticketClient TicketServiceClient,
) (uuid.UUID, error) {
	ctx, span := c.tracer.Start(ctx, "grpc.draw_service.create_draw")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", game.ID.String()),
		attribute.String("game.name", game.Name),
		attribute.String("schedule.id", schedule.ID.String()),
		attribute.String("scheduled_draw", schedule.ScheduledDraw.Format(time.RFC3339)),
	)

	// Fetch ticket statistics from Ticket Service
	var totalTickets int64
	var totalStakes int64
	if ticketClient != nil {
		tickets, stakes, err := ticketClient.GetTicketStatsBySchedule(ctx, schedule.ID.String())
		if err != nil {
			// Log error but don't fail draw creation
			span.RecordError(err)
			span.AddEvent("failed to fetch ticket statistics", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
		} else {
			totalTickets = tickets
			totalStakes = stakes
			span.SetAttributes(
				attribute.Int64("tickets.count", totalTickets),
				attribute.Int64("tickets.total_stakes", totalStakes),
			)
		}
	}

	// Create draw request with ticket statistics
	req := &drawpb.CreateDrawRequest{
		GameId:           game.ID.String(),
		GameName:         game.Name,
		GameCode:         game.Code, // Pass game code for efficient ticket querying
		DrawName:         fmt.Sprintf("%s - %s", game.Name, schedule.ScheduledDraw.Format("2006-01-02 15:04")),
		ScheduledTime:    timestamppb.New(schedule.ScheduledDraw),
		DrawLocation:     "Automated Scheduler", // Indicates this was auto-created by scheduler
		TotalTicketsSold: totalTickets,
		TotalPrizePool:   totalStakes,
		GameScheduleId:   schedule.ID.String(),
	}

	// Call Draw Service to create draw
	resp, err := c.client.CreateDraw(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create draw")
		return uuid.Nil, fmt.Errorf("failed to create draw via gRPC: %w", err)
	}

	if !resp.Success {
		err := fmt.Errorf("draw creation failed: %s", resp.Message)
		span.RecordError(err)
		span.SetStatus(codes.Error, "draw service returned failure")
		return uuid.Nil, err
	}

	// Parse draw ID from response
	drawID, err := uuid.Parse(resp.Draw.Id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid draw ID format")
		return uuid.Nil, fmt.Errorf("invalid draw ID returned from draw service: %w", err)
	}

	span.SetAttributes(attribute.String("draw.id", drawID.String()))
	span.SetStatus(codes.Ok, "draw created successfully")

	return drawID, nil
}

// Close closes the gRPC connection
func (c *drawServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// MockDrawServiceClient is a mock implementation for testing
type MockDrawServiceClient struct {
	CreateDrawFunc func(ctx context.Context, game *models.Game, schedule *models.GameSchedule, ticketClient TicketServiceClient) (uuid.UUID, error)
	CloseFunc      func() error
}

func (m *MockDrawServiceClient) CreateDraw(ctx context.Context, game *models.Game, schedule *models.GameSchedule, ticketClient TicketServiceClient) (uuid.UUID, error) {
	if m.CreateDrawFunc != nil {
		return m.CreateDrawFunc(ctx, game, schedule, ticketClient)
	}
	return uuid.New(), nil
}

func (m *MockDrawServiceClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
