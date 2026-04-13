package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/cache"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/randco/randco-microservices/services/service-game/internal/repositories"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/randco/randco-microservices/shared/storage"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// GameService defines the interface for game business logic
type GameService interface {
	CreateGame(ctx context.Context, req models.CreateGameRequest) (*models.Game, error)
	GetGame(ctx context.Context, id uuid.UUID) (*models.Game, error)
	UpdateGame(ctx context.Context, req models.UpdateGameRequest) (*models.Game, error)
	DeleteGame(ctx context.Context, id uuid.UUID) error
	ListGames(ctx context.Context, filter models.GameFilter, page, limit int) ([]*models.Game, int64, error)
	ActivateGame(ctx context.Context, id uuid.UUID) (*models.Game, error)
	SuspendGame(ctx context.Context, id uuid.UUID) (*models.Game, error)
	ValidateGameConfiguration(ctx context.Context, game *models.Game) error
	UploadGameLogo(ctx context.Context, gameID uuid.UUID, fileName string, contentType string, size int64, fileData []byte, brandColor *string) (*GameLogoUploadResult, error)
	UpdateGameLogo(ctx context.Context, gameID uuid.UUID, logoURL *string, brandColor *string) (*GameLogoUploadResult, error)
	DeleteGameLogo(ctx context.Context, gameID uuid.UUID) error
}

// GameLogoUploadResult represents the result of a logo upload operation
type GameLogoUploadResult struct {
	LogoURL    string
	CDNURL     string
	BrandColor *string
}

const (
	// Cache operation timeout
	defaultCacheTimeout = 2 * time.Second
)

// gameService implements GameService interface
type gameService struct {
	db           *sql.DB
	redisClient  *redis.Client
	gameRepo     repositories.GameRepository
	gameCache    cache.GameCache
	cacheTimeout time.Duration
	eventBus     events.EventBus
	storage      storage.Storage
}

// ServiceConfig holds configuration for services
type ServiceConfig struct {
	KafkaBrokers []string
}

// NewGameService creates a new instance of GameService
func NewGameService(
	db *sql.DB,
	redisClient *redis.Client,
	gameRepo repositories.GameRepository,
	storageClient storage.Storage,
	config *ServiceConfig,
) GameService {
	// Initialize event bus
	var eventBus events.EventBus
	if config != nil && len(config.KafkaBrokers) > 0 {
		bus, err := events.NewKafkaEventBus(config.KafkaBrokers)
		if err != nil {
			log.Printf("Failed to initialize Kafka event bus: %v, using in-memory event bus", err)
			eventBus = events.NewInMemoryEventBus()
		} else {
			eventBus = bus
		}
	} else {
		eventBus = events.NewInMemoryEventBus()
	}

	return &gameService{
		db:           db,
		redisClient:  redisClient,
		gameRepo:     gameRepo,
		gameCache:    cache.NewGameCache(redisClient),
		cacheTimeout: defaultCacheTimeout,
		eventBus:     eventBus,
		storage:      storageClient,
	}
}

