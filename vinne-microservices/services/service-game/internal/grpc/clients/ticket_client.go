package clients

import (
	"context"
	"fmt"

	ticketpb "github.com/randco/randco-microservices/proto/ticket/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TicketServiceClient defines the interface for communicating with the Ticket Service
type TicketServiceClient interface {
	GetTicketStatsBySchedule(ctx context.Context, scheduleID string) (int64, int64, error)
	Close() error
}

// ticketServiceClient implements TicketServiceClient interface
type ticketServiceClient struct {
	conn   *grpc.ClientConn
	client ticketpb.TicketServiceClient
	tracer trace.Tracer
	addr   string
}

// NewTicketServiceClient creates a new Ticket Service gRPC client
func NewTicketServiceClient(addr string) (TicketServiceClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ticket service at %s: %w", addr, err)
	}

	return &ticketServiceClient{
		conn:   conn,
		client: ticketpb.NewTicketServiceClient(conn),
		tracer: otel.Tracer("service-game"),
		addr:   addr,
	}, nil
}

// GetTicketStatsBySchedule gets ticket statistics for a game schedule
func (c *ticketServiceClient) GetTicketStatsBySchedule(
	ctx context.Context,
	scheduleID string,
) (int64, int64, error) {
	ctx, span := c.tracer.Start(ctx, "grpc.ticket_service.get_stats")
	defer span.End()

	span.SetAttributes(
		attribute.String("schedule.id", scheduleID),
	)

	// Create filter with game_schedule_id to get only relevant tickets
	req := &ticketpb.ListTicketsRequest{
		Filter: &ticketpb.TicketFilter{
			GameScheduleId: scheduleID,
		},
		Page:     1,
		PageSize: 10000, // Large page size to get all tickets for this schedule
	}

	resp, err := c.client.ListTickets(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get tickets")
		return 0, 0, fmt.Errorf("failed to get tickets via gRPC: %w", err)
	}

	// Calculate totals from filtered response
	var totalTickets int64
	var totalStakes int64

	for _, ticket := range resp.Tickets {
		totalTickets++
		totalStakes += ticket.TotalAmount
	}

	// If we hit the page size limit, use the total count from response
	// This assumes the ticket service returns accurate total counts
	if resp.Total > int64(len(resp.Tickets)) {
		// We got partial results due to pagination
		// Calculate average stake and estimate total
		if totalTickets > 0 {
			avgStake := totalStakes / totalTickets
			totalTickets = resp.Total
			totalStakes = avgStake * resp.Total
		}
	}

	span.SetAttributes(
		attribute.Int64("tickets.count", totalTickets),
		attribute.Int64("tickets.total_stakes", totalStakes),
		attribute.Bool("partial_results", resp.Total > int64(len(resp.Tickets))),
	)
	span.SetStatus(codes.Ok, "stats retrieved successfully")

	return totalTickets, totalStakes, nil
}

// Close closes the gRPC connection
func (c *ticketServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// MockTicketServiceClient is a mock implementation for testing
type MockTicketServiceClient struct {
	GetTicketStatsByScheduleFunc func(ctx context.Context, scheduleID string) (int64, int64, error)
	CloseFunc                    func() error
}

func (m *MockTicketServiceClient) GetTicketStatsBySchedule(ctx context.Context, scheduleID string) (int64, int64, error) {
	if m.GetTicketStatsByScheduleFunc != nil {
		return m.GetTicketStatsByScheduleFunc(ctx, scheduleID)
	}
	return 0, 0, nil
}

func (m *MockTicketServiceClient) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}
