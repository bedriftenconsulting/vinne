package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/randco/service-player/internal/models"
)

type PlayerRepository interface {
	Create(ctx context.Context, req models.CreatePlayerRequest) (*models.Player, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Player, error)
	GetByPhoneNumber(ctx context.Context, phoneNumber string) (*models.Player, error)
	GetByEmail(ctx context.Context, email string) (*models.Player, error)
	Update(ctx context.Context, req models.UpdatePlayerRequest) (*models.Player, error)
	SetEmailVerified(ctx context.Context, id uuid.UUID) error
	SetPhoneVerified(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter models.PlayerFilter) ([]*models.Player, error)
	Count(ctx context.Context, filter models.PlayerFilter) (int64, error)
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	Search(ctx context.Context, query string, limit, offset int) ([]*models.Player, error)
	Suspend(ctx context.Context, id uuid.UUID, reason string) error
	Activate(ctx context.Context, id uuid.UUID) error
	VerifyPhoneNumber(ctx context.Context, phoneNumber string) error
}

type playerRepository struct {
	db *sql.DB
}

func NewPlayerRepository(db *sql.DB) PlayerRepository {
	return &playerRepository{db: db}
}

// scanPlayerWithNulls scans a Player from database rows, handling NULL values
func scanPlayerWithNulls(rows *sql.Rows) (*models.Player, error) {
	var player models.Player
	var email, firstName, lastName, nationalID, mobileMoneyPhone sql.NullString
	var dateOfBirth, lastLoginAt, deletedAt sql.NullTime

	err := rows.Scan(
		&player.ID,
		&player.PhoneNumber,
		&email,
		&player.PasswordHash,
		&firstName,
		&lastName,
		&dateOfBirth,
		&nationalID,
		&mobileMoneyPhone,
		&player.Status,
		&player.EmailVerified,
		&player.PhoneVerified,
		&player.RegistrationChannel,
		&player.TermsAccepted,
		&player.MarketingConsent,
		&player.CreatedAt,
		&player.UpdatedAt,
		&lastLoginAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}

	// Convert NULL values to empty strings/zero times
	if email.Valid {
		player.Email = email.String
	}
	if firstName.Valid {
		player.FirstName = firstName.String
	}
	if lastName.Valid {
		player.LastName = lastName.String
	}
	if nationalID.Valid {
		player.NationalID = nationalID.String
	}
	if mobileMoneyPhone.Valid {
		player.MobileMoneyPhone = mobileMoneyPhone.String
	}
	if dateOfBirth.Valid {
		player.DateOfBirth = dateOfBirth.Time
	}
	if lastLoginAt.Valid {
		player.LastLoginAt = lastLoginAt.Time
	}
	if deletedAt.Valid {
		player.DeletedAt = deletedAt.Time
	}

	return &player, nil
}

func (r *playerRepository) Create(ctx context.Context, req models.CreatePlayerRequest) (*models.Player, error) {
	query := `
		INSERT INTO players (
			id, phone_number, email, password_hash, first_name, last_name,
			date_of_birth, national_id, mobile_money_phone, status,
			email_verified, phone_verified, registration_channel,
			terms_accepted, marketing_consent, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		) RETURNING *
	`

	now := time.Now()
	playerID := uuid.New()

	rows, err := r.db.QueryContext(ctx, query,
		playerID, req.PhoneNumber, req.Email, req.PasswordHash,
		req.FirstName, req.LastName, req.DateOfBirth, req.NationalID,
		req.MobileMoneyPhone, models.PlayerStatusActive, false, false,
		req.RegistrationChannel, req.TermsAccepted, req.MarketingConsent,
		now, now,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create player: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("failed to get created player")
	}

	return scanPlayerWithNulls(rows)
}

func (r *playerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Player, error) {
	query := `SELECT * FROM players WHERE id = $1 AND deleted_at IS NULL`

	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query player by ID: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return scanPlayerWithNulls(rows)
}