// CreateGame creates a new game with validation
func (s *gameService) CreateGame(ctx context.Context, req models.CreateGameRequest) (*models.Game, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.name", req.Name),
		attribute.String("game.format", req.GameFormat),
		attribute.String("game.organizer", req.Organizer),
	)

	// Business validation using the request's Validate method
	if err := req.Validate(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "validation failed")
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create game model using the ConvertToGame method with time validation
	game, err := req.ConvertToGame()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "time format validation failed")
		return nil, fmt.Errorf("time format validation failed: %w", err)
	}

	// Validate business rules
	if err := game.ValidateBusinessRules(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "business rules validation failed")
		return nil, fmt.Errorf("business rules validation failed: %w", err)
	}

	// Begin transaction to prevent race condition
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to begin transaction")
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Ignore error as it's expected after successful commit
	}()

	// Check for duplicate names within transaction (with row lock)
	existingGame, err := s.gameRepo.GetByNameWithTx(ctx, tx, req.Name)
	if err == nil && existingGame != nil {
		err := fmt.Errorf("game with name '%s' already exists", req.Name)
		span.RecordError(err)
		span.SetStatus(codes.Error, "duplicate game name")
		return nil, err
	}

	// Create in database using the same transaction
	if err := s.gameRepo.CreateWithTx(ctx, tx, game); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create game")
		return nil, fmt.Errorf("failed to create game: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to commit transaction")
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// TODO: Create game rules if provided (game rules service not implemented yet)
	// When game rules service is ready, implement logic to create rules if
	// req.NumberRangeMin > 0 && req.NumberRangeMax > 0 && req.SelectionCount > 0
	//
	// Example implementation:
	// gameRules := &models.GameRules{
	// 	GameID:         game.ID,
	// 	NumbersToPick:  req.SelectionCount,
	// 	TotalNumbers:   req.NumberRangeMax,
	// 	MinSelections:  1,
	// 	MaxSelections:  10,
	// 	NumberRangeMin: &req.NumberRangeMin,
	// 	NumberRangeMax: &req.NumberRangeMax,
	// 	SelectionCount: &req.SelectionCount,
	// 	AllowQuickPick: true,
	// 	EffectiveFrom:  time.Now(),
	// }
	// if err := s.gameRulesService.CreateGameRules(ctx, gameRules); err != nil {
	// 	span.RecordError(err)
	// 	return nil, fmt.Errorf("failed to create game rules: %w", err)
	// }

	// Cache the game with error handling
	if err := s.cacheGameWithError(ctx, game); err != nil {
		// Log cache error but don't fail the operation
		span.RecordError(err)
		span.SetAttributes(attribute.String("cache.error", err.Error()))
	}

	// Publish game.created event
	if s.eventBus != nil {
		var drawTimeStr *string
		if game.DrawTime != nil {
			timeStr := game.DrawTime.Format("15:04")
			drawTimeStr = &timeStr
		}
		gameData := events.GameData{
			ID:          game.ID.String(),
			Name:        game.Name,
			GameFormat:  game.GameFormat,
			Organizer:   game.Organizer,
			Status:      game.Status,
			MinStake:    int64(game.MinStakeAmount * 100), // Convert GHS to pesewas
			MaxStake:    int64(game.MaxStakeAmount * 100), // Convert GHS to pesewas
			Description: game.Description,
			DrawTime:    drawTimeStr,
			CreatedAt:   game.CreatedAt,
			UpdatedAt:   game.UpdatedAt,
		}
		event := events.NewGameCreatedEvent("service-game", gameData, "system")
		if err := s.eventBus.Publish(ctx, "game.events", event); err != nil {
			log.Printf("Failed to publish game created event for game %s: %v", game.ID, err)
		}
	}

	span.SetAttributes(attribute.String("game.id", game.ID.String()))
	return game, nil
}

// GetGame retrieves a game by ID with caching
func (s *gameService) GetGame(ctx context.Context, id uuid.UUID) (*models.Game, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.get")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", id.String()))

	// Try to get from cache first
	if game, err := s.getCachedGame(ctx, id); err == nil && game != nil {
		span.SetAttributes(attribute.Bool("cache.hit", true))
		return game, nil
	}

	span.SetAttributes(attribute.Bool("cache.hit", false))

	// Get from database
	game, err := s.gameRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get game")
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Cache the result with error handling
	if err := s.cacheGameWithError(ctx, game); err != nil {
		// Log cache error but don't fail the operation
		span.RecordError(err)
		span.SetAttributes(attribute.String("cache.error", err.Error()))
	}

	return game, nil
}

