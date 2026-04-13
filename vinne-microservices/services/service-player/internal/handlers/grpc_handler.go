package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	playerv1 "github.com/randco/randco-microservices/proto/player/v1"
	"github.com/randco/randco-microservices/shared/validation"
	"github.com/randco/service-player/internal/clients"
	"github.com/randco/service-player/internal/models"
	"github.com/randco/service-player/internal/services"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PlayerServiceHandler struct {
	playerv1.UnimplementedPlayerServiceServer

	// Services
	authService         services.AuthService
	registrationService services.RegistrationService
	profileService      services.ProfileService
	sessionService      services.SessionService
	adminService        services.AdminService
	otpService          services.OTPService
	// External service clients
	walletClient       *clients.WalletClient
	paymentClient      *clients.PaymentClient
	notificationClient *clients.NotificationClient
}

func NewPlayerServiceHandler(
	authService services.AuthService,
	registrationService services.RegistrationService,
	profileService services.ProfileService,
	sessionService services.SessionService,
	adminService services.AdminService,
	walletClient *clients.WalletClient,
	paymentClient *clients.PaymentClient,
	notificationClient *clients.NotificationClient,
	otpService services.OTPService,
) *PlayerServiceHandler {
	return &PlayerServiceHandler{
		authService:         authService,
		registrationService: registrationService,
		profileService:      profileService,
		sessionService:      sessionService,
		adminService:        adminService,
		walletClient:        walletClient,
		paymentClient:       paymentClient,
		notificationClient:  notificationClient,
		otpService:          otpService,
	}
}

func (h *PlayerServiceHandler) RegisterPlayer(ctx context.Context, req *playerv1.RegisterPlayerRequest) (*playerv1.RegisterPlayerResponse, error) {
	registrationReq := models.RegistrationRequest{
		PhoneNumber:         req.PhoneNumber,
		Password:            req.Password,
		DeviceID:            req.DeviceId,
		RegistrationChannel: req.Channel,
		TermsAccepted:       req.TermsAccepted,
		MarketingConsent:    req.MarketingConsent,
	}

	player, err := h.registrationService.RegisterPlayer(ctx, registrationReq)
	if err != nil {
		return nil, h.handleError(err, "RegisterPlayer")
	}

	err = h.otpService.GenerateAndSendOTP(ctx, player.PhoneNumber, "registration")
	if err != nil {
		return nil, h.handleError(err, "RegisterPlayer")
	}

	return &playerv1.RegisterPlayerResponse{
		RequiresOtp: true,
		SessionId:   player.ID.String(),
		Message:     "Registration initiated. Please verify OTP.",
	}, nil
}

func (h *PlayerServiceHandler) VerifyOTP(ctx context.Context, req *playerv1.VerifyOTPRequest) (*playerv1.VerifyOTPResponse, error) {
	err := h.otpService.VerifyOTP(ctx, req.SessionId, req.Otp, "registration")
	if err != nil {
		return nil, h.handleError(err, "VerifyOTP")
	}

	playerID, err := uuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid session ID")
	}

	player, err := h.profileService.GetProfile(ctx, playerID)
	if err != nil {
		return nil, h.handleError(err, "VerifyOTP")
	}

	tokens, err := h.authService.GenerateTokens(ctx, player, "", "", "", "", "", "")
	if err != nil {
		return nil, h.handleError(err, "VerifyOTP")
	}

	profile := h.convertPlayerToProto(player)

	return &playerv1.VerifyOTPResponse{
		Success:      true,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Profile:      profile,
	}, nil
}

func (h *PlayerServiceHandler) Login(ctx context.Context, req *playerv1.LoginRequest) (*playerv1.LoginResponse, error) {
	player, err := h.authService.ValidateCredentials(ctx, models.ValidateLoginRequest{
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
		DeviceID:    req.DeviceId,
		Channel:     req.Channel,
		DeviceType:  req.DeviceInfo.DeviceType,
		AppVersion:  req.DeviceInfo.AppVersion,
		IPAddress:   req.DeviceInfo.IpAddress,
		UserAgent:   req.DeviceInfo.UserAgent,
	})
	if err != nil {
		return nil, h.handleError(err, "Login")
	}

	tokens, err := h.authService.GenerateTokens(ctx, player, req.DeviceId, req.Channel, req.DeviceInfo.DeviceType, req.DeviceInfo.AppVersion, req.DeviceInfo.IpAddress, req.DeviceInfo.UserAgent)
	if err != nil {
		return nil, h.handleError(err, "Login")
	}

	profile := h.convertPlayerToProto(player)

	return &playerv1.LoginResponse{
		RequiresOtp:  false, // For now, assume no OTP required
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		Profile:      profile,
	}, nil
}

