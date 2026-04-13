package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	paymentpb "github.com/randco/randco-microservices/proto/payment/v1"
	playerv1 "github.com/randco/randco-microservices/proto/player/v1"
	"github.com/randco/randco-microservices/services/api-gateway/internal/grpc"
	"github.com/randco/randco-microservices/services/api-gateway/internal/router"
	"github.com/randco/randco-microservices/shared/common/logger"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PlayerHandler struct {
	grpcManager *grpc.ClientManager
	log         logger.Logger
}

func NewPlayerHandler(grpcManager *grpc.ClientManager, log logger.Logger) *PlayerHandler {
	return &PlayerHandler{
		grpcManager: grpcManager,
		log:         log,
	}
}

func (h *PlayerHandler) RegisterPlayer(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PhoneNumber      string `json:"phone_number"`
		Password         string `json:"password"`
		DeviceID         string `json:"device_id"`
		Channel          string `json:"channel"`
		TermsAccepted    bool   `json:"terms_accepted"`
		MarketingConsent bool   `json:"marketing_consent"`
		DeviceInfo       struct {
			DeviceType string `json:"device_type"`
			OS         string `json:"os"`
			OSVersion  string `json:"os_version"`
			AppVersion string `json:"app_version"`
			UserAgent  string `json:"user_agent"`
		} `json:"device_info"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.PhoneNumber == "" || req.Password == "" || req.Channel == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number, password, and channel are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}

	grpcReq := &playerv1.RegisterPlayerRequest{
		PhoneNumber:      req.PhoneNumber,
		Password:         req.Password,
		DeviceId:         req.DeviceID,
		Channel:          req.Channel,
		TermsAccepted:    req.TermsAccepted,
		MarketingConsent: req.MarketingConsent,
		DeviceInfo: &playerv1.DeviceInfo{
			DeviceType: req.DeviceInfo.DeviceType,
			Os:         req.DeviceInfo.OS,
			OsVersion:  req.DeviceInfo.OSVersion,
			AppVersion: req.DeviceInfo.AppVersion,
			IpAddress:  clientIP,
			UserAgent:  req.DeviceInfo.UserAgent,
		},
	}

	resp, err := client.RegisterPlayer(ctx, grpcReq)
	if err != nil {
		h.log.Error("Player registration failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Registration failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"requires_otp": resp.RequiresOtp,
		"session_id":   resp.SessionId,
		"message":      resp.Message,
	})
}

func (h *PlayerHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		SessionID string `json:"session_id"`
		OTP       string `json:"otp"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.SessionID == "" || req.OTP == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Session ID and OTP are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.VerifyOTPRequest{
		SessionId: req.SessionID,
		Otp:       req.OTP,
	}

	resp, err := client.VerifyOTP(ctx, grpcReq)
	if err != nil {
		h.log.Error("OTP verification failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "OTP verification failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success":       resp.Success,
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"profile":       convertPlayerProfileToMap(resp.Profile),
	})
}

