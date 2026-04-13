package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-game/internal/models"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// Cache key prefixes
	gameKeyPrefix           = "game:"
	gameRulesKeyPrefix      = "game_rules:"
	prizeStructureKeyPrefix = "prize_structure:"
	activeGamesKey          = "active_games"
)

// Default cache TTLs - can be overridden via configuration
const (
	defaultGameCacheTTL           = 1 * time.Hour
	defaultGameRulesCacheTTL      = 2 * time.Hour
	defaultPrizeStructureCacheTTL = 2 * time.Hour
	defaultActiveGamesCacheTTL    = 5 * time.Minute
)

// GameCache defines the interface for game caching operations
type GameCache interface {
	// Game caching
	SetGame(ctx context.Context, game *models.Game) error
	GetGame(ctx context.Context, id uuid.UUID) (*models.Game, error)
	DeleteGame(ctx context.Context, id uuid.UUID) error

	// Game rules caching
	SetGameRules(ctx context.Context, rules *models.GameRules) error
	GetGameRules(ctx context.Context, gameID uuid.UUID) (*models.GameRules, error)
	DeleteGameRules(ctx context.Context, gameID uuid.UUID) error

	// Prize structure caching
	SetPrizeStructure(ctx context.Context, structure *models.PrizeStructure) error
	GetPrizeStructure(ctx context.Context, gameID uuid.UUID) (*models.PrizeStructure, error)
	DeletePrizeStructure(ctx context.Context, gameID uuid.UUID) error

	// Active games caching
	SetActiveGames(ctx context.Context, games []*models.Game) error
	GetActiveGames(ctx context.Context) ([]*models.Game, error)
	DeleteActiveGames(ctx context.Context) error

	// Bulk operations
	InvalidateGameCache(ctx context.Context, gameID uuid.UUID) error
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	GameCacheTTL           time.Duration
	GameRulesCacheTTL      time.Duration
	PrizeStructureCacheTTL time.Duration
	ActiveGamesCacheTTL    time.Duration
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		GameCacheTTL:           defaultGameCacheTTL,
		GameRulesCacheTTL:      defaultGameRulesCacheTTL,
		PrizeStructureCacheTTL: defaultPrizeStructureCacheTTL,
		ActiveGamesCacheTTL:    defaultActiveGamesCacheTTL,
	}
}

// gameCache implements GameCache interface
type gameCache struct {
	client *redis.Client
	config *CacheConfig
}

// NewGameCache creates a new instance of GameCache with default config
func NewGameCache(client *redis.Client) GameCache {
	return NewGameCacheWithConfig(client, DefaultCacheConfig())
}

// NewGameCacheWithConfig creates a new instance of GameCache with custom config
func NewGameCacheWithConfig(client *redis.Client, config *CacheConfig) GameCache {
	if config == nil {
		config = DefaultCacheConfig()
	}
	return &gameCache{
		client: client,
		config: config,
	}
}

// SetGame caches a game
func (c *gameCache) SetGame(ctx context.Context, game *models.Game) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.game.set")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "SET"),
		attribute.String("cache.key_prefix", gameKeyPrefix),
		attribute.String("game.id", game.ID.String()),
	)

	key := gameKeyPrefix + game.ID.String()
	data, err := json.Marshal(game)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal game")
		return fmt.Errorf("failed to marshal game: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.config.GameCacheTTL).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to cache game")
		return fmt.Errorf("failed to cache game: %w", err)
	}

	span.SetAttributes(attribute.Bool("cache.hit", false))
	return nil
}

// GetGame retrieves a cached game
func (c *gameCache) GetGame(ctx context.Context, id uuid.UUID) (*models.Game, error) {
	// If no Redis client is configured, return cache miss
	if c.client == nil {
		return nil, nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.game.get")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "GET"),
		attribute.String("cache.key_prefix", gameKeyPrefix),
		attribute.String("game.id", id.String()),
	)

	key := gameKeyPrefix + id.String()
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			span.SetAttributes(attribute.Bool("cache.hit", false))
			return nil, nil // Cache miss
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get cached game")
		return nil, fmt.Errorf("failed to get cached game: %w", err)
	}

	var game models.Game
	if err := json.Unmarshal([]byte(data), &game); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal game")
		return nil, fmt.Errorf("failed to unmarshal game: %w", err)
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	return &game, nil
}