func (h *PlayerServiceHandler) RefreshToken(ctx context.Context, req *playerv1.RefreshTokenRequest) (*playerv1.RefreshTokenResponse, error) {
	tokens, err := h.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, h.handleError(err, "RefreshToken")
	}

	return &playerv1.RefreshTokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}

func (h *PlayerServiceHandler) Logout(ctx context.Context, req *playerv1.LogoutRequest) (*emptypb.Empty, error) {
	// Revoke token
	err := h.authService.RevokeToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, h.handleError(err, "Logout")
	}

	return &emptypb.Empty{}, nil
}

// USSD Authentication Methods

func (h *PlayerServiceHandler) USSDRegister(ctx context.Context, req *playerv1.USSDRegisterRequest) (*playerv1.USSDAuthResponse, error) {
	// Convert proto request to internal model
	ussdReq := models.USSDRegisterRequest{
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
	}

	// Register player via USSD
	player, err := h.registrationService.USSDRegister(ctx, ussdReq)
	if err != nil {
		return nil, h.handleError(err, "USSDRegister")
	}

	// Generate tokens
	tokens, err := h.authService.GenerateTokens(ctx, player, "", "", "", "", "", "")
	if err != nil {
		return nil, h.handleError(err, "USSDRegister")
	}

	// Convert player to proto
	profile := h.convertPlayerToProto(player)

	return &playerv1.USSDAuthResponse{
		Success:      true,
		AccessToken:  tokens.AccessToken,
		SessionToken: tokens.RefreshToken, // Use refresh token as session token for USSD
		Profile:      profile,
		Message:      "USSD registration successful",
	}, nil
}

func (h *PlayerServiceHandler) USSDLogin(ctx context.Context, req *playerv1.USSDLoginRequest) (*playerv1.USSDAuthResponse, error) {
	// Validate credentials
	validateLoginReq := models.ValidateLoginRequest{
		PhoneNumber: req.PhoneNumber,
		Password:    req.Password,
		DeviceID:    req.SessionId,
		Channel:     "ussd",
		IPAddress:   "",
		AppVersion:  "",
		UserAgent:   "",
	}
	player, err := h.authService.ValidateCredentials(ctx, validateLoginReq)
	if err != nil {
		return nil, h.handleError(err, "USSDLogin")
	}

	// Generate tokens
	tokens, err := h.authService.GenerateTokens(ctx, player, req.SessionId, "ussd", "", "", "", "")
	if err != nil {
		return nil, h.handleError(err, "USSDLogin")
	}

	// Convert player to proto
	profile := h.convertPlayerToProto(player)

	return &playerv1.USSDAuthResponse{
		Success:      true,
		AccessToken:  tokens.AccessToken,
		SessionToken: tokens.RefreshToken, // Use refresh token as session token for USSD
		Profile:      profile,
		Message:      "USSD login successful",
	}, nil
}

// Profile Management Methods

func (h *PlayerServiceHandler) GetProfile(ctx context.Context, req *playerv1.GetProfileRequest) (*playerv1.PlayerProfile, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	player, err := h.profileService.GetProfile(ctx, playerID)
	if err != nil {
		return nil, h.handleError(err, "GetProfile")
	}

	return h.convertPlayerToProto(player), nil
}

func (h *PlayerServiceHandler) UpdateProfile(ctx context.Context, req *playerv1.UpdateProfileRequest) (*playerv1.PlayerProfile, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	updateReq := models.UpdatePlayerRequest{
		ID:         playerID,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Email:      req.Email,
		NationalID: req.NationalId,
	}

	player, err := h.profileService.UpdateProfile(ctx, updateReq)
	if err != nil {
		return nil, h.handleError(err, "UpdateProfile")
	}

	return h.convertPlayerToProto(player), nil
}

func (h *PlayerServiceHandler) ChangePassword(ctx context.Context, req *playerv1.ChangePasswordRequest) (*emptypb.Empty, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	err = h.profileService.ChangePassword(ctx, playerID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		return nil, h.handleError(err, "ChangePassword")
	}

	return &emptypb.Empty{}, nil
}

