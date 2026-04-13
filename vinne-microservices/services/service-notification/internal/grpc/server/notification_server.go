package server

import (
	"context"
	"fmt"
	"time"

	pb "github.com/randco/randco-microservices/proto/notification/v1"
	"github.com/randco/randco-microservices/services/service-notification/internal/metrics"
	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/queue"
	"github.com/randco/randco-microservices/services/service-notification/internal/services"
	"github.com/randco/randco-microservices/shared/idempotency"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NotificationServer struct {
	pb.UnimplementedNotificationServiceServer
	sendNotificationService     services.SendNotificationService
	notificationService         services.NotificationService
	retailerNotificationService services.RetailerNotificationService
	queueManager                queue.QueueManager
	metrics                     metrics.MetricsInterface
	idempotencyStore            idempotency.IdempotencyStore
}

func NewNotificationServer(sendNotificationService services.SendNotificationService,
	notificationService services.NotificationService,
	retailerNotificationService services.RetailerNotificationService,
	queueManager queue.QueueManager, metrics metrics.MetricsInterface, idempotencyStore idempotency.IdempotencyStore) *NotificationServer {
	return &NotificationServer{
		sendNotificationService:     sendNotificationService,
		notificationService:         notificationService,
		retailerNotificationService: retailerNotificationService,
		queueManager:                queueManager,
		metrics:                     metrics,
		idempotencyStore:            idempotencyStore,
	}
}

func (s *NotificationServer) HealthCheck(ctx context.Context, req *pb.EmptyRequest) (*pb.HealthCheckResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("grpc.method", "HealthCheck"))

	return &pb.HealthCheckResponse{
		Status: "SERVING",
	}, nil
}

func (s *NotificationServer) SendEmail(ctx context.Context, req *pb.SendEmailRequest) (*pb.SendResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "SendEmail"),
		attribute.String("notification.type", "email"),
		attribute.String("recipient", req.To),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	if req.To == "" {
		return nil, status.Error(codes.InvalidArgument, "recipient email is required")
	}

	if req.IdempotencyKey == "" || req.TemplateId == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency key and template ID are required")
	}

	createReq := convertEmailRequestToCreateNotification(req)
	notification, err := s.sendNotificationService.SendEmail(ctx, createReq)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to send email notification: %v", err))
	}

	s.metrics.RecordGRPCRequest("SendEmail", time.Since(start), len(req.String()), 0, "success")

	return &pb.SendResponse{
		NotificationId: notification.ID,
		Status:         string(notification.Status),
		Message:        "Email notification sent successfully",
		IsDuplicate:    false,
	}, nil
}

func (s *NotificationServer) SendSMS(ctx context.Context, req *pb.SendSMSRequest) (*pb.SendResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "SendSMS"),
		attribute.String("notification.type", "sms"),
		attribute.String("recipient", req.To),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	if req.To == "" {
		return nil, status.Error(codes.InvalidArgument, "recipient phone number is required")
	}

	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency key is required")
	}

	if req.Content == "" && req.TemplateId == "" {
		return nil, status.Error(codes.InvalidArgument, "template ID is required when content is empty")
	}

	createReq := convertSMSRequestToCreateNotification(req)
	notification, err := s.sendNotificationService.SendSMS(ctx, createReq)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to send SMS notification: %v", err))
	}

	s.metrics.RecordGRPCRequest("SendSMS", time.Since(start), len(req.String()), 0, "success")

	return &pb.SendResponse{
		NotificationId: notification.ID,
		Status:         string(notification.Status),
		Message:        "SMS notification queued successfully",
	}, nil
}

