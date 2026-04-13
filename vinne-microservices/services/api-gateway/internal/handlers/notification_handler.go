package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	pb "github.com/randco/randco-microservices/proto/notification/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
)

// NotificationHandler handles notification-related HTTP requests
type NotificationHandler struct {
	grpcManager *grpc.ClientManager
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(grpcManager *grpc.ClientManager) *NotificationHandler {
	return &NotificationHandler{
		grpcManager: grpcManager,
	}
}

// RegisterDeviceTokenRequest represents the request to register a device token
type RegisterDeviceTokenRequest struct {
	DeviceID   string `json:"device_id"`
	FCMToken   string `json:"fcm_token"`
	Platform   string `json:"platform"`
	AppVersion string `json:"app_version,omitempty"`
}

// GetNotificationsQueryParams represents query parameters for listing notifications
type GetNotificationsQueryParams struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	Type       string `json:"type,omitempty"`
	UnreadOnly bool   `json:"unread_only"`
}

// RegisterDeviceToken registers a device FCM token for push notifications
func (h *NotificationHandler) RegisterDeviceToken(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get retailer ID from context (set by auth middleware)
	retailerID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Retailer ID not found in context")
	}

	// Parse request body
	var req RegisterDeviceTokenRequest
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
	}

	// Validate required fields
	if req.DeviceID == "" || req.FCMToken == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "device_id and fcm_token are required")
	}

	// Validate FCM token format (FCM tokens are typically 150+ characters)
	if len(req.FCMToken) < 20 {
		return router.ErrorResponse(w, http.StatusBadRequest, "fcm_token appears to be invalid or too short")
	}

	// Validate device ID is not empty after trimming
	if len(req.DeviceID) < 1 {
		return router.ErrorResponse(w, http.StatusBadRequest, "device_id cannot be empty")
	}

	// Default platform to android if not specified
	if req.Platform == "" {
		req.Platform = "android"
	}

	// Validate platform is supported
	if req.Platform != "android" && req.Platform != "ios" {
		return router.ErrorResponse(w, http.StatusBadRequest, "platform must be 'android' or 'ios'")
	}

	// Get notification service client
	conn, err := h.grpcManager.GetNotificationServiceConn()
	if err != nil {
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable: "+err.Error())
	}
	client := pb.NewNotificationServiceClient(conn)

	// Call gRPC service
	resp, err := client.RegisterDeviceToken(ctx, &pb.RegisterDeviceTokenRequest{
		RetailerId: retailerID,
		DeviceId:   req.DeviceID,
		FcmToken:   req.FCMToken,
		Platform:   req.Platform,
		AppVersion: req.AppVersion,
	})
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to register device token: "+err.Error())
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":  resp.Success,
		"message":  resp.Message,
		"token_id": resp.TokenId,
	})
}

// GetNotifications retrieves notifications for the authenticated retailer
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get retailer ID from context
	retailerID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Retailer ID not found in context")
	}

	// Parse query parameters
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(query.Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	unreadOnly := query.Get("unread_only") == "true"

	// Get notification service client
	conn, err := h.grpcManager.GetNotificationServiceConn()
	if err != nil {
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable: "+err.Error())
	}
	client := pb.NewNotificationServiceClient(conn)

	// Call gRPC service
	resp, err := client.GetRetailerNotifications(ctx, &pb.GetRetailerNotificationsRequest{
		RetailerId: retailerID,
		Page:       int32(page),
		PageSize:   int32(pageSize),
		UnreadOnly: unreadOnly,
	})
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve notifications: "+err.Error())
	}

	// Handle nil notifications array - convert to empty array for JSON serialization
	notifications := resp.Notifications
	if notifications == nil {
		notifications = []*pb.RetailerNotification{}
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Notifications retrieved successfully",
		"data": map[string]interface{}{
			"notifications": notifications,
			"total":         resp.TotalCount,
			"page":          resp.Page,
			"page_size":     resp.PageSize,
			"unread_count":  resp.UnreadCount,
		},
	})
}

// MarkNotificationAsRead marks a specific notification as read
func (h *NotificationHandler) MarkNotificationAsRead(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get retailer ID from context
	retailerID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Retailer ID not found in context")
	}

	// Get notification ID from URL path
	notificationID := router.GetParam(r, "id")
	if notificationID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "notification ID is required")
	}

	// Get notification service client
	conn, err := h.grpcManager.GetNotificationServiceConn()
	if err != nil {
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable: "+err.Error())
	}
	client := pb.NewNotificationServiceClient(conn)

	// Call gRPC service
	resp, err := client.MarkNotificationAsRead(ctx, &pb.MarkNotificationAsReadRequest{
		RetailerId:     retailerID,
		NotificationId: notificationID,
	})
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to mark notification as read: "+err.Error())
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": resp.Message,
	})
}

// MarkAllNotificationsAsRead marks all notifications for a retailer as read
func (h *NotificationHandler) MarkAllNotificationsAsRead(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get retailer ID from context
	retailerID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Retailer ID not found in context")
	}

	// Get notification service client
	conn, err := h.grpcManager.GetNotificationServiceConn()
	if err != nil {
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable: "+err.Error())
	}
	client := pb.NewNotificationServiceClient(conn)

	// Call gRPC service
	resp, err := client.MarkAllNotificationsAsRead(ctx, &pb.MarkAllNotificationsAsReadRequest{
		RetailerId: retailerID,
	})
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to mark all notifications as read: "+err.Error())
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": resp.Message,
	})
}

// GetUnreadCount retrieves the count of unread notifications for the retailer
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get retailer ID from context
	retailerID, ok := r.Context().Value(router.ContextUserID).(string)
	if !ok || retailerID == "" {
		return router.ErrorResponse(w, http.StatusUnauthorized, "Retailer ID not found in context")
	}

	// Get notification service client
	conn, err := h.grpcManager.GetNotificationServiceConn()
	if err != nil {
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Service unavailable: "+err.Error())
	}
	client := pb.NewNotificationServiceClient(conn)

	// Call gRPC service
	resp, err := client.GetUnreadCount(ctx, &pb.GetUnreadCountRequest{
		RetailerId: retailerID,
	})
	if err != nil {
		return router.ErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve unread count: "+err.Error())
	}

	return router.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"success":      true,
		"message":      "Unread count retrieved successfully",
		"unread_count": resp.UnreadCount,
	})
}
