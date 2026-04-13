package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/shared/common/jwt"
	"github.com/randco/randco-microservices/shared/validation"
	"github.com/randco/service-player/internal/config"
	"github.com/randco/service-player/internal/models"
	"github.com/randco/service-player/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	playerRepo     repositories.PlayerRepository
	authRepo       repositories.PlayerAuthRepository
	sessionRepo    repositories.SessionRepository
	jwtService     jwt.Service
	securityConfig *config.SecurityConfig
}

func NewAuthService(
	playerRepo repositories.PlayerRepository,
	authRepo repositories.PlayerAuthRepository,
	sessionRepo repositories.SessionRepository,
	securityConfig *config.SecurityConfig,
) AuthService {
	jwtService := jwt.NewService(jwt.Config{
		AccessSecret:    securityConfig.JWTSecret,
		RefreshSecret:   securityConfig.JWTSecret + "-refresh",
		AccessDuration:  securityConfig.AccessTokenExpiry,
		RefreshDuration: securityConfig.RefreshTokenExpiry,
	})

	return &authService{
		playerRepo:     playerRepo,
		authRepo:       authRepo,
		sessionRepo:    sessionRepo,
		jwtService:     jwtService,
		securityConfig: securityConfig,
	}
}

func (s *authService) ValidateCredentials(ctx context.Context, req models.ValidateLoginRequest) (*models.Player, error) {
	normalizedPhone := validation.NormalizePhone(req.PhoneNumber)
	player, err := s.playerRepo.GetByPhoneNumber(ctx, normalizedPhone)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if player == nil {
		ctx := context.Background()
		failureReason := "player_not_found"
		_ = s.authRepo.RecordLoginAttempt(ctx, normalizedPhone, nil, req.DeviceID, req.Channel, req.IPAddress, "password", false, &failureReason)
		return nil, fmt.Errorf("invalid credentials")
	}

	if player.Status != models.PlayerStatusActive {
		return nil, fmt.Errorf("account is not active")
	}

	err = bcrypt.CompareHashAndPassword([]byte(player.PasswordHash), []byte(req.Password))
	if err != nil {
		ctx := context.Background()
		failureReason := "invalid_password"
		_ = s.authRepo.RecordLoginAttempt(ctx, normalizedPhone, &player.ID, req.DeviceID, req.Channel, req.IPAddress, "password", false, &failureReason)
		return nil, fmt.Errorf("invalid credentials")
	}

	_ = s.authRepo.RecordLoginAttempt(ctx, normalizedPhone, &player.ID, req.DeviceID, req.Channel, req.IPAddress, "password", true, nil)

	return player, nil
}

func (s *authService) GenerateTokens(ctx context.Context, player *models.Player, deviceID, channel, deviceType, appVersion, ipAddress, userAgent string) (*models.TokenPair, error) {
	now := time.Now()

	accessToken, err := s.generateAccessToken(player, deviceID, channel, now)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateRefreshToken(player, now)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	sessionReq := models.CreateSessionRequest{
		PlayerID:     player.ID,
		DeviceID:     deviceID,
		RefreshToken: refreshToken,
		Channel:      channel,
		DeviceType:   deviceType,
		AppVersion:   appVersion,
		ExpiresAt:    now.Add(s.securityConfig.RefreshTokenExpiry),
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
	}

	_, err = s.sessionRepo.Create(ctx, sessionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(s.securityConfig.AccessTokenExpiry),
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*models.TokenPair, error) {
	// Get session by refresh token
	session, err := s.sessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Check if session exists
	if session == nil {
		return nil, fmt.Errorf("invalid refresh token: session not found")
	}

	// Check if session is active and not expired
	if !session.IsActive || time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("refresh token expired")
	}

	// Get player
	player, err := s.playerRepo.GetByID(ctx, session.PlayerID)
	if err != nil {
		return nil, fmt.Errorf("player not found")
	}

	// Check if player is still active
	if player.Status != models.PlayerStatusActive {
		return nil, fmt.Errorf("account is not active")
	}

	now := time.Now()
	newAccessToken, err := s.generateAccessToken(player, session.DeviceID, session.Channel, now)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.generateRefreshToken(player, now)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	updateReq := models.UpdateSessionRequest{
		ID:         session.ID,
		LastUsedAt: now,
	}

	_, err = s.sessionRepo.Update(ctx, updateReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return &models.TokenPair{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    now.Add(s.securityConfig.AccessTokenExpiry),
	}, nil
}

func (s *authService) RevokeToken(ctx context.Context, refreshToken string) error {
	session, err := s.sessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		return fmt.Errorf("invalid refresh token")
	}

	now := time.Now()
	reason := "logout"
	updateReq := models.UpdateSessionRequest{
		ID:            session.ID,
		IsActive:      false,
		RevokedAt:     now,
		RevokedReason: reason,
	}

	_, err = s.sessionRepo.Update(ctx, updateReq)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	return nil
}