func (s *NotificationServer) SendMobilePushNotification(ctx context.Context, req *pb.SendPushNotificationRequest) (*pb.SendResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "SendMobilePushNotification"),
		attribute.String("notification.type", "push"),
		attribute.String("recipient", req.To),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	if req.To == "" {
		return nil, status.Error(codes.InvalidArgument, "device token is required")
	}
	if req.Body == "" && req.TemplateId == "" {
		return nil, status.Error(codes.InvalidArgument, "push notification body or template ID is required")
	}

	createReq := convertPushRequestToCreateNotification(req)
	notification, err := s.sendNotificationService.SendPush(ctx, createReq)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to send push notification: %v", err))
	}

	s.metrics.RecordGRPCRequest("SendMobilePushNotification", time.Since(start), len(req.String()), 0, "success")

	return &pb.SendResponse{
		NotificationId: notification.ID,
		Status:         string(notification.Status),
		Message:        "Push notification queued successfully",
	}, nil
}

func (s *NotificationServer) SendBulkEmail(ctx context.Context, req *pb.SendBulkEmailRequest) (*pb.GenericResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "SendBulkEmail"),
		attribute.Int("bulk.count", len(req.Requests)),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	if len(req.Requests) == 0 {
		span.RecordError(fmt.Errorf("at least one email request is required"))
		return nil, status.Error(codes.InvalidArgument, "at least one email request is required")
	}

	if req.TemplateId == "" {
		span.RecordError(fmt.Errorf("template ID is required"))
		return nil, status.Error(codes.InvalidArgument, "template ID is required")
	}

	// Convert GRPC requests to internal models
	createReq := convertBulkEmailRequestToCreateNotification(req, s.idempotencyStore)
	err := s.sendNotificationService.SendBulkEmail(ctx, createReq)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to send bulk email notifications: %v", err))
	}

	s.metrics.RecordGRPCRequest("SendBulkEmail", time.Since(start), len(req.String()), 0, "success")

	return &pb.GenericResponse{
		Message: "Bulk email notifications are being processed",
	}, nil

}

func (s *NotificationServer) SendBulkSMS(ctx context.Context, req *pb.SendBulkSMSRequest) (*pb.GenericResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "SendBulkSMS"),
		attribute.Int("bulk.count", len(req.Requests)),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	if len(req.Requests) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one SMS request is required")
	}

	if req.TemplateId == "" && req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "either template ID or content is required")
	}

	createReq := convertBulkSMSRequestToCreateNotification(req, s.idempotencyStore)
	err := s.sendNotificationService.SendBulkSMS(ctx, createReq)

	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to send bulk SMS notifications: %v", err))
	}

	s.metrics.RecordGRPCRequest("SendBulkSMS", time.Since(start), len(req.String()), 0, "success")

	return &pb.GenericResponse{
		Message: "Bulk SMS notifications are being processed",
	}, nil
}

func (s *NotificationServer) SendBulkPush(ctx context.Context, req *pb.SendBulkPushRequest) (*pb.GenericResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "SendBulkPush"),
		attribute.Int("bulk.count", len(req.Requests)),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	if len(req.Requests) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one push request is required")
	}

	if req.TemplateId == "" && req.Body == "" {
		return nil, status.Error(codes.InvalidArgument, "either template ID or body is required")
	}

	createReq := convertBulkPushRequestToCreateNotification(req, s.idempotencyStore)
	err := s.sendNotificationService.SendBulkPush(ctx, createReq)

	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to send bulk push notifications: %v", err))
	}

	s.metrics.RecordGRPCRequest("SendBulkPush", time.Since(start), len(req.String()), 0, "success")

	return &pb.GenericResponse{
		Message: "Bulk push notifications are being processed",
	}, nil
}

func (s *NotificationServer) GetNotificationStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.NotificationStatus, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GetNotificationStatus"),
		attribute.String("notification.id", req.NotificationId),
	)

	if req.NotificationId == "" {
		return nil, status.Error(codes.InvalidArgument, "notification ID is required")
	}

	notification, err := s.notificationService.GetNotification(ctx, req.NotificationId)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.NotFound, "notification not found")
	}

	return convertNotificationStatusToProto(notification), nil
}