// UpdateGame updates an existing game with validation
func (s *gameService) UpdateGame(ctx context.Context, req models.UpdateGameRequest) (*models.Game, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.update")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", req.ID.String()))

	// Get existing game
	game, err := s.gameRepo.GetByID(ctx, req.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Check if game can be modified
	if !game.CanBeModified() {
		err := fmt.Errorf("game with status '%s' cannot be modified", game.Status)
		span.RecordError(err)
		span.SetStatus(codes.Error, "game cannot be modified")
		return nil, err
	}

	// Apply updates
	if req.Name != nil {
		game.Name = *req.Name
	}
	if req.Description != nil {
		game.Description = req.Description
	}
	if req.DrawFrequency != nil {
		game.DrawFrequency = *req.DrawFrequency
	}
	if req.DrawTime != nil {
		game.DrawTime = req.DrawTime
		// Also update DrawTimeStr for frontend display
		timeStr := req.DrawTime.Format("15:04")
		game.DrawTimeStr = &timeStr
	}
	if len(req.DrawDays) > 0 {
		game.DrawDays = req.DrawDays
	}
	if req.SalesCutoffMinutes != nil {
		fmt.Printf("[GameService] UpdateGame: Updating sales_cutoff_minutes: game_id=%s, old_value=%d, new_value=%d\n",
			req.ID.String(), game.SalesCutoffMinutes, *req.SalesCutoffMinutes)
		game.SalesCutoffMinutes = *req.SalesCutoffMinutes
	} else {
		fmt.Printf("[GameService] UpdateGame: sales_cutoff_minutes is nil in request, game_id=%s, current_value=%d\n",
			req.ID.String(), game.SalesCutoffMinutes)
	}
	if req.BasePrice != nil {
		game.BasePrice = *req.BasePrice
	}
	if req.MinStake != nil {
		game.MinStakeAmount = *req.MinStake
	}
	if req.MaxStake != nil {
		game.MaxStakeAmount = *req.MaxStake
	}
	if req.MaxTicketsPerPlayer != nil {
		game.MaxTicketsPerPlayer = *req.MaxTicketsPerPlayer
	}
	if req.MultiDrawEnabled != nil {
		game.MultiDrawEnabled = *req.MultiDrawEnabled
	}
	if req.MaxDrawsAdvance != nil {
		game.MaxDrawsAdvance = req.MaxDrawsAdvance
	}
	// Note: BetTypes are updated through the game rules service, not directly here

	// Validate business rules after updates
	if err := game.ValidateBusinessRules(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "business rules validation failed")
		return nil, fmt.Errorf("business rules validation failed: %w", err)
	}

	// Update in database
	if err := s.gameRepo.Update(ctx, game); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update game")
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	// Update cache with error handling
	if err := s.cacheGameWithError(ctx, game); err != nil {
		// Log cache error but don't fail the operation
		span.RecordError(err)
		span.SetAttributes(attribute.String("cache.error", err.Error()))
	}

	// Publish game.updated event
	if s.eventBus != nil {
		var drawTimeStr *string
		if game.DrawTime != nil {
			timeStr := game.DrawTime.Format("15:04")
			drawTimeStr = &timeStr
		}
		gameData := events.GameData{
			ID:          game.ID.String(),
			Name:        game.Name,
			GameFormat:  game.GameFormat,
			Organizer:   game.Organizer,
			Status:      game.Status,
			MinStake:    int64(game.MinStakeAmount * 100), // Convert GHS to pesewas
			MaxStake:    int64(game.MaxStakeAmount * 100), // Convert GHS to pesewas
			Description: game.Description,
			DrawTime:    drawTimeStr,
			CreatedAt:   game.CreatedAt,
			UpdatedAt:   game.UpdatedAt,
		}
		// Track what changed
		changes := make(map[string]interface{})
		if req.Name != nil {
			changes["name"] = *req.Name
		}
		if req.Description != nil {
			changes["description"] = *req.Description
		}
		if req.DrawTime != nil {
			changes["draw_time"] = *req.DrawTime
		}
		event := events.NewGameUpdatedEvent("service-game", gameData, "system", changes)
		if err := s.eventBus.Publish(ctx, "game.events", event); err != nil {
			log.Printf("Failed to publish game updated event for game %s: %v", game.ID, err)
		}
	}

	return game, nil
}

// DeleteGame soft deletes a game
func (s *gameService) DeleteGame(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.delete")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", id.String()))

	// Get existing game to check status
	game, err := s.gameRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return fmt.Errorf("game not found: %w", err)
	}

	// Check if game can be deleted
	if game.Status == "ACTIVE" {
		err := fmt.Errorf("active games cannot be deleted")
		span.RecordError(err)
		span.SetStatus(codes.Error, "active games cannot be deleted")
		return err
	}

	// Soft delete (mark as terminated)
	if err := s.gameRepo.Delete(ctx, id); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete game")
		return fmt.Errorf("failed to delete game: %w", err)
	}

	// Remove from cache with error handling
	if err := s.removeCachedGameWithError(ctx, id); err != nil {
		// Log cache error but don't fail the operation
		span.RecordError(err)
		span.SetAttributes(attribute.String("cache.error", err.Error()))
	}

	// Publish game.deleted event
	if s.eventBus != nil {
		event := events.NewGameDeletedEvent("service-game", game.ID.String(), game.Name, "system")
		if err := s.eventBus.Publish(ctx, "game.events", event); err != nil {
			log.Printf("Failed to publish game deleted event for game %s: %v", game.ID, err)
		}
	}

	return nil
}

