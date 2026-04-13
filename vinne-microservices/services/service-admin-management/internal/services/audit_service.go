package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuditService handles audit log operations
type AuditService interface {
	GetAuditLogs(ctx context.Context, req *adminmanagementv1.GetAuditLogsRequest) (*adminmanagementv1.GetAuditLogsResponse, error)
}

type auditService struct {
	auditRepo repositories.AuditLogRepository
}

// NewAuditService creates a new audit service
func NewAuditService(auditRepo repositories.AuditLogRepository) AuditService {
	return &auditService{
		auditRepo: auditRepo,
	}
}

func (s *auditService) GetAuditLogs(ctx context.Context, req *adminmanagementv1.GetAuditLogsRequest) (*adminmanagementv1.GetAuditLogsResponse, error) {
	// Build filter
	filter := models.AuditLogFilter{
		Page:     int(req.Page),
		PageSize: int(req.PageSize),
	}

	if req.UserId != nil && *req.UserId != "" {
		userID, err := uuid.Parse(*req.UserId)
		if err != nil {
			return nil, errors.NewBadRequestError("invalid user ID")
		}
		filter.UserID = &userID
	}

	if req.Action != nil && *req.Action != "" {
		filter.Action = req.Action
	}

	if req.Resource != nil && *req.Resource != "" {
		filter.Resource = req.Resource
	}

	if req.StartDate != nil {
		startTime := req.StartDate.AsTime()
		filter.StartDate = &startTime
	}

	if req.EndDate != nil {
		endTime := req.EndDate.AsTime()
		filter.EndDate = &endTime
	}

	// Get audit logs
	logs, total, err := s.auditRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}

	// Convert to proto
	protoLogs := make([]*adminmanagementv1.AuditLog, len(logs))
	for i, log := range logs {
		protoLogs[i] = modelAuditLogToProto(log)
	}

	return &adminmanagementv1.GetAuditLogsResponse{
		Logs:       protoLogs,
		TotalCount: int32(total),
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: int32((total + int(req.PageSize) - 1) / int(req.PageSize)),
	}, nil
}

// Helper function to convert model to proto
func modelAuditLogToProto(log *models.AuditLog) *adminmanagementv1.AuditLog {
	if log == nil {
		return nil
	}

	protoLog := &adminmanagementv1.AuditLog{
		Id:             log.ID.String(),
		AdminUserId:    log.AdminUserID.String(),
		Action:         log.Action,
		Resource:       log.Resource,
		ResourceId:     log.ResourceID,
		IpAddress:      log.IPAddress,
		UserAgent:      log.UserAgent,
		ResponseStatus: int32(log.ResponseStatus),
		CreatedAt:      timestamppb.New(log.CreatedAt),
	}

	// Convert request data to JSON string
	if log.RequestData != nil {
		requestJSON, err := json.Marshal(log.RequestData)
		if err == nil {
			protoLog.RequestData = string(requestJSON)
		}
	}

	return protoLog
}