func (s *NotificationServer) ListNotifications(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "ListNotifications"),
		attribute.Int("page", int(req.Page)),
		attribute.Int("page_size", int(req.PageSize)),
	)

	page := int(req.Page)
	if page <= 0 {
		page = 1
	}

	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 20
	}

	filter := models.NotificationFilter{}
	if req.Status != "" {
		filter.Status = models.NotificationStatus(req.Status)
	}

	notifications, total, err := s.notificationService.ListNotifications(ctx, filter, page, limit)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to list notifications: %v", err))
	}

	var protoNotifications []*pb.Notification
	for _, notification := range notifications {
		protoNotifications = append(protoNotifications, convertNotificationToProto(notification))
	}

	return &pb.ListResponse{
		Notifications: protoNotifications,
		TotalCount:    int32(total),
		Page:          int32(page),
		PageSize:      int32(limit),
	}, nil
}

func (s *NotificationServer) RetryNotification(ctx context.Context, req *pb.RetryRequest) (*pb.SendResponse, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "RetryNotification"),
		attribute.String("notification.id", req.NotificationId),
	)

	if req.NotificationId == "" {
		return nil, status.Error(codes.InvalidArgument, "notification ID is required")
	}

	notification, err := s.notificationService.RetryNotification(ctx, req.NotificationId)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to retry notification: %v", err))
	}

	// Create a background context for the enqueue operation to prevent cancellation
	// when the request context is done
	enqueueCtx := context.Background()

	// Enqueue notification for retry (use actual notification type, not hardcoded SMS)
	if err := s.queueManager.Enqueue(enqueueCtx, &queue.QueueItem{
		ID:      notification.ID,
		Type:    string(notification.Type),
		Channel: string(notification.Type),
		Payload: map[string]interface{}{
			"id":          notification.ID,
			"type":        string(notification.Type),
			"recipient":   notification.Recipient,
			"subject":     notification.Subject,
			"content":     notification.Content,
			"template_id": notification.TemplateID,
			"variables":   notification.Variables,
		},
		ScheduledFor: notification.ScheduledFor,
		CreatedAt:    notification.CreatedAt,
	}); err != nil {
		span.RecordError(err)
		span.AddEvent("failed to enqueue retry notification", trace.WithAttributes(
			attribute.String("notification.id", notification.ID),
			attribute.String("error", err.Error()),
		))
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to enqueue notification for retry: %v", err))
	}

	span.SetAttributes(
		attribute.String("notification.type", string(notification.Type)),
		attribute.Bool("retry.enqueued", true),
	)

	return &pb.SendResponse{
		NotificationId: notification.ID,
		Status:         string(notification.Status),
		Message:        "Notification retry initiated successfully",
		IsDuplicate:    false,
	}, nil
}

// RegisterDeviceToken registers or updates a device FCM token for push notifications
func (s *NotificationServer) RegisterDeviceToken(ctx context.Context, req *pb.RegisterDeviceTokenRequest) (*pb.RegisterDeviceTokenResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "RegisterDeviceToken"),
		attribute.String("retailer.id", req.RetailerId),
		attribute.String("device.platform", req.Platform),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.FcmToken == "" {
		return nil, status.Error(codes.InvalidArgument, "fcm_token is required")
	}
	if req.Platform == "" {
		return nil, status.Error(codes.InvalidArgument, "platform is required")
	}

	// Create device token request
	createReq := &models.CreateDeviceTokenRequest{
		RetailerID: req.RetailerId,
		DeviceID:   req.DeviceId,
		FCMToken:   req.FcmToken,
		Platform:   req.Platform,
		AppVersion: req.AppVersion,
	}

	token, err := s.retailerNotificationService.RegisterDeviceToken(ctx, createReq)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to register device token: %v", err))
	}

	s.metrics.RecordGRPCRequest("RegisterDeviceToken", time.Since(start), len(req.String()), 0, "success")

	return &pb.RegisterDeviceTokenResponse{
		Success: true,
		Message: "Device token registered successfully",
		TokenId: token.ID,
	}, nil
}