// ListGames retrieves games with filtering and pagination
func (s *gameService) ListGames(ctx context.Context, filter models.GameFilter, page, limit int) ([]*models.Game, int64, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.list")
	defer span.End()

	span.SetAttributes(
		attribute.Int("pagination.page", page),
		attribute.Int("pagination.limit", limit),
	)

	// Validate pagination
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	games, total, err := s.gameRepo.List(ctx, filter, page, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list games")
		return nil, 0, fmt.Errorf("failed to list games: %w", err)
	}

	span.SetAttributes(
		attribute.Int("games.count", len(games)),
		attribute.Int64("games.total", total),
	)

	return games, total, nil
}

// ActivateGame activates a game
func (s *gameService) ActivateGame(ctx context.Context, id uuid.UUID) (*models.Game, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.activate")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", id.String()))

	// Get existing game
	game, err := s.gameRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Check if game can be activated (must be draft or suspended)
	// Suspended games can be reactivated directly
	if game.Status != "DRAFT" && game.Status != "SUSPENDED" {
		err := fmt.Errorf("only draft or suspended games can be activated")
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not in draft or suspended status")
		return nil, err
	}

	// Validate game configuration before activation
	if err := s.ValidateGameConfiguration(ctx, game); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game configuration invalid")
		return nil, fmt.Errorf("game configuration invalid: %w", err)
	}

	// Update status using model's SetStatus method
	if err := game.SetStatus(models.GameStatusActive); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid status transition")
		return nil, fmt.Errorf("cannot activate game: %w", err)
	}
	if err := s.gameRepo.Update(ctx, game); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to activate game")
		return nil, fmt.Errorf("failed to activate game: %w", err)
	}

	// Update cache with error handling
	if err := s.cacheGameWithError(ctx, game); err != nil {
		// Log cache error but don't fail the operation
		span.RecordError(err)
		span.SetAttributes(attribute.String("cache.error", err.Error()))
	}

	// Publish game.activated event
	if s.eventBus != nil {
		event := events.NewGameActivatedEvent("service-game", game.ID.String(), game.Name, "system")
		if err := s.eventBus.Publish(ctx, "game.events", event); err != nil {
			log.Printf("Failed to publish game activated event for game %s: %v", game.ID, err)
		}
	}

	return game, nil
}

// SuspendGame suspends an active game
func (s *gameService) SuspendGame(ctx context.Context, id uuid.UUID) (*models.Game, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.suspend")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", id.String()))

	// Get existing game
	game, err := s.gameRepo.GetByID(ctx, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Check if game can be suspended (must be active)
	if game.Status != "ACTIVE" {
		err := fmt.Errorf("only active games can be suspended")
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not active")
		return nil, err
	}

	// Update status using model's SetStatus method
	if err := game.SetStatus(models.GameStatusSuspended); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid status transition")
		return nil, fmt.Errorf("cannot suspend game: %w", err)
	}
	if err := s.gameRepo.Update(ctx, game); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to suspend game")
		return nil, fmt.Errorf("failed to suspend game: %w", err)
	}

	// Update cache with error handling
	if err := s.cacheGameWithError(ctx, game); err != nil {
		// Log cache error but don't fail the operation
		span.RecordError(err)
		span.SetAttributes(attribute.String("cache.error", err.Error()))
	}

	// Publish game.suspended event
	if s.eventBus != nil {
		event := events.NewGameSuspendedEvent("service-game", game.ID.String(), game.Name, "system", "Suspended via API")
		if err := s.eventBus.Publish(ctx, "game.events", event); err != nil {
			log.Printf("Failed to publish game suspended event for game %s: %v", game.ID, err)
		}
	}

	return game, nil
}