func (h *PlayerServiceHandler) UpdateMobileMoneyPhone(ctx context.Context, req *playerv1.UpdatePhoneRequest) (*emptypb.Empty, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	normalizedPhone := validation.NormalizePhone(req.PhoneNumber)

	err = h.profileService.UpdateMobileMoneyPhone(ctx, playerID, normalizedPhone, req.Otp)
	if err != nil {
		return nil, h.handleError(err, "UpdateMobileMoneyPhone")
	}

	return &emptypb.Empty{}, nil
}

// Wallet Operations Methods

func (h *PlayerServiceHandler) InitiateDeposit(ctx context.Context, req *playerv1.DepositRequest) (*playerv1.DepositResponse, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deposit amount")
	}

	if req.Amount < 100 {
		return nil, status.Errorf(codes.InvalidArgument, "minimum deposit amount is 1 GHS")
	}

	if req.Amount > 1000000 {
		return nil, status.Errorf(codes.InvalidArgument, "maximum deposit amount is 10,000 GHS")
	}

	depositReq := models.DepositRequest{
		PlayerID:         playerID,
		Amount:           req.Amount,
		MobileMoneyPhone: req.MobileMoneyPhone,
		PaymentMethod:    req.PaymentMethod,
	}

	response, err := h.walletClient.InitiateDeposit(ctx, depositReq)
	if err != nil {
		slog.Error("Failed to initiate deposit", "error", err)
		return nil, h.handleError(err, "InitiateDeposit")
	}

	return &playerv1.DepositResponse{
		TransactionId: response.TransactionID,
		Status:        response.Status,
		Message:       response.Message,
	}, nil
}

func (h *PlayerServiceHandler) InitiateWithdrawal(ctx context.Context, req *playerv1.WithdrawalRequest) (*playerv1.WithdrawalResponse, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	if req.Amount <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid withdrawal amount")
	}

	if req.Amount < 100 {
		return nil, status.Errorf(codes.InvalidArgument, "minimum withdrawal amount is 1 GHS")
	}

	if req.Amount > 500000 {
		return nil, status.Errorf(codes.InvalidArgument, "maximum withdrawal amount is 5,000 GHS")
	}

	withdrawalReq := models.WithdrawalRequest{
		PlayerID:         playerID,
		Amount:           req.Amount,
		MobileMoneyPhone: req.MobileMoneyPhone,
	}

	response, err := h.walletClient.InitiateWithdrawal(ctx, withdrawalReq)
	if err != nil {
		return nil, h.handleError(err, "InitiateWithdrawal")
	}

	return &playerv1.WithdrawalResponse{
		TransactionId: response.TransactionID,
		Status:        response.Status,
		Message:       response.Message,
	}, nil
}

func (h *PlayerServiceHandler) GetTransactionHistory(ctx context.Context, req *playerv1.TransactionHistoryRequest) (*playerv1.TransactionHistoryResponse, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	filter := models.TransactionFilter{
		Type:    req.Type,
		Page:    int(req.Page),
		PerPage: int(req.PerPage),
	}

	if req.FromDate != nil {
		fromTime := req.FromDate.AsTime()
		filter.FromDate = &fromTime
	}
	if req.ToDate != nil {
		toTime := req.ToDate.AsTime()
		filter.ToDate = &toTime
	}

	transactions, err := h.walletClient.GetTransactionHistory(ctx, playerID, filter)
	if err != nil {
		return nil, h.handleError(err, "GetTransactionHistory")
	}

	var protoTransactions []*playerv1.Transaction
	for _, tx := range transactions {
		protoTransactions = append(protoTransactions, &playerv1.Transaction{
			Id:        tx.ID.String(),
			PlayerId:  tx.PlayerID.String(),
			Type:      playerv1.Transaction_Type(playerv1.Transaction_Type_value[tx.Type]),
			Amount:    tx.Amount,
			Reference: tx.Reference,
			Status:    tx.Status,
			CreatedAt: timestamppb.New(tx.CreatedAt),
		})
	}

	return &playerv1.TransactionHistoryResponse{
		Transactions: protoTransactions,
		Total:        int32(len(protoTransactions)),
		Page:         req.Page,
		PerPage:      req.PerPage,
	}, nil
}