func (h *PlayerHandler) Login(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PhoneNumber string `json:"phone_number"`
		Password    string `json:"password"`
		DeviceID    string `json:"device_id"`
		Channel     string `json:"channel"`
		DeviceInfo  struct {
			DeviceType string `json:"device_type"`
			OS         string `json:"os"`
			OSVersion  string `json:"os_version"`
			AppVersion string `json:"app_version"`
			UserAgent  string `json:"user_agent"`
		} `json:"device_info"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.PhoneNumber == "" || req.Password == "" || req.Channel == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number, password, and channel are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	clientIP := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		clientIP = host
	}

	grpcReq := &playerv1.LoginRequest{
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
		DeviceId:    req.DeviceID,
		Channel:     req.Channel,
		DeviceInfo: &playerv1.DeviceInfo{
			DeviceType: req.DeviceInfo.DeviceType,
			Os:         req.DeviceInfo.OS,
			OsVersion:  req.DeviceInfo.OSVersion,
			AppVersion: req.DeviceInfo.AppVersion,
			IpAddress:  clientIP,
			UserAgent:  req.DeviceInfo.UserAgent,
		},
	}

	resp, err := client.Login(ctx, grpcReq)
	if err != nil {
		h.log.Error("Player login failed", "error", err)
		return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"requires_otp":  resp.RequiresOtp,
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"profile":       convertPlayerProfileToMap(resp.Profile),
	})
}

func (h *PlayerHandler) PostFeedback(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "player id is required")
	}

	var req struct {
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Message  string `json:"message"`
	}
	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}
	if req.FullName == "" || req.Message == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "full_name and message are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.SubmitFeedbackRequest{
		PlayerId: playerID,
		FullName: req.FullName,
		Email:    req.Email,
		Message:  req.Message,
	}

	resp, err := client.SubmitFeedback(ctx, grpcReq)
	if err != nil {
		h.log.Error("Submit feedback failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Could not submit feedback")
	}

	return router.WriteJSON(w, http.StatusCreated, map[string]any{
		"id":         resp.Id,
		"created_at": resp.CreatedAt.AsTime(),
	})
}

func (h *PlayerHandler) RefreshToken(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.RefreshToken == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Refresh token is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	}

	resp, err := client.RefreshToken(ctx, grpcReq)
	if err != nil {
		h.log.Error("Token refresh failed", "error", err)
		return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid refresh token")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
	})
}

func (h *PlayerHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.RefreshToken == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Refresh token is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.LogoutRequest{
		RefreshToken: req.RefreshToken,
	}

	_, err = client.Logout(ctx, grpcReq)
	if err != nil {
		h.log.Error("Logout failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Logout failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Logged out successfully",
	})
}

func (h *PlayerHandler) USSDRegister(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PhoneNumber string `json:"phone_number"`
		Password    string `json:"password"`
		SessionID   string `json:"session_id"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.PhoneNumber == "" || req.Password == "" || req.SessionID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number, password, and session ID are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.USSDRegisterRequest{
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
		SessionId:   req.SessionID,
	}

	resp, err := client.USSDRegister(ctx, grpcReq)
	if err != nil {
		h.log.Error("USSD registration failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "USSD registration failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success":       resp.Success,
		"access_token":  resp.AccessToken,
		"session_token": resp.SessionToken,
		"profile":       convertPlayerProfileToMap(resp.Profile),
		"message":       resp.Message,
	})
}

func (h *PlayerHandler) USSDLogin(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PhoneNumber string `json:"phone_number"`
		Password    string `json:"password"`
		SessionID   string `json:"session_id"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.PhoneNumber == "" || req.Password == "" || req.SessionID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number, password, and session ID are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.USSDLoginRequest{
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
		SessionId:   req.SessionID,
	}

	resp, err := client.USSDLogin(ctx, grpcReq)
	if err != nil {
		h.log.Error("USSD login failed", "error", err)
		return router.ErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success":       resp.Success,
		"access_token":  resp.AccessToken,
		"session_token": resp.SessionToken,
		"profile":       convertPlayerProfileToMap(resp.Profile),
		"message":       resp.Message,
	})
}

func (h *PlayerHandler) GetProfile(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.GetProfileRequest{
		PlayerId: playerID,
	}

	resp, err := client.GetProfile(ctx, grpcReq)
	if err != nil {
		h.log.Error("Get profile failed", "error", err)
		return router.ErrorResponse(w, http.StatusNotFound, "Player not found")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"profile": convertPlayerProfileToMap(resp),
	})
}

func (h *PlayerHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	var req struct {
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
		Email       string `json:"email"`
		DateOfBirth string `json:"date_of_birth"`
		NationalID  string `json:"national_id"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.UpdateProfileRequest{
		PlayerId:   playerID,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Email:      req.Email,
		NationalId: req.NationalID,
	}

	if req.DateOfBirth != "" {
		if dateOfBirth, err := time.Parse(time.RFC3339, req.DateOfBirth); err == nil {
			grpcReq.DateOfBirth = timestamppb.New(dateOfBirth)
		}
	}

	resp, err := client.UpdateProfile(ctx, grpcReq)
	if err != nil {
		h.log.Error("Update profile failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Profile update failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"profile": convertPlayerProfileToMap(resp),
	})
}

