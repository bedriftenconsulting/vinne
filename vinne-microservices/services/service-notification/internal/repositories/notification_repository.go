package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/randco/randco-microservices/services/service-notification/internal/database"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type NotificationRepository interface {
	Create(ctx context.Context, notification *models.Notification) error
	CreateMany(ctx context.Context, notifications []*models.Notification) error
	GetByID(ctx context.Context, id string) (*models.Notification, error)
	Update(ctx context.Context, notification *models.Notification) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter models.NotificationFilter, page, limit int) ([]*models.Notification, int64, error)
	GetByStatus(ctx context.Context, status models.NotificationStatus) ([]*models.Notification, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*models.Notification, error)
}

type notificationRepository struct {
	db database.TracedDBInterface
}

func NewNotificationRepository(db *sql.DB) NotificationRepository {
	return &notificationRepository{
		db: database.NewTracedDBInterface(db),
	}
}

func (r *notificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "db.notification.create")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.table", "notifications"),
		attribute.String("notification.type", string(notification.Type)),
		attribute.String("notification.status", string(notification.Status)),
	)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to begin transaction")
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Ignore error - will fail if transaction is committed
	}()

	variablesJSON, err := json.Marshal(notification.Variables)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal variables: %w", err)
	}

	query := `
		INSERT INTO notifications (id, idempotency_key, type, subject, content, 
		status, provider, scheduled_for, variables, template_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at`

	err = tx.QueryRowContext(ctx, query,
		notification.ID, notification.IdempotencyKey, notification.Type, notification.Subject,
		notification.Content, notification.Status, notification.Provider, notification.ScheduledFor,
		variablesJSON, notification.TemplateID,
	).Scan(&notification.CreatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to insert notification")
		return fmt.Errorf("failed to create notification: %w", err)
	}

	for i := range notification.Recipient {
		err = r.createRecipient(ctx, tx, notification.ID, "to", &notification.Recipient[i])
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to create recipient: %w", err)
		}
	}

	for i := range notification.CC {
		err = r.createRecipient(ctx, tx, notification.ID, "cc", &notification.CC[i])
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to create cc recipient: %w", err)
		}
	}

	for i := range notification.BCC {
		err = r.createRecipient(ctx, tx, notification.ID, "bcc", &notification.BCC[i])
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to create bcc recipient: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	span.SetAttributes(attribute.String("notification.id", notification.ID))
	return nil
}

