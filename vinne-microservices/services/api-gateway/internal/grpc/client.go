package grpc

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/randco/randco-microservices/shared/circuitbreaker"
	"github.com/randco/randco-microservices/shared/retry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	// authv1 "github.com/randco/randco-microservices/proto/admin/auth/v1" // Not used anymore - merged into admin-management
	adminmgmtpb "github.com/randco/randco-microservices/proto/admin/management/v1"
	agentauthv1 "github.com/randco/randco-microservices/proto/agent/auth/v1"
	agentmgmtpb "github.com/randco/randco-microservices/proto/agent/management/v1"
	gamepb "github.com/randco/randco-microservices/proto/game/v1"
	notificationv1 "github.com/randco/randco-microservices/proto/notification/v1"
	playerv1 "github.com/randco/randco-microservices/proto/player/v1"
	terminalpb "github.com/randco/randco-microservices/proto/terminal/v1"
	ticketv1 "github.com/randco/randco-microservices/proto/ticket/v1"
	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/randco-microservices/shared/common/logger"
)

// ServiceConfig holds configuration for a service
type ServiceConfig struct {
	Name    string
	Address string
	Timeout time.Duration
}

// ClientManager manages gRPC client connections
type ClientManager struct {
	connections     map[string]*grpc.ClientConn
	configs         map[string]ServiceConfig
	circuitBreakers map[string]*circuitbreaker.CircuitBreaker
	retriers        map[string]*retry.Retrier
	mu              sync.RWMutex
	log             logger.Logger
}

// NewClientManager creates a new gRPC client manager
func NewClientManager(log logger.Logger) *ClientManager {
	return &ClientManager{
		connections:     make(map[string]*grpc.ClientConn),
		configs:         make(map[string]ServiceConfig),
		circuitBreakers: make(map[string]*circuitbreaker.CircuitBreaker),
		retriers:        make(map[string]*retry.Retrier),
		log:             log,
	}
}

// RegisterService registers a service configuration
func (m *ClientManager) RegisterService(config ServiceConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[config.Name] = config

	// Initialize circuit breaker for this service
	cbConfig := circuitbreaker.Config{
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenRequests: 3,
		OnStateChange: func(from, to circuitbreaker.State) {
			m.log.Info("Circuit breaker state changed",
				"service", config.Name,
				"from", from.String(),
				"to", to.String())
		},
	}
	m.circuitBreakers[config.Name] = circuitbreaker.New(cbConfig)

	// Initialize retrier for this service
	retryConfig := retry.Config{
		MaxAttempts:     3,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2.0,
		Jitter:          true,
		OnRetry: func(attempt int, err error) {
			m.log.Debug("Retrying gRPC call",
				"service", config.Name,
				"attempt", attempt,
				"error", err)
		},
	}
	m.retriers[config.Name] = retry.New(retryConfig)

	m.log.Info("Service registered", "name", config.Name, "address", config.Address)
}

// GetConnection gets or creates a connection to a service
func (m *ClientManager) GetConnection(service string) (*grpc.ClientConn, error) {
	if m == nil {
		m.log.Error("[GetConnection] Client manager is nil")
		return nil, fmt.Errorf("client manager is nil")
	}

	m.log.Debug("[GetConnection] Getting connection for service", "service", service)

	// Use full lock to avoid race conditions when checking connection health
	m.mu.Lock()
	conn, exists := m.connections[service]

	if exists && conn != nil && m.isConnectionHealthyLocked(conn) {
		m.mu.Unlock()
		m.log.Debug("[GetConnection] Existing connection is healthy", "service", service)
		return conn, nil
	}
	m.mu.Unlock()

	if exists && conn != nil {
		m.log.Warn("[GetConnection] Existing connection is unhealthy, creating new one", "service", service)
	} else {
		m.log.Debug("[GetConnection] No existing connection found", "service", service, "exists", exists, "connNil", conn == nil)
	}

	return m.createConnection(service)
}

// createConnection creates a new gRPC connection
func (m *ClientManager) createConnection(service string) (*grpc.ClientConn, error) {
	if m == nil {
		m.log.Error("[createConnection] Client manager is nil")
		return nil, fmt.Errorf("client manager is nil")
	}

	m.log.Debug("[createConnection] Creating new connection", "service", service)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if connection was created while waiting for lock
	if conn, exists := m.connections[service]; exists && conn != nil && m.isConnectionHealthyLocked(conn) {
		m.log.Debug("[createConnection] Connection created by another routine", "service", service)
		return conn, nil
	}

	config, ok := m.configs[service]
	if !ok {
		m.log.Error("[createConnection] Service not registered", "service", service)
		m.log.Debug("[createConnection] Available services", "services", m.configs)
		return nil, fmt.Errorf("service %s not registered", service)
	}

	m.log.Debug("[createConnection] Dialing service", "service", service, "address", config.Address)

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
		m.log.Error("[createConnection] Failed to dial service", "service", service, "address", config.Address, "error", err)
		return nil, err
	}

	m.connections[service] = conn
	m.log.Info("Connected to service", "service", service, "address", config.Address)
	return conn, nil
}