func (h *PlayerServiceHandler) GetWalletBalance(ctx context.Context, req *playerv1.WalletBalanceRequest) (*playerv1.WalletBalanceResponse, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	balance, err := h.walletClient.GetPlayerWalletBalance(ctx, playerID)
	if err != nil {
		return nil, h.handleError(err, "GetWalletBalance")
	}

	return &playerv1.WalletBalanceResponse{
		Balance:        balance.Balance,
		PendingBalance: balance.PendingBalance,
		Currency:       "GHS",
	}, nil
}

// Admin Operations Methods

func (h *PlayerServiceHandler) GetPlayerByID(ctx context.Context, req *playerv1.GetPlayerByIDRequest) (*playerv1.PlayerProfile, error) {
	// Parse player ID
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	// Get profile
	player, err := h.profileService.GetProfile(ctx, playerID)
	if err != nil {
		return nil, h.handleError(err, "GetPlayerByID")
	}

	// Convert to proto
	return h.convertPlayerToProto(player), nil
}

func (h *PlayerServiceHandler) SearchPlayers(ctx context.Context, req *playerv1.SearchPlayersRequest) (*playerv1.SearchPlayersResponse, error) {
	// Validate pagination parameters
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PerPage < 1 || req.PerPage > 100 {
		req.PerPage = 20 // Default page size
	}

	// Search players using admin service
	players, totalCount, err := h.adminService.SearchPlayers(ctx, req.Query, int(req.Page), int(req.PerPage))
	if err != nil {
		return nil, h.handleError(err, "SearchPlayers")
	}

	// Convert players to proto format
	var protoPlayers []*playerv1.PlayerProfile
	for _, player := range players {
		protoPlayers = append(protoPlayers, h.convertPlayerToProto(player))
	}

	return &playerv1.SearchPlayersResponse{
		Players: protoPlayers,
		Total:   int32(totalCount),
		Page:    req.Page,
		PerPage: req.PerPage,
	}, nil
}

func (h *PlayerServiceHandler) SuspendPlayer(ctx context.Context, req *playerv1.SuspendPlayerRequest) (*emptypb.Empty, error) {
	// Parse player ID
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	// Validate reason
	if req.Reason == "" {
		return nil, status.Errorf(codes.InvalidArgument, "suspension reason is required")
	}

	// Suspend player using admin service
	err = h.adminService.SuspendPlayer(ctx, playerID, req.Reason)
	if err != nil {
		return nil, h.handleError(err, "SuspendPlayer")
	}

	return &emptypb.Empty{}, nil
}

func (h *PlayerServiceHandler) ActivatePlayer(ctx context.Context, req *playerv1.ActivatePlayerRequest) (*emptypb.Empty, error) {
	// Parse player ID
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	// Activate player using admin service
	err = h.adminService.ActivatePlayer(ctx, playerID)
	if err != nil {
		return nil, h.handleError(err, "ActivatePlayer")
	}

	return &emptypb.Empty{}, nil
}

func (h *PlayerServiceHandler) convertPlayerToProto(player *models.Player) *playerv1.PlayerProfile {
	profile := &playerv1.PlayerProfile{
		Id:               player.ID.String(),
		PhoneNumber:      player.PhoneNumber,
		Email:            player.Email,
		FirstName:        player.FirstName,
		LastName:         player.LastName,
		NationalId:       player.NationalID,
		MobileMoneyPhone: player.MobileMoneyPhone,
		Status:           h.convertPlayerStatusToProto(player.Status),
		CreatedAt:        timestamppb.New(player.CreatedAt),
		LastLogin:        timestamppb.New(player.LastLoginAt),
	}

	// Add date of birth if not zero
	if !player.DateOfBirth.IsZero() {
		profile.DateOfBirth = timestamppb.New(player.DateOfBirth)
	}

	return profile
}

func (h *PlayerServiceHandler) convertPlayerStatusToProto(status models.PlayerStatus) playerv1.PlayerStatus {
	switch status {
	case models.PlayerStatusActive:
		return playerv1.PlayerStatus_ACTIVE
	case models.PlayerStatusSuspended:
		return playerv1.PlayerStatus_SUSPENDED
	case models.PlayerStatusBanned:
		return playerv1.PlayerStatus_BANNED
	default:
		return playerv1.PlayerStatus_ACTIVE
	}
}

func (h *PlayerServiceHandler) handleError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Log the error (TODO: Add proper logging)
	fmt.Printf("Error in %s: %v\n", operation, err)

	// Return appropriate gRPC status
	return status.Errorf(codes.Internal, "internal server error during %s", operation)
}

