package services

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/config"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/models"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/repositories"
	"github.com/randco/randco-microservices/shared/common/errors"
	"github.com/randco/randco-microservices/shared/events"
	"github.com/randco/randco-microservices/shared/middleware/auth"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthService handles authentication logic
type AuthService interface {
	Login(ctx context.Context, req *adminmanagementv1.LoginRequest) (*adminmanagementv1.LoginResponse, error)
	Logout(ctx context.Context, req *adminmanagementv1.LogoutRequest) (*emptypb.Empty, error)
	RefreshToken(ctx context.Context, req *adminmanagementv1.RefreshTokenRequest) (*adminmanagementv1.RefreshTokenResponse, error)
	ChangePassword(ctx context.Context, req *adminmanagementv1.ChangePasswordRequest) (*emptypb.Empty, error)
	EnableMFA(ctx context.Context, req *adminmanagementv1.EnableMFARequest) (*adminmanagementv1.EnableMFAResponse, error)
	VerifyMFA(ctx context.Context, req *adminmanagementv1.VerifyMFARequest) (*emptypb.Empty, error)
	DisableMFA(ctx context.Context, req *adminmanagementv1.DisableMFARequest) (*emptypb.Empty, error)
}

type authService struct {
	adminUserRepo     repositories.AdminUserRepository
	adminUserAuthRepo repositories.AdminUserAuthRepository
	sessionRepo       repositories.SessionRepository
	auditRepo         repositories.AuditLogRepository
	jwtManager        *auth.JWTManager
	eventBus          events.EventBus
	config            *config.Config
}

func NewAuthService(
	adminUserRepo repositories.AdminUserRepository,
	adminUserAuthRepo repositories.AdminUserAuthRepository,
	sessionRepo repositories.SessionRepository,
	auditRepo repositories.AuditLogRepository,
	jwtManager *auth.JWTManager,
	eventBus events.EventBus,
	cfg *config.Config,
) AuthService {
	return &authService{
		adminUserRepo:     adminUserRepo,
		adminUserAuthRepo: adminUserAuthRepo,
		sessionRepo:       sessionRepo,
		auditRepo:         auditRepo,
		jwtManager:        jwtManager,
		eventBus:          eventBus,
		config:            cfg,
	}
}

