package main

// Rebuild trigger: 2025-09-28T21:28:03Z

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/randco/randco-microservices/services/api-gateway/internal/config"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/handlers"
	"github.com/randco/randco-microservices/services/api-gateway/internal/middleware"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/services/api-gateway/internal/timeutil"
	"github.com/randco/randco-microservices/shared/common/jwt"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/randco-microservices/shared/storage"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Version information (injected at build time)
var (
	Version        = "dev"
	GitBranch      = "unknown"
	GitCommit      = "unknown"
	GitCommitCount = "0"
	BuildTime      = "unknown"
)

func main() {
	// Log version information
	fmt.Printf("Starting API Gateway\n")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Git Branch: %s, Commit: %s (#%s)\n", GitBranch, GitCommit, GitCommitCount)
	fmt.Printf("Build Time: %s\n", BuildTime)

	// Load configuration (Viper handles .env files and environment variables)
	cfg, err := config.Load()
	if err != nil {
		// Use basic logging until logger is initialized
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger using config
	log := logger.NewLogger(logger.Config{
		Level:       cfg.Server.LogLevel,
		Format:      cfg.Server.LogFormat,
		ServiceName: "api-gateway",
		LogFile:     cfg.Server.LogFile,
	})
	defer func() {
		_ = log.Close()
	}()

	log.Info("Configuration loaded successfully")

	// Initialize tracing if enabled
	if cfg.Tracing.Enabled {
		tp, err := initTracer(cfg.Tracing)
		if err != nil {
			log.Error("Failed to initialize tracing", "error", err)
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := tp.Shutdown(ctx); err != nil {
					log.Error("Error shutting down tracer provider", "error", err)
				}
			}()
			log.Info("Tracing initialized successfully",
				"endpoint", cfg.Tracing.JaegerEndpoint,
				"sample_rate", cfg.Tracing.SampleRate,
				"environment", cfg.Tracing.Environment)
		}
	}

	// Initialize Redis client
	log.Info("Redis configuration", "host", cfg.Redis.Host, "port", cfg.Redis.Port)
	redisClient := initRedis(cfg, log)
	if redisClient != nil {
		defer func() {
			_ = redisClient.Close()
		}()
	}

	// Initialize JWT service
	jwtService := jwt.NewService(jwt.Config{
		AccessSecret:    cfg.Security.JWTSecret,
		RefreshSecret:   cfg.Security.JWTSecret + "-refresh",
		AccessDuration:  15 * time.Minute,
		RefreshDuration: 7 * 24 * time.Hour,
	})
	// Log JWT secret info for debugging (mask most of it)
	secretPrefix := cfg.Security.JWTSecret
	if len(secretPrefix) > 8 {
		secretPrefix = secretPrefix[:8] + "..."
	}
	log.Info("JWT service initialized",
		"secret_prefix", secretPrefix,
		"secret_length", len(cfg.Security.JWTSecret))

	// Initialize gRPC client manager
	grpcManager := grpc.NewClientManager(log)
	defer func() {
		_ = grpcManager.Close()
	}()

	// Initialize NTP time service for accurate time from internet
	// This ensures correct time even if server clock is wrong
	ntpTime := timeutil.NewNTPTimeService()
	log.Info("NTP time service initialized",
		"offset", ntpTime.GetOffset(),
		"last_sync", ntpTime.LastSyncTime(),
		"ntp_time", ntpTime.Now().Format(time.RFC3339),
		"system_time", time.Now().Format(time.RFC3339))

	// Initialize storage client for file uploads (e.g., game logos)
	var storageClient storage.Storage
	if cfg.Storage.Provider != "" && cfg.Storage.Bucket != "" {
		storageConfig := storage.Config{
			Provider:        storage.Provider(cfg.Storage.Provider),
			Endpoint:        cfg.Storage.Endpoint,
			Region:          cfg.Storage.Region,
			Bucket:          cfg.Storage.Bucket,
			AccessKeyID:     cfg.Storage.AccessKeyID,
			SecretAccessKey: cfg.Storage.SecretAccessKey,
			CDNEndpoint:     cfg.Storage.CDNEndpoint,
			ForcePathStyle:  cfg.Storage.ForcePathStyle,
		}
		var err error
		storageClient, err = storage.New(storageConfig)
		if err != nil {
			log.Warn("Failed to initialize storage client, file uploads will be disabled", "error", err)
			storageClient = nil
		} else {
			defer func() {
				_ = storageClient.Close()
			}()
			log.Info("Storage client initialized successfully",
				"provider", cfg.Storage.Provider,
				"bucket", cfg.Storage.Bucket,
				"region", cfg.Storage.Region)
		}
	} else {
		log.Info("Storage not configured, file uploads will be disabled")
	}

	// Register services
	registerServices(grpcManager, cfg.Services)

	// Create router
	r := router.NewRouter(log)

	// Apply global middlewares
	// Note: Tracing is handled at the HTTP server level, not as router middleware
	r.Use(middleware.LoggingMiddleware(log))

	// Security headers middleware
	if cfg.Security.EnableSecurityHeaders {
		r.Use(middleware.SecurityHeadersMiddleware())
	}

	// Request size limiting middleware
	if cfg.Security.MaxRequestBodySize > 0 {
		r.Use(middleware.RequestSizeLimitMiddleware(cfg.Security.MaxRequestBodySize))
	}

	// Create CORS config from security config
	corsConfig := middleware.NewCORSConfig(&middleware.SecurityConfig{
		JWTSecret:        cfg.Security.JWTSecret,
		JWTIssuer:        cfg.Security.JWTIssuer,
		AllowedOrigins:   cfg.Security.AllowedOrigins,
		AllowedHeaders:   cfg.Security.AllowedHeaders,
		AllowedMethods:   cfg.Security.AllowedMethods,
		ExposeHeaders:    cfg.Security.ExposeHeaders,
		AllowCredentials: cfg.Security.AllowCredentials,
	})
	// Check if we're in development mode
	isDevelopment := cfg.Tracing.Environment != "production"
	r.Use(middleware.CORSMiddleware(corsConfig, isDevelopment))

	// Rate limiting middleware
	rateLimiter := middleware.NewTokenBucketLimiter(100, 10, time.Second, redisClient)
	r.Use(middleware.RateLimitMiddleware(rateLimiter, middleware.IPKeyFunc, log))

	// Circuit breaker middleware
	cbManager := middleware.NewCircuitBreakerManager(log)
	r.Use(middleware.CircuitBreakerMiddleware(cbManager, middleware.ServiceFromPath))

	// Setup routes
	setupRoutes(r, grpcManager, jwtService, redisClient, log, cfg, ntpTime, storageClient)

	// Wrap router with tracing if enabled
	var handler http.Handler = r
	if cfg.Tracing.Enabled {
		handler = otelhttp.NewHandler(handler, cfg.Tracing.ServiceName,
			otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
		)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server
	go func() {
		log.Info("API Gateway starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", "error", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("Server exited")
}

// initRedis initializes Redis client
func initRedis(cfg *config.Config, log logger.Logger) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Warn("Failed to connect to Redis, using in-memory fallback", "error", err)
		return nil
	}

	log.Info("Connected to Redis")
	return client
}

// registerServices registers microservices with gRPC manager
func registerServices(manager *grpc.ClientManager, services []config.ServiceConfig) {
	for _, service := range services {
		manager.RegisterService(grpc.ServiceConfig{
			Name:    service.Name,
			Address: service.URL,
			Timeout: service.Timeout,
		})
	}
}

// setupRoutes sets up all API routes
func setupRoutes(r *router.Router, grpcManager *grpc.ClientManager, jwtService jwt.Service, redisClient *redis.Client, log logger.Logger, cfg *config.Config, ntpTime *timeutil.NTPTimeService, storageClient storage.Storage) {
	// Health handlers
	healthHandler := handlers.NewHealthHandler(grpcManager, log, Version)
	r.GET("/health", healthHandler.Health)
	r.GET("/health/detailed", healthHandler.HealthDetailed)

	// Admin auth handler with standard response format
	adminAuth := handlers.NewAdminAuthHandler(grpcManager, log)

	// Admin auth public routes
	r.POST("/api/v1/admin/auth/login", adminAuth.Login)
	r.POST("/api/v1/admin/auth/refresh", adminAuth.RefreshToken)
	r.POST("/api/v1/admin/auth/register", adminAuth.Register)

	// Admin management handlers
	adminUser := handlers.NewAdminUserHandler(grpcManager, log)
	adminRole := handlers.NewAdminRoleHandler(grpcManager, log)
	adminAudit := handlers.NewAdminAuditHandler(grpcManager, log)

	// Agent management handlers
	agentHandler := handlers.NewAgentHandler(grpcManager, log, cfg)
	// TODO: Uncomment when retailer service is implemented
	retailerHandler := handlers.NewRetailerHandler(grpcManager, log, cfg)

	// Player handler
	playerHandler := handlers.NewPlayerHandler(grpcManager, log)

	// Notification handler
	notificationHandler := handlers.NewNotificationHandler(grpcManager)

	// Protected admin routes - create a group with auth middleware
	adminGroup := r.Group("/api/v1/admin")
	authConfig := &middleware.AuthConfig{
		MinTokenLength:          20,
		MaxTokenLength:          2048,
		RequireSecureConnection: cfg.Security.RequireHTTPS,
	}
	adminGroup.Use(middleware.AuthMiddleware(jwtService, log, authConfig))

	// Auth-related endpoints
	adminGroup.POST("/auth/logout", adminAuth.Logout)
	adminGroup.GET("/profile", adminAuth.GetProfile)
	adminGroup.PUT("/profile", adminAuth.UpdateProfile)
	adminGroup.POST("/auth/change-password", adminAuth.ChangePassword)

	// MFA endpoints
	adminGroup.POST("/auth/mfa/enable", adminAuth.EnableMFA)
	adminGroup.POST("/auth/mfa/verify", adminAuth.VerifyMFA)
	adminGroup.POST("/auth/mfa/disable", adminAuth.DisableMFA)

	// User management endpoints
	adminGroup.POST("/users", adminUser.CreateAdminUser)
	adminGroup.GET("/users", adminUser.ListAdminUsers)
	adminGroup.GET("/users/{id}", adminUser.GetAdminUser)
	adminGroup.PUT("/users/{id}", adminUser.UpdateAdminUser)
	adminGroup.DELETE("/users/{id}", adminUser.DeleteAdminUser)
	adminGroup.POST("/users/{id}/activate", adminUser.UpdateAdminUserStatus)
	adminGroup.POST("/users/{id}/deactivate", adminUser.UpdateAdminUserStatus)

	// Role management endpoints
	adminGroup.POST("/roles", adminRole.CreateRole)
	adminGroup.GET("/roles", adminRole.ListRoles)
	adminGroup.GET("/roles/{id}", adminRole.GetRole)
	adminGroup.PUT("/roles/{id}", adminRole.UpdateRole)
	adminGroup.DELETE("/roles/{id}", adminRole.DeleteRole)

	// Permission management endpoints
	adminGroup.GET("/permissions", adminRole.ListPermissions)

	// Role assignment endpoints
	adminGroup.POST("/role-assignments", adminRole.AssignRole)
	adminGroup.DELETE("/role-assignments", adminRole.AssignRole)

	// Audit log endpoints
	adminGroup.GET("/audit-logs", adminAudit.GetAuditLogs)

	// Agent management endpoints (admin only)
	adminGroup.POST("/agents", agentHandler.CreateAgent)
	adminGroup.GET("/agents", agentHandler.ListAgents)
	adminGroup.GET("/agents/{id}", agentHandler.GetAgent)
	adminGroup.PUT("/agents/{id}", agentHandler.UpdateAgent)
	adminGroup.PUT("/agents/{id}/status", agentHandler.UpdateAgentStatus)
	adminGroup.GET("/agents/{id}/commissions", agentHandler.GetAgentCommissions)

	// Retailer management endpoints (admin only)
	adminGroup.POST("/retailers", retailerHandler.CreateRetailer)
	adminGroup.GET("/retailers", retailerHandler.ListRetailers)
	adminGroup.GET("/retailers/{id}", retailerHandler.GetRetailer)
	adminGroup.PUT("/retailers/{id}", retailerHandler.UpdateRetailer)
	adminGroup.PUT("/retailers/{id}/status", retailerHandler.UpdateRetailerStatus)

	// Wallet management endpoints (admin only)
	walletHandler := handlers.NewWalletHandler(grpcManager, log)

	// Agent wallet endpoints
	adminGroup.POST("/agents/{agentId}/wallet/credit", walletHandler.CreditAgentWallet)
	adminGroup.GET("/agents/{agentId}/wallet/balance", walletHandler.GetAgentWalletBalance)
	adminGroup.GET("/agents/{agentId}/wallet/transactions", walletHandler.GetAgentTransactionHistory)

	// Commission endpoints
	adminGroup.PUT("/agents/{agentId}/commission-rate", walletHandler.SetCommissionRate)
	adminGroup.GET("/agents/{agentId}/commission-rate", walletHandler.GetCommissionRate)

	// Retailer wallet endpoints
	adminGroup.POST("/retailers/{retailerId}/wallet/credit", walletHandler.CreditRetailerWallet)
	adminGroup.GET("/retailers/{retailerId}/wallet/stake/balance", walletHandler.GetRetailerStakeBalance)
	adminGroup.GET("/retailers/{retailerId}/wallet/winning/balance", walletHandler.GetRetailerWinningBalance)
	adminGroup.GET("/retailers/{retailerId}/wallet/transactions", walletHandler.GetRetailerTransactionHistory)

	// Admin wallet endpoints - for viewing all transactions across all wallets
	adminGroup.GET("/wallet/transactions", walletHandler.GetAllTransactions)
	adminGroup.POST("/wallet/transactions/{transactionId}/reverse", walletHandler.ReverseTransaction)

	// Note: Commission tier endpoints removed - replaced with simple commission percentage on agents

	// Game management endpoints
	gameHandler := handlers.NewGameHandler(grpcManager, log, ntpTime, storageClient)

	// Game routes
	adminGroup.POST("/games", gameHandler.CreateGame)
	adminGroup.GET("/games", gameHandler.ListGames)
	adminGroup.GET("/games/{id}", gameHandler.GetGame)
	adminGroup.PUT("/games/{id}", gameHandler.UpdateGame)
	adminGroup.DELETE("/games/{id}", gameHandler.DeleteGame)

	// Game logo and branding management
	adminGroup.POST("/games/{id}/logo", gameHandler.UploadGameLogo)
	adminGroup.DELETE("/games/{id}/logo", gameHandler.DeleteGameLogo)
	adminGroup.PATCH("/games/{id}/brand-color", gameHandler.UpdateBrandColor)

	// Game approval workflow - DISABLED (approval workflow removed)
	// adminGroup.POST("/games/{id}/submit-approval", gameHandler.SubmitForApproval)
	// adminGroup.POST("/games/{id}/approve", gameHandler.ApproveGame)
	// adminGroup.POST("/games/{id}/reject", gameHandler.RejectGame)
	// adminGroup.GET("/games/{id}/approval-status", gameHandler.GetApprovalStatus)
	// adminGroup.GET("/games/pending-approvals", gameHandler.GetPendingApprovals)

	// Prize structure management
	adminGroup.GET("/games/{id}/prize-structure", gameHandler.GetPrizeStructure)
	adminGroup.PUT("/games/{id}/prize-structure", gameHandler.UpdatePrizeStructure)

	// Game scheduling
	adminGroup.POST("/games/{id}/schedule", gameHandler.ScheduleGame)
	adminGroup.GET("/games/{id}/schedule", gameHandler.GetGameSchedule)
	adminGroup.PUT("/games/schedules/{id}", gameHandler.UpdateScheduledGame)

	// Weekly scheduling
	adminGroup.POST("/scheduling/weekly/generate", gameHandler.GenerateWeeklySchedule)
	adminGroup.GET("/scheduling/weekly", gameHandler.GetWeeklySchedule)
	adminGroup.DELETE("/scheduling/weekly/clear", gameHandler.ClearWeeklySchedule)
	adminGroup.GET("/scheduling/schedules/{scheduleId}", gameHandler.GetScheduleByID)

	// Game status and statistics
	adminGroup.PUT("/games/{id}/status", gameHandler.UpdateGameStatus)
	adminGroup.GET("/games/{id}/statistics", gameHandler.GetGameStatistics)

	// Game operations
	adminGroup.POST("/games/{id}/clone", gameHandler.CloneGame)

	// Game rules
	adminGroup.GET("/games/{id}/rules", gameHandler.GetGameRules)
	adminGroup.PUT("/games/{id}/rules", gameHandler.UpdateGameRules)

	// Ticket management endpoints
	ticketHandler := handlers.NewTicketHandler(grpcManager, log)

	// Admin ticket routes - for monitoring and admin operations
	adminGroup.GET("/tickets", ticketHandler.ListTickets)
	adminGroup.GET("/tickets/{id}", ticketHandler.GetTicket)
	adminGroup.GET("/tickets/serial/{serial}", ticketHandler.GetTicketBySerial)
	adminGroup.POST("/tickets/{id}/void", ticketHandler.VoidTicket)
	adminGroup.POST("/tickets/{id}/cancel", ticketHandler.CancelTicket)

	// Analytics endpoints
	adminGroup.GET("/analytics/daily-metrics", ticketHandler.GetDailyMetrics)
	adminGroup.GET("/analytics/monthly-metrics", ticketHandler.GetMonthlyMetrics)
	adminGroup.GET("/analytics/top-performing-agents", ticketHandler.GetTopPerformingAgents)

	// Payment handler for mobile money topup
	paymentHandler := handlers.NewPaymentHandler(grpcManager, log)

	// Webhook handler for payment provider callbacks
	webhookHandler := handlers.NewWebhookHandler(grpcManager, log)

	// Terminal Management endpoints (admin only)
	terminalHandler := handlers.NewTerminalHandler(grpcManager, log)
	adminGroup.POST("/terminals", terminalHandler.RegisterTerminal)
	adminGroup.GET("/terminals", terminalHandler.ListTerminals)
	adminGroup.GET("/terminals/{id}", terminalHandler.GetTerminal)
	adminGroup.PUT("/terminals/{id}", terminalHandler.UpdateTerminal)
	adminGroup.DELETE("/terminals/{id}", terminalHandler.DeleteTerminal)
	adminGroup.POST("/terminals/{id}/assign", terminalHandler.AssignTerminal)
	adminGroup.POST("/terminals/{id}/unassign", terminalHandler.UnassignTerminal)
	adminGroup.GET("/terminals/{id}/health", terminalHandler.GetTerminalHealth)
	adminGroup.PUT("/terminals/{id}/status", terminalHandler.UpdateTerminalStatus)
	adminGroup.GET("/terminals/{id}/config", terminalHandler.GetTerminalConfig)
	adminGroup.PUT("/terminals/{id}/config", terminalHandler.UpdateTerminalConfig)

	// Game Management endpoints (admin only) - duplicate handler creation removed
	adminGroup.POST("/games", gameHandler.CreateGame)
	adminGroup.GET("/games", gameHandler.ListGames)
	adminGroup.GET("/games/{id}", gameHandler.GetGame)
	adminGroup.PUT("/games/{id}", gameHandler.UpdateGame)
	adminGroup.DELETE("/games/{id}", gameHandler.DeleteGame)
	// Approval endpoints disabled (approval workflow removed)
	// adminGroup.POST("/games/{id}/approve", gameHandler.ApproveGame)
	// adminGroup.POST("/games/{id}/reject", gameHandler.RejectGame)
	adminGroup.GET("/games/{id}/prize-structure", gameHandler.GetPrizeStructure)
	adminGroup.PUT("/games/{id}/prize-structure", gameHandler.UpdatePrizeStructure)
	adminGroup.POST("/games/{id}/schedule", gameHandler.ScheduleGame)
	adminGroup.GET("/games/{id}/schedule", gameHandler.GetGameSchedule)
	adminGroup.PUT("/games/{id}/status", gameHandler.UpdateGameStatus)
	adminGroup.GET("/games/{id}/statistics", gameHandler.GetGameStatistics)
	adminGroup.POST("/games/{id}/clone", gameHandler.CloneGame)
	adminGroup.GET("/games/{id}/rules", gameHandler.GetGameRules)
	adminGroup.PUT("/games/{id}/rules", gameHandler.UpdateGameRules)

	// Draw Service endpoints (admin only)
	drawHandler := handlers.NewDrawHandler(grpcManager, log)
	adminGroup.POST("/draws", drawHandler.CreateDraw)
	adminGroup.GET("/draws", drawHandler.ListDraws)
	adminGroup.GET("/draws/{id}", drawHandler.GetDraw)
	adminGroup.PUT("/draws/{id}", drawHandler.UpdateDraw)
	adminGroup.DELETE("/draws/{id}", drawHandler.DeleteDraw)
	adminGroup.POST("/draws/{id}/prepare", drawHandler.PrepareDraw)
	adminGroup.POST("/draws/{id}/execute", drawHandler.ExecuteDraw)
	adminGroup.POST("/draws/{id}/save-progress", drawHandler.SaveDrawProgress)
	adminGroup.POST("/draws/{id}/restart", drawHandler.RestartDraw)
	adminGroup.POST("/draws/{id}/commit-results", drawHandler.CommitDrawResults)
	adminGroup.POST("/draws/{id}/process-payout", drawHandler.ProcessPayout)
	adminGroup.POST("/draws/{id}/machine-numbers", drawHandler.UpdateMachineNumbers)
	adminGroup.GET("/draws/{id}/results", drawHandler.GetDrawResults)
	adminGroup.POST("/draws/{id}/validate", drawHandler.ValidateDraw)
	adminGroup.GET("/draws/{id}/statistics", drawHandler.GetDrawStatistics)
	adminGroup.GET("/draws/{id}/winning-numbers", drawHandler.GetWinningNumbers)
	adminGroup.GET("/draws/{id}/tickets", drawHandler.GetDrawTickets)
	adminGroup.POST("/draws/{id}/record-physical", drawHandler.RecordPhysicalDraw)

	// Player admin operations
	adminGroup.GET("/players/{id}", playerHandler.GetPlayerByID)
	adminGroup.GET("/players/search", playerHandler.SearchPlayers)
	adminGroup.GET("/players/{id}/wallet/balance", playerHandler.GetWalletBalance)
	adminGroup.POST("/players/{id}/suspend", playerHandler.SuspendPlayer)
	adminGroup.POST("/players/{id}/activate", playerHandler.ActivatePlayer)

	// publicRateLimiter := middleware.NewTokenBucketLimiter(100, 100, time.Minute, redisClient)

	// Player public routes
	r.POST("/api/v1/players/register", playerHandler.RegisterPlayer)
	r.POST("/api/v1/players/verify-otp", playerHandler.VerifyOTP)
	r.POST("/api/v1/players/resend-otp", playerHandler.ResendOTP)
	r.POST("/api/v1/players/login", playerHandler.Login)
	r.POST("/api/v1/players/refresh-token", playerHandler.RefreshToken)
	r.POST("/api/v1/players/ussd/register", playerHandler.USSDRegister)
	r.POST("/api/v1/players/ussd/login", playerHandler.USSDLogin)

	// Player password reset public routes
	r.POST("/api/v1/players/password-reset/request", playerHandler.RequestPasswordReset)
	r.POST("/api/v1/players/password-reset/validate-otp", playerHandler.ValidatePasswordResetOTP)
	r.POST("/api/v1/players/password-reset/confirm", playerHandler.ConfirmPasswordReset)
	r.POST("/api/v1/players/password-reset/resend-otp", playerHandler.ResendPasswordResetOTP)

	// Public player game routes - no authentication required
	r.GET("/api/v1/players/games", gameHandler.GetActiveGames)
	r.GET("/api/v1/players/games/{id}", gameHandler.GetGame)
	r.GET("/api/v1/players/games/{id}/schedule", gameHandler.GetGameSchedule)
	r.GET("/api/v1/players/games/schedules/weekly", gameHandler.GetScheduledGamesForPlayer)

	// Public draw results - no authentication required
	r.GET("/api/v1/public/draws/completed", drawHandler.GetPublicCompletedDraws)
	r.GET("/api/v1/public/winners", drawHandler.GetPublicWinners)

	// Public games list - no authentication required
	r.GET("/api/v1/public/games", gameHandler.GetActiveGames)

	// Webhook routes - no JWT authentication (uses signature verification)
	// These endpoints receive callbacks from payment providers (Orange, MTN, Telecel)
	r.POST("/api/v1/webhooks/orange", webhookHandler.HandleOrangeWebhook)
	r.POST("/api/v1/webhooks/mtn", webhookHandler.HandleMTNWebhook)
	r.POST("/api/v1/webhooks/telecel", webhookHandler.HandleTelecelWebhook)

	// USSD callback - no JWT authentication (called by mNotify)
	ussdHandler := handlers.NewUSSDHandler(grpcManager, log)
	r.POST("/api/v1/ussd/callback", ussdHandler.HandleCallback)
	r.POST("/ussd/callback", ussdHandler.HandleCallback) // also handle root path for USSD

	// Retailer auth handler
	retailerAuth := handlers.NewRetailerAuthHandler(grpcManager, log, jwtService, cfg)

	// Retailer auth public routes (POS login)
	r.POST("/api/v1/retailer/auth/pos-login", retailerAuth.POSLogin)
	r.POST("/api/v1/retailer/auth/refresh", retailerAuth.RefreshToken)
	playerAuthConfig := &middleware.AuthConfig{
		MinTokenLength:          20,
		MaxTokenLength:          2048,
		RequireSecureConnection: cfg.Security.RequireHTTPS,
	}

	// Protected player routes - create a group with auth middleware
	playerGroup := r.Group("/api/v1/players")
	playerGroup.Use(middleware.AuthMiddleware(jwtService, log, playerAuthConfig))

	// Player authenticated endpoints
	playerGroup.POST("/logout", playerHandler.Logout)
	playerGroup.GET("/{id}/profile", playerHandler.GetProfile)
	playerGroup.PUT("/{id}/profile", playerHandler.UpdateProfile)
	playerGroup.POST("/{id}/change-password", playerHandler.ChangePassword)
	playerGroup.PUT("/{id}/mobile-money-phone", playerHandler.UpdateMobileMoneyPhone)
	playerGroup.POST("/{id}/deposit", playerHandler.InitiateDeposit)
	playerGroup.POST("/{id}/feedback", playerHandler.PostFeedback)
	playerGroup.POST("/{id}/deposit/verify", paymentHandler.VerifyDepositStatus)
	playerGroup.POST("/{id}/withdrawal", playerHandler.InitiateWithdrawal)
	playerGroup.GET("/{id}/transactions", playerHandler.GetTransactionHistory)
	playerGroup.GET("/{id}/payment-transactions", playerHandler.GetPaymentHistory)
	playerGroup.GET("/{id}/wallet/balance", playerHandler.GetWalletBalance)
	playerGroup.POST("/wallet/verify-account", paymentHandler.VerifyWallet)

	// Player ticket operations - players can stake and view their tickets
	playerGroup.POST("/{id}/tickets", ticketHandler.IssueTicketForPlayer)
	playerGroup.GET("/{id}/tickets", ticketHandler.ListPlayerTickets)
	playerGroup.GET("/{id}/tickets/{ticket_id}", ticketHandler.GetPlayerTicket)
	playerGroup.GET("/{id}/tickets/serial/{serial}", ticketHandler.GetPlayerTicketBySerial)
	playerGroup.POST("/{id}/tickets/{ticket_id}/validate", ticketHandler.ValidateTicket)
	playerGroup.GET("/{id}/tickets/{ticket_id}/winnings", ticketHandler.CheckWinnings)

	// Protected retailer routes - create a group with auth middleware
	retailerGroup := r.Group("/api/v1/retailer")
	retailerGroup.Use(middleware.AuthMiddleware(jwtService, log, playerAuthConfig))

	// Apply rate limiting for retailer endpoints (100 requests per minute per retailer)
	retailerRateLimiter := middleware.NewTokenBucketLimiter(100, 100, time.Minute, redisClient)
	retailerGroup.Use(middleware.RateLimitMiddleware(retailerRateLimiter, middleware.UserKeyFunc, log))

	// Retailer auth-related endpoints
	retailerGroup.POST("/auth/logout", retailerAuth.Logout)
	retailerGroup.POST("/auth/pin/change", retailerAuth.ChangeRetailerPIN)

	// Retailer mobile money topup endpoints
	retailerGroup.POST("/wallet/topup", paymentHandler.InitiateTopup)
	retailerGroup.GET("/wallet/topup/:reference", paymentHandler.GetTopupStatus)
	retailerGroup.POST("/wallet/verify-status", paymentHandler.VerifyDepositStatus)
	retailerGroup.POST("/wallet/verify-account", paymentHandler.VerifyWallet)

	// POS-specific handlers for retailers
	posHandler := handlers.NewPOSHandler(grpcManager, log)

	// POS wallet endpoints - retailers can access their own wallets
	retailerGroup.GET("/pos/wallet/stake", posHandler.GetMyStakeBalance)
	retailerGroup.GET("/pos/wallet/winnings", posHandler.GetMyWinningsBalance)
	retailerGroup.GET("/pos/wallets", posHandler.GetMyWallets)
	retailerGroup.GET("/pos/transactions", posHandler.GetMyTransactions)

	// Retailer terminal endpoints
	retailerGroup.GET("/terminal", retailerHandler.GetAssignedTerminal)
	retailerGroup.PUT("/terminal/heartbeat", retailerHandler.UpdateHeartbeat)
	retailerGroup.GET("/terminal/config", retailerHandler.GetAssignedTerminalConfig)

	// Retailer ticket operations - retailers can issue and manage their own tickets
	retailerGroup.POST("/tickets", ticketHandler.IssueTicket)
	retailerGroup.GET("/tickets", ticketHandler.ListTickets)
	retailerGroup.GET("/tickets/{id}", ticketHandler.GetTicket)
	retailerGroup.GET("/tickets/serial/{serial}", ticketHandler.GetTicketBySerial)
	retailerGroup.POST("/tickets/{id}/validate", ticketHandler.ValidateTicket)
	retailerGroup.POST("/tickets/{id}/reprint", ticketHandler.ReprintTicket)
	retailerGroup.GET("/tickets/{id}/winnings", ticketHandler.CheckWinnings)

	// Retailer game endpoints (read-only)
	retailerGroup.GET("/games/active", gameHandler.GetActiveGames)
	retailerGroup.GET("/games/scheduled", gameHandler.GetScheduledGamesForRetailer)

	// Retailer draw results (read-only)
	retailerGroup.GET("/draws/completed", drawHandler.GetCompletedDraws)

	// Retailer notification endpoints
	retailerGroup.POST("/notifications/register-device", notificationHandler.RegisterDeviceToken)
	retailerGroup.GET("/notifications", notificationHandler.GetNotifications)
	retailerGroup.PUT("/notifications/{id}/read", notificationHandler.MarkNotificationAsRead)
	retailerGroup.PUT("/notifications/read-all", notificationHandler.MarkAllNotificationsAsRead)
	retailerGroup.GET("/notifications/unread-count", notificationHandler.GetUnreadCount)

	// Agent auth handler
	agentAuth := handlers.NewAgentAuthHandler(grpcManager, log)
	// Agent auth public routes
	r.POST("/api/v1/agent/auth/login", agentAuth.Login)
	r.POST("/api/v1/agent/auth/forgot-password", agentAuth.RequestPasswordReset)
	r.POST("/api/v1/agent/auth/password-reset/validate-otp", agentAuth.ValidateResetOTP)
	r.POST("/api/v1/agent/auth/password-reset/confirm", agentAuth.ConfirmPasswordReset)
	r.POST("/api/v1/agent/auth/password-reset/resend-otp", agentAuth.ResendPasswordResetOTP)
	r.POST("/api/v1/agent/auth/refresh", agentAuth.RefreshToken)

	agentGroup := r.Group("/api/v1/agent")
	agentAuthConfig := &middleware.AuthConfig{
		MinTokenLength:          20,
		MaxTokenLength:          2048,
		RequireSecureConnection: cfg.Security.RequireHTTPS,
	}
	agentGroup.Use(middleware.AuthMiddleware(jwtService, log, agentAuthConfig))

	// Apply stricter rate limiting for agent endpoints (50 requests per minute per agent)
	agentRateLimiter := middleware.NewTokenBucketLimiter(50, 50, time.Minute, redisClient)
	agentGroup.Use(middleware.RateLimitMiddleware(agentRateLimiter, middleware.UserKeyFunc, log))

	agentGroup.POST("/auth/logout", agentAuth.Logout)
	agentGroup.POST("/auth/change-password", agentAuth.ChangePassword)

	agentGroup.GET("/profile", agentAuth.GetProfile)
	agentGroup.PUT("/profile", agentAuth.UpdateProfile)

	// Agent mobile money topup endpoints
	agentGroup.POST("/wallet/topup", paymentHandler.InitiateTopup)
	agentGroup.GET("/wallet/topup/:reference", paymentHandler.GetTopupStatus)
	agentGroup.POST("/wallet/verify-status", paymentHandler.VerifyDepositStatus)
	agentGroup.POST("/wallet/verify-account", paymentHandler.VerifyWallet)

	agentGroup.GET("/overview", agentHandler.GetAgentOverview)
	agentGroup.GET("/devices", agentAuth.ListDevices)
	agentGroup.GET("/sessions", agentAuth.ListAgentSessions)
	agentGroup.GET("/sessions/current", agentAuth.AgentCurrentSession)
	agentGroup.GET("/permissions", agentAuth.GetPermissions)

	// Agent ticket operations - agents cannot issue or cancel tickets
	agentGroup.GET("/tickets", ticketHandler.ListTickets)
	agentGroup.GET("/tickets/{id}", ticketHandler.GetTicket)
	agentGroup.GET("/tickets/serial/{serial}", ticketHandler.GetTicketBySerial)
	agentGroup.POST("/tickets/{id}/validate", ticketHandler.ValidateTicket)
	agentGroup.POST("/tickets/{id}/reprint", ticketHandler.ReprintTicket)
	agentGroup.GET("/tickets/{id}/winnings", ticketHandler.CheckWinnings)

	// Agent wallet endpoints
	agentGroup.GET("/{agentId}/wallet/balance", walletHandler.GetAgentWalletBalance)
	// agentGroup.POST("/wallet/topup", walletHandler.AgentSelfTopUp)
	agentGroup.GET("/{agentId}/wallet/transactions", walletHandler.GetAgentTransactionHistory)
	// agentGroup.POST("/wallet/transfer/{retailerId}", walletHandler.AgentTransferToRetailer)

	// Agent retailer management endpoints
	agentGroup.GET("/retailers", retailerHandler.ListAgentRetailers)
	agentGroup.GET("/retailers/{id}", retailerHandler.GetRetailerDetails)
	agentGroup.PUT("/retailers/{id}/update", retailerHandler.UpdateRetailer)
	agentGroup.GET("/retailers/{id}/pos-devices", retailerHandler.GetRetailerPOSDevices)
	agentGroup.GET("/retailers/{id}/performance", retailerHandler.GetRetailerPerformance)
	agentGroup.GET("/retailers/{id}/winning-tickets", retailerHandler.GetRetailerWinningTickets)
	agentGroup.POST("/retailers", retailerHandler.OnboardRetailer)
	agentGroup.PUT("/retailers/{id}/status", retailerHandler.UpdateRetailerStatus)
	agentGroup.GET("/retailers/{retailerId}/wallet/stake/balance", walletHandler.GetRetailerStakeBalance)
	agentGroup.GET("/retailers/{retailerId}/wallet/winning/balance", walletHandler.GetRetailerWinningBalance)
	agentGroup.GET("/retailers/{retailerId}/wallet/transactions", walletHandler.GetRetailerTransactionHistory)

	// Agent draw history endpoint
	agentGroup.GET("/draws/history", drawHandler.GetAgentDrawHistory)

	agentGroup.POST("/retailers/wallet/hold", walletHandler.PlaceHoldOnWallet)
	agentGroup.POST("/retailers/wallet/hold/{hold_id}/release", walletHandler.ReleaseHoldOnWallet)
	agentGroup.GET("/retailers/wallet/hold/{hold_id}", walletHandler.GetHoldOnWallet)
	agentGroup.GET("/retailers/{retailerId}/wallet/hold", walletHandler.GetHoldByRetailer)
	// Agent POS request endpoints
	// TODO: Uncomment when retailer service is implemented
	// agentGroup.POST("/pos-requests", retailerHandler.CreatePOSRequest)
	// agentGroup.GET("/pos-requests", retailerHandler.ListPOSRequests)
	// agentGroup.GET("/pos-requests/{id}", retailerHandler.GetPOSRequest)

	// Agent commission endpoints
	agentGroup.GET("/agent/{agentId}/commission/rate", walletHandler.GetCommissionRate)
	// agentGroup.GET("/commission/history", walletHandler.GetAgentCommissionHistory)

	// Agent payment endpoints
	// TODO: Uncomment when payment service is implemented
	// agentGroup.POST("/payments", paymentHandler.ProcessPayment)
	// agentGroup.GET("/payments/{id}", paymentHandler.GetPayment)
}

// initTracer initializes OpenTelemetry tracing
func initTracer(cfg config.TracingConfig) (*sdktrace.TracerProvider, error) {
	// Create OTLP exporter for Jaeger
	ctx := context.Background()

	// Extract the host from the Jaeger endpoint
	// If it's the old collector endpoint format, convert it
	endpoint := cfg.JaegerEndpoint
	if strings.Contains(endpoint, "/api/traces") {
		// Convert from old Jaeger format to OTLP format
		endpoint = strings.Replace(endpoint, ":14268/api/traces", ":4318", 1)
		endpoint = strings.Replace(endpoint, "/api/traces", "", 1)
	}

	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint(strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")),
		otlptracehttp.WithInsecure(),
	)

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	// Create resource with service information
	res := resource.NewWithAttributes(
		"", // No schema URL to avoid conflicts
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", cfg.ServiceVersion),
		attribute.String("environment", cfg.Environment),
		attribute.String("service.namespace", "randco"),
	)

	// Create tracer provider with sampling
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, nil
}

// Build trigger: 1759076135
