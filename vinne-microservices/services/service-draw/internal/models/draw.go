package models

import (
	"database/sql/driver"
	"fmt"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"time"
)

type DrawStatus string

const (
	DrawStatusScheduled  DrawStatus = "scheduled"
	DrawStatusInProgress DrawStatus = "in_progress"
	DrawStatusCompleted  DrawStatus = "completed"
	DrawStatusFailed     DrawStatus = "failed"
	DrawStatusCancelled  DrawStatus = "cancelled"
)

type ValidationStatus string

const (
	ValidationStatusPending  ValidationStatus = "pending"
	ValidationStatusVerified ValidationStatus = "verified"
	ValidationStatusRejected ValidationStatus = "rejected"
)

type ScheduleFrequency string

const (
	FrequencyOneTime ScheduleFrequency = "one_time"
	FrequencyDaily   ScheduleFrequency = "daily"
	FrequencyWeekly  ScheduleFrequency = "weekly"
	FrequencyMonthly ScheduleFrequency = "monthly"
	FrequencyCustom  ScheduleFrequency = "custom"
)

type Draw struct {
	ID                   uuid.UUID     `json:"id" db:"id"`
	GameID               uuid.UUID     `json:"game_id" db:"game_id" validate:"required"`
	DrawNumber           int           `json:"draw_number" db:"draw_number"`
	GameName             string        `json:"game_name" db:"game_name"`
	GameCode             string        `json:"game_code" db:"game_code"`
	GameLogoURL          *string       `json:"game_logo_url,omitempty" db:"game_logo_url"`       // Fetched via JOIN from games table
	GameBrandColor       *string       `json:"game_brand_color,omitempty" db:"game_brand_color"` // Fetched via JOIN from games table
	GameScheduleID       uuid.UUID     `json:"game_schedule_id" db:"game_schedule_id"`           // Link to the game schedule this draw was created from
	DrawName             string        `json:"draw_name" db:"draw_name" validate:"required,min=2,max=255"`
	Status               DrawStatus    `json:"status" db:"status"`
	ScheduledTime        time.Time     `json:"scheduled_time" db:"scheduled_time" validate:"required"`
	ExecutedTime         *time.Time    `json:"executed_time,omitempty" db:"executed_time"`
	WinningNumbers       pq.Int32Array `json:"winning_numbers" db:"winning_numbers"`
	MachineNumbers       pq.Int32Array `json:"machine_numbers" db:"machine_numbers"` // Cosmetic only, not used in calculations
	NLADrawReference     *string       `json:"nla_draw_reference,omitempty" db:"nla_draw_reference"`
	DrawLocation         *string       `json:"draw_location,omitempty" db:"draw_location"`
	NLAOfficialSignature *string       `json:"nla_official_signature,omitempty" db:"nla_official_signature"`
	TotalTicketsSold     int64         `json:"total_tickets_sold" db:"total_tickets_sold"`
	TotalPrizePool       int64         `json:"total_prize_pool" db:"total_prize_pool"` // in pesewas
	VerificationHash     *string       `json:"verification_hash,omitempty" db:"verification_hash"`
	StageData            *DrawStage    `json:"stage_data,omitempty" db:"stage_data"` // Execution workflow stage data
	CreatedAt            time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at" db:"updated_at"`
}

type DrawSchedule struct {
	ID            uuid.UUID         `json:"id" db:"id"`
	GameID        uuid.UUID         `json:"game_id" db:"game_id" validate:"required"`
	DrawName      string            `json:"draw_name" db:"draw_name" validate:"required,min=2,max=255"`
	ScheduledTime time.Time         `json:"scheduled_time" db:"scheduled_time" validate:"required"`
	Frequency     ScheduleFrequency `json:"frequency" db:"frequency"`
	IsActive      bool              `json:"is_active" db:"is_active"`
	CreatedBy     string            `json:"created_by" db:"created_by" validate:"required"`
	Notes         *string           `json:"notes,omitempty" db:"notes"`
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
}

type DrawResult struct {
	ID                 uuid.UUID           `json:"id" db:"id"`
	DrawID             uuid.UUID           `json:"draw_id" db:"draw_id" validate:"required"`
	WinningNumbers     pq.Int32Array       `json:"winning_numbers" db:"winning_numbers" validate:"required"`
	PrizeDistributions []PrizeDistribution `json:"prize_distributions"`
	TotalWinners       int64               `json:"total_winners" db:"total_winners"`
	TotalPrizePaid     int64               `json:"total_prize_paid" db:"total_prize_paid"` // in pesewas
	IsPublished        bool                `json:"is_published" db:"is_published"`
	PublishedAt        *time.Time          `json:"published_at,omitempty" db:"published_at"`
	VerificationHash   *string             `json:"verification_hash,omitempty" db:"verification_hash"`
	CreatedAt          time.Time           `json:"created_at" db:"created_at"`
}

