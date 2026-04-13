package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
)

// AuditLogRepository defines the interface for audit log data operations
type AuditLogRepository interface {
	Create(ctx context.Context, log *models.AuditLog) error
	List(ctx context.Context, filter models.AuditLogFilter) ([]*models.AuditLog, int, error)
}

type auditLogRepository struct {
	db *sql.DB
}

// NewAuditLogRepository creates a new instance of AuditLogRepository
func NewAuditLogRepository(db *sql.DB) AuditLogRepository {
	return &auditLogRepository{db: db}
}

func (r *auditLogRepository) Create(ctx context.Context, log *models.AuditLog) error {
	// Convert request data to JSON
	requestDataJSON, err := json.Marshal(log.RequestData)
	if err != nil {
		return fmt.Errorf("failed to marshal request data: %w", err)
	}

	query := `
		INSERT INTO admin_audit_logs (
			id, admin_user_id, action, resource, resource_id,
			ip_address, user_agent, request_data, response_status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	logID := uuid.New()
	log.ID = logID
	log.CreatedAt = time.Now()

	_, err = r.db.ExecContext(ctx, query,
		logID,
		log.AdminUserID,
		log.Action,
		log.Resource,
		log.ResourceID,
		log.IPAddress,
		log.UserAgent,
		requestDataJSON,
		log.ResponseStatus,
		log.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

func (r *auditLogRepository) List(ctx context.Context, filter models.AuditLogFilter) ([]*models.AuditLog, int, error) {
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	// Apply filters
	if filter.UserID != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("al.admin_user_id = $%d", argIndex))
		args = append(args, *filter.UserID)
		argIndex++
	}
	if filter.Action != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("al.action = $%d", argIndex))
		args = append(args, *filter.Action)
		argIndex++
	}
	if filter.Resource != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("al.resource = $%d", argIndex))
		args = append(args, *filter.Resource)
		argIndex++
	}
	if filter.StartDate != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("al.created_at >= $%d", argIndex))
		args = append(args, *filter.StartDate)
		argIndex++
	}
	if filter.EndDate != nil {
		whereConditions = append(whereConditions, fmt.Sprintf("al.created_at <= $%d", argIndex))
		args = append(args, *filter.EndDate)
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Count total records
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM admin_audit_logs al
		LEFT JOIN admin_users au ON al.admin_user_id = au.id
		%s
	`, whereClause)
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Apply pagination
	if filter.PageSize <= 0 {
		filter.PageSize = 50
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}

	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)

	// Main query with join to get admin user details
	query := fmt.Sprintf(`
		SELECT 
			al.id, al.admin_user_id, al.action, al.resource, al.resource_id,
			al.ip_address, al.user_agent, al.request_data, al.response_status, al.created_at,
			au.email, au.username, au.first_name, au.last_name
		FROM admin_audit_logs al
		LEFT JOIN admin_users au ON al.admin_user_id = au.id
		%s
		ORDER BY al.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list audit logs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		var requestDataJSON sql.NullString
		var userEmail, username sql.NullString
		var firstName, lastName sql.NullString

		err := rows.Scan(
			&log.ID,
			&log.AdminUserID,
			&log.Action,
			&log.Resource,
			&log.ResourceID,
			&log.IPAddress,
			&log.UserAgent,
			&requestDataJSON,
			&log.ResponseStatus,
			&log.CreatedAt,
			&userEmail,
			&username,
			&firstName,
			&lastName,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}

		// Parse request data JSON if present
		if requestDataJSON.Valid && requestDataJSON.String != "" {
			err = json.Unmarshal([]byte(requestDataJSON.String), &log.RequestData)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal request data: %w", err)
			}
		}

		// Populate admin user details if available
		if userEmail.Valid {
			log.AdminUser = &models.AdminUser{
				Email:    userEmail.String,
				Username: username.String,
			}
			log.AdminUser.ID = log.AdminUserID
			if firstName.Valid {
				log.AdminUser.FirstName = &firstName.String
			}
			if lastName.Valid {
				log.AdminUser.LastName = &lastName.String
			}
		}

		logs = append(logs, log)
	}

	return logs, totalCount, nil
}
