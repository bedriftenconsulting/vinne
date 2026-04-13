package server

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/randco/randco-microservices/proto/agent/auth/v1"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/services"
)

type AgentAuthServer struct {
	pb.UnimplementedAgentAuthServiceServer
	authService services.AuthService
}

func NewAgentAuthServer(authService services.AuthService) *AgentAuthServer {
	return &AgentAuthServer{
		authService: authService,
	}
}

// Authentication operations

func (s *AgentAuthServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.AgentCode == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_code and password are required")
	}

	// Extract IP and user agent from metadata if available
	ipAddress := req.IpAddress
	if ipAddress == "" {
		ipAddress = "127.0.0.1" // Default to localhost for tests
	}
	userAgent := req.UserAgent
	if userAgent == "" {
		userAgent = "unknown"
	}

	// For now, assume this is always an agent login - we can expand later
	loginResp, err := s.authService.AgentLogin(ctx, &services.LoginRequest{
		Identifier: req.AgentCode,
		Password:   req.Password,
		UserType:   "AGENT",
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
	}

	// Create auth user info for the response
	authUser := &pb.AuthUserInfo{
		Id:        loginResp.UserID.String(),
		AgentCode: loginResp.UserCode,
		Email:     loginResp.Email,
		Phone:     loginResp.Phone,
		Role:      pb.AgentRole_AGENT_ROLE_AGENT,
		IsActive:  true,
	}

	return &pb.LoginResponse{
		AccessToken:      loginResp.AccessToken,
		RefreshToken:     loginResp.RefreshToken,
		ExpiresIn:        int32(loginResp.ExpiresIn),
		User:             authUser,
		DeviceRegistered: req.DeviceId != "",
	}, nil
}

func (s *AgentAuthServer) Logout(ctx context.Context, req *pb.LogoutRequest) (*emptypb.Empty, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	err := s.authService.Logout(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *AgentAuthServer) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	loginResp, err := s.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "token refresh failed: %v", err)
	}

	return &pb.RefreshTokenResponse{
		AccessToken:  loginResp.AccessToken,
		RefreshToken: loginResp.RefreshToken,
		ExpiresIn:    int32(loginResp.ExpiresIn),
	}, nil
}

func (s *AgentAuthServer) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	// This would typically validate JWT tokens - placeholder for now
	return &pb.ValidateTokenResponse{
		Valid:     true,
		AgentId:   "placeholder",
		AgentCode: "TEST001",
		Role:      pb.AgentRole_AGENT_ROLE_AGENT,
	}, nil
}

func (s *AgentAuthServer) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*emptypb.Empty, error) {
	if req.AgentId == "" || req.CurrentPassword == "" || req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "agent ID, current password, and new password are required")
	}

	userID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid agent ID: %v", err)
	}

	err = s.authService.ChangePassword(ctx, userID, "AGENT", req.CurrentPassword, req.NewPassword)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "password change failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

func (s *AgentAuthServer) ChangeRetailerPIN(ctx context.Context, req *pb.ChangeRetailerPINRequest) (*pb.ChangeRetailerPINResponse, error) {
	if req.RetailerId == "" || req.CurrentPin == "" || req.NewPin == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer ID, current pin, and new pin are required")
	}

	userID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}

	if req.NewPin != req.ConfirmNewPin {
		return nil, status.Error(codes.InvalidArgument, "new pin and confirm new pin do not match")
	}

	sessionsInvalidated, err := s.authService.ChangeRetailerPIN(ctx, userID, req.CurrentPin, req.NewPin, req.DeviceImei, req.RetailerCode, req.IpAddress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pin change failed: %v", err)
	}

	res := &pb.ChangeRetailerPINResponse{
		Success:             true,
		NextChangeAllowed:   timestamppb.New(time.Now().Add(24 * time.Hour)), // 24-hour cooldown
		SessionsInvalidated: int32(sessionsInvalidated),
		Message:             "Pin successfully changed",
	}

	return res, nil
}

func (s *AgentAuthServer) CreateAgentAuth(ctx context.Context, req *pb.CreateAgentAuthRequest) (*pb.CreateAgentAuthResponse, error) {
	if req.AgentId == "" || req.AgentCode == "" || req.Phone == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id, agent_code, phone, and password are required")
	}

	userID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid agent ID: %v", err)
	}

	err = s.authService.CreateAgentAuth(ctx, userID, req.AgentCode, req.Email, req.Phone, req.Password, req.CreatedBy)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create agent auth: %v", err)
	}

	return &pb.CreateAgentAuthResponse{
		AgentId:   req.AgentId,
		AgentCode: req.AgentCode,
		Success:   true,
		Message:   "Agent authentication credentials created successfully",
	}, nil
}