type PrizeDistribution struct {
	ID               uuid.UUID `json:"id" db:"id"`
	DrawResultID     uuid.UUID `json:"draw_result_id" db:"draw_result_id" validate:"required"`
	Tier             int       `json:"tier" db:"tier" validate:"required,min=1"`
	TierName         string    `json:"tier_name" db:"tier_name" validate:"required,max=100"`
	MatchesRequired  int       `json:"matches_required" db:"matches_required" validate:"required,min=0"`
	WinnersCount     int64     `json:"winners_count" db:"winners_count"`
	PrizePerWinner   int64     `json:"prize_per_winner" db:"prize_per_winner"`     // in pesewas
	TotalPrizeAmount int64     `json:"total_prize_amount" db:"total_prize_amount"` // in pesewas
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

type DrawValidation struct {
	ID                  uuid.UUID        `json:"id" db:"id"`
	DrawID              uuid.UUID        `json:"draw_id" db:"draw_id" validate:"required"`
	NLAReference        string           `json:"nla_reference" db:"nla_reference" validate:"required"`
	DrawCertificate     *string          `json:"draw_certificate,omitempty" db:"draw_certificate"`
	WitnessSignature    *string          `json:"witness_signature,omitempty" db:"witness_signature"`
	SupportingDocuments pq.StringArray   `json:"supporting_documents" db:"supporting_documents"`
	ValidationStatus    ValidationStatus `json:"validation_status" db:"validation_status"`
	ValidatedBy         *string          `json:"validated_by,omitempty" db:"validated_by"`
	ValidatedAt         *time.Time       `json:"validated_at,omitempty" db:"validated_at"`
	CreatedAt           time.Time        `json:"created_at" db:"created_at"`
}

// Helper methods for currency conversion
func (d *Draw) GetTotalPrizePoolInGHS() float64 {
	return PesewasToGHS(d.TotalPrizePool)
}

func (d *Draw) SetTotalPrizePoolFromGHS(ghs float64) {
	d.TotalPrizePool = GHSToPesewas(ghs)
}

func (dr *DrawResult) GetTotalPrizePaidInGHS() float64 {
	return PesewasToGHS(dr.TotalPrizePaid)
}

func (dr *DrawResult) SetTotalPrizePaidFromGHS(ghs float64) {
	dr.TotalPrizePaid = GHSToPesewas(ghs)
}

func (pd *PrizeDistribution) GetPrizePerWinnerInGHS() float64 {
	return PesewasToGHS(pd.PrizePerWinner)
}

func (pd *PrizeDistribution) SetPrizePerWinnerFromGHS(ghs float64) {
	pd.PrizePerWinner = GHSToPesewas(ghs)
}

func (pd *PrizeDistribution) GetTotalPrizeAmountInGHS() float64 {
	return PesewasToGHS(pd.TotalPrizeAmount)
}

func (pd *PrizeDistribution) SetTotalPrizeAmountFromGHS(ghs float64) {
	pd.TotalPrizeAmount = GHSToPesewas(ghs)
}

// Currency conversion helpers
func PesewasToGHS(pesewas int64) float64 {
	return float64(pesewas) / 100.0
}

func GHSToPesewas(ghs float64) int64 {
	return int64(ghs * 100)
}

// Validation helpers
func (d *Draw) IsValidForExecution() bool {
	return d.Status == DrawStatusScheduled &&
		d.GameID != uuid.Nil &&
		!d.ScheduledTime.IsZero() &&
		len(d.DrawName) > 0
}

func (d *Draw) IsCompleted() bool {
	return d.Status == DrawStatusCompleted &&
		len(d.WinningNumbers) > 0 &&
		d.ExecutedTime != nil
}

func (ds *DrawSchedule) IsCurrentlyActive() bool {
	return ds.IsActive && time.Now().Before(ds.ScheduledTime)
}

func (dv *DrawValidation) IsVerified() bool {
	return dv.ValidationStatus == ValidationStatusVerified &&
		dv.ValidatedBy != nil &&
		dv.ValidatedAt != nil
}

// String methods for enums
func (ds DrawStatus) String() string {
	return string(ds)
}

func (vs ValidationStatus) String() string {
	return string(vs)
}

func (sf ScheduleFrequency) String() string {
	return string(sf)
}

// Value and Scan methods for database compatibility
func (ds DrawStatus) Value() (driver.Value, error) {
	return string(ds), nil
}

func (ds *DrawStatus) Scan(value interface{}) error {
	if value == nil {
		*ds = DrawStatusScheduled
		return nil
	}
	if str, ok := value.(string); ok {
		*ds = DrawStatus(str)
		return nil
	}
	return fmt.Errorf("cannot scan %T into DrawStatus", value)
}

func (vs ValidationStatus) Value() (driver.Value, error) {
	return string(vs), nil
}

func (vs *ValidationStatus) Scan(value interface{}) error {
	if value == nil {
		*vs = ValidationStatusPending
		return nil
	}
	if str, ok := value.(string); ok {
		*vs = ValidationStatus(str)
		return nil
	}
	return fmt.Errorf("cannot scan %T into ValidationStatus", value)
}

func (sf ScheduleFrequency) Value() (driver.Value, error) {
	return string(sf), nil
}

func (sf *ScheduleFrequency) Scan(value interface{}) error {
	if value == nil {
		*sf = FrequencyOneTime
		return nil
	}
	if str, ok := value.(string); ok {
		*sf = ScheduleFrequency(str)
		return nil
	}
	return fmt.Errorf("cannot scan %T into ScheduleFrequency", value)
}
