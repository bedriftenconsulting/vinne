package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// BetType represents a betting type available for a game
type BetType struct {
	ID         string  `json:"id" db:"id"`
	Name       string  `json:"name" db:"name"`
	Enabled    bool    `json:"enabled" db:"enabled"`
	Multiplier float64 `json:"multiplier" db:"multiplier"`
}

// GameBetType represents the relationship between games and bet types
type GameBetType struct {
	ID         uuid.UUID `json:"id" db:"id"`
	GameID     uuid.UUID `json:"game_id" db:"game_id"`
	BetTypeID  string    `json:"bet_type_id" db:"bet_type_id"`
	Enabled    bool      `json:"enabled" db:"enabled"`
	Multiplier float64   `json:"multiplier" db:"multiplier"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// PrizeDetail represents a single prize entry in a competition
type PrizeDetail struct {
	Rank        int32  `json:"rank"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// Game represents a lottery game
type Game struct {
	ID                  uuid.UUID  `json:"id" db:"id"`
	Code                string     `json:"code" db:"code"`
	Name                string     `json:"name" db:"name" validate:"required,min=1,max=255"`
	Organizer           string     `json:"organizer" db:"organizer" validate:"required"`
	GameCategory        string     `json:"game_category" db:"game_category"`
	GameFormat          string     `json:"game_format" db:"game_format"`
	GameType            *string    `json:"game_type,omitempty" db:"game_type"` // Optional game type
	Type                string     `json:"type" db:"type" validate:"required"` // Original type field from database
	NumberRangeMin      int32      `json:"number_range_min" db:"number_range_min"`
	NumberRangeMax      int32      `json:"number_range_max" db:"number_range_max"`
	SelectionCount      int32      `json:"selection_count" db:"selection_count"`
	StartTime           *string    `json:"start_time,omitempty" db:"start_time"` // Time in HH:MM format
	StartTimeStr        *string    `json:"start_time_str,omitempty"`             // String format for frontend
	EndTime             *string    `json:"end_time,omitempty" db:"end_time"`     // Time in HH:MM format
	EndTimeStr          *string    `json:"end_time_str,omitempty"`               // String format for frontend
	DrawFrequency       string     `json:"draw_frequency" db:"draw_frequency"`
	DrawDays            []string   `json:"draw_days,omitempty" db:"draw_days"` // JSON array for weekly/bi_weekly
	DrawTime            *time.Time `json:"draw_time,omitempty" db:"draw_time"` // Time field in database
	DrawTimeStr         *string    `json:"draw_time_str,omitempty"`            // String format for frontend
	SalesCutoffMinutes  int32      `json:"sales_cutoff_minutes" db:"sales_cutoff_minutes"`
	MinStakeAmount      float64    `json:"min_stake" db:"min_stake_amount"` // in GHS
	MaxStakeAmount      float64    `json:"max_stake" db:"max_stake_amount"` // in GHS
	BasePrice           float64    `json:"base_price" db:"base_price"`      // in GHS
	MaxTicketsPerPlayer int32      `json:"max_tickets_per_player" db:"max_tickets_per_player"`
	MultiDrawEnabled    bool       `json:"multi_draw_enabled" db:"multi_draw_enabled"`
	MaxDrawsAdvance     *int32     `json:"max_draws_advance,omitempty" db:"max_draws_advance"`
	WeeklySchedule      *bool      `json:"weekly_schedule,omitempty" db:"weekly_schedule"`
	Description         *string    `json:"description,omitempty" db:"description"`
	PrizeDetails        []PrizeDetail `json:"prize_details,omitempty" db:"prize_details"` // structured JSONB array
	Rules               *string       `json:"rules,omitempty" db:"rules"`
	TotalTickets        int32         `json:"total_tickets" db:"total_tickets"`
	StartDate           *time.Time    `json:"start_date,omitempty" db:"start_date"` // stored as DATE — maps to draw_date in API
	EndDate             *time.Time    `json:"end_date,omitempty" db:"end_date"`     // stored as DATE — maps to draw_date in API
	LogoURL             *string    `json:"logo_url,omitempty" db:"logo_url"`
	BrandColor          *string    `json:"brand_color,omitempty" db:"brand_color" validate:"omitempty,hexcolor"`
	Status              string     `json:"status" db:"status"`
	Version             string     `json:"version" db:"version"`
	CreatedAt           time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at" db:"updated_at"`
	// Bet types for this game (not stored in DB, populated from relationships)
	BetTypes []BetType `json:"bet_types,omitempty" db:"-"`
}

// GameRules represents the rules for a specific game
type GameRules struct {
	ID     uuid.UUID `json:"id" db:"id"`
	GameID uuid.UUID `json:"game_id" db:"game_id"`
	// Original database fields
	NumbersToPick int32 `json:"numbers_to_pick" db:"numbers_to_pick"` // e.g., 5 for 5/90
	TotalNumbers  int32 `json:"total_numbers" db:"total_numbers"`     // e.g., 90 for 5/90
	MinSelections int32 `json:"min_selections" db:"min_selections"`
	MaxSelections int32 `json:"max_selections" db:"max_selections"`
	// New frontend-aligned fields
	NumberRangeMin *int32     `json:"number_range_min,omitempty" db:"number_range_min"` // e.g., 1 for 1-90
	NumberRangeMax *int32     `json:"number_range_max,omitempty" db:"number_range_max"` // e.g., 90 for 1-90
	SelectionCount *int32     `json:"selection_count,omitempty" db:"selection_count"`   // How many numbers to pick (e.g., 5 for 5/90)
	AllowQuickPick bool       `json:"allow_quick_pick" db:"allow_quick_pick"`
	SpecialRules   *string    `json:"special_rules,omitempty" db:"special_rules"`
	EffectiveFrom  time.Time  `json:"effective_from" db:"effective_from"`
	EffectiveTo    *time.Time `json:"effective_to,omitempty" db:"effective_to"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

// PrizeStructure represents the prize structure for a game
type PrizeStructure struct {
	ID                  uuid.UUID   `json:"id" db:"id"`
	GameID              uuid.UUID   `json:"game_id" db:"game_id"`
	TotalPrizePool      int64       `json:"total_prize_pool" db:"total_prize_pool" validate:"min=0"` // in pesewas
	HouseEdgePercentage float64     `json:"house_edge_percentage" db:"house_edge_percentage" validate:"min=0,max=100"`
	EffectiveFrom       time.Time   `json:"effective_from" db:"effective_from"`
	EffectiveTo         *time.Time  `json:"effective_to,omitempty" db:"effective_to"`
	CreatedAt           time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at" db:"updated_at"`
	Tiers               []PrizeTier `json:"tiers,omitempty"`
}

// PrizeTier represents an individual prize tier
type PrizeTier struct {
	ID               uuid.UUID `json:"id" db:"id"`
	PrizeStructureID uuid.UUID `json:"prize_structure_id" db:"prize_structure_id"`
	TierNumber       int32     `json:"tier_number" db:"tier_number" validate:"min=1"`
	Name             string    `json:"name" db:"name" validate:"required,max=100"`
	MatchesRequired  int32     `json:"matches_required" db:"matches_required" validate:"min=0"`
	PrizeAmount      int64     `json:"prize_amount" db:"prize_amount" validate:"min=0"` // in pesewas
	PrizePercentage  float64   `json:"prize_percentage" db:"prize_percentage" validate:"min=0,max=100"`
	EstimatedWinners int64     `json:"estimated_winners" db:"estimated_winners" validate:"min=0"`
	Description      *string   `json:"description,omitempty" db:"description"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// GameVersion represents version history for games
type GameVersion struct {
	ID           uuid.UUID   `json:"id" db:"id"`
	GameID       uuid.UUID   `json:"game_id" db:"game_id"`
	Version      string      `json:"version" db:"version"`
	Changes      interface{} `json:"changes" db:"changes"` // JSONB
	ChangedBy    uuid.UUID   `json:"changed_by" db:"changed_by"`
	ChangeReason *string     `json:"change_reason,omitempty" db:"change_reason"`
	CreatedAt    time.Time   `json:"created_at" db:"created_at"`
}

// GameApproval represents the approval workflow for games
type GameApproval struct {
	ID                  uuid.UUID     `json:"id" db:"id"`
	GameID              uuid.UUID     `json:"game_id" db:"game_id"`
	ApprovalStage       ApprovalStage `json:"approval_stage" db:"approval_stage"`
	ApprovedBy          *uuid.UUID    `json:"approved_by,omitempty" db:"approved_by"`
	RejectedBy          *uuid.UUID    `json:"rejected_by,omitempty" db:"rejected_by"`
	ApprovalDate        *time.Time    `json:"approval_date,omitempty" db:"approval_date"`
	RejectionDate       *time.Time    `json:"rejection_date,omitempty" db:"rejection_date"`
	Notes               *string       `json:"notes,omitempty" db:"notes"`
	Reason              *string       `json:"reason,omitempty" db:"reason"`
	FirstApprovedBy     *uuid.UUID    `json:"first_approved_by,omitempty" db:"first_approved_by"`
	FirstApprovalDate   *time.Time    `json:"first_approval_date,omitempty" db:"first_approval_date"`
	FirstApprovalNotes  *string       `json:"first_approval_notes,omitempty" db:"first_approval_notes"`
	SecondApprovedBy    *uuid.UUID    `json:"second_approved_by,omitempty" db:"second_approved_by"`
	SecondApprovalDate  *time.Time    `json:"second_approval_date,omitempty" db:"second_approval_date"`
	SecondApprovalNotes *string       `json:"second_approval_notes,omitempty" db:"second_approval_notes"`
	ApprovalCount       int           `json:"approval_count" db:"approval_count"`
	CreatedAt           time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at" db:"updated_at"`
}

// GameSchedule represents scheduled games
type GameSchedule struct {
	ID             uuid.UUID      `json:"id" db:"id"`
	GameID         uuid.UUID      `json:"game_id" db:"game_id"`
	GameName       *string        `json:"game_name,omitempty" db:"game_name"`
	GameCode       *string        `json:"game_code,omitempty" db:"game_code"`         // Fetched via JOIN from games table
	GameCategory   *string        `json:"game_category,omitempty" db:"game_category"` // Fetched via JOIN from games table
	ScheduledStart time.Time      `json:"scheduled_start" db:"scheduled_start"`
	ScheduledEnd   time.Time      `json:"scheduled_end" db:"scheduled_end"`
	ScheduledDraw  time.Time      `json:"scheduled_draw" db:"scheduled_draw"`
	Frequency      DrawFrequency  `json:"frequency" db:"frequency"`
	IsActive       bool           `json:"is_active" db:"is_active"`
	Status         ScheduleStatus `json:"status" db:"status"`
	DrawResultID   *uuid.UUID     `json:"draw_result_id,omitempty" db:"draw_result_id"`
	Notes          *string        `json:"notes,omitempty" db:"notes"`
	LogoURL        *string        `json:"logo_url,omitempty" db:"logo_url"`       // Fetched via JOIN from games table
	BrandColor     *string        `json:"brand_color,omitempty" db:"brand_color"` // Fetched via JOIN from games table
	CreatedAt      time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" db:"updated_at"`
	BetTypes       []BetType      `json:"bet_types,omitempty" db:"-"`
}

// GameAudit represents audit trail for game changes
type GameAudit struct {
	ID        uuid.UUID   `json:"id" db:"id"`
	GameID    uuid.UUID   `json:"game_id" db:"game_id"`
	Action    string      `json:"action" db:"action"`
	ActionBy  uuid.UUID   `json:"action_by" db:"action_by"`
	OldValue  interface{} `json:"old_value,omitempty" db:"old_value"` // JSONB
	NewValue  interface{} `json:"new_value,omitempty" db:"new_value"` // JSONB
	Reason    *string     `json:"reason,omitempty" db:"reason"`
	IPAddress *string     `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent *string     `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
}

// Enums
type GameFormat string

const (
	GameFormat5By90  GameFormat = "5_by_90"
	GameFormatDirect GameFormat = "direct"
	GameFormatPerm   GameFormat = "perm"
	GameFormatBanker GameFormat = "banker"
)

type GameCategory string

const (
	GameCategoryNational GameCategory = "national"
	GameCategoryPrivate  GameCategory = "private"
	GameCategorySpecial  GameCategory = "special"
)

type DrawFrequency string

const (
	DrawFrequencyDaily    DrawFrequency = "daily"
	DrawFrequencyWeekly   DrawFrequency = "weekly"
	DrawFrequencyBiWeekly DrawFrequency = "bi_weekly"
	DrawFrequencyMonthly  DrawFrequency = "monthly"
	DrawFrequencySpecial  DrawFrequency = "special"
)

type ScheduleStatus string

const (
	ScheduleStatusScheduled  ScheduleStatus = "SCHEDULED"
	ScheduleStatusInProgress ScheduleStatus = "IN_PROGRESS"
	ScheduleStatusCompleted  ScheduleStatus = "COMPLETED"
	ScheduleStatusCancelled  ScheduleStatus = "CANCELLED"
	ScheduleStatusFailed     ScheduleStatus = "FAILED"
)

type Organizer string

const (
	OrganizerNLA       Organizer = "ORGANIZER_NLA"
	OrganizerWinBig    Organizer = "ORGANIZER_WINBIG_AFRICA"
)

type GameStatus string

const (
	GameStatusDraft           GameStatus = "DRAFT"
	GameStatusPendingApproval GameStatus = "PENDING_APPROVAL"
	GameStatusApproved        GameStatus = "APPROVED"
	GameStatusActive          GameStatus = "ACTIVE"
	GameStatusSuspended       GameStatus = "SUSPENDED"
	GameStatusTerminated      GameStatus = "TERMINATED"
)

type ApprovalStage string

const (
	ApprovalStageSubmitted     ApprovalStage = "SUBMITTED"
	ApprovalStageReviewed      ApprovalStage = "REVIEWED"
	ApprovalStageFirstApproved ApprovalStage = "FIRST_APPROVED"
	ApprovalStageApproved      ApprovalStage = "APPROVED"
	ApprovalStageRejected      ApprovalStage = "REJECTED"
)

// Filter types for queries
type GameFilter struct {
	GameFormat   *GameFormat   `json:"game_format,omitempty"`
	GameCategory *GameCategory `json:"game_category,omitempty"`
	Organizer    *Organizer    `json:"organizer,omitempty"`
	Status       *GameStatus   `json:"status,omitempty"`
	SearchQuery  *string       `json:"search_query,omitempty"`
}

// UpdateGameScheduleRequest represents a request to update a scheduled game
type UpdateGameScheduleRequest struct {
	ScheduledEnd  *time.Time      `json:"scheduled_end,omitempty"`
	ScheduledDraw *time.Time      `json:"scheduled_draw,omitempty"`
	Status        *ScheduleStatus `json:"status,omitempty"`
	IsActive      *bool           `json:"is_active,omitempty"`
	Notes         *string         `json:"notes,omitempty"`
}

// Request/Response types
type CreateGameRequest struct {
	Code                string    `json:"code" validate:"required,min=1,max=50"`
	Name                string    `json:"name" validate:"required,min=1,max=255"`
	Organizer           string    `json:"organizer" validate:"required"`
	GameCategory        string    `json:"game_category" validate:"required"`
	GameFormat          string    `json:"game_format" validate:"required"`
	GameType            *string   `json:"game_type,omitempty"`
	BetTypes            []BetType `json:"bet_types" validate:"required,min=1"`
	NumberRangeMin      int32     `json:"number_range_min" validate:"required,min=1"`
	NumberRangeMax      int32     `json:"number_range_max" validate:"required,min=1"`
	SelectionCount      int32     `json:"selection_count" validate:"required,min=1"`
	StartTime           *string   `json:"start_time,omitempty"` // HH:MM format
	EndTime             *string   `json:"end_time,omitempty"`   // HH:MM format
	DrawFrequency       string    `json:"draw_frequency" validate:"required"`
	DrawDays            []string  `json:"draw_days,omitempty"`
	DrawTime            *string   `json:"draw_time,omitempty"` // HH:MM format
	SalesCutoffMinutes  int32     `json:"sales_cutoff_minutes" validate:"min=0"`
	MinStake            float64   `json:"min_stake" validate:"min=0.5,max=200000"` // GHS amount
	MaxStake            float64   `json:"max_stake" validate:"min=0.5,max=200000"` // GHS amount
	BasePrice           float64   `json:"base_price" validate:"min=0"`             // GHS amount
	MaxTicketsPerPlayer int32     `json:"max_tickets_per_player" validate:"min=1,max=1000"`
	MultiDrawEnabled    bool      `json:"multi_draw_enabled"`
	MaxDrawsAdvance     *int32    `json:"max_draws_advance,omitempty"`
	WeeklySchedule      *bool     `json:"weekly_schedule,omitempty"`
	Description         *string   `json:"description,omitempty"`
	PrizeDetails        []PrizeDetail `json:"prize_details,omitempty"`
	Rules               *string       `json:"rules,omitempty"`
	TotalTickets        int32         `json:"total_tickets,omitempty"`
	StartDate           *time.Time    `json:"start_date,omitempty"` // maps to draw_date in API
	EndDate             *time.Time    `json:"end_date,omitempty"`   // maps to draw_date in API
	Status              string        `json:"status,omitempty"`
}

type UpdateGameRequest struct {
	ID                  uuid.UUID  `json:"id" validate:"required"`
	Name                *string    `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description         *string    `json:"description,omitempty"`
	DrawFrequency       *string    `json:"draw_frequency,omitempty"`
	DrawDays            []string   `json:"draw_days,omitempty"`
	DrawTime            *time.Time `json:"draw_time,omitempty"`
	SalesCutoffMinutes  *int32     `json:"sales_cutoff_minutes,omitempty"`
	StartTime           *string    `json:"start_time,omitempty"`                                        // Sales start time (HH:MM format)
	EndTime             *string    `json:"end_time,omitempty"`                                          // Sales end time (HH:MM format)
	MinStake            *float64   `json:"min_stake,omitempty" validate:"omitempty,min=0.5,max=200000"` // in GHS
	MaxStake            *float64   `json:"max_stake,omitempty" validate:"omitempty,min=0.5,max=200000"` // in GHS
	BasePrice           *float64   `json:"base_price,omitempty" validate:"omitempty,min=0"`             // in GHS
	MaxTicketsPerPlayer *int32     `json:"max_tickets_per_player,omitempty" validate:"omitempty,min=1,max=1000"`
	MultiDrawEnabled    *bool      `json:"multi_draw_enabled,omitempty"`
	MaxDrawsAdvance     *int32     `json:"max_draws_advance,omitempty"`
	WeeklySchedule      *bool      `json:"weekly_schedule,omitempty"`
	PrizeDetails        []PrizeDetail `json:"prize_details,omitempty"`
	Rules               *string       `json:"rules,omitempty"`
	TotalTickets        *int32        `json:"total_tickets,omitempty"`
	DrawDate            *time.Time    `json:"draw_date,omitempty"` // nil = don't touch; non-nil = set both start+end date
}

type CreateGameRulesRequest struct {
	GameID         uuid.UUID `json:"game_id" validate:"required"`
	NumbersToPick  int32     `json:"numbers_to_pick" validate:"required,min=1"`
	TotalNumbers   int32     `json:"total_numbers" validate:"required,min=1"`
	MinSelections  int32     `json:"min_selections" validate:"min=1"`
	MaxSelections  int32     `json:"max_selections" validate:"max=1"`
	AllowQuickPick bool      `json:"allow_quick_pick"`
	SpecialRules   *string   `json:"special_rules,omitempty"`
}

type CreatePrizeStructureRequest struct {
	GameID              uuid.UUID   `json:"game_id" validate:"required"`
	TotalPrizePool      int64       `json:"total_prize_pool" validate:"min=0"`
	HouseEdgePercentage float64     `json:"house_edge_percentage" validate:"min=0,max=100"`
	Tiers               []PrizeTier `json:"tiers" validate:"required,min=1"`
}

// Utility functions

// PesewasToGHS converts pesewas to Ghana Cedis for display
func PesewasToGHS(pesewas int64) float64 {
	return float64(pesewas) / 100.0
}

// GHSToPesewas converts Ghana Cedis to pesewas for storage
func GHSToPesewas(ghs float64) int64 {
	return int64(ghs * 100)
}

// ValidateBusinessRules validates game business rules
func (g *Game) ValidateBusinessRules() error {
	// Validate required fields
	if g.Code == "" {
		return fmt.Errorf("game code cannot be empty")
	}
	if g.Name == "" {
		return fmt.Errorf("game name cannot be empty")
	}

	// Validate stake amounts only when set
	if g.MinStakeAmount > 0 && g.MaxStakeAmount > 0 {
		if g.MinStakeAmount > g.MaxStakeAmount {
			return fmt.Errorf("minimum stake amount cannot be greater than maximum stake amount")
		}
	}

	// Validate number range only when set
	if g.NumberRangeMin > 0 && g.NumberRangeMax > 0 {
		if g.NumberRangeMax <= g.NumberRangeMin {
			return fmt.Errorf("number range maximum must be greater than minimum")
		}
		if g.SelectionCount > 0 {
			totalNumbers := g.NumberRangeMax - g.NumberRangeMin + 1
			if g.SelectionCount > totalNumbers {
				return fmt.Errorf("selection count %d cannot exceed total available numbers %d", g.SelectionCount, totalNumbers)
			}
		}
	}

	// Validate tickets per player only when set
	if g.MaxTicketsPerPlayer > 0 && g.MaxTicketsPerPlayer > 1000 {
		return fmt.Errorf("maximum tickets per player must be at most 1000")
	}

	// Validate sales cutoff
	if g.SalesCutoffMinutes < 0 {
		return fmt.Errorf("sales cutoff minutes cannot be negative")
	}

	// Validate multi-draw configuration
	if g.MultiDrawEnabled && g.MaxDrawsAdvance != nil && *g.MaxDrawsAdvance < 1 {
		return fmt.Errorf("maximum draws in advance must be at least 1 when multi-draw is enabled")
	}

	return nil
}

// ConvertToGame converts CreateGameRequest to Game model with proper time validation
func (req *CreateGameRequest) ConvertToGame() (*Game, error) {
	var drawTime *time.Time

	// Parse and validate draw time if provided (HH:MM format to time.Time)
	if req.DrawTime != nil && *req.DrawTime != "" {
		parsed, err := time.Parse("15:04", *req.DrawTime)
		if err != nil {
			return nil, fmt.Errorf("invalid draw_time format '%s': expected HH:MM (e.g., '14:30')", *req.DrawTime)
		}
		drawTime = &parsed
	}

	// Validate start time format if provided
	if req.StartTime != nil && *req.StartTime != "" {
		if _, err := time.Parse("15:04", *req.StartTime); err != nil {
			return nil, fmt.Errorf("invalid start_time format '%s': expected HH:MM (e.g., '08:00')", *req.StartTime)
		}
	}

	// Validate end time format if provided
	if req.EndTime != nil && *req.EndTime != "" {
		if _, err := time.Parse("15:04", *req.EndTime); err != nil {
			return nil, fmt.Errorf("invalid end_time format '%s': expected HH:MM (e.g., '20:00')", *req.EndTime)
		}
	}

	// Apply defaults for lottery-specific fields not used by WinBig competitions
	organizer := req.Organizer
	if organizer == "" {
		organizer = "ORGANIZER_WINBIG_AFRICA"
	}
	gameFormat := req.GameFormat
	if gameFormat == "" {
		gameFormat = "competition"
	}
	gameCategory := req.GameCategory
	if gameCategory == "" {
		gameCategory = "private"
	}
	drawFrequency := req.DrawFrequency
	if drawFrequency == "" {
		drawFrequency = "special"
	}
	numberRangeMin := req.NumberRangeMin
	if numberRangeMin == 0 {
		numberRangeMin = 1
	}
	numberRangeMax := req.NumberRangeMax
	if numberRangeMax == 0 {
		numberRangeMax = 90
	}
	selectionCount := req.SelectionCount
	if selectionCount == 0 {
		selectionCount = 5
	}
	minStake := req.MinStake
	if minStake == 0 {
		minStake = req.BasePrice
	}
	maxStake := req.MaxStake
	if maxStake == 0 {
		maxStake = req.BasePrice
	}
	maxTickets := req.MaxTicketsPerPlayer
	if maxTickets == 0 {
		maxTickets = 1000
	}

	status := req.Status
	if status == "" {
		status = "DRAFT"
	}

	return &Game{
		Code:                req.Code,
		Name:                req.Name,
		GameFormat:          gameFormat,
		GameCategory:        gameCategory,
		Organizer:           organizer,
		GameType:            req.GameType,
		NumberRangeMin:      numberRangeMin,
		NumberRangeMax:      numberRangeMax,
		SelectionCount:      selectionCount,
		MinStakeAmount:      minStake,
		MaxStakeAmount:      maxStake,
		BasePrice:           req.BasePrice,
		MaxTicketsPerPlayer: maxTickets,
		TotalTickets:        req.TotalTickets,
		MultiDrawEnabled:    req.MultiDrawEnabled,
		MaxDrawsAdvance:     req.MaxDrawsAdvance,
		DrawFrequency:       drawFrequency,
		DrawDays:            req.DrawDays,
		DrawTime:            drawTime,
		SalesCutoffMinutes:  req.SalesCutoffMinutes,
		WeeklySchedule:      req.WeeklySchedule,
		Status:              status,
		Description:         req.Description,
		PrizeDetails:        req.PrizeDetails, // []PrizeDetail
		Rules:               req.Rules,
		StartDate:           req.StartDate, // *time.Time — same as draw_date
		EndDate:             req.EndDate,   // *time.Time — same as draw_date
		StartTime:           req.StartTime,
		StartTimeStr:        req.StartTime,
		EndTime:             req.EndTime,
		EndTimeStr:          req.EndTime,
		DrawTimeStr:         req.DrawTime,
		Version:             "1.0.0",
		Type:                "standard",
	}, nil
}

// ConvertToGameRules converts CreateGameRequest to GameRules model
func (req *CreateGameRequest) ConvertToGameRules(gameID uuid.UUID) *GameRules {
	return &GameRules{
		GameID:         gameID,
		NumberRangeMin: &req.NumberRangeMin,
		NumberRangeMax: &req.NumberRangeMax,
		SelectionCount: &req.SelectionCount,
		AllowQuickPick: true,
		EffectiveFrom:  time.Now(),
	}
}

// Validate validates the create game request
func (req *CreateGameRequest) Validate() error {
	// Validate required fields
	if req.Code == "" {
		return fmt.Errorf("game code is required")
	}
	if req.Name == "" {
		return fmt.Errorf("game name is required")
	}

	// Validate stake amounts only when explicitly provided
	if req.MinStake > 0 && req.MaxStake > 0 {
		if req.MinStake > req.MaxStake {
			return fmt.Errorf("minimum stake cannot be greater than maximum stake")
		}
	}

	// Validate number range only when explicitly provided
	if req.NumberRangeMin > 0 && req.NumberRangeMax > 0 {
		if req.NumberRangeMax <= req.NumberRangeMin {
			return fmt.Errorf("number range maximum must be greater than minimum")
		}
		if req.SelectionCount > 0 && req.SelectionCount > (req.NumberRangeMax-req.NumberRangeMin+1) {
			return fmt.Errorf("selection count cannot be greater than available numbers")
		}
	}

	// Validate tickets per player only when explicitly provided
	if req.MaxTicketsPerPlayer > 0 && req.MaxTicketsPerPlayer > 1000 {
		return fmt.Errorf("maximum tickets per player must be between 1 and 1000")
	}

	// Validate time formats if provided
	if req.DrawTime != nil && *req.DrawTime != "" {
		if _, err := time.Parse("15:04", *req.DrawTime); err != nil {
			return fmt.Errorf("invalid draw time format, use HH:MM")
		}
	}
	if req.StartTime != nil && *req.StartTime != "" {
		if _, err := time.Parse("15:04", *req.StartTime); err != nil {
			return fmt.Errorf("invalid start time format, use HH:MM")
		}
	}
	if req.EndTime != nil && *req.EndTime != "" {
		if _, err := time.Parse("15:04", *req.EndTime); err != nil {
			return fmt.Errorf("invalid end time format, use HH:MM")
		}
	}

	// Validate multi-draw configuration
	if req.MultiDrawEnabled && req.MaxDrawsAdvance != nil && *req.MaxDrawsAdvance < 1 {
		return fmt.Errorf("maximum draws in advance must be at least 1 when multi-draw is enabled")
	}

	return nil
}

// IsActive checks if the game is currently active
func (g *Game) IsActive() bool {
	return g.Status == string(GameStatusActive)
}

// CanBeModified checks if the game can be modified
func (g *Game) CanBeModified() bool {
	// Allow modification of games in any status except Terminated
	// Admins can modify Draft, PendingApproval, Approved, Active, and Suspended games
	return g.Status != string(GameStatusTerminated)
}

// CanTransitionTo checks if the game can transition to a new status
func (g *Game) CanTransitionTo(newStatus GameStatus) bool {
	currentStatus := GameStatus(g.Status)
	switch currentStatus {
	case GameStatusDraft:
		// Allow direct activation from DRAFT (approval workflow removed)
		return newStatus == GameStatusActive || newStatus == GameStatusTerminated
	case GameStatusPendingApproval:
		// Legacy status - allow transition to DRAFT or TERMINATED
		return newStatus == GameStatusDraft || newStatus == GameStatusTerminated
	case GameStatusApproved:
		// Legacy status - allow transition to ACTIVE or TERMINATED
		return newStatus == GameStatusActive || newStatus == GameStatusTerminated
	case GameStatusActive:
		return newStatus == GameStatusSuspended || newStatus == GameStatusTerminated
	case GameStatusSuspended:
		return newStatus == GameStatusActive || newStatus == GameStatusTerminated
	case GameStatusTerminated:
		return false // Terminal state
	default:
		return false
	}
}

// SetStatus updates the game status with validation
func (g *Game) SetStatus(newStatus GameStatus) error {
	if !g.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition from %s to %s", g.Status, newStatus)
	}
	g.Status = string(newStatus)
	g.UpdatedAt = time.Now()
	return nil
}

// IsSalesOpen checks if sales are currently open based on time
func (g *Game) IsSalesOpen() bool {
	if !g.IsActive() {
		return false
	}

	now := time.Now()

	// Check if we have start and end times
	if g.StartTime != nil && g.EndTime != nil {
		// Parse time strings (HH:MM format)
		startTime, err := time.Parse("15:04", *g.StartTime)
		if err != nil {
			return true // Default to open if parse fails
		}
		endTime, err := time.Parse("15:04", *g.EndTime)
		if err != nil {
			return true // Default to open if parse fails
		}

		// Convert to today's date
		currentTime := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), startTime.Hour(), startTime.Minute(), 0, 0, now.Location())
		todayEnd := time.Date(now.Year(), now.Month(), now.Day(), endTime.Hour(), endTime.Minute(), 0, 0, now.Location())

		// Handle cases where end time is after midnight
		if todayEnd.Before(todayStart) {
			todayEnd = todayEnd.AddDate(0, 0, 1)
		}

		return currentTime.After(todayStart) && currentTime.Before(todayEnd)
	}

	return true // Default to open if no times specified
}

// GetSalesCutoffTime calculates the sales cutoff time for the next draw
func (g *Game) GetSalesCutoffTime() *time.Time {
	if g.DrawTime == nil {
		return nil
	}

	cutoffTime := g.DrawTime.Add(-time.Duration(g.SalesCutoffMinutes) * time.Minute)
	return &cutoffTime
}

// ValidateStakeAmount validates if a stake amount is within allowed limits
func (g *Game) ValidateStakeAmount(amount float64) error {
	if amount < g.MinStakeAmount {
		return fmt.Errorf("stake amount %.2f is below minimum %.2f", amount, g.MinStakeAmount)
	}
	if amount > g.MaxStakeAmount {
		return fmt.Errorf("stake amount %.2f exceeds maximum %.2f", amount, g.MaxStakeAmount)
	}
	return nil
}

// IsNumberInRange checks if a number is within the game's valid range
func (g *Game) IsNumberInRange(number int32) bool {
	return number >= g.NumberRangeMin && number <= g.NumberRangeMax
}

// ValidateSelection validates a player's number selection
func (g *Game) ValidateSelection(numbers []int32) error {
	if int32(len(numbers)) != g.SelectionCount {
		return fmt.Errorf("expected %d numbers, got %d", g.SelectionCount, len(numbers))
	}

	// Check for duplicates
	seen := make(map[int32]bool)
	for _, num := range numbers {
		if seen[num] {
			return fmt.Errorf("duplicate number %d in selection", num)
		}
		seen[num] = true

		// Check if number is in valid range
		if !g.IsNumberInRange(num) {
			return fmt.Errorf("number %d is outside valid range %d-%d", num, g.NumberRangeMin, g.NumberRangeMax)
		}
	}

	return nil
}

// NeedsApproval checks if the game needs approval
func (g *Game) NeedsApproval() bool {
	return g.Status == string(GameStatusPendingApproval)
}

// CanBeActivated checks if the game can be activated
func (g *Game) CanBeActivated() bool {
	return g.Status == string(GameStatusApproved)
}

// CanBeSuspended checks if the game can be suspended
func (g *Game) CanBeSuspended() bool {
	return g.Status == string(GameStatusActive)
}