func (h *PlayerServiceHandler) ResendOTP(ctx context.Context, req *playerv1.ResendOTPRequest) (*emptypb.Empty, error) {
	if req.PhoneNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "phone number is required")
	}

	err := h.otpService.ResendRegistrationOTP(ctx, req.PhoneNumber)
	if err != nil {
		return nil, h.handleError(err, "ResendOTP")
	}

	return &emptypb.Empty{}, nil
}

// Password Reset Methods

func (h *PlayerServiceHandler) RequestPasswordReset(ctx context.Context, req *playerv1.RequestPasswordResetRequest) (*playerv1.RequestPasswordResetResponse, error) {
	if req.PhoneNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "phone number is required")
	}

	sessionID, err := h.authService.RequestPasswordReset(ctx, req.PhoneNumber)
	if err != nil {
		return nil, h.handleError(err, "RequestPasswordReset")
	}

	err = h.otpService.GenerateAndSendOTP(ctx, req.PhoneNumber, "password_reset")
	if err != nil {
		return nil, h.handleError(err, "RequestPasswordReset")
	}

	return &playerv1.RequestPasswordResetResponse{
		SessionId: sessionID,
		Message:   "Password reset OTP sent to your phone number",
	}, nil
}

func (h *PlayerServiceHandler) ValidatePasswordResetOTP(ctx context.Context, req *playerv1.ValidatePasswordResetOTPRequest) (*playerv1.ValidatePasswordResetOTPResponse, error) {
	if req.SessionId == "" || req.Otp == "" {
		return nil, status.Errorf(codes.InvalidArgument, "session ID and OTP are required")
	}

	// NOTE: We don't verify OTP here to avoid marking it as "used"
	// The actual OTP verification happens in ConfirmPasswordReset
	// This endpoint just validates the format/presence of OTP

	return &playerv1.ValidatePasswordResetOTPResponse{
		Success:   true,
		SessionId: req.SessionId,
		Message:   "Ready to reset password.",
	}, nil
}

func (h *PlayerServiceHandler) ConfirmPasswordReset(ctx context.Context, req *playerv1.ConfirmPasswordResetRequest) (*emptypb.Empty, error) {
	if req.SessionId == "" || req.NewPassword == "" || req.Otp == "" {
		return nil, status.Errorf(codes.InvalidArgument, "session ID, OTP, and new password are required")
	}

	playerID, err := uuid.Parse(req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid session ID")
	}

	err = h.otpService.VerifyOTP(ctx, req.SessionId, req.Otp, "password_reset")
	if err != nil {
		return nil, h.handleError(err, "ConfirmPasswordReset")
	}

	err = h.authService.ConfirmPasswordReset(ctx, playerID, req.NewPassword, req.Otp)
	if err != nil {
		return nil, h.handleError(err, "ConfirmPasswordReset")
	}

	return &emptypb.Empty{}, nil
}

func (h *PlayerServiceHandler) SubmitFeedback(ctx context.Context, req *playerv1.SubmitFeedbackRequest) (*playerv1.SubmitFeedbackResponse, error) {
	if req.PlayerId == "" || req.FullName == "" || req.Message == "" {
		return nil, status.Errorf(codes.InvalidArgument, "player_id, full_name and message are required")
	}

	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player ID")
	}

	fb, err := h.authService.SubmitFeedback(ctx, models.CreateFeedbackRequest{
		PlayerID: playerID,
		FullName: req.FullName,
		Email:    req.Email,
		Message:  req.Message,
	})
	if err != nil {
		return nil, h.handleError(err, "SubmitFeedback")
	}

	return &playerv1.SubmitFeedbackResponse{
		Id:        fb.ID.String(),
		CreatedAt: timestamppb.New(fb.CreatedAt),
	}, nil
}

func (h *PlayerServiceHandler) ResendPasswordResetOTP(ctx context.Context, req *playerv1.ResendPasswordResetOTPRequest) (*emptypb.Empty, error) {
	if req.PhoneNumber == "" {
		return nil, status.Errorf(codes.InvalidArgument, "phone number is required")
	}

	// Resend OTP for password reset
	err := h.otpService.GenerateAndSendOTP(ctx, req.PhoneNumber, "password_reset")
	if err != nil {
		return nil, h.handleError(err, "ResendPasswordResetOTP")
	}

	return &emptypb.Empty{}, nil
}