// isConnectionHealthyLocked checks if a connection is healthy (must be called with lock held)
func (m *ClientManager) isConnectionHealthyLocked(conn *grpc.ClientConn) bool {
	if conn == nil {
		return false
	}

	// Try to get connection state
	state := conn.GetState()
	switch state {
	case connectivity.Shutdown, connectivity.TransientFailure:
		return false
	case connectivity.Ready:
		return true
	case connectivity.Idle, connectivity.Connecting:
		// For idle/connecting, we consider it healthy as it can become ready
		return true
	default:
		return false
	}
}

// ExecuteWithProtection executes a gRPC call with circuit breaker and retry protection
func (m *ClientManager) ExecuteWithProtection(ctx context.Context, service string, fn func(context.Context) error) error {
	// Get circuit breaker for this service
	m.mu.RLock()
	cb, cbExists := m.circuitBreakers[service]
	r, rExists := m.retriers[service]
	m.mu.RUnlock()

	// If no circuit breaker or retrier, execute directly
	if !cbExists || !rExists {
		return fn(ctx)
	}

	// Execute with circuit breaker and retry
	return cb.Execute(ctx, func(ctx context.Context) error {
		return r.Execute(ctx, fn)
	})
}

// ExecuteWithRetry executes a function with retry and circuit breaker
func (m *ClientManager) ExecuteWithRetry(ctx context.Context, service string, fn func(context.Context) error) error {
	// Get retrier for service
	m.mu.RLock()
	retrier, hasRetrier := m.retriers[service]
	cb, hasCB := m.circuitBreakers[service]
	m.mu.RUnlock()

	// If no retrier configured, execute directly
	if !hasRetrier {
		return fn(ctx)
	}

	// Wrap with circuit breaker if available
	var operation func(context.Context) error
	if hasCB {
		operation = func(ctx context.Context) error {
			return cb.Execute(ctx, func(innerCtx context.Context) error {
				return fn(innerCtx)
			})
		}
	} else {
		operation = fn
	}

	// Execute with retry
	return retrier.Execute(ctx, operation)
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
				m.log.Error("Failed to close connection", "service", service, "error", err)
			}
		}
	}

	m.connections = make(map[string]*grpc.ClientConn)
	return nil
}

// AdminAuthClient is deprecated - use AdminManagementClient instead
// Admin auth functionality has been merged into AdminManagementService

// AgentAuthClient returns the agent auth service client
func (m *ClientManager) AgentAuthClient() (agentauthv1.AgentAuthServiceClient, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("agent-auth")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return agentauthv1.NewAgentAuthServiceClient(conn), nil
}

// AdminManagementClient returns the admin management service client
func (m *ClientManager) AdminManagementClient() (adminmgmtpb.AdminManagementServiceClient, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("admin-management")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return adminmgmtpb.NewAdminManagementServiceClient(conn), nil
}

// AgentManagementClient returns the agent management service client
func (m *ClientManager) AgentManagementClient() (agentmgmtpb.AgentManagementServiceClient, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("agent-management")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return agentmgmtpb.NewAgentManagementServiceClient(conn), nil
}

// GameServiceClient returns the game service client
func (m *ClientManager) GameServiceClient() (gamepb.GameServiceClient, error) {
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

	return gamepb.NewGameServiceClient(conn), nil
}

// TicketServiceClient returns the ticket service client
func (m *ClientManager) TicketServiceClient() (ticketv1.TicketServiceClient, error) {
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

// PlayerServiceClient returns the player service client
func (m *ClientManager) PlayerServiceClient() (playerv1.PlayerServiceClient, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("player")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return playerv1.NewPlayerServiceClient(conn), nil
}

// GetNotificationServiceConn returns the connection to the notification service
func (m *ClientManager) GetNotificationServiceConn() (*grpc.ClientConn, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("notification")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return conn, nil
}

// NotificationServiceClient returns the notification service client
func (m *ClientManager) NotificationServiceClient() (notificationv1.NotificationServiceClient, error) {
	conn, err := m.GetNotificationServiceConn()
	if err != nil {
		return nil, err
	}

	return notificationv1.NewNotificationServiceClient(conn), nil
}

// WalletServiceClient returns the wallet service client
func (m *ClientManager) WalletServiceClient() (walletv1.WalletServiceClient, error) {
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

// TerminalServiceClient returns the terminal service client
func (m *ClientManager) TerminalServiceClient() (terminalpb.TerminalServiceClient, error) {
	if m == nil {
		return nil, fmt.Errorf("client manager is nil")
	}

	conn, err := m.GetConnection("terminal")
	if err != nil {
		return nil, err
	}

	if conn == nil {
		return nil, fmt.Errorf("connection is nil")
	}

	return terminalpb.NewTerminalServiceClient(conn), nil
}

// TODO: Add more service clients as they are implemented