// ValidateGameConfiguration validates the complete game configuration
func (s *gameService) ValidateGameConfiguration(ctx context.Context, game *models.Game) error {
	_, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.validate_configuration")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", game.ID.String()))

	// Basic game validation
	if err := game.ValidateBusinessRules(); err != nil {
		return err
	}

	// TODO: Add more comprehensive validation
	// - Check if game has rules defined
	// - Check if game has prize structure defined
	// - Check if schedule is properly configured
	// - Validate ticket price ranges

	return nil
}

// Private helper methods

// cacheGameWithError caches a game in Redis and returns any errors
func (s *gameService) cacheGameWithError(ctx context.Context, game *models.Game) error {
	if game == nil {
		return nil
	}

	// Create a timeout context for cache operation
	cacheCtx, cancel := context.WithTimeout(ctx, s.cacheTimeout)
	defer cancel()

	// Try to cache the game
	if err := s.gameCache.SetGame(cacheCtx, game); err != nil {
		// Return error for logging, but don't fail the main operation
		return fmt.Errorf("failed to cache game %s: %w", game.ID, err)
	}

	return nil
}

// removeCachedGameWithError removes a game from Redis cache and returns any errors
func (s *gameService) removeCachedGameWithError(ctx context.Context, id uuid.UUID) error {
	// Create a timeout context for cache operation
	cacheCtx, cancel := context.WithTimeout(ctx, s.cacheTimeout)
	defer cancel()

	// Try to remove from cache
	if err := s.gameCache.DeleteGame(cacheCtx, id); err != nil {
		// Return error for logging, but don't fail the main operation
		return fmt.Errorf("failed to remove cached game %s: %w", id, err)
	}

	return nil
}

// getCachedGame retrieves a game from Redis cache
func (s *gameService) getCachedGame(ctx context.Context, id uuid.UUID) (*models.Game, error) {
	// Create a timeout context for cache operation
	cacheCtx, cancel := context.WithTimeout(ctx, s.cacheTimeout)
	defer cancel()

	// Try to get from cache
	game, err := s.gameCache.GetGame(cacheCtx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached game %s: %w", id, err)
	}

	return game, nil
}

// UploadGameLogo uploads a game logo to object storage and updates the database
func (s *gameService) UploadGameLogo(ctx context.Context, gameID uuid.UUID, fileName string, contentType string, size int64, fileData []byte, brandColor *string) (*GameLogoUploadResult, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.upload_logo")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", gameID.String()),
		attribute.String("file.name", fileName),
		attribute.String("file.content_type", contentType),
		attribute.Int64("file.size", size),
	)

	// Check if storage is configured
	if s.storage == nil {
		err := fmt.Errorf("storage not configured")
		span.RecordError(err)
		span.SetStatus(codes.Error, "storage not configured")
		return nil, err
	}

	// Verify game exists and get current logo
	game, err := s.GetGame(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Validate brand color format if provided
	if brandColor != nil && *brandColor != "" {
		if !isValidHexColor(*brandColor) {
			err := fmt.Errorf("invalid brand color format: must be hex color like #FF5733")
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid brand color")
			return nil, err
		}
	}

	// Get file extension
	ext := filepath.Ext(fileName)
	if ext == "" {
		// Infer extension from content type
		switch contentType {
		case "image/png":
			ext = ".png"
		case "image/jpeg", "image/jpg":
			ext = ".jpg"
		case "image/webp":
			ext = ".webp"
		default:
			ext = ".png"
		}
	}

	// Create standardized filename: logo.{ext}
	standardFileName := "logo" + ext

	// Delete existing logo if any
	if game.LogoURL != nil && *game.LogoURL != "" {
		if err := s.storage.Delete(ctx, gameID.String()); err != nil {
			// Log error but don't fail the upload
			log.Printf("Failed to delete existing logo for game %s: %v", gameID, err)
		}
	}

	// Upload to storage
	uploadInfo := storage.UploadInfo{
		GameID:      gameID.String(),
		FileName:    standardFileName,
		ContentType: contentType,
		Size:        size,
		Data:        fileData,
		Permission:  "public", // Logos are public
	}

	objectInfo, err := s.storage.Upload(ctx, uploadInfo)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to upload logo")
		return nil, fmt.Errorf("failed to upload logo: %w", err)
	}

	// Update database with logo URL and brand color
	logoURL := objectInfo.CDNURL
	if logoURL == "" {
		logoURL = objectInfo.URL
	}

	// Debug logging to verify upload bucket
	log.Printf("[UploadGameLogo] Generated logo URL for game %s: %s", gameID, logoURL)

	if err := s.gameRepo.UpdateLogo(ctx, gameID, &logoURL, brandColor); err != nil {
		// Try to clean up uploaded file
		if deleteErr := s.storage.Delete(ctx, gameID.String()); deleteErr != nil {
			// Log cleanup failure - orphaned file will remain in storage
			log.Printf("CRITICAL: Failed to delete orphaned file for game %s after database update failure. File: %s, DB Error: %v, Delete Error: %v",
				gameID, logoURL, err, deleteErr)
			span.AddEvent("orphaned_file_detected", trace.WithAttributes(
				attribute.String("game_id", gameID.String()),
				attribute.String("file_url", logoURL),
				attribute.String("db_error", err.Error()),
				attribute.String("delete_error", deleteErr.Error()),
			))
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update database")
		return nil, fmt.Errorf("failed to update game logo in database: %w", err)
	}

	span.SetAttributes(
		attribute.String("logo.url", logoURL),
		attribute.String("logo.cdn_url", objectInfo.CDNURL),
	)

	return &GameLogoUploadResult{
		LogoURL:    logoURL,
		CDNURL:     objectInfo.CDNURL,
		BrandColor: brandColor,
	}, nil
}