func (s *AgentAuthServer) RequestPasswordReset(ctx context.Context, req *pb.PasswordResetRequest) (*pb.PasswordResetResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier and channel are required")
	}

	serviceReq := &pb.PasswordResetRequest{
		Identifier: req.Identifier,
		Channel:    req.Channel,
		IpAddress:  req.IpAddress,
		UserAgent:  req.UserAgent,
		UserType:   req.UserType,
	}

	resp, err := s.authService.RequestPasswordReset(ctx, serviceReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "password reset request failed: %v", err)
	}

	return &pb.PasswordResetResponse{
		Success:          resp.Success,
		Message:          resp.Message,
		ResetToken:       resp.ResetToken,
		OtpExpirySeconds: int32(resp.OtpExpirySeconds),
	}, nil
}

func (s *AgentAuthServer) ValidateResetOTP(ctx context.Context, req *pb.ValidateResetOTPRequest) (*pb.ValidateResetOTPResponse, error) {
	if req.ResetToken == "" || req.OtpCode == "" {
		return nil, status.Error(codes.InvalidArgument, "reset_token and otp_code are required")
	}

	serviceReq := &pb.ValidateResetOTPRequest{
		ResetToken: req.ResetToken,
		OtpCode:    req.OtpCode,
	}

	resp, err := s.authService.ValidateResetOTP(ctx, serviceReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "OTP validation failed: %v", err)
	}

	return &pb.ValidateResetOTPResponse{
		Valid:   resp.Valid,
		Message: resp.Message,
	}, nil
}

func (s *AgentAuthServer) ConfirmPasswordReset(ctx context.Context, req *pb.ConfirmPasswordResetRequest) (*pb.ConfirmPasswordResetResponse, error) {
	if req.ResetToken == "" || req.OtpCode == "" || req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "reset_token, otp_code and new_password are required")
	}

	serviceReq := &pb.ConfirmPasswordResetRequest{
		ResetToken:      req.ResetToken,
		OtpCode:         req.OtpCode,
		NewPassword:     req.NewPassword,
		ConfirmPassword: req.ConfirmPassword,
	}

	resp, err := s.authService.ConfirmPasswordReset(ctx, serviceReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "password reset confirmation failed: %v", err)
	}

	return &pb.ConfirmPasswordResetResponse{
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}

func (s *AgentAuthServer) ResendPasswordResetOTP(ctx context.Context, req *pb.ResendPasswordResetOTPRequest) (*pb.ResendPasswordResetOTPResponse, error) {
	if req.Identifier == "" || req.ResetToken == "" || req.Channel == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier, reset_token and channel are required")
	}

	serviceReq := &pb.ResendPasswordResetOTPRequest{
		Identifier: req.Identifier,
		ResetToken: req.ResetToken,
		Channel:    req.Channel,
	}

	resp, err := s.authService.ResendPasswordResetOTP(ctx, serviceReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resend OTP failed: %v", err)
	}

	return &pb.ResendPasswordResetOTPResponse{
		Success:           resp.Success,
		Message:           resp.Message,
		NextResendSeconds: int32(resp.NextResendSeconds),
		OtpExpirySeconds:  int32(resp.OtpExpirySeconds),
	}, nil
}

// CreateRetailerAuth creates authentication credentials for a retailer
func (s *AgentAuthServer) CreateRetailerAuth(ctx context.Context, req *pb.CreateRetailerAuthRequest) (*pb.CreateRetailerAuthResponse, error) {
	if req.RetailerId == "" || req.RetailerCode == "" || req.Phone == "" || req.Pin == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_id, retailer_code, phone, and pin are required")
	}

	retailerID, err := uuid.Parse(req.RetailerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid retailer ID: %v", err)
	}

	err = s.authService.CreateRetailerAuth(ctx, retailerID, req.RetailerCode, req.Email, req.Phone, req.Pin, req.CreatedBy)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create retailer auth: %v", err)
	}

	return &pb.CreateRetailerAuthResponse{
		RetailerId:   req.RetailerId,
		RetailerCode: req.RetailerCode,
		Success:      true,
		Message:      "Retailer authentication credentials created successfully",
	}, nil
}