// DeleteGame removes a game from cache
func (c *gameCache) DeleteGame(ctx context.Context, id uuid.UUID) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.game.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "DELETE"),
		attribute.String("cache.key_prefix", gameKeyPrefix),
		attribute.String("game.id", id.String()),
	)

	key := gameKeyPrefix + id.String()
	if err := c.client.Del(ctx, key).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete cached game")
		return fmt.Errorf("failed to delete cached game: %w", err)
	}

	return nil
}

// SetGameRules caches game rules
func (c *gameCache) SetGameRules(ctx context.Context, rules *models.GameRules) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.game_rules.set")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "SET"),
		attribute.String("cache.key_prefix", gameRulesKeyPrefix),
		attribute.String("game.id", rules.GameID.String()),
	)

	key := gameRulesKeyPrefix + rules.GameID.String()
	data, err := json.Marshal(rules)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal game rules")
		return fmt.Errorf("failed to marshal game rules: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.config.GameRulesCacheTTL).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to cache game rules")
		return fmt.Errorf("failed to cache game rules: %w", err)
	}

	return nil
}

// GetGameRules retrieves cached game rules
func (c *gameCache) GetGameRules(ctx context.Context, gameID uuid.UUID) (*models.GameRules, error) {
	// If no Redis client is configured, return cache miss
	if c.client == nil {
		return nil, nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.game_rules.get")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "GET"),
		attribute.String("cache.key_prefix", gameRulesKeyPrefix),
		attribute.String("game.id", gameID.String()),
	)

	key := gameRulesKeyPrefix + gameID.String()
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			span.SetAttributes(attribute.Bool("cache.hit", false))
			return nil, nil // Cache miss
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get cached game rules")
		return nil, fmt.Errorf("failed to get cached game rules: %w", err)
	}

	var rules models.GameRules
	if err := json.Unmarshal([]byte(data), &rules); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal game rules")
		return nil, fmt.Errorf("failed to unmarshal game rules: %w", err)
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	return &rules, nil
}

// DeleteGameRules removes game rules from cache
func (c *gameCache) DeleteGameRules(ctx context.Context, gameID uuid.UUID) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.game_rules.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "DELETE"),
		attribute.String("cache.key_prefix", gameRulesKeyPrefix),
		attribute.String("game.id", gameID.String()),
	)

	key := gameRulesKeyPrefix + gameID.String()
	if err := c.client.Del(ctx, key).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete cached game rules")
		return fmt.Errorf("failed to delete cached game rules: %w", err)
	}

	return nil
}

// SetPrizeStructure caches a prize structure
func (c *gameCache) SetPrizeStructure(ctx context.Context, structure *models.PrizeStructure) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.prize_structure.set")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "SET"),
		attribute.String("cache.key_prefix", prizeStructureKeyPrefix),
		attribute.String("game.id", structure.GameID.String()),
	)

	key := prizeStructureKeyPrefix + structure.GameID.String()
	data, err := json.Marshal(structure)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal prize structure")
		return fmt.Errorf("failed to marshal prize structure: %w", err)
	}

	if err := c.client.Set(ctx, key, data, c.config.PrizeStructureCacheTTL).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to cache prize structure")
		return fmt.Errorf("failed to cache prize structure: %w", err)
	}

	return nil
}

