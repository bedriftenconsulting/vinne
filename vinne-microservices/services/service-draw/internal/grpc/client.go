package grpc

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	gamev1 "github.com/randco/randco-microservices/proto/game/v1"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ServiceConfig holds configuration for a service
type ServiceConfig struct {
	Name    string
	Address string
	Timeout time.Duration
}

// ClientManager manages gRPC client connections
type ClientManager struct {
	connections map[string]*grpc.ClientConn
	configs     map[string]ServiceConfig
	mu          sync.RWMutex
	logger      *log.Logger
}

// NewClientManager creates a new gRPC client manager
func NewClientManager(logger *log.Logger) *ClientManager {
	return &ClientManager{
		connections: make(map[string]*grpc.ClientConn),
		configs:     make(map[string]ServiceConfig),
		logger:      logger,
	}
}

// RegisterService registers a service configuration
func (m *ClientManager) RegisterService(config ServiceConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[config.Name] = config
	m.logger.Printf("Service registered: %s at %s", config.Name, config.Address)
}

// GetConnection gets or creates a connection to a service
func (m *ClientManager) GetConnection(service string) (*grpc.ClientConn, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	m.mu.Lock()
	conn, exists := m.connections[service]

	if exists && conn != nil && m.isConnectionHealthyLocked(conn) {
		m.mu.Unlock()
		return conn, nil
	}
	m.mu.Unlock()

	if exists && conn != nil {
		m.logger.Printf("Existing connection to %s is unhealthy, creating new one", service)
	}

	return m.createConnection(service)
}

// createConnection creates a new gRPC connection
func (m *ClientManager) createConnection(service string) (*grpc.ClientConn, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if connection was created while waiting for lock
	if conn, exists := m.connections[service]; exists && conn != nil && m.isConnectionHealthyLocked(conn) {
		return conn, nil
	}

	config, ok := m.configs[service]
	if !ok {
		return nil, fmt.Errorf("service %s not registered", service)
	}

	// Create connection with options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	// Add tracing interceptors if tracing is enabled
	if os.Getenv("TRACING_ENABLED") != "false" {
		opts = append(opts,
			grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		)
	}

	conn, err := grpc.NewClient(config.Address, opts...)
	if err != nil {
		m.logger.Printf("Failed to dial service %s at %s: %v", service, config.Address, err)
		return nil, err
	}

	m.connections[service] = conn
	m.logger.Printf("Connected to service: %s at %s", service, config.Address)
	return conn, nil
}

// isConnectionHealthyLocked checks if a connection is healthy (must be called with lock held)
func (m *ClientManager) isConnectionHealthyLocked(conn *grpc.ClientConn) bool {
	if conn == nil {
		return false
	}

	state := conn.GetState()
	switch state {
	case connectivity.Shutdown, connectivity.TransientFailure:
		return false
	case connectivity.Ready, connectivity.Idle, connectivity.Connecting:
		return true
	default:
		return false
	}
}

// Close closes all connections
func (m *ClientManager) Close() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for service, conn := range m.connections {
		if conn != nil {
			if err := conn.Close(); err != nil {
				m.logger.Printf("Failed to close connection to %s: %v", service, err)
			}
		}
	}

	m.connections = make(map[string]*grpc.ClientConn)
	return nil
}

// TicketServiceClient returns the ticket service client as interface{}
func (m *ClientManager) TicketServiceClient(ctx context.Context) (interface{}, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("ticket")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return ticketv1.NewTicketServiceClient(conn), nil
}

// WalletServiceClient returns the wallet service client as interface{}
func (m *ClientManager) WalletServiceClient(ctx context.Context) (interface{}, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("wallet")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return walletv1.NewWalletServiceClient(conn), nil
}

// GameServiceClient returns the game service client
func (m *ClientManager) GameServiceClient(ctx context.Context) (gamev1.GameServiceClient, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("game")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return gamev1.NewGameServiceClient(conn), nil
}