// GetRetailerNotifications retrieves notifications for a retailer
func (s *NotificationServer) GetRetailerNotifications(ctx context.Context, req *pb.GetRetailerNotificationsRequest) (*pb.GetRetailerNotificationsResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GetRetailerNotifications"),
		attribute.String("retailer.id", req.RetailerId),
		attribute.Int("page", int(req.Page)),
		attribute.Int("page_size", int(req.PageSize)),
		attribute.Bool("unread_only", req.UnreadOnly),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}

	// Default pagination
	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}

	// Create filter
	filter := &models.RetailerNotificationFilter{
		RetailerID: req.RetailerId,
		Page:       page,
		PageSize:   pageSize,
		UnreadOnly: req.UnreadOnly,
	}

	notifications, total, err := s.retailerNotificationService.GetRetailerNotifications(ctx, filter)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get retailer notifications: %v", err))
	}

	// Get unread count
	unreadCount, err := s.retailerNotificationService.GetUnreadCount(ctx, req.RetailerId)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get unread count: %v", err))
	}

	// Convert to proto
	protoNotifications := make([]*pb.RetailerNotification, len(notifications))
	for i, notif := range notifications {
		protoNotifications[i] = convertRetailerNotificationToProto(notif)
	}

	// Calculate total pages
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	s.metrics.RecordGRPCRequest("GetRetailerNotifications", time.Since(start), len(req.String()), 0, "success")

	return &pb.GetRetailerNotificationsResponse{
		Notifications: protoNotifications,
		TotalCount:    int32(total),
		Page:          int32(page),
		PageSize:      int32(pageSize),
		TotalPages:    int32(totalPages),
		UnreadCount:   int32(unreadCount),
	}, nil
}

// MarkNotificationAsRead marks a specific notification as read
func (s *NotificationServer) MarkNotificationAsRead(ctx context.Context, req *pb.MarkNotificationAsReadRequest) (*pb.GenericResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "MarkNotificationAsRead"),
		attribute.String("retailer.id", req.RetailerId),
		attribute.String("notification.id", req.NotificationId),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}
	if req.NotificationId == "" {
		return nil, status.Error(codes.InvalidArgument, "notification_id is required")
	}

	err := s.retailerNotificationService.MarkAsRead(ctx, req.RetailerId, req.NotificationId)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to mark notification as read: %v", err))
	}

	s.metrics.RecordGRPCRequest("MarkNotificationAsRead", time.Since(start), len(req.String()), 0, "success")

	return &pb.GenericResponse{
		Message: "Notification marked as read successfully",
	}, nil
}

// MarkAllNotificationsAsRead marks all notifications as read for a retailer
func (s *NotificationServer) MarkAllNotificationsAsRead(ctx context.Context, req *pb.MarkAllNotificationsAsReadRequest) (*pb.GenericResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "MarkAllNotificationsAsRead"),
		attribute.String("retailer.id", req.RetailerId),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}

	err := s.retailerNotificationService.MarkAllAsRead(ctx, req.RetailerId)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to mark all notifications as read: %v", err))
	}

	s.metrics.RecordGRPCRequest("MarkAllNotificationsAsRead", time.Since(start), len(req.String()), 0, "success")

	return &pb.GenericResponse{
		Message: "All notifications marked as read successfully",
	}, nil
}

// GetUnreadCount retrieves the count of unread notifications for a retailer
func (s *NotificationServer) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
	start := time.Now()
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("grpc.method", "GetUnreadCount"),
		attribute.String("retailer.id", req.RetailerId),
	)
	s.metrics.IncrementActiveRequests()
	defer s.metrics.DecrementActiveRequests()

	// Validate request
	if req.RetailerId == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id is required")
	}

	count, err := s.retailerNotificationService.GetUnreadCount(ctx, req.RetailerId)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get unread count: %v", err))
	}

	s.metrics.RecordGRPCRequest("GetUnreadCount", time.Since(start), len(req.String()), 0, "success")

	return &pb.GetUnreadCountResponse{
		UnreadCount: int32(count),
	}, nil
}