func (s *authService) Login(ctx context.Context, req *adminmanagementv1.LoginRequest) (*adminmanagementv1.LoginResponse, error) {
	// Simple service-level tracing
	span := tracing.TraceService(ctx, "auth", "login")
	span.SetUser("", req.Email, "").SetRequestID(req.IpAddress)
	ctx = span.Context()
	defer func() { _ = span.End(nil) }()

	// Add login attempt event
	span.AddEvent("login.attempt",
		attribute.String("email", req.Email),
		attribute.String("ip", req.IpAddress))

	// Check if repositories are initialized
	if s.adminUserAuthRepo == nil {
		return nil, span.End(fmt.Errorf("admin user auth repository not initialized"))
	}

	// Verify credentials
	user, err := s.adminUserAuthRepo.VerifyCredentials(ctx, req.Email, req.Password)
	if err != nil {
		// Check if it's an inactive account error
		if err.Error() == "account is deactivated" {
			span.AddEvent("login.failed", attribute.String("reason", "account_deactivated"))
			return nil, span.End(errors.NewUnauthorizedError("account is not active"))
		}
		span.AddEvent("login.failed", attribute.String("reason", "invalid_credentials"))
		return nil, span.End(errors.NewUnauthorizedError("invalid credentials"))
	}

	// Update span with authenticated user info
	span.SetUser(user.ID.String(), user.Email, strings.Join(func() []string {
		roles := make([]string, len(user.Roles))
		for i, r := range user.Roles {
			roles[i] = r.Name
		}
		return roles
	}(), ","))

	log.Printf("[AuthService.Login] Credentials verified successfully for user ID: %s, email: %s", user.ID, user.Email)

	// MFA verification if enabled
	if user.MFAEnabled {
		if req.MfaToken == nil || *req.MfaToken == "" {
			return nil, errors.NewUnauthorizedError("MFA token required")
		}

		// Get the MFA secret from the database
		secret, err := s.adminUserAuthRepo.GetMFASecret(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get MFA secret: %w", err)
		}

		if secret == "" {
			return nil, errors.NewInternalError("MFA secret not found", fmt.Errorf("MFA enabled but secret is empty"))
		}

		// Verify the TOTP token
		valid := totp.Validate(*req.MfaToken, secret)
		if !valid {
			// Log failed MFA attempt
			s.logAuditEvent(ctx, &models.AuditLog{
				ID:             uuid.New(),
				AdminUserID:    user.ID,
				Action:         "admin.login.mfa_failed",
				RequestData:    map[string]interface{}{"email": req.Email},
				ResponseStatus: 401,
				CreatedAt:      time.Now(),
			})
			return nil, errors.NewUnauthorizedError("invalid MFA code")
		}
		log.Printf("[AuthService.Login] MFA verification successful for user: %s", user.ID)
	}

	// Extract role names for JWT
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}
	log.Printf("[AuthService.Login] User has %d roles: %v", len(roleNames), roleNames)

	// Check JWT manager
	if s.jwtManager == nil {
		log.Printf("[AuthService.Login] ERROR: JWT manager is nil")
		return nil, fmt.Errorf("JWT manager not initialized")
	}

	log.Printf("[AuthService.Login] Generating JWT tokens for user: %s", user.ID)
	// Generate JWT tokens
	accessToken, refreshToken, err := s.jwtManager.GenerateTokenPair(
		user.ID.String(),
		user.Email,
		user.Username,
		roleNames,
	)
	if err != nil {
		log.Printf("[AuthService.Login] Failed to generate tokens: %v", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}
	log.Printf("[AuthService.Login] JWT tokens generated successfully")

	// Create session
	session := &models.AdminSession{
		ID:           uuid.New(),
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    req.UserAgent,
		IPAddress:    req.IpAddress,
		ExpiresAt:    time.Now().Add(s.config.Security.SessionExpiry),
		IsActive:     true,
	}

	err = s.sessionRepo.Create(ctx, session)
	if err != nil {
		return nil, span.End(fmt.Errorf("failed to create session: %w", err))
	}

	// Update last login
	err = s.adminUserAuthRepo.UpdateLastLogin(ctx, user.ID, req.IpAddress)
	if err != nil {
		// Log error but don't fail login
		span.AddEvent("warning.last_login_update_failed", attribute.String("error", err.Error()))
	}

	// Publish login event
	if s.eventBus != nil {
		event := events.NewUserLoggedInEvent("service-admin-management", events.UserData{
			ID:       user.ID.String(),
			Email:    user.Email,
			Username: user.Username,
			Roles:    roleNames,
		})
		_ = s.eventBus.Publish(ctx, "admin.events", event)
	}

	// Log audit event
	s.logAuditEvent(ctx, &models.AuditLog{
		ID:             uuid.New(),
		AdminUserID:    user.ID,
		Action:         "admin.login",
		IPAddress:      req.IpAddress,
		UserAgent:      req.UserAgent,
		ResponseStatus: 200,
		CreatedAt:      time.Now(),
	})

	// Add successful login event
	span.AddEvent("login.success",
		attribute.String("user.id", user.ID.String()),
		attribute.String("session.id", session.ID.String()))

	// Convert user to protobuf format
	pbUser := modelUserToProto(user)

	return &adminmanagementv1.LoginResponse{
		User:         pbUser,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int32(s.config.Security.AccessTokenExpiry),
	}, span.End(nil)
}

func (s *authService) Logout(ctx context.Context, req *adminmanagementv1.LogoutRequest) (*emptypb.Empty, error) {
	// Simple service-level tracing
	span := tracing.TraceService(ctx, "auth", "logout")
	span.SetUser(req.UserId, "", "")
	ctx = span.Context()
	defer func() { _ = span.End(nil) }()

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, span.End(fmt.Errorf("invalid user ID: %w", err))
	}

	// Invalidate session
	err = s.sessionRepo.InvalidateByToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, span.End(fmt.Errorf("failed to invalidate session: %w", err))
	}

	// Publish logout event
	if s.eventBus != nil {
		event := events.NewBaseEvent(events.UserLoggedOut, "service-admin-management").
			WithUserID(req.UserId)
		_ = s.eventBus.Publish(ctx, "admin.events", event)
	}

	// Log audit event
	s.logAuditEvent(ctx, &models.AuditLog{
		ID:             uuid.New(),
		AdminUserID:    userID,
		Action:         "admin.logout",
		ResponseStatus: 200,
		CreatedAt:      time.Now(),
	})

	return &emptypb.Empty{}, nil
}

func (s *authService) RefreshToken(ctx context.Context, req *adminmanagementv1.RefreshTokenRequest) (*adminmanagementv1.RefreshTokenResponse, error) {
	// First check if session exists (this catches expired/inactive sessions with fake tokens)
	session, err := s.sessionRepo.GetByToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, errors.NewUnauthorizedError("session not found")
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return nil, errors.NewUnauthorizedError("session expired")
	}

	// Check if session is active
	if !session.IsActive {
		return nil, errors.NewUnauthorizedError("session inactive")
	}

	// Now validate the refresh token JWT
	claims, err := s.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil {
		return nil, errors.NewUnauthorizedError("invalid refresh token")
	}

	if claims.TokenType != "refresh" {
		return nil, errors.NewUnauthorizedError("invalid token type")
	}

	// Get fresh user data
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID in token: %w", err)
	}

	user, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil || !user.IsActive {
		return nil, errors.NewUnauthorizedError("user not found or inactive")
	}

	// Extract role names for JWT
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	// Generate new token pair
	newAccessToken, newRefreshToken, err := s.jwtManager.GenerateTokenPair(
		user.ID.String(),
		user.Email,
		user.Username,
		roleNames,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Update session with new refresh token
	session.RefreshToken = newRefreshToken
	session.ExpiresAt = time.Now().Add(s.config.Security.SessionExpiry)
	err = s.sessionRepo.Update(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	return &adminmanagementv1.RefreshTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int32(s.config.Security.AccessTokenExpiry),
	}, nil
}