func (h *PlayerHandler) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Current password and new password are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.ChangePasswordRequest{
		PlayerId:        playerID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}

	_, err = client.ChangePassword(ctx, grpcReq)
	if err != nil {
		h.log.Error("Change password failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Password change failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Password changed successfully",
	})
}

func (h *PlayerHandler) UpdateMobileMoneyPhone(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	var req struct {
		PhoneNumber string `json:"phone_number"`
		OTP         string `json:"otp"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.PhoneNumber == "" || req.OTP == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number and OTP are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.UpdatePhoneRequest{
		PlayerId:    playerID,
		PhoneNumber: req.PhoneNumber,
		Otp:         req.OTP,
	}

	_, err = client.UpdateMobileMoneyPhone(ctx, grpcReq)
	if err != nil {
		h.log.Error("Update mobile money phone failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone update failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Mobile money phone updated successfully",
	})
}

func (h *PlayerHandler) InitiateDeposit(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	var req struct {
		Amount           int64  `json:"amount"`
		MobileMoneyPhone string `json:"mobile_money_phone"`
		PaymentMethod    string `json:"payment_method"`
		CustomerName     string `json:"customer_name"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.Amount <= 0 || req.MobileMoneyPhone == "" || req.PaymentMethod == "" || req.CustomerName == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Amount, mobile money phone, payment method, and customer name are required")
	}

	// Get payment service client
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.log.Error("Failed to get payment service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Payment service unavailable")
	}
	client := paymentpb.NewPaymentServiceClient(conn)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	reference := generateReference()

	walletProvider := mapPaymentMethodToProvider(req.PaymentMethod)

	depositReq := &paymentpb.InitiateDepositRequest{
		UserId:         playerID,
		WalletNumber:   req.MobileMoneyPhone,
		WalletProvider: walletProvider,
		Amount:         req.Amount,
		Currency:       "GHS",
		Narration:      fmt.Sprintf("Player deposit from %s via %s", req.MobileMoneyPhone, req.PaymentMethod),
		Reference:      reference,
		CustomerName:   req.CustomerName,
		Metadata: map[string]string{
			"user_role": "player",
			"source":    "api_gateway",
		},
	}

	resp, err := client.InitiateDeposit(ctx, depositReq)
	if err != nil {
		h.log.Error("Initiate deposit failed", "error", err)
		return router.ErrorResponse(w, http.StatusInternalServerError, "Deposit initiation failed")
	}

	if !resp.Success {
		h.log.Warn("Deposit initiation failed", "message", resp.Message)
		return router.ErrorResponse(w, http.StatusUnprocessableEntity, resp.Message)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"transaction_id": resp.Transaction.Id,
		"status":         resp.Transaction.Status.String(),
		"message":        resp.Message,
		"reference":      resp.Transaction.Reference,
	})
}

// mapPaymentMethodToProvider maps payment method string to payment proto provider
func mapPaymentMethodToProvider(method string) paymentpb.WalletProvider {
	switch strings.ToUpper(method) {
	case "MTN":
		return paymentpb.WalletProvider_WALLET_PROVIDER_MTN
	case "TELECEL", "VODAFONE": // Vodafone rebranded to Telecel in Ghana (2023)
		return paymentpb.WalletProvider_WALLET_PROVIDER_TELECEL
	case "AIRTELTIGO", "AT": // AT is short form for AirtelTigo
		return paymentpb.WalletProvider_WALLET_PROVIDER_AIRTELTIGO
	case "ORANGE":
		return paymentpb.WalletProvider_WALLET_PROVIDER_UNSPECIFIED
	default:
		return paymentpb.WalletProvider_WALLET_PROVIDER_MTN
	}
}

