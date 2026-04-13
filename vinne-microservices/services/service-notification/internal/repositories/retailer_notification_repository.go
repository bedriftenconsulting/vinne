package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/database"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
)

// RetailerNotificationRepository defines the interface for retailer notification data operations
type RetailerNotificationRepository interface {
	Create(ctx context.Context, req *models.CreateRetailerNotificationRequest) (*models.RetailerNotification, error)
	GetByID(ctx context.Context, id string) (*models.RetailerNotification, error)
	List(ctx context.Context, filter *models.RetailerNotificationFilter) ([]*models.RetailerNotification, int, error)
	MarkAsRead(ctx context.Context, retailerID string, notificationID string) error
	MarkAllAsRead(ctx context.Context, retailerID string) error
	GetUnreadCount(ctx context.Context, retailerID string) (int, error)
}

type retailerNotificationRepository struct {
	db database.DBInterface
}

// NewRetailerNotificationRepository creates a new retailer notification repository
func NewRetailerNotificationRepository(rawDB *sql.DB) RetailerNotificationRepository {
	return &retailerNotificationRepository{db: database.NewTracedDBInterface(rawDB)}
}

// Create creates a new retailer notification
func (r *retailerNotificationRepository) Create(ctx context.Context, req *models.CreateRetailerNotificationRequest) (*models.RetailerNotification, error) {
	query := `
		INSERT INTO retailer_notifications (
			retailer_id, type, title, body, amount, transaction_id, notification_id, is_read
		) VALUES ($1, $2, $3, $4, $5, $6, $7, false)
		RETURNING id, retailer_id, type, title, body, amount, transaction_id, is_read, read_at, notification_id, created_at, updated_at
	`

	var notification models.RetailerNotification
	var amount sql.NullInt64
	var transactionID, notificationID sql.NullString
	var readAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query,
		req.RetailerID, req.Type, req.Title, req.Body,
		req.Amount, req.TransactionID, req.NotificationID).
		Scan(&notification.ID, &notification.RetailerID, &notification.Type, &notification.Title, &notification.Body,
			&amount, &transactionID, &notification.IsRead, &readAt, &notificationID, &notification.CreatedAt, &notification.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create retailer notification: %w", err)
	}

	if amount.Valid {
		notification.Amount = &amount.Int64
	}
	if transactionID.Valid {
		notification.TransactionID = &transactionID.String
	}
	if notificationID.Valid {
		notification.NotificationID = &notificationID.String
	}
	if readAt.Valid {
		notification.ReadAt = &readAt.Time
	}

	return &notification, nil
}

// GetByID retrieves a retailer notification by ID
func (r *retailerNotificationRepository) GetByID(ctx context.Context, id string) (*models.RetailerNotification, error) {
	query := `
		SELECT id, retailer_id, type, title, body, amount, transaction_id, is_read, read_at, notification_id, created_at, updated_at
		FROM retailer_notifications
		WHERE id = $1
	`

	var notification models.RetailerNotification
	var amount sql.NullInt64
	var transactionID, notificationID sql.NullString
	var readAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&notification.ID, &notification.RetailerID, &notification.Type, &notification.Title, &notification.Body,
			&amount, &transactionID, &notification.IsRead, &readAt, &notificationID, &notification.CreatedAt, &notification.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get retailer notification: %w", err)
	}

	if amount.Valid {
		notification.Amount = &amount.Int64
	}
	if transactionID.Valid {
		notification.TransactionID = &transactionID.String
	}
	if notificationID.Valid {
		notification.NotificationID = &notificationID.String
	}
	if readAt.Valid {
		notification.ReadAt = &readAt.Time
	}

	return &notification, nil
}

// List retrieves retailer notifications with filtering and pagination
func (r *retailerNotificationRepository) List(ctx context.Context, filter *models.RetailerNotificationFilter) ([]*models.RetailerNotification, int, error) {
	// Build query conditions
	conditions := []string{"retailer_id = $1"}
	args := []interface{}{filter.RetailerID}
	argCount := 1

	if filter.Type != "" {
		argCount++
		conditions = append(conditions, fmt.Sprintf("type = $%d", argCount))
		args = append(args, filter.Type)
	}

	if filter.UnreadOnly {
		conditions = append(conditions, "is_read = false")
	}

	whereClause := strings.Join(conditions, " AND ")

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM retailer_notifications WHERE %s", whereClause)
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	// Get paginated results
	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)

	listQuery := fmt.Sprintf(`
		SELECT id, retailer_id, type, title, body, amount, transaction_id, is_read, read_at, notification_id, created_at, updated_at
		FROM retailer_notifications
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argCount+1, argCount+2)

	rows, err := r.db.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list retailer notifications: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore error in defer cleanup
	}()

	var notifications []*models.RetailerNotification
	for rows.Next() {
		var notification models.RetailerNotification
		var amount sql.NullInt64
		var transactionID, notificationID sql.NullString
		var readAt sql.NullTime

		if err := rows.Scan(&notification.ID, &notification.RetailerID, &notification.Type, &notification.Title, &notification.Body,
			&amount, &transactionID, &notification.IsRead, &readAt, &notificationID, &notification.CreatedAt, &notification.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("failed to scan retailer notification: %w", err)
		}

		if amount.Valid {
			notification.Amount = &amount.Int64
		}
		if transactionID.Valid {
			notification.TransactionID = &transactionID.String
		}
		if notificationID.Valid {
			notification.NotificationID = &notificationID.String
		}
		if readAt.Valid {
			notification.ReadAt = &readAt.Time
		}

		notifications = append(notifications, &notification)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating notification rows: %w", err)
	}

	return notifications, totalCount, nil
}

// MarkAsRead marks a notification as read
func (r *retailerNotificationRepository) MarkAsRead(ctx context.Context, retailerID string, notificationID string) error {
	query := `
		UPDATE retailer_notifications
		SET is_read = true, read_at = $3, updated_at = NOW()
		WHERE retailer_id = $1 AND id = $2 AND is_read = false
	`

	result, err := r.db.ExecContext(ctx, query, retailerID, notificationID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("notification not found or already read")
	}

	return nil
}

// MarkAllAsRead marks all unread notifications for a retailer as read
func (r *retailerNotificationRepository) MarkAllAsRead(ctx context.Context, retailerID string) error {
	query := `
		UPDATE retailer_notifications
		SET is_read = true, read_at = $2, updated_at = NOW()
		WHERE retailer_id = $1 AND is_read = false
	`

	_, err := r.db.ExecContext(ctx, query, retailerID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	return nil
}

// GetUnreadCount returns the count of unread notifications for a retailer
func (r *retailerNotificationRepository) GetUnreadCount(ctx context.Context, retailerID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM retailer_notifications
		WHERE retailer_id = $1 AND is_read = false
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, retailerID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}

	return count, nil
}