func (s *authService) ChangePassword(ctx context.Context, req *adminmanagementv1.ChangePasswordRequest) (*emptypb.Empty, error) {
	log.Printf("ChangePassword request for user: %s", req.UserId)

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get user
	user, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.NewNotFoundError("user not found")
	}

	// Verify current password
	_, err = s.adminUserAuthRepo.VerifyCredentials(ctx, user.Email, req.CurrentPassword)
	if err != nil {
		return nil, errors.NewUnauthorizedError("current password is incorrect")
	}

	// Validate new password strength
	if err := validatePassword(req.NewPassword); err != nil {
		return nil, errors.NewValidationError(err.Error())
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password in database
	err = s.adminUserAuthRepo.UpdatePassword(ctx, userID, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to update password: %w", err)
	}

	// Invalidate all sessions to force re-login
	err = s.sessionRepo.InvalidateAllForUser(ctx, userID)
	if err != nil {
		// Log error but don't fail
		log.Printf("Failed to invalidate sessions: %v", err)
	}

	// Publish event
	if s.eventBus != nil {
		event := events.NewBaseEvent(events.UserPasswordChanged, "service-admin-management").
			WithUserID(req.UserId)
		_ = s.eventBus.Publish(ctx, "admin.events", event)
	}

	// Log audit event
	s.logAuditEvent(ctx, &models.AuditLog{
		ID:             uuid.New(),
		AdminUserID:    userID,
		Action:         "admin.password.changed",
		ResponseStatus: 200,
		CreatedAt:      time.Now(),
	})

	return &emptypb.Empty{}, nil
}

func (s *authService) EnableMFA(ctx context.Context, req *adminmanagementv1.EnableMFARequest) (*adminmanagementv1.EnableMFAResponse, error) {
	log.Printf("EnableMFA request for user: %s", req.UserId)

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get user to verify they exist
	user, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.NewNotFoundError("user not found")
	}

	// Generate secret
	secret := make([]byte, 20)
	_, err = rand.Read(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate secret: %w", err)
	}
	secretBase32 := base32.StdEncoding.EncodeToString(secret)

	// Generate TOTP key
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.config.Security.MFAIssuer,
		AccountName: user.Email,
		Secret:      secret,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// Generate backup codes
	backupCodes := make([]string, 10)
	for i := 0; i < 10; i++ {
		code := make([]byte, 4)
		if _, err := rand.Read(code); err != nil {
			return nil, fmt.Errorf("failed to generate backup code: %w", err)
		}
		backupCodes[i] = base32.StdEncoding.EncodeToString(code)[:8]
	}

	// Store the secret in the database
	err = s.adminUserAuthRepo.UpdateMFASecret(ctx, userID, secretBase32)
	if err != nil {
		return nil, fmt.Errorf("failed to store MFA secret: %w", err)
	}

	return &adminmanagementv1.EnableMFAResponse{
		Secret:      secretBase32,
		QrCodeUrl:   key.URL(),
		BackupCodes: backupCodes,
	}, nil
}

func (s *authService) VerifyMFA(ctx context.Context, req *adminmanagementv1.VerifyMFARequest) (*emptypb.Empty, error) {
	log.Printf("VerifyMFA request for user: %s", req.UserId)

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get the MFA secret from the database
	secret, err := s.adminUserAuthRepo.GetMFASecret(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA secret: %w", err)
	}

	if secret == "" {
		return nil, errors.NewBadRequestError("MFA not enabled for this user")
	}

	// Verify the TOTP token
	valid := totp.Validate(req.Token, secret)
	if !valid {
		return nil, errors.NewUnauthorizedError("invalid MFA code")
	}

	// Update user to enable MFA (in case this is first-time verification)
	err = s.adminUserAuthRepo.UpdateMFAStatus(ctx, userID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to enable MFA: %w", err)
	}

	// Log audit event
	s.logAuditEvent(ctx, &models.AuditLog{
		ID:             uuid.New(),
		AdminUserID:    userID,
		Action:         "admin.mfa.enabled",
		ResponseStatus: 200,
		CreatedAt:      time.Now(),
	})

	return &emptypb.Empty{}, nil
}