func (s *authService) ValidateToken(ctx context.Context, token string) (*models.TokenClaims, error) {
	claims, err := s.jwtService.ValidateAccessToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	playerID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid player ID format")
	}

	return &models.TokenClaims{
		PlayerID: playerID,
		Phone:    claims.Phone,
		Channel:  claims.Channel,
		DeviceID: claims.DeviceID,
		Exp:      claims.Exp,
		Iat:      claims.Iat,
		Jti:      claims.JTI,
	}, nil
}

func (s *authService) generateAccessToken(player *models.Player, deviceID, channel string, now time.Time) (string, error) {
	claims := jwt.Claims{
		UserID:   player.ID.String(),
		Email:    player.PhoneNumber,
		Username: player.PhoneNumber,
		Phone:    player.PhoneNumber,
		Channel:  channel,
		DeviceID: deviceID,
		JTI:      uuid.New().String(),
		Issuer:   s.securityConfig.JWTIssuer,
		Roles:    []string{"player"},
		Exp:      now.Add(s.securityConfig.AccessTokenExpiry).Unix(),
		Iat:      now.Unix(),
	}

	return s.jwtService.GenerateAccessToken(claims)
}

func (s *authService) generateRefreshToken(player *models.Player, now time.Time) (string, error) {
	claims := jwt.Claims{
		UserID:   player.ID.String(),
		Email:    player.PhoneNumber,
		Username: player.PhoneNumber,
		Phone:    player.PhoneNumber,
		JTI:      uuid.New().String(),
		Issuer:   s.securityConfig.JWTIssuer,
		Roles:    []string{"player"},
		Exp:      now.Add(s.securityConfig.RefreshTokenExpiry).Unix(),
		Iat:      now.Unix(),
	}

	return s.jwtService.GenerateRefreshToken(claims)
}

// RequestPasswordReset initiates password reset by sending OTP to phone number
func (s *authService) RequestPasswordReset(ctx context.Context, phoneNumber string) (string, error) {
	// Normalize phone number
	normalizedPhone := validation.NormalizePhone(phoneNumber)

	// Check if player exists
	player, err := s.playerRepo.GetByPhoneNumber(ctx, normalizedPhone)
	if err != nil {
		return "", fmt.Errorf("phone number not registered")
	}

	if player == nil {
		return "", fmt.Errorf("phone number not registered")
	}

	// Check if player is active
	if player.Status != models.PlayerStatusActive {
		return "", fmt.Errorf("account is not active")
	}

	return player.ID.String(), nil
}

// ValidatePasswordResetOTP verifies the OTP for password reset
func (s *authService) ValidatePasswordResetOTP(ctx context.Context, sessionID, otp string) error {
	// The sessionID is actually the player ID
	// The OTP service verifies the OTP using session ID and purpose
	// For password reset, we don't need additional validation here
	// The OTP verification is handled by the OTPService in the handler
	return nil
}

// ConfirmPasswordReset completes password reset by updating the password
func (s *authService) ConfirmPasswordReset(ctx context.Context, playerID uuid.UUID, newPassword, otp string) error {
	// Get player to verify it exists
	player, err := s.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("player not found: %w", err)
	}

	if player == nil {
		return fmt.Errorf("player not found")
	}

	// Validate new password
	if len(newPassword) < 6 {
		return fmt.Errorf("new password must be at least 6 characters")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password
	err = s.authRepo.UpdatePassword(ctx, playerID, string(hashedPassword))
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Revoke all active sessions for the player after password reset
	_ = s.sessionRepo.RevokeAllPlayerSessions(context.Background(), playerID, "password_reset")

	return nil
}

func (s *authService) SubmitFeedback(ctx context.Context, req models.CreateFeedbackRequest) (*models.PlayerFeedback, error) {
	if req.PlayerID == uuid.Nil {
		return nil, fmt.Errorf("player ID is required")
	}
	if req.FullName == "" {
		return nil, fmt.Errorf("full name is required")
	}
	if req.Message == "" {
		return nil, fmt.Errorf("message is required")
	}
	return s.authRepo.CreateFeedback(ctx, req)
}
