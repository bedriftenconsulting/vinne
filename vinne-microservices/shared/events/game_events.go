package events

import (
	"encoding/json"
	"time"
)

// GameData represents game information in events
type GameData struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	GameFormat  string    `json:"game_format"`
	Organizer   string    `json:"organizer"`
	Status      string    `json:"status"`
	MinStake    int64     `json:"min_stake"`
	MaxStake    int64     `json:"max_stake"`
	Description *string   `json:"description,omitempty"`
	DrawTime    *string   `json:"draw_time,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GameCreatedEvent represents a game creation event
type GameCreatedEvent struct {
	BaseEvent
	Game      GameData `json:"game"`
	CreatedBy string   `json:"created_by"`
}

// NewGameCreatedEvent creates a new game created event
func NewGameCreatedEvent(source string, game GameData, createdBy string) *GameCreatedEvent {
	return &GameCreatedEvent{
		BaseEvent: NewBaseEvent(GameCreated, source).
			WithUserID(createdBy).
			WithMetadata("game_id", game.ID).
			WithMetadata("game_name", game.Name),
		Game:      game,
		CreatedBy: createdBy,
	}
}

// Marshal serializes the event to JSON
func (e *GameCreatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// GameUpdatedEvent represents a game update event
type GameUpdatedEvent struct {
	BaseEvent
	Game      GameData               `json:"game"`
	UpdatedBy string                 `json:"updated_by"`
	Changes   map[string]interface{} `json:"changes,omitempty"`
}

// NewGameUpdatedEvent creates a new game updated event
func NewGameUpdatedEvent(source string, game GameData, updatedBy string, changes map[string]interface{}) *GameUpdatedEvent {
	return &GameUpdatedEvent{
		BaseEvent: NewBaseEvent(GameUpdated, source).
			WithUserID(updatedBy).
			WithMetadata("game_id", game.ID).
			WithMetadata("game_name", game.Name),
		Game:      game,
		UpdatedBy: updatedBy,
		Changes:   changes,
	}
}

// Marshal serializes the event to JSON
func (e *GameUpdatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// GameDeletedEvent represents a game deletion event
type GameDeletedEvent struct {
	BaseEvent
	GameID    string    `json:"game_id"`
	GameName  string    `json:"game_name"`
	DeletedBy string    `json:"deleted_by"`
	DeletedAt time.Time `json:"deleted_at"`
}

// NewGameDeletedEvent creates a new game deleted event
func NewGameDeletedEvent(source string, gameID, gameName, deletedBy string) *GameDeletedEvent {
	return &GameDeletedEvent{
		BaseEvent: NewBaseEvent(GameDeleted, source).
			WithUserID(deletedBy).
			WithMetadata("game_id", gameID).
			WithMetadata("game_name", gameName),
		GameID:    gameID,
		GameName:  gameName,
		DeletedBy: deletedBy,
		DeletedAt: time.Now().UTC(),
	}
}

// Marshal serializes the event to JSON
func (e *GameDeletedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// GameActivatedEvent represents a game activation event
type GameActivatedEvent struct {
	BaseEvent
	GameID      string    `json:"game_id"`
	GameName    string    `json:"game_name"`
	ActivatedBy string    `json:"activated_by"`
	ActivatedAt time.Time `json:"activated_at"`
}

// NewGameActivatedEvent creates a new game activated event
func NewGameActivatedEvent(source string, gameID, gameName, activatedBy string) *GameActivatedEvent {
	return &GameActivatedEvent{
		BaseEvent: NewBaseEvent(GameActivated, source).
			WithUserID(activatedBy).
			WithMetadata("game_id", gameID).
			WithMetadata("game_name", gameName),
		GameID:      gameID,
		GameName:    gameName,
		ActivatedBy: activatedBy,
		ActivatedAt: time.Now().UTC(),
	}
}

// Marshal serializes the event to JSON
func (e *GameActivatedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// GameSuspendedEvent represents a game suspension event
type GameSuspendedEvent struct {
	BaseEvent
	GameID      string    `json:"game_id"`
	GameName    string    `json:"game_name"`
	SuspendedBy string    `json:"suspended_by"`
	SuspendedAt time.Time `json:"suspended_at"`
	Reason      string    `json:"reason,omitempty"`
}

// NewGameSuspendedEvent creates a new game suspended event
func NewGameSuspendedEvent(source string, gameID, gameName, suspendedBy, reason string) *GameSuspendedEvent {
	return &GameSuspendedEvent{
		BaseEvent: NewBaseEvent(GameDeactivated, source).
			WithUserID(suspendedBy).
			WithMetadata("game_id", gameID).
			WithMetadata("game_name", gameName),
		GameID:      gameID,
		GameName:    gameName,
		SuspendedBy: suspendedBy,
		SuspendedAt: time.Now().UTC(),
		Reason:      reason,
	}
}

// Marshal serializes the event to JSON
func (e *GameSuspendedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// GameDrawExecutedEvent represents a game draw execution event
type GameDrawExecutedEvent struct {
	BaseEvent
	ScheduleID        string    `json:"schedule_id"`
	GameID            string    `json:"game_id"`
	GameName          string    `json:"game_name"`
	GameCode          string    `json:"game_code"`
	DrawID            string    `json:"draw_id"`
	ScheduledDrawTime time.Time `json:"scheduled_draw_time"`
	ActualDrawTime    time.Time `json:"actual_draw_time"`
	ExecutedBy        string    `json:"executed_by"`
}

// NewGameDrawExecutedEvent creates a new game draw executed event
func NewGameDrawExecutedEvent(
	source string,
	scheduleID, gameID, gameName, gameCode, drawID string,
	scheduledDrawTime time.Time,
) *GameDrawExecutedEvent {
	return &GameDrawExecutedEvent{
		BaseEvent: NewBaseEvent(DrawExecuted, source).
			WithMetadata("schedule_id", scheduleID).
			WithMetadata("game_id", gameID).
			WithMetadata("game_name", gameName).
			WithMetadata("draw_id", drawID),
		ScheduleID:        scheduleID,
		GameID:            gameID,
		GameName:          gameName,
		GameCode:          gameCode,
		DrawID:            drawID,
		ScheduledDrawTime: scheduledDrawTime,
		ActualDrawTime:    time.Now().UTC(),
		ExecutedBy:        "scheduler-service",
	}
}

// Marshal serializes the event to JSON
func (e *GameDrawExecutedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// GameSalesCutoffReachedEvent represents a game sales cutoff event
type GameSalesCutoffReachedEvent struct {
	BaseEvent
	ScheduleID       string    `json:"schedule_id"`
	GameID           string    `json:"game_id"`
	GameName         string    `json:"game_name"`
	GameCode         string    `json:"game_code"`
	ScheduledEndTime time.Time `json:"scheduled_end_time"`
	ActualCutoffTime time.Time `json:"actual_cutoff_time"`
	NextDrawTime     time.Time `json:"next_draw_time"`
	ExecutedBy       string    `json:"executed_by"`
}

// NewGameSalesCutoffReachedEvent creates a new game sales cutoff reached event
func NewGameSalesCutoffReachedEvent(
	source string,
	scheduleID, gameID, gameName, gameCode string,
	scheduledEndTime, nextDrawTime time.Time,
) *GameSalesCutoffReachedEvent {
	return &GameSalesCutoffReachedEvent{
		BaseEvent: NewBaseEvent(SalesCutoffReached, source).
			WithMetadata("schedule_id", scheduleID).
			WithMetadata("game_id", gameID).
			WithMetadata("game_name", gameName),
		ScheduleID:       scheduleID,
		GameID:           gameID,
		GameName:         gameName,
		GameCode:         gameCode,
		ScheduledEndTime: scheduledEndTime,
		ActualCutoffTime: time.Now().UTC(),
		NextDrawTime:     nextDrawTime,
		ExecutedBy:       "scheduler-service",
	}
}

// Marshal serializes the event to JSON
func (e *GameSalesCutoffReachedEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
