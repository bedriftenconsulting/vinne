package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	terminalpb "github.com/randco/randco-microservices/proto/terminal/v1"
	terminalv1 "github.com/randco/randco-microservices/proto/terminal/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/response"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type terminalHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

func NewTerminalHandler(grpcManager *grpc.ClientManager, log logger.Logger) TerminalHandler {
	return &terminalHandler{
		grpcManager: grpcManager,
		log:         log,
	}
}

// RegisterTerminal - Admin only
func (h *terminalHandler) RegisterTerminal(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		DeviceID       string            `json:"device_id"`
		Name           string            `json:"name"`
		Model          string            `json:"model"`
		SerialNumber   string            `json:"serial_number"`
		IMEI           string            `json:"imei,omitempty"`
		AndroidVersion string            `json:"android_version,omitempty"`
		AppVersion     string            `json:"app_version,omitempty"`
		Vendor         string            `json:"vendor"`
		Manufacturer   string            `json:"manufacturer"`
		PurchaseDate   string            `json:"purchase_date,omitempty"`
		Metadata       map[string]string `json:"metadata,omitempty"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", nil)
	}

	if req.DeviceID == "" || req.Name == "" || req.Model == "" || req.SerialNumber == "" || req.Vendor == "" || req.Manufacturer == "" {
		return response.ValidationError(w, "Missing required fields", nil)
	}

	// Validate and convert model
	protoModel, err := convertModelToProto(req.Model)
	if err != nil {
		return response.ValidationError(w, "Invalid model type", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Convert response to grpc
	grpcReq := &terminalpb.RegisterTerminalRequest{
		DeviceId:       req.DeviceID,
		Name:           req.Name,
		Model:          protoModel,
		SerialNumber:   req.SerialNumber,
		Imei:           req.IMEI,
		AndroidVersion: req.AndroidVersion,
		AppVersion:     req.AppVersion,
		Vendor:         req.Vendor,
		Manufacturer:   req.Manufacturer,
		Metadata:       req.Metadata,
	}

	if req.PurchaseDate != "" {
		ts, err := convertDateToTimestamp(req.PurchaseDate)
		if err != nil {
			return response.ValidationError(w, "Invalid purchase date format, expected YYYY-MM-DD", nil)
		}
		grpcReq.PurchaseDate = ts
	}

	resp, err := client.RegisterTerminal(ctx, grpcReq)
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to register terminal")
	}

	return response.Success(w, http.StatusCreated, "Terminal registered successfully",
		h.convertTerminal(resp.Terminal))

}

func (h *terminalHandler) GetTerminal(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetTerminal(ctx, &terminalpb.GetTerminalRequest{
		TerminalId: terminalID,
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to get terminal")
	}

	if resp.Terminal == nil {
		return response.NotFoundError(w, "Terminal")
	}

	responseBody := map[string]interface{}{
		"terminal": h.convertTerminal(resp.Terminal),
		"config":   resp.Config,
		"health":   h.convertHealth(resp.Health),
	}

	if resp.Assignment != nil {
		responseBody["assignment"] = map[string]interface{}{
			"retailer_id": resp.Assignment.RetailerId,
			"assigned_at": resp.Assignment.AssignedAt,
		}
	} else {
		responseBody["assignment"] = "No assignment found"
	}

	return response.Success(w, http.StatusOK, "Terminal retrieved successfully", responseBody)
}

func (h *terminalHandler) ListTerminals(w http.ResponseWriter, r *http.Request) error {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	statusStr := r.URL.Query().Get("status")
	var status terminalpb.TerminalStatus

	switch strings.ToUpper(statusStr) {
	case "ACTIVE":
		status = terminalpb.TerminalStatus_ACTIVE
	case "INACTIVE":
		status = terminalpb.TerminalStatus_INACTIVE
	case "FAULTY":
		status = terminalpb.TerminalStatus_FAULTY
	case "MAINTENANCE":
		status = terminalpb.TerminalStatus_MAINTENANCE
	case "SUSPENDED":
		status = terminalpb.TerminalStatus_SUSPENDED
	case "DECOMMISSIONED":
		status = terminalpb.TerminalStatus_DECOMMISSIONED
	default:
		status = terminalpb.TerminalStatus_TERMINAL_STATUS_UNSPECIFIED
	}

	resp, err := client.ListTerminals(ctx, &terminalpb.ListTerminalsRequest{
		Page:     int32(page),
		PageSize: int32(perPage),
		Status:   status,
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to list terminals")
	}

	terminals := make([]interface{}, 0, len(resp.Terminals))
	for _, t := range resp.Terminals {
		terminals = append(terminals, h.convertTerminal(t))
	}

	return response.SuccessWithPagination(w, "Terminals retrieved successfully", terminals, page, perPage, int(resp.TotalCount))
}

func (h *terminalHandler) UpdateTerminal(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	var req struct {
		Name           string `json:"name"`
		Model          string `json:"model"`
		AndroidVersion string `json:"android_version"`
		AppVersion     string `json:"app_version"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	if req.Name == "" || req.Model == "" {
		return response.ValidationError(w, "Missing required fields", nil)
	}

	// Validate and convert model
	protoModel, err := convertModelToProto(req.Model)
	if err != nil {
		return response.ValidationError(w, "Invalid model type", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &terminalpb.UpdateTerminalRequest{
		TerminalId:     terminalID,
		Name:           req.Name,
		Model:          protoModel,
		AndroidVersion: req.AndroidVersion,
		AppVersion:     req.AppVersion,
	}

	resp, err := client.UpdateTerminal(ctx, grpcReq)
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to update terminal")
	}

	return response.Success(w, http.StatusOK, "Terminal updated successfully", h.convertTerminal(resp.Terminal))
}

func (h *terminalHandler) DeleteTerminal(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	userID := router.GetUserID(r)
	if userID == "" {
		return response.UnauthorizedError(w, "")
	}

	deletedBy, err := uuid.Parse(userID)
	if err != nil {
		return response.ValidationError(w, "Invalid deleted_by UUID", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, err = client.DeleteTerminal(ctx, &terminalpb.DeleteTerminalRequest{
		TerminalId: terminalID,
		DeletedBy:  deletedBy.String(),
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to delete terminal")
	}

	return response.Success(w, http.StatusOK, "Terminal deleted successfully", nil)
}

func (h *terminalHandler) AssignTerminal(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	var req struct {
		// TerminalID string `json:"terminal_id"`
		RetailerID string `json:"retailer_id"`
		AssignedBy string `json:"assigned_by"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	if req.RetailerID == "" {
		return response.ValidationError(w, "Retailer ID is required", nil)
	}

	if req.AssignedBy == "" {
		return response.ValidationError(w, "AssignedBy ID is required", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.AssignTerminalToRetailer(ctx, &terminalpb.AssignTerminalRequest{
		TerminalId: terminalID,
		RetailerId: req.RetailerID,
		AssignedBy: req.AssignedBy,
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to assign terminal")
	}

	return response.Success(w, http.StatusOK, "Terminal assigned successfully", map[string]interface{}{
		"terminal_id": terminalID,
		"retailer_id": req.RetailerID,
		"assigned_at": resp.AssignedAt,
	})
}

func (h *terminalHandler) UnassignTerminal(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, err = client.UnassignTerminal(ctx, &terminalpb.UnassignTerminalRequest{
		TerminalId: terminalID,
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to unassign terminal")
	}

	return response.Success(w, http.StatusOK, "Terminal unassigned successfully", nil)
}

func (h *terminalHandler) GetTerminalHealth(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetTerminalHealth(ctx, &terminalpb.GetTerminalHealthRequest{
		TerminalId: terminalID,
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to get terminal health")
	}

	return response.Success(w, http.StatusOK, "Terminal health retrieved successfully", h.convertHealth(resp.Health))

}

func (h *terminalHandler) UpdateTerminalStatus(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	var req struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// statusStr := r.URL.Query().Get("status")
	var status terminalpb.TerminalStatus

	switch strings.ToUpper(req.Status) {
	case "ACTIVE":
		status = terminalpb.TerminalStatus_ACTIVE
	case "INACTIVE":
		status = terminalpb.TerminalStatus_INACTIVE
	case "FAULTY":
		status = terminalpb.TerminalStatus_FAULTY
	case "MAINTENANCE":
		status = terminalpb.TerminalStatus_MAINTENANCE
	case "SUSPENDED":
		status = terminalpb.TerminalStatus_SUSPENDED
	case "DECOMMISSIONED":
		status = terminalpb.TerminalStatus_DECOMMISSIONED
	default:
		status = terminalpb.TerminalStatus_TERMINAL_STATUS_UNSPECIFIED
		return response.ValidationError(w, "Invalid status", nil)
	}

	resp, err := client.UpdateTerminalStatus(ctx, &terminalpb.UpdateTerminalStatusRequest{
		TerminalId: terminalID,
		Status:     status,
		Reason:     req.Reason,
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to update terminal status")
	}

	return response.Success(w, http.StatusOK, "Terminal status updated successfully", h.convertTerminal(resp.Terminal))
}

func (h *terminalHandler) GetTerminalConfig(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := client.GetTerminalConfig(ctx, &terminalpb.GetTerminalConfigRequest{
		TerminalId: terminalID,
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to get terminal configuration")
	}

	return response.Success(w, http.StatusOK, "Terminal configuration retrieved successfully", map[string]interface{}{
		"terminal_id": terminalID,
		"config":      resp.Config,
		// "version":      resp.Version,
		// "last_updated": resp.LastUpdated,
	})
}

func (h *terminalHandler) UpdateTerminalConfig(w http.ResponseWriter, r *http.Request) error {
	terminalID := router.GetParam(r, "id")
	if terminalID == "" {
		return response.ValidationError(w, "Terminal ID is required", nil)
	}

	var req struct {
		TransactionLimit    int32             `json:"transaction_limit"`
		DailyLimit          int32             `json:"daily_limit"`
		OfflineModeEnabled  bool              `json:"offline_mode_enabled"`
		OfflineSyncInterval int32             `json:"offline_sync_interval"`
		AutoUpdateEnabled   bool              `json:"auto_update_enabled"`
		MinimumAppVersion   string            `json:"minimum_app_version"`
		Settings            map[string]string `json:"settings"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return response.ValidationError(w, "Invalid request body", map[string]string{
			"error": err.Error(),
		})
	}

	if req.TransactionLimit < 0 || req.DailyLimit < 0 || req.OfflineSyncInterval < 0 {
		return response.ValidationError(w, "Invalid request", nil)
	}

	if req.MinimumAppVersion == "" {
		return response.ValidationError(w, "Minimum app version is required", nil)
	}

	client, err := h.grpcManager.TerminalServiceClient()
	if err != nil {
		return response.ServiceUnavailableError(w, "Terminal")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, err = client.UpdateTerminalConfig(ctx, &terminalpb.UpdateTerminalConfigRequest{
		TerminalId:          terminalID,
		TransactionLimit:    req.TransactionLimit,
		DailyLimit:          req.DailyLimit,
		OfflineModeEnabled:  req.OfflineModeEnabled,
		OfflineSyncInterval: req.OfflineSyncInterval,
		AutoUpdateEnabled:   req.AutoUpdateEnabled,
		MinimumAppVersion:   req.MinimumAppVersion,
		Settings:            req.Settings,
	})
	if err != nil {
		return h.handleGRPCError(w, err, "Failed to update terminal configuration")
	}

	return response.Success(w, http.StatusOK, "Terminal configuration updated successfully", nil)
}

// handleGRPCError converts gRPC errors to HTTP responses
func (h *terminalHandler) handleGRPCError(w http.ResponseWriter, err error, defaultMsg string) error {
	st, ok := status.FromError(err)
	if !ok {
		h.log.Error(defaultMsg, "error", err)
		return response.InternalError(w, defaultMsg)
	}

	switch st.Code() {
	case codes.NotFound:
		return response.NotFoundError(w, "User")
	case codes.InvalidArgument:
		return response.ValidationError(w, st.Message(), nil)
	case codes.AlreadyExists:
		return response.ConflictError(w, st.Message())
	case codes.PermissionDenied:
		return response.ForbiddenError(w, st.Message())
	case codes.Unauthenticated:
		return response.UnauthorizedError(w, st.Message())
	case codes.Unavailable:
		return response.ServiceUnavailableError(w, "Terminal")
	default:
		h.log.Error(defaultMsg, "error", err, "code", st.Code())
		return response.InternalError(w, defaultMsg)
	}
}

func convertDateToTimestamp(dateStr string) (*timestamppb.Timestamp, error) {
	layout := "2006-01-02"
	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return nil, err
	}
	return timestamppb.New(t), nil
}

func (h *terminalHandler) convertTerminal(t *terminalpb.Terminal) map[string]interface{} {
	if t == nil {
		return nil
	}

	result := map[string]interface{}{
		"id":              t.Id,
		"device_id":       t.DeviceId,
		"name":            t.Name,
		"model":           t.Model,
		"serial_number":   t.SerialNumber,
		"imei":            t.Imei,
		"android_version": t.AndroidVersion,
		"vendor":          t.Vendor,
		"status":          t.Status,
		"created_at":      t.CreatedAt,
		"updated_at":      t.UpdatedAt,
	}

	if t.RetailerId != "" {
		result["retailer_id"] = t.RetailerId
		result["assignment_date"] = t.AssignmentDate
	}

	return result
}

func (h *terminalHandler) convertHealth(health *terminalpb.TerminalHealth) map[string]interface{} {
	if health == nil {
		return nil
	}

	result := map[string]interface{}{
		"terminal_id":       health.TerminalId,
		"status":            health.Status,
		"battery_level":     health.BatteryLevel,
		"signal_strength":   health.SignalStrength,
		"storage_available": health.StorageAvailable,
		"storage_total":     health.StorageTotal,
		"memory_usage":      health.MemoryUsage,
		"cpu_usage":         health.CpuUsage,
		"last_heartbeat":    health.LastHeartbeat.AsTime(),
		"diagnostics":       health.Diagnostics,
	}

	return result
}

func convertModelToProto(model string) (terminalv1.TerminalModel, error) {
	switch strings.ToUpper(model) {
	case "ANDROID_POS_V1":
		return terminalv1.TerminalModel_ANDROID_POS_V1, nil
	case "ANDROID_POS_V2":
		return terminalv1.TerminalModel_ANDROID_POS_V2, nil
	case "WEB_TERMINAL":
		return terminalv1.TerminalModel_WEB_TERMINAL, nil
	case "MOBILE_TERMINAL":
		return terminalv1.TerminalModel_MOBILE_TERMINAL, nil
	default:
		return terminalv1.TerminalModel_TERMINAL_MODEL_UNSPECIFIED,
			fmt.Errorf("unknown terminal model: %s", model)
	}
}