func (s *authService) DisableMFA(ctx context.Context, req *adminmanagementv1.DisableMFARequest) (*emptypb.Empty, error) {
	log.Printf("DisableMFA request for user: %s", req.UserId)

	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get user to verify MFA is enabled
	user, err := s.adminUserRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.NewNotFoundError("user not found")
	}

	if !user.MFAEnabled {
		return nil, errors.NewBadRequestError("MFA is not enabled")
	}

	// Verify the MFA token before allowing disable
	if req.Token == "" {
		return nil, errors.NewBadRequestError("MFA token required to disable MFA")
	}

	// Get the MFA secret from the database
	secret, err := s.adminUserAuthRepo.GetMFASecret(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA secret: %w", err)
	}

	if secret == "" {
		return nil, errors.NewInternalError("MFA secret not found", fmt.Errorf("MFA enabled but secret is empty"))
	}

	// Verify the TOTP token
	valid := totp.Validate(req.Token, secret)
	if !valid {
		// Log failed MFA disable attempt for security audit
		s.logAuditEvent(ctx, &models.AuditLog{
			ID:             uuid.New(),
			AdminUserID:    userID,
			Action:         "admin.mfa.disable_failed",
			RequestData:    map[string]interface{}{"reason": "invalid_mfa_token"},
			ResponseStatus: 401,
			CreatedAt:      time.Now(),
		})
		return nil, errors.NewUnauthorizedError("invalid MFA code")
	}

	// Update user to disable MFA
	err = s.adminUserAuthRepo.UpdateMFAStatus(ctx, userID, false)
	if err != nil {
		return nil, fmt.Errorf("failed to disable MFA: %w", err)
	}

	// Clear MFA secret
	err = s.adminUserAuthRepo.UpdateMFASecret(ctx, userID, "")
	if err != nil {
		// Log but don't fail - MFA is already disabled
		log.Printf("Failed to clear MFA secret: %v", err)
	}

	// Log audit event
	s.logAuditEvent(ctx, &models.AuditLog{
		ID:             uuid.New(),
		AdminUserID:    userID,
		Action:         "admin.mfa.disabled",
		ResponseStatus: 200,
		CreatedAt:      time.Now(),
	})

	return &emptypb.Empty{}, nil
}

func (s *authService) logAuditEvent(ctx context.Context, event *models.AuditLog) {
	err := s.auditRepo.Create(ctx, event)
	if err != nil {
		log.Printf("Failed to log audit event: %v", err)
	}
}

// Helper function to convert model user to proto user
func modelUserToProto(user *models.AdminUser) *adminmanagementv1.AdminUser {
	if user == nil {
		return nil
	}

	pbUser := &adminmanagementv1.AdminUser{
		Id:          user.ID.String(),
		Email:       user.Email,
		Username:    user.Username,
		MfaEnabled:  user.MFAEnabled,
		IsActive:    user.IsActive,
		IpWhitelist: user.IPWhitelist,
		CreatedAt:   timestamppb.New(user.CreatedAt),
		UpdatedAt:   timestamppb.New(user.UpdatedAt),
	}

	if user.FirstName != nil {
		pbUser.FirstName = user.FirstName
	}
	if user.LastName != nil {
		pbUser.LastName = user.LastName
	}
	if user.LastLogin != nil {
		pbUser.LastLogin = timestamppb.New(*user.LastLogin)
	}
	if user.LastLoginIP != nil {
		pbUser.LastLoginIp = user.LastLoginIP
	}

	// Convert roles
	pbUser.Roles = make([]*adminmanagementv1.Role, len(user.Roles))
	for i, role := range user.Roles {
		pbUser.Roles[i] = &adminmanagementv1.Role{
			Id:          role.ID.String(),
			Name:        role.Name,
			Description: role.Description,
			CreatedAt:   timestamppb.New(role.CreatedAt),
		}

		// Convert permissions
		pbUser.Roles[i].Permissions = make([]*adminmanagementv1.Permission, len(role.Permissions))
		for j, perm := range role.Permissions {
			pbUser.Roles[i].Permissions[j] = &adminmanagementv1.Permission{
				Id:       perm.ID.String(),
				Resource: perm.Resource,
				Action:   perm.Action,
			}
			if perm.Description != nil {
				pbUser.Roles[i].Permissions[j].Description = perm.Description
			}
		}
	}

	return pbUser
}

// validatePassword validates password strength
func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(password)

	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must contain at least one number")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}

	// Check for common weak passwords
	weakPasswords := []string{"password", "12345678", "qwerty", "abc123", "admin123"}
	lowerPassword := strings.ToLower(password)
	for _, weak := range weakPasswords {
		if strings.Contains(lowerPassword, weak) {
			return fmt.Errorf("password is too weak")
		}
	}

	return nil
}