// RetailerPOSLogin handles retailer POS authentication with PIN
func (s *AgentAuthServer) RetailerPOSLogin(ctx context.Context, req *pb.RetailerPOSLoginRequest) (*pb.LoginResponse, error) {
	if req.RetailerCode == "" || req.Pin == "" {
		return nil, status.Error(codes.InvalidArgument, "retailer_code and pin are required")
	}

	// Call the service method for retailer POS login
	// Note: IP address and user agent from request are not currently used by the service
	loginResp, err := s.authService.RetailerPOSLogin(ctx, req.RetailerCode, req.Pin, req.DeviceImei)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
	}

	// Create auth user info for the response
	authUser := &pb.AuthUserInfo{
		Id:        loginResp.UserID.String(),
		AgentCode: loginResp.UserCode, // Will contain retailer code
		Email:     loginResp.Email,
		Phone:     loginResp.Phone,
		Role:      pb.AgentRole_AGENT_ROLE_UNSPECIFIED, // Retailers don't have agent roles
		IsActive:  true,
	}

	return &pb.LoginResponse{
		AccessToken:      loginResp.AccessToken,
		RefreshToken:     loginResp.RefreshToken,
		ExpiresIn:        int32(loginResp.ExpiresIn),
		User:             authUser,
		DeviceRegistered: req.DeviceImei != "",
	}, nil
}

// Device authentication methods (appropriate for auth service)
func (s *AgentAuthServer) RegisterDevice(ctx context.Context, req *pb.RegisterDeviceRequest) (*pb.AuthDevice, error) {
	return nil, status.Error(codes.Unimplemented, "method RegisterDevice not implemented yet")
}

func (s *AgentAuthServer) UpdateDevice(ctx context.Context, req *pb.UpdateDeviceRequest) (*pb.AuthDevice, error) {
	return nil, status.Error(codes.Unimplemented, "method UpdateDevice not implemented yet")
}

func (s *AgentAuthServer) GetDevice(ctx context.Context, req *pb.GetDeviceRequest) (*pb.AuthDevice, error) {
	return nil, status.Error(codes.Unimplemented, "method GetDevice not implemented yet")
}

func (s *AgentAuthServer) ListAgentDevices(ctx context.Context, req *pb.ListAgentDevicesRequest) (*pb.ListAgentDevicesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListAgentDevices not implemented yet")
}

func (s *AgentAuthServer) ActivateDevice(ctx context.Context, req *pb.ActivateDeviceRequest) (*pb.AuthDevice, error) {
	return nil, status.Error(codes.Unimplemented, "method ActivateDevice not implemented yet")
}

func (s *AgentAuthServer) DeactivateDevice(ctx context.Context, req *pb.DeactivateDeviceRequest) (*pb.AuthDevice, error) {
	return nil, status.Error(codes.Unimplemented, "method DeactivateDevice not implemented yet")
}

// Offline token methods (appropriate for auth service)
func (s *AgentAuthServer) GenerateOfflineToken(ctx context.Context, req *pb.GenerateOfflineTokenRequest) (*pb.OfflineTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GenerateOfflineToken not implemented yet")
}

func (s *AgentAuthServer) ValidateOfflineToken(ctx context.Context, req *pb.ValidateOfflineTokenRequest) (*pb.ValidateOfflineTokenResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ValidateOfflineToken not implemented yet")
}

func (s *AgentAuthServer) RevokeOfflineToken(ctx context.Context, req *pb.RevokeOfflineTokenRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method RevokeOfflineToken not implemented yet")
}

// Session management methods (appropriate for auth service)
func (s *AgentAuthServer) InvalidateAgentSessions(ctx context.Context, req *pb.InvalidateAgentSessionsRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method InvalidateAgentSessions not implemented yet")
}

func (s *AgentAuthServer) ListAgentSessions(ctx context.Context, req *pb.ListAgentSessionsRequest) (*pb.ListAgentSessionsResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid agent ID: %v", err)
	}

	sessions, err := s.authService.ListAgentSessions(ctx, agentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list agent sessions: %v", err)
	}

	return &pb.ListAgentSessionsResponse{
		Sessions: sessions,
	}, nil
}

func (s *AgentAuthServer) AgentCurrentSession(ctx context.Context, req *pb.ListAgentSessionsRequest) (*pb.AgentCurrentSessionResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid agent ID: %v", err)
	}

	session, err := s.authService.CurrentSession(ctx, agentID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list agent sessions: %v", err)
	}

	return &pb.AgentCurrentSessionResponse{
		Session: session,
	}, nil
}

func (s *AgentAuthServer) GetAgentPermissions(ctx context.Context, req *pb.GetAgentPermissionsRequest) (*pb.GetAgentPermissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetAgentPermissions not implemented yet")
}