// UpdateGameLogo updates just the logo URL and brand color in the database
// This is used when the file has already been uploaded to storage
func (s *gameService) UpdateGameLogo(ctx context.Context, gameID uuid.UUID, logoURL *string, brandColor *string) (*GameLogoUploadResult, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.update_logo_metadata")
	defer span.End()

	span.SetAttributes(
		attribute.String("game.id", gameID.String()),
	)

	// Verify game exists
	_, err := s.GetGame(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Validate brand color format if provided
	if brandColor != nil && *brandColor != "" {
		if !isValidHexColor(*brandColor) {
			err := fmt.Errorf("invalid brand color format: must be hex color like #FF5733")
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid brand color")
			return nil, err
		}
	}

	// Update database
	if err := s.gameRepo.UpdateLogo(ctx, gameID, logoURL, brandColor); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update database")
		return nil, fmt.Errorf("failed to update game logo in database: %w", err)
	}

	var logoURLStr, cdnURLStr string
	if logoURL != nil {
		logoURLStr = *logoURL
		cdnURLStr = *logoURL // For now, CDN URL is same as logo URL
	}

	return &GameLogoUploadResult{
		LogoURL:    logoURLStr,
		CDNURL:     cdnURLStr,
		BrandColor: brandColor,
	}, nil
}

// DeleteGameLogo removes the game logo from object storage and database
func (s *gameService) DeleteGameLogo(ctx context.Context, gameID uuid.UUID) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "service.game.delete_logo")
	defer span.End()

	span.SetAttributes(attribute.String("game.id", gameID.String()))

	// Check if storage is configured
	if s.storage == nil {
		err := fmt.Errorf("storage not configured")
		span.RecordError(err)
		span.SetStatus(codes.Error, "storage not configured")
		return err
	}

	// Verify game exists
	game, err := s.GetGame(ctx, gameID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "game not found")
		return fmt.Errorf("game not found: %w", err)
	}

	// Check if game has a logo
	if game.LogoURL == nil || *game.LogoURL == "" {
		return fmt.Errorf("game has no logo to delete")
	}

	// Delete from storage
	if err := s.storage.Delete(ctx, gameID.String()); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete logo from storage")
		return fmt.Errorf("failed to delete logo from storage: %w", err)
	}

	// Update database (set logo_url and brand_color to NULL)
	if err := s.gameRepo.UpdateLogo(ctx, gameID, nil, nil); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update database")
		return fmt.Errorf("failed to update game logo in database: %w", err)
	}

	return nil
}

// isValidHexColor validates if a string is a valid hex color code
func isValidHexColor(color string) bool {
	matched, _ := regexp.MatchString(`^#[0-9A-Fa-f]{6}$`, color)
	return matched
}

// GetEventBus returns the event bus instance (for scheduler service)
func (s *gameService) GetEventBus() events.EventBus {
	return s.eventBus
}