func (r *notificationRepository) CreateMany(ctx context.Context, notifications []*models.Notification) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "db.notification.create_many")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "BATCH_INSERT"),
		attribute.String("db.table", "notifications"),
		attribute.Int("batch.size", len(notifications)),
	)

	if len(notifications) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to begin transaction")
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Ignore error - will fail if transaction is committed
	}()

	var notificationValues []string
	var notificationArgs []interface{}

	for i, notification := range notifications {
		var variablesJSON interface{}
		if len(notification.Variables) > 0 {
			jsonBytes, err := json.Marshal(notification.Variables)
			if err != nil {
				span.RecordError(err)
				return fmt.Errorf("failed to marshal variables: %w", err)
			}
			variablesJSON = string(jsonBytes)
		} else {
			variablesJSON = nil
		}

		var templateID interface{}
		if notification.TemplateID != "" {
			templateID = notification.TemplateID
		} else {
			templateID = nil
		}

		argBase := i * 10
		notificationValues = append(notificationValues, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			argBase+1, argBase+2, argBase+3, argBase+4, argBase+5, argBase+6, argBase+7, argBase+8, argBase+9, argBase+10))

		notificationArgs = append(notificationArgs,
			notification.ID,
			notification.IdempotencyKey,
			notification.Type,
			notification.Subject,
			notification.Content,
			notification.Status,
			notification.Provider,
			notification.ScheduledFor,
			variablesJSON,
			templateID,
		)
	}

	// Insert all notifications
	notificationQuery := fmt.Sprintf(`
		INSERT INTO notifications (id, idempotency_key, type, subject, content, 
		status, provider, scheduled_for, variables, template_id)
		VALUES %s`, strings.Join(notificationValues, ", "))

	_, err = tx.ExecContext(ctx, notificationQuery, notificationArgs...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to bulk insert notifications")
		return fmt.Errorf("failed to bulk insert notifications: %w", err)
	}

	var recipientValues []string
	var recipientArgs []interface{}
	recipientArgIndex := 1

	for _, notification := range notifications {
		for _, recipient := range notification.Recipient {
			recipientValues = append(recipientValues, fmt.Sprintf("($%d, $%d, $%d)", recipientArgIndex, recipientArgIndex+1, recipientArgIndex+2))
			recipientArgs = append(recipientArgs, notification.ID, "to", recipient.Address)
			recipientArgIndex += 3
		}

		for _, recipient := range notification.CC {
			recipientValues = append(recipientValues, fmt.Sprintf("($%d, $%d, $%d)", recipientArgIndex, recipientArgIndex+1, recipientArgIndex+2))
			recipientArgs = append(recipientArgs, notification.ID, "cc", recipient.Address)
			recipientArgIndex += 3
		}

		for _, recipient := range notification.BCC {
			recipientValues = append(recipientValues, fmt.Sprintf("($%d, $%d, $%d)", recipientArgIndex, recipientArgIndex+1, recipientArgIndex+2))
			recipientArgs = append(recipientArgs, notification.ID, "bcc", recipient.Address)
			recipientArgIndex += 3
		}
	}

	if len(recipientValues) > 0 {
		recipientQuery := fmt.Sprintf(`
			INSERT INTO recipients (notification_id, type, address)
			VALUES %s`, strings.Join(recipientValues, ", "))

		_, err = tx.ExecContext(ctx, recipientQuery, recipientArgs...)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to bulk insert recipients")
			return fmt.Errorf("failed to bulk insert recipients: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	now := time.Now()
	for _, notification := range notifications {
		notification.CreatedAt = now
		notification.UpdatedAt = now
	}

	span.SetAttributes(attribute.Int("notifications.created", len(notifications)))
	return nil
}

func (r *notificationRepository) createRecipient(ctx context.Context, tx database.TracedTxInterface, notificationID, recipientType string, recipient *models.Recipient) error {
	query := `
		INSERT INTO recipients (notification_id, type, address)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	return tx.QueryRowContext(ctx, query, notificationID, recipientType, recipient.Address).
		Scan(&recipient.ID, &recipient.CreatedAt)
}

func (r *notificationRepository) GetByID(ctx context.Context, id string) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "db.notification.get_by_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "notifications"),
		attribute.String("notification.id", id),
	)

	query := `
		SELECT id, idempotency_key, type, subject, content, status, provider,
			provider_message_id, provider_response, retry_count, scheduled_for,
			sent_at, delivered_at, failed_at, error_message, variables, template_id, created_at, updated_at
		FROM notifications
		WHERE id = $1`

	notification := &models.Notification{}
	var providerResponseJSON []byte
	var variablesJSON []byte
	var provider sql.NullString
	var templateID sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&notification.ID, &notification.IdempotencyKey, &notification.Type,
		&notification.Subject, &notification.Content, &notification.Status,
		&provider, &notification.ProviderMessageID, &providerResponseJSON,
		&notification.RetryCount, &notification.ScheduledFor, &notification.SentAt,
		&notification.DeliveredAt, &notification.FailedAt, &notification.ErrorMessage,
		&variablesJSON, &templateID, &notification.CreatedAt, &notification.UpdatedAt,
	)

	if err == nil {
		if len(providerResponseJSON) > 0 {
			if err := json.Unmarshal(providerResponseJSON, &notification.ProviderResponse); err != nil {
				span.RecordError(err)
				return nil, fmt.Errorf("failed to unmarshal provider response: %w", err)
			}
		}

		if len(variablesJSON) > 0 {
			if err := json.Unmarshal(variablesJSON, &notification.Variables); err != nil {
				span.RecordError(err)
				return nil, fmt.Errorf("failed to unmarshal variables: %w", err)
			}
		}

		if templateID.Valid {
			notification.TemplateID = templateID.String
		}

		if provider.Valid {
			notification.Provider = provider.String
		}
	}

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Error, "notification not found")
			return nil, fmt.Errorf("notification not found: %w", err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get notification")
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	recipients, err := r.getRecipients(ctx, id)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get recipients: %w", err)
	}

	for _, recipient := range recipients {
		switch recipient.Type {
		case "to":
			notification.Recipient = append(notification.Recipient, *recipient)
		case "cc":
			notification.CC = append(notification.CC, *recipient)
		case "bcc":
			notification.BCC = append(notification.BCC, *recipient)
		}
	}

	if provider.Valid {
		notification.Provider = provider.String
	}

	span.SetAttributes(
		attribute.Bool("notification.found", true),
		attribute.String("notification.type", string(notification.Type)),
	)
	return notification, nil
}

func (r *notificationRepository) getRecipients(ctx context.Context, notificationID string) ([]*models.Recipient, error) {
	query := `
		SELECT id, notification_id, type, address, created_at
		FROM recipients
		WHERE notification_id = $1
		ORDER BY id`

	rows, err := r.db.QueryContext(ctx, query, notificationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query recipients: %w", err)
	}
	defer func() {
		_ = rows.Close() // Ignore error in defer cleanup
	}()

	var recipients []*models.Recipient
	for rows.Next() {
		recipient := &models.Recipient{}
		err := rows.Scan(
			&recipient.ID, &recipient.NotificationID, &recipient.Type,
			&recipient.Address, &recipient.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recipient: %w", err)
		}
		recipients = append(recipients, recipient)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recipients: %w", err)
	}

	return recipients, nil
}

func (r *notificationRepository) Update(ctx context.Context, notification *models.Notification) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "db.notification.update")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.table", "notifications"),
		attribute.String("notification.id", notification.ID),
	)

	providerResponseJSON, err := json.Marshal(notification.ProviderResponse)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal provider response: %w", err)
	}

	variablesJSON, err := json.Marshal(notification.Variables)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to marshal variables: %w", err)
	}

	query := `
		UPDATE notifications SET
			idempotency_key = $2, type = $3, subject = $4, content = $5, status = $6,
			provider = $7, provider_message_id = $8, provider_response = $9, retry_count = $10,
			scheduled_for = $11, sent_at = $12, delivered_at = $13, failed_at = $14,
			error_message = $15, variables = $16, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err = r.db.QueryRowContext(ctx, query,
		notification.ID, notification.IdempotencyKey, notification.Type, notification.Subject,
		notification.Content, notification.Status, notification.Provider, notification.ProviderMessageID,
		providerResponseJSON, notification.RetryCount, notification.ScheduledFor,
		notification.SentAt, notification.DeliveredAt, notification.FailedAt, notification.ErrorMessage,
		variablesJSON,
	).Scan(&notification.UpdatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update notification")
		return fmt.Errorf("failed to update notification: %w", err)
	}

	return nil
}

func (r *notificationRepository) Delete(ctx context.Context, id string) error {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "db.notification.delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "DELETE"),
		attribute.String("db.table", "notifications"),
		attribute.String("notification.id", id),
	)

	query := `DELETE FROM notifications WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete notification")
		return fmt.Errorf("failed to delete notification: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		span.SetStatus(codes.Error, "notification not found")
		return fmt.Errorf("notification not found")
	}

	return nil
}

func (r *notificationRepository) List(ctx context.Context, filter models.NotificationFilter, page, limit int) ([]*models.Notification, int64, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "db.notification.list")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "notifications"),
		attribute.Int("page", page),
		attribute.Int("limit", limit),
	)

	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIndex))
		args = append(args, string(filter.Type))
		argIndex++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, string(filter.Status))
		argIndex++
	}

	if filter.Provider != "" {
		conditions = append(conditions, fmt.Sprintf("provider = $%d", argIndex))
		args = append(args, filter.Provider)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM notifications %s", whereClause)
	var total int64
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	offset := (page - 1) * limit
	dataQuery := fmt.Sprintf(`
		SELECT id, idempotency_key, type, subject, content, status, provider,
			provider_message_id, provider_response, retry_count, scheduled_for,
			sent_at, delivered_at, failed_at, error_message, variables, template_id, created_at, updated_at
		FROM notifications %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var notifications []*models.Notification
	for rows.Next() {
		notification := &models.Notification{}
		var providerResponseJSON []byte
		var variablesJSON []byte
		var templateID sql.NullString

		err := rows.Scan(
			&notification.ID, &notification.IdempotencyKey, &notification.Type,
			&notification.Subject, &notification.Content, &notification.Status,
			&notification.Provider, &notification.ProviderMessageID, &providerResponseJSON,
			&notification.RetryCount, &notification.ScheduledFor, &notification.SentAt,
			&notification.DeliveredAt, &notification.FailedAt, &notification.ErrorMessage,
			&variablesJSON, &templateID, &notification.CreatedAt, &notification.UpdatedAt,
		)
		if err != nil {
			span.RecordError(err)
			return nil, 0, fmt.Errorf("failed to scan notification: %w", err)
		}

		if len(providerResponseJSON) > 0 {
			if err := json.Unmarshal(providerResponseJSON, &notification.ProviderResponse); err != nil {
				span.RecordError(err)
				return nil, 0, fmt.Errorf("failed to unmarshal provider response: %w", err)
			}
		}

		if len(variablesJSON) > 0 {
			if err := json.Unmarshal(variablesJSON, &notification.Variables); err != nil {
				span.RecordError(err)
				return nil, 0, fmt.Errorf("failed to unmarshal variables: %w", err)
			}
		}

		if templateID.Valid {
			notification.TemplateID = templateID.String
		}

		recipients, err := r.getRecipients(ctx, notification.ID)
		if err != nil {
			span.RecordError(err)
			return nil, 0, fmt.Errorf("failed to get recipients: %w", err)
		}

		for _, recipient := range recipients {
			switch recipient.Type {
			case "to":
				notification.Recipient = append(notification.Recipient, *recipient)
			case "cc":
				notification.CC = append(notification.CC, *recipient)
			case "bcc":
				notification.BCC = append(notification.BCC, *recipient)
			}
		}

		notifications = append(notifications, notification)
	}

	if err = rows.Err(); err != nil {
		span.RecordError(err)
		return nil, 0, fmt.Errorf("error iterating notifications: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(notifications)))
	return notifications, total, nil
}

func (r *notificationRepository) GetByStatus(ctx context.Context, status models.NotificationStatus) ([]*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "db.notification.get_by_status")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "notifications"),
		attribute.String("notification.status", string(status)),
	)

	query := `
		SELECT id, idempotency_key, type, subject, content, status, provider,
			provider_message_id, provider_response, retry_count, scheduled_for,
			sent_at, delivered_at, failed_at, error_message, variables, created_at, updated_at
		FROM notifications
		WHERE status = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query, string(status))
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to query notifications by status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var notifications []*models.Notification
	for rows.Next() {
		notification := &models.Notification{}
		var providerResponseJSON []byte
		var variablesJSON []byte

		err := rows.Scan(
			&notification.ID, &notification.IdempotencyKey, &notification.Type,
			&notification.Subject, &notification.Content, &notification.Status,
			&notification.Provider, &notification.ProviderMessageID, &providerResponseJSON,
			&notification.RetryCount, &notification.ScheduledFor, &notification.SentAt,
			&notification.DeliveredAt, &notification.FailedAt, &notification.ErrorMessage,
			&variablesJSON, &notification.CreatedAt, &notification.UpdatedAt,
		)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to scan notification: %w", err)
		}

		if len(providerResponseJSON) > 0 {
			if err := json.Unmarshal(providerResponseJSON, &notification.ProviderResponse); err != nil {
				span.RecordError(err)
				return nil, fmt.Errorf("failed to unmarshal provider response: %w", err)
			}
		}

		if len(variablesJSON) > 0 {
			if err := json.Unmarshal(variablesJSON, &notification.Variables); err != nil {
				span.RecordError(err)
				return nil, fmt.Errorf("failed to unmarshal variables: %w", err)
			}
		}

		recipients, err := r.getRecipients(ctx, notification.ID)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to get recipients: %w", err)
		}

		for _, recipient := range recipients {
			switch recipient.Type {
			case "to":
				notification.Recipient = append(notification.Recipient, *recipient)
			case "cc":
				notification.CC = append(notification.CC, *recipient)
			case "bcc":
				notification.BCC = append(notification.BCC, *recipient)
			}
		}

		notifications = append(notifications, notification)
	}

	if err = rows.Err(); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("error iterating notifications: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(notifications)))
	return notifications, nil
}

func (r *notificationRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Notification, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().
		Tracer("service-notification").Start(ctx, "db.notification.get_by_idempotency_key")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "notifications"),
		attribute.String("notification.idempotency_key", key),
	)

	query := `
		SELECT id, idempotency_key, type, subject, content, status, provider,
			provider_message_id, provider_response, retry_count, scheduled_for,
			sent_at, delivered_at, failed_at, error_message, created_at, updated_at
		FROM notifications
		WHERE idempotency_key = $1`

	notification := &models.Notification{}
	var providerResponseJSON []byte

	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&notification.ID, &notification.IdempotencyKey, &notification.Type,
		&notification.Subject, &notification.Content, &notification.Status,
		&notification.Provider, &notification.ProviderMessageID, &providerResponseJSON,
		&notification.RetryCount, &notification.ScheduledFor, &notification.SentAt,
		&notification.DeliveredAt, &notification.FailedAt, &notification.ErrorMessage,
		&notification.CreatedAt, &notification.UpdatedAt,
	)

	if err == nil {
		if len(providerResponseJSON) > 0 {
			if err := json.Unmarshal(providerResponseJSON, &notification.ProviderResponse); err != nil {
				span.RecordError(err)
				return nil, fmt.Errorf("failed to unmarshal provider response: %w", err)
			}
		}
	}

	if err != nil {
		if err == sql.ErrNoRows {
			span.SetStatus(codes.Error, "notification not found")
			return nil, fmt.Errorf("notification not found: %w", err)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get notification")
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	recipients, err := r.getRecipients(ctx, notification.ID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get recipients: %w", err)
	}

	for _, recipient := range recipients {
		switch recipient.Type {
		case "to":
			notification.Recipient = append(notification.Recipient, *recipient)
		case "cc":
			notification.CC = append(notification.CC, *recipient)
		case "bcc":
			notification.BCC = append(notification.BCC, *recipient)
		}
	}

	span.SetAttributes(
		attribute.Bool("notification.found", true),
		attribute.String("notification.type", string(notification.Type)),
	)
	return notification, nil
}