// GetPrizeStructure retrieves a cached prize structure
func (c *gameCache) GetPrizeStructure(ctx context.Context, gameID uuid.UUID) (*models.PrizeStructure, error) {
	// If no Redis client is configured, return cache miss
	if c.client == nil {
		return nil, nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.prize_structure.get")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "GET"),
		attribute.String("cache.key_prefix", prizeStructureKeyPrefix),
		attribute.String("game.id", gameID.String()),
	)

	key := prizeStructureKeyPrefix + gameID.String()
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			span.SetAttributes(attribute.Bool("cache.hit", false))
			return nil, nil // Cache miss
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get cached prize structure")
		return nil, fmt.Errorf("failed to get cached prize structure: %w", err)
	}

	var structure models.PrizeStructure
	if err := json.Unmarshal([]byte(data), &structure); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal prize structure")
		return nil, fmt.Errorf("failed to unmarshal prize structure: %w", err)
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))
	return &structure, nil
}

// DeletePrizeStructure removes a prize structure from cache
func (c *gameCache) DeletePrizeStructure(ctx context.Context, gameID uuid.UUID) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.prize_structure.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "DELETE"),
		attribute.String("cache.key_prefix", prizeStructureKeyPrefix),
		attribute.String("game.id", gameID.String()),
	)

	key := prizeStructureKeyPrefix + gameID.String()
	if err := c.client.Del(ctx, key).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete cached prize structure")
		return fmt.Errorf("failed to delete cached prize structure: %w", err)
	}

	return nil
}

// SetActiveGames caches the list of active games
func (c *gameCache) SetActiveGames(ctx context.Context, games []*models.Game) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.active_games.set")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "SET"),
		attribute.String("cache.key", activeGamesKey),
		attribute.Int("games.count", len(games)),
	)

	data, err := json.Marshal(games)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal active games")
		return fmt.Errorf("failed to marshal active games: %w", err)
	}

	if err := c.client.Set(ctx, activeGamesKey, data, c.config.ActiveGamesCacheTTL).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to cache active games")
		return fmt.Errorf("failed to cache active games: %w", err)
	}

	return nil
}

// GetActiveGames retrieves cached active games
func (c *gameCache) GetActiveGames(ctx context.Context) ([]*models.Game, error) {
	// If no Redis client is configured, return cache miss
	if c.client == nil {
		return nil, nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.active_games.get")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "GET"),
		attribute.String("cache.key", activeGamesKey),
	)

	data, err := c.client.Get(ctx, activeGamesKey).Result()
	if err != nil {
		if err == redis.Nil {
			span.SetAttributes(attribute.Bool("cache.hit", false))
			return nil, nil // Cache miss
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get cached active games")
		return nil, fmt.Errorf("failed to get cached active games: %w", err)
	}

	var games []*models.Game
	if err := json.Unmarshal([]byte(data), &games); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal active games")
		return nil, fmt.Errorf("failed to unmarshal active games: %w", err)
	}

	span.SetAttributes(
		attribute.Bool("cache.hit", true),
		attribute.Int("games.count", len(games)),
	)
	return games, nil
}

// DeleteActiveGames removes active games from cache
func (c *gameCache) DeleteActiveGames(ctx context.Context) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.active_games.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "DELETE"),
		attribute.String("cache.key", activeGamesKey),
	)

	if err := c.client.Del(ctx, activeGamesKey).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete cached active games")
		return fmt.Errorf("failed to delete cached active games: %w", err)
	}

	return nil
}

// InvalidateGameCache invalidates all cache entries related to a game
func (c *gameCache) InvalidateGameCache(ctx context.Context, gameID uuid.UUID) error {
	// If no Redis client is configured, silently skip caching
	if c.client == nil {
		return nil
	}

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-game").Start(ctx, "cache.game.invalidate")
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.operation", "INVALIDATE"),
		attribute.String("game.id", gameID.String()),
	)

	// Delete all related cache entries
	keys := []string{
		gameKeyPrefix + gameID.String(),
		gameRulesKeyPrefix + gameID.String(),
		prizeStructureKeyPrefix + gameID.String(),
		activeGamesKey, // Also invalidate active games list
	}

	pipe := c.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to invalidate game cache")
		return fmt.Errorf("failed to invalidate game cache: %w", err)
	}

	span.SetAttributes(attribute.Int("keys.deleted", len(keys)))
	return nil
}