func (h *PlayerHandler) InitiateWithdrawal(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	var req struct {
		Amount           int64  `json:"amount"`
		MobileMoneyPhone string `json:"mobile_money_phone"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.Amount <= 0 || req.MobileMoneyPhone == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Amount and mobile money phone are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.WithdrawalRequest{
		PlayerId:         playerID,
		Amount:           req.Amount,
		MobileMoneyPhone: req.MobileMoneyPhone,
	}

	resp, err := client.InitiateWithdrawal(ctx, grpcReq)
	if err != nil {
		h.log.Error("Initiate withdrawal failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Withdrawal initiation failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"transaction_id": resp.TransactionId,
		"status":         resp.Status,
		"message":        resp.Message,
	})
}

func (h *PlayerHandler) GetTransactionHistory(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	page, _ := strconv.ParseInt(r.URL.Query().Get("page"), 10, 32)
	if page <= 0 {
		page = 1
	}
	perPage, _ := strconv.ParseInt(r.URL.Query().Get("per_page"), 10, 32)
	if perPage <= 0 {
		perPage = 50
	}
	transactionType := r.URL.Query().Get("type")
	fromDate := r.URL.Query().Get("from_date")
	toDate := r.URL.Query().Get("to_date")

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.TransactionHistoryRequest{
		PlayerId: playerID,
		Page:     int32(page),
		PerPage:  int32(perPage),
		Type:     transactionType,
	}

	if fromDate != "" {
		if parsedDate, err := time.Parse(time.RFC3339, fromDate); err == nil {
			grpcReq.FromDate = timestamppb.New(parsedDate)
		}
	}
	if toDate != "" {
		if parsedDate, err := time.Parse(time.RFC3339, toDate); err == nil {
			grpcReq.ToDate = timestamppb.New(parsedDate)
		}
	}

	resp, err := client.GetTransactionHistory(ctx, grpcReq)
	if err != nil {
		h.log.Error("Get transaction history failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to get transaction history")
	}

	transactions := make([]map[string]any, len(resp.Transactions))
	for i, tx := range resp.Transactions {
		transactions[i] = convertTransactionToMap(tx)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"transactions": transactions,
		"total":        resp.Total,
		"page":         resp.Page,
		"per_page":     resp.PerPage,
	})
}

// GetPaymentHistory returns payment transactions (deposits/withdrawals) from payment service
func (h *PlayerHandler) GetPaymentHistory(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	page, _ := strconv.ParseInt(r.URL.Query().Get("page"), 10, 32)
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.ParseInt(r.URL.Query().Get("page_size"), 10, 32)
	if pageSize <= 0 {
		pageSize = 20
	}

	// Get payment service client
	conn, err := h.grpcManager.GetConnection("payment")
	if err != nil {
		h.log.Error("Failed to get payment service connection", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Payment service unavailable")
	}

	client := paymentpb.NewPaymentServiceClient(conn)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Call ListTransactions with user_id filter
	grpcReq := &paymentpb.ListTransactionsRequest{
		UserId:   playerID,
		Page:     int32(page),
		PageSize: int32(pageSize),
	}

	resp, err := client.ListTransactions(ctx, grpcReq)
	if err != nil {
		h.log.Error("List payment transactions failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to get payment transactions")
	}

	// Convert transactions to response format
	transactions := make([]map[string]any, len(resp.Transactions))
	for i, tx := range resp.Transactions {
		transactions[i] = map[string]any{
			"id":                      tx.Id,
			"reference":               tx.Reference,
			"provider_transaction_id": tx.ProviderTransactionId,
			"type":                    tx.Type.String(),
			"status":                  tx.Status.String(),
			"amount":                  tx.Amount,
			"currency":                tx.Currency,
			"narration":               tx.Narration,
			"provider_name":           tx.ProviderName,
			"source_type":             tx.SourceType,
			"source_identifier":       tx.SourceIdentifier,
			"source_name":             tx.SourceName,
			"destination_type":        tx.DestinationType,
			"destination_identifier":  tx.DestinationIdentifier,
			"destination_name":        tx.DestinationName,
			"error_message":           tx.ErrorMessage,
			"error_code":              tx.ErrorCode,
			"created_at":              tx.CreatedAt.AsTime().Format(time.RFC3339),
			"requested_at":            tx.RequestedAt.AsTime().Format(time.RFC3339),
		}

		if tx.CompletedAt != nil {
			transactions[i]["completed_at"] = tx.CompletedAt.AsTime().Format(time.RFC3339)
		}
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"transactions": transactions,
		"pagination": map[string]any{
			"current_page": resp.Pagination.CurrentPage,
			"page_size":    resp.Pagination.PageSize,
			"total_items":  resp.Pagination.TotalItems,
			"total_pages":  resp.Pagination.TotalPages,
		},
	})
}

func (h *PlayerHandler) GetWalletBalance(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.WalletBalanceRequest{
		PlayerId: playerID,
	}

	resp, err := client.GetWalletBalance(ctx, grpcReq)
	if err != nil {
		h.log.Error("Get wallet balance failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to get wallet balance")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"balance":         resp.Balance,
		"pending_balance": resp.PendingBalance,
		"currency":        resp.Currency,
	})
}

func (h *PlayerHandler) GetPlayerByID(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.GetPlayerByIDRequest{
		PlayerId: playerID,
	}

	resp, err := client.GetPlayerByID(ctx, grpcReq)
	if err != nil {
		h.log.Error("Get player by ID failed", "error", err)
		return router.ErrorResponse(w, http.StatusNotFound, "Player not found")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"player": convertPlayerProfileToMap(resp),
	})
}

func (h *PlayerHandler) SearchPlayers(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query().Get("query")
	page, _ := strconv.ParseInt(r.URL.Query().Get("page"), 10, 32)
	if page <= 0 {
		page = 1
	}
	perPage, _ := strconv.ParseInt(r.URL.Query().Get("per_page"), 10, 32)
	if perPage <= 0 {
		perPage = 50
	}
	statusStr := r.URL.Query().Get("status")

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.SearchPlayersRequest{
		Query:   query,
		Page:    int32(page),
		PerPage: int32(perPage),
	}

	if statusStr != "" {
		switch statusStr {
		case "ACTIVE":
			grpcReq.Status = playerv1.PlayerStatus_ACTIVE
		case "SUSPENDED":
			grpcReq.Status = playerv1.PlayerStatus_SUSPENDED
		case "BANNED":
			grpcReq.Status = playerv1.PlayerStatus_BANNED
		}
	}

	resp, err := client.SearchPlayers(ctx, grpcReq)
	if err != nil {
		h.log.Error("Search players failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Player search failed")
	}

	players := make([]map[string]any, len(resp.Players))
	for i, player := range resp.Players {
		players[i] = convertPlayerProfileToMap(player)
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"players":  players,
		"total":    resp.Total,
		"page":     resp.Page,
		"per_page": resp.PerPage,
	})
}

func (h *PlayerHandler) SuspendPlayer(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	var req struct {
		Reason      string `json:"reason"`
		SuspendedBy string `json:"suspended_by"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.Reason == "" || req.SuspendedBy == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Reason and suspended_by are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.SuspendPlayerRequest{
		PlayerId:    playerID,
		Reason:      req.Reason,
		SuspendedBy: req.SuspendedBy,
	}

	_, err = client.SuspendPlayer(ctx, grpcReq)
	if err != nil {
		h.log.Error("Suspend player failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Player suspension failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Player suspended successfully",
	})
}

func (h *PlayerHandler) ActivatePlayer(w http.ResponseWriter, r *http.Request) error {
	playerID := router.GetParam(r, "id")
	if playerID == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Player ID is required")
	}

	var req struct {
		ActivatedBy string `json:"activated_by"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.ActivatedBy == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "activated_by is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.ActivatePlayerRequest{
		PlayerId:    playerID,
		ActivatedBy: req.ActivatedBy,
	}

	_, err = client.ActivatePlayer(ctx, grpcReq)
	if err != nil {
		h.log.Error("Activate player failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Player activation failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Player activated successfully",
	})
}

func convertPlayerProfileToMap(profile *playerv1.PlayerProfile) map[string]any {
	if profile == nil {
		return nil
	}

	result := map[string]any{
		"id":                 profile.Id,
		"phone_number":       profile.PhoneNumber,
		"email":              profile.Email,
		"first_name":         profile.FirstName,
		"last_name":          profile.LastName,
		"national_id":        profile.NationalId,
		"wallet_id":          profile.WalletId,
		"mobile_money_phone": profile.MobileMoneyPhone,
		"status":             profile.Status.String(),
	}

	if profile.DateOfBirth != nil {
		result["date_of_birth"] = profile.DateOfBirth.AsTime().Format(time.RFC3339)
	}
	if profile.CreatedAt != nil {
		result["created_at"] = profile.CreatedAt.AsTime().Format(time.RFC3339)
	}
	if profile.LastLogin != nil {
		result["last_login"] = profile.LastLogin.AsTime().Format(time.RFC3339)
	}

	return result
}

func convertTransactionToMap(tx *playerv1.Transaction) map[string]any {
	if tx == nil {
		return nil
	}

	result := map[string]any{
		"id":        tx.Id,
		"player_id": tx.PlayerId,
		"type":      tx.Type.String(),
		"amount":    tx.Amount,
		"reference": tx.Reference,
		"status":    tx.Status,
	}

	if tx.CreatedAt != nil {
		result["created_at"] = tx.CreatedAt.AsTime().Format(time.RFC3339)
	}

	return result
}

func (h *PlayerHandler) ResendOTP(w http.ResponseWriter, r *http.Request) error {
	phoneNumber := r.URL.Query().Get("phone_number")
	if phoneNumber == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.ResendOTPRequest{
		PhoneNumber: phoneNumber,
	}

	_, err = client.ResendOTP(ctx, grpcReq)
	if err != nil {
		h.log.Error("Resend OTP failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Resend OTP failed")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "OTP resent successfully",
	})
}

// Password Reset Handlers

func (h *PlayerHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PhoneNumber string `json:"phone_number"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.PhoneNumber == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.RequestPasswordResetRequest{
		PhoneNumber: req.PhoneNumber,
	}

	resp, err := client.RequestPasswordReset(ctx, grpcReq)
	if err != nil {
		h.log.Error("Request password reset failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to initiate password reset")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"session_id": resp.SessionId,
		"message":    resp.Message,
	})
}