func (r *playerRepository) GetByPhoneNumber(ctx context.Context, phoneNumber string) (*models.Player, error) {
	query := `SELECT * FROM players WHERE phone_number = $1 AND deleted_at IS NULL`

	rows, err := r.db.QueryContext(ctx, query, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to query player by phone number: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return scanPlayerWithNulls(rows)
}

func (r *playerRepository) GetByEmail(ctx context.Context, email string) (*models.Player, error) {
	query := `SELECT * FROM players WHERE email = $1 AND deleted_at IS NULL`

	rows, err := r.db.QueryContext(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query player by email: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	return scanPlayerWithNulls(rows)
}

func (r *playerRepository) Update(ctx context.Context, req models.UpdatePlayerRequest) (*models.Player, error) {
	setParts := []string{"updated_at = $2"}
	args := []interface{}{req.ID, time.Now()}
	argIndex := 3

	if req.Email != "" {
		setParts = append(setParts, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, req.Email)
		argIndex++
	}
	if req.FirstName != "" {
		setParts = append(setParts, fmt.Sprintf("first_name = $%d", argIndex))
		args = append(args, req.FirstName)
		argIndex++
	}
	if req.LastName != "" {
		setParts = append(setParts, fmt.Sprintf("last_name = $%d", argIndex))
		args = append(args, req.LastName)
		argIndex++
	}
	if !req.DateOfBirth.IsZero() {
		setParts = append(setParts, fmt.Sprintf("date_of_birth = $%d", argIndex))
		args = append(args, req.DateOfBirth)
		argIndex++
	}
	if req.NationalID != "" {
		setParts = append(setParts, fmt.Sprintf("national_id = $%d", argIndex))
		args = append(args, req.NationalID)
		argIndex++
	}
	if req.MobileMoneyPhone != "" {
		setParts = append(setParts, fmt.Sprintf("mobile_money_phone = $%d", argIndex))
		args = append(args, req.MobileMoneyPhone)
		argIndex++
	}
	if req.Status != "" {
		setParts = append(setParts, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, req.Status)
		argIndex++
	}
	if !req.LastLoginAt.IsZero() {
		setParts = append(setParts, fmt.Sprintf("last_login_at = $%d", argIndex))
		args = append(args, req.LastLoginAt)
		argIndex++
	}

	if len(setParts) == 1 {
		return nil, fmt.Errorf("no fields to update")
	}

	query := fmt.Sprintf(`
		UPDATE players 
		SET %s 
		WHERE id = $1 AND deleted_at IS NULL 
		RETURNING *
	`, strings.Join(setParts, ", "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update player: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("player not found")
	}

	return scanPlayerWithNulls(rows)
}

func (r *playerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM players WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete player: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

func (r *playerRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE players SET deleted_at = $2 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to soft delete player: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

func (r *playerRepository) List(ctx context.Context, filter models.PlayerFilter) ([]*models.Player, error) {
	query := `SELECT * FROM players WHERE deleted_at IS NULL`
	args := []interface{}{}
	argIndex := 1

	if filter.PhoneNumber != "" {
		query += fmt.Sprintf(" AND phone_number = $%d", argIndex)
		args = append(args, filter.PhoneNumber)
		argIndex++
	}
	if filter.Email != "" {
		query += fmt.Sprintf(" AND email = $%d", argIndex)
		args = append(args, filter.Email)
		argIndex++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, filter.Status)
		argIndex++
	}
	if filter.Channel != "" {
		query += fmt.Sprintf(" AND registration_channel = $%d", argIndex)
		args = append(args, filter.Channel)
		argIndex++
	}
	if !filter.CreatedFrom.IsZero() {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, filter.CreatedFrom)
		argIndex++
	}
	if !filter.CreatedTo.IsZero() {
		query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, filter.CreatedTo)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list players: %w", err)
	}
	defer rows.Close()

	var players []*models.Player
	for rows.Next() {
		player, err := scanPlayerWithNulls(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan player: %w", err)
		}
		players = append(players, player)
	}

	return players, nil
}

func (r *playerRepository) Count(ctx context.Context, filter models.PlayerFilter) (int64, error) {
	query := `SELECT COUNT(*) FROM players WHERE deleted_at IS NULL`
	args := []interface{}{}
	argIndex := 1

	if filter.PhoneNumber != "" {
		query += fmt.Sprintf(" AND phone_number = $%d", argIndex)
		args = append(args, filter.PhoneNumber)
		argIndex++
	}
	if filter.Email != "" {
		query += fmt.Sprintf(" AND email = $%d", argIndex)
		args = append(args, filter.Email)
		argIndex++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, filter.Status)
		argIndex++
	}
	if filter.Channel != "" {
		query += fmt.Sprintf(" AND registration_channel = $%d", argIndex)
		args = append(args, filter.Channel)
		argIndex++
	}
	if !filter.CreatedFrom.IsZero() {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, filter.CreatedFrom)
		argIndex++
	}
	if !filter.CreatedTo.IsZero() {
		query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, filter.CreatedTo)
		argIndex++
	}

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count players: %w", err)
	}

	return count, nil
}

func (r *playerRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE players SET last_login_at = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, id, now, now)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

func (r *playerRepository) SetEmailVerified(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE players SET email_verified = true, updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set email verified: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

func (r *playerRepository) SetPhoneVerified(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE players SET phone_verified = true, updated_at = $2 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set phone verified: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

// Search searches for players by phone number, email, first name, or last name
func (r *playerRepository) Search(ctx context.Context, query string, limit, offset int) ([]*models.Player, error) {
	searchQuery := `
		SELECT * FROM players 
		WHERE deleted_at IS NULL 
		AND (
			phone_number ILIKE $1 
			OR email ILIKE $1 
			OR first_name ILIKE $1 
			OR last_name ILIKE $1
			OR CONCAT(first_name, ' ', last_name) ILIKE $1
		)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	searchPattern := "%" + query + "%"
	rows, err := r.db.QueryContext(ctx, searchQuery, searchPattern, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search players: %w", err)
	}
	defer rows.Close()

	var players []*models.Player
	for rows.Next() {
		player, err := scanPlayerWithNulls(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan player: %w", err)
		}
		players = append(players, player)
	}

	return players, nil
}

// Suspend suspends a player by setting their status to SUSPENDED
func (r *playerRepository) Suspend(ctx context.Context, id uuid.UUID, reason string) error {
	query := `UPDATE players SET status = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id, models.PlayerStatusSuspended, time.Now())
	if err != nil {
		return fmt.Errorf("failed to suspend player: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

// Activate activates a player by setting their status to ACTIVE
func (r *playerRepository) Activate(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE players SET status = $2, updated_at = $3 WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, id, models.PlayerStatusActive, time.Now())
	if err != nil {
		return fmt.Errorf("failed to activate player: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}

func (r *playerRepository) VerifyPhoneNumber(ctx context.Context, phoneNumber string) error {
	query := `UPDATE players SET phone_verified = true, updated_at = $2 WHERE phone_number = $1 AND deleted_at IS NULL`

	result, err := r.db.ExecContext(ctx, query, phoneNumber, time.Now())
	if err != nil {
		return fmt.Errorf("failed to verify phone number: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("player not found")
	}

	return nil
}