func (h *PlayerHandler) ValidatePasswordResetOTP(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		SessionID string `json:"session_id"`
		OTP       string `json:"otp"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.SessionID == "" || req.OTP == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Session ID and OTP are required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.ValidatePasswordResetOTPRequest{
		SessionId: req.SessionID,
		Otp:       req.OTP,
	}

	resp, err := client.ValidatePasswordResetOTP(ctx, grpcReq)
	if err != nil {
		h.log.Error("Validate password reset OTP failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid OTP")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"success":    resp.Success,
		"session_id": resp.SessionId,
		"message":    resp.Message,
	})
}

func (h *PlayerHandler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		SessionID   string `json:"session_id"`
		NewPassword string `json:"new_password"`
		OTP         string `json:"otp"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.SessionID == "" || req.NewPassword == "" || req.OTP == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Session ID, OTP, and new password are required")
	}

	if len(req.NewPassword) < 6 {
		return router.ErrorResponse(w, http.StatusBadRequest, "Password must be at least 6 characters")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.ConfirmPasswordResetRequest{
		SessionId:   req.SessionID,
		NewPassword: req.NewPassword,
		Otp:         req.OTP,
	}

	_, err = client.ConfirmPasswordReset(ctx, grpcReq)
	if err != nil {
		h.log.Error("Confirm password reset failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to reset password")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "Password reset successfully",
	})
}

func (h *PlayerHandler) ResendPasswordResetOTP(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		PhoneNumber string `json:"phone_number"`
	}

	if err := router.ReadJSON(r, &req); err != nil {
		return router.ErrorResponse(w, http.StatusBadRequest, "Invalid request body")
	}

	if req.PhoneNumber == "" {
		return router.ErrorResponse(w, http.StatusBadRequest, "Phone number is required")
	}

	client, err := h.grpcManager.PlayerServiceClient()
	if err != nil {
		h.log.Error("Failed to get player service client", "error", err)
		return router.ErrorResponse(w, http.StatusServiceUnavailable, "Player service unavailable")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	grpcReq := &playerv1.ResendPasswordResetOTPRequest{
		PhoneNumber: req.PhoneNumber,
	}

	_, err = client.ResendPasswordResetOTP(ctx, grpcReq)
	if err != nil {
		h.log.Error("Resend password reset OTP failed", "error", err)
		return router.ErrorResponse(w, http.StatusBadRequest, "Failed to resend OTP")
	}

	return router.WriteJSON(w, http.StatusOK, map[string]any{
		"message": "OTP resent successfully",
	})
}
