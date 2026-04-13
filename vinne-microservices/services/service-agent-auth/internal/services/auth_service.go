package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/randco/randco-microservices/shared/common/jwt"
	"github.com/randco/randco-microservices/shared/middleware/tracing"
	"github.com/randco/randco-microservices/shared/validation"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/randco/randco-microservices/proto/agent/auth/v1"
	notificationv1 "github.com/randco/randco-microservices/proto/notification/v1"

	"github.com/randco/randco-microservices/services/service-agent-auth/internal/clients"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/models"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/repositories"
)

// txKey is a custom type for context keys to avoid collisions
type txKey struct{}

// AuthService defines the interface for authentication operations
type AuthService interface {
	AgentLogin(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
	RetailerLogin(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
	RetailerPOSLogin(ctx context.Context, retailerCode, pin, deviceIMEI string) (*LoginResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, userType, oldPassword, newPassword string) error
	ChangeRetailerPIN(ctx context.Context, userID uuid.UUID, oldPassword, newPassword, deviceIMEI, retailerCode, ipAddress string) (int64, error)
	CreateAgentAuth(ctx context.Context, agentID uuid.UUID, agentCode, email, phone, password, createdBy string) error
	CreateRetailerAuth(ctx context.Context, retailerID uuid.UUID, retailerCode, email, phone, pin, createdBy string) error
	RequestPasswordReset(ctx context.Context, req *pb.PasswordResetRequest) (*pb.PasswordResetResponse, error)
	ValidateResetOTP(ctx context.Context, req *pb.ValidateResetOTPRequest) (*pb.ValidateResetOTPResponse, error)
	ConfirmPasswordReset(ctx context.Context, req *pb.ConfirmPasswordResetRequest) (*pb.ConfirmPasswordResetResponse, error)
	ResendPasswordResetOTP(ctx context.Context, req *pb.ResendPasswordResetOTPRequest) (*pb.ResendPasswordResetOTPResponse, error)
	ListAgentSessions(ctx context.Context, agentID uuid.UUID) ([]*pb.Session, error)
	CurrentSession(ctx context.Context, agentID uuid.UUID) (*pb.Session, error)
}

// authService implements the AuthService interface
type authService struct {
	authRepo                repositories.AuthRepository
	sessionRepo             repositories.SessionRepository
	tokenRepo               repositories.TokenRepositoryInterface
	offlineTokenRepo        repositories.OfflineTokenRepository
	accessTokenExpiry       time.Duration
	refreshTokenExpiry      time.Duration
	maxFailedLogins         int
	lockoutDuration         time.Duration
	agentNotificationClient clients.AgentNotificationClient
	jwtService              jwt.Service
}

// NewAuthService creates a new authentication service
func NewAuthService(
	authRepo repositories.AuthRepository,
	sessionRepo repositories.SessionRepository,
	tokenRepo repositories.TokenRepositoryInterface,
	offlineTokenRepo repositories.OfflineTokenRepository,
	accessTokenExpiry time.Duration,
	refreshTokenExpiry time.Duration,
	maxFailedLogins int,
	lockoutDuration time.Duration,
	agentNotificationClient clients.AgentNotificationClient,
	jwtService jwt.Service,

) AuthService {
	return &authService{
		authRepo:                authRepo,
		sessionRepo:             sessionRepo,
		tokenRepo:               tokenRepo,
		offlineTokenRepo:        offlineTokenRepo,
		accessTokenExpiry:       accessTokenExpiry,
		refreshTokenExpiry:      refreshTokenExpiry,
		maxFailedLogins:         maxFailedLogins,
		lockoutDuration:         lockoutDuration,
		agentNotificationClient: agentNotificationClient,
		jwtService:              jwtService,
	}
}

// LoginRequest represents a login attempt
type LoginRequest struct {
	Identifier string // email, phone, or code
	Password   string
	UserType   string // AGENT or RETAILER
	DeviceID   string // optional, for tracking
	IPAddress  string
	UserAgent  string
}

// LoginResponse contains tokens and user info
type LoginResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	UserID       uuid.UUID
	UserType     string
	UserCode     string
	Email        string
	Phone        string
}

// AgentLogin authenticates an agent
func (s *authService) AgentLogin(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	// Start service tracing
	span := tracing.TraceService(ctx, "auth", "agent.login")
	span.SetUser("", req.Identifier, "").SetRequestID(req.IPAddress)
	ctx = span.Context()
	defer func() { _ = span.End(nil) }()

	// Add login attempt event
	span.AddEvent("agent.login.attempt", attribute.String("identifier", req.Identifier))

	// Try to find agent by email or code
	var user *repositories.AuthUser
	var err error

	// Check identifier type - prioritize phone number
	if validation.ValidatePhone(req.Identifier) == nil {
		// It's a valid phone number, normalize it
		normalizedPhone := validation.NormalizePhone(req.Identifier)
		user, err = s.authRepo.GetAgentByPhone(ctx, normalizedPhone)
	} else if validation.ValidateEmail(req.Identifier) == nil {
		// It's a valid email
		user, err = s.authRepo.GetAgentByEmail(ctx, req.Identifier)
	} else {
		// Assume it's an agent code
		user, err = s.authRepo.GetAgentByCode(ctx, req.Identifier)
	}

	if err != nil {
		span.AddEvent("agent.login.failed", attribute.String("reason", "user_not_found"))
		return nil, span.End(fmt.Errorf("invalid credentials"))
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		span.AddEvent("agent.login.failed", attribute.String("reason", "account_locked"))
		return nil, span.End(fmt.Errorf("account locked until %v", user.LockedUntil))
	}

	// Check if account is active
	if !user.IsActive {
		span.AddEvent("agent.login.failed", attribute.String("reason", "account_inactive"))
		return nil, span.End(fmt.Errorf("account is inactive"))
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		// Handle failed login attempt in a transaction to ensure atomicity
		var attempts int
		var locked bool

		err := s.authRepo.WithTx(ctx, func(tx *sqlx.Tx) error {
			// Pass the transaction context to ensure operations use the same transaction
			txCtx := context.WithValue(ctx, txKey{}, tx)

			// Increment failed login attempts
			var incrementErr error
			attempts, incrementErr = s.authRepo.IncrementAgentFailedLogin(txCtx, user.ID)
			if incrementErr != nil {
				return fmt.Errorf("failed to increment login attempts: %w", incrementErr)
			}

			// Lock account if max attempts reached
			if attempts >= s.maxFailedLogins {
				lockUntil := time.Now().Add(s.lockoutDuration)
				if lockErr := s.authRepo.LockAgentAccount(txCtx, user.ID, lockUntil); lockErr != nil {
					return fmt.Errorf("failed to lock account: %w", lockErr)
				}
				locked = true
			}
			return nil
		})

		if err != nil {
			span.AddEvent("agent.login.transaction.failed", attribute.String("error", err.Error()))
			// Even if transaction fails, we still reject the login
			return nil, span.End(fmt.Errorf("invalid credentials"))
		}

		if locked {
			span.AddEvent("agent.login.locked", attribute.Int("attempts", attempts))
			return nil, span.End(fmt.Errorf("account locked due to too many failed attempts"))
		}

		span.AddEvent("agent.login.failed", attribute.String("reason", "invalid_password"))
		return nil, span.End(fmt.Errorf("invalid credentials"))
	}

	// Update last login
	if err := s.authRepo.UpdateAgentLastLogin(ctx, user.ID); err != nil {
		// Log error but don't fail the login
		span.AddEvent("agent.last_login_update.failed", attribute.String("error", err.Error()))
	}

	// Add success event
	span.AddEvent("agent.login.success", attribute.String("agent_id", user.ID.String()))

	// Generate tokens
	accessToken, err := s.generateAccessToken(user.ID, "AGENT", user.Code, *user.Phone)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		UserType:     models.UserTypeAgent,
		RefreshToken: refreshToken,
		UserAgent:    req.UserAgent,
		IPAddress:    req.IPAddress,
		DeviceID:     req.DeviceID,
		IsActive:     true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(s.refreshTokenExpiry),
		LastActivity: time.Now(),
	}

	err = s.sessionRepo.Create(ctx, session)
	if err != nil {
		return nil, span.End(fmt.Errorf("failed to create session: %w", err))
	}

	// Get email and phone, handling nil pointers
	email := ""
	if user.Email != nil {
		email = *user.Email
	}
	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTokenExpiry.Seconds()),
		UserID:       user.ID,
		UserType:     "AGENT",
		UserCode:     user.Code,
		Email:        email,
		Phone:        phone,
	}, span.End(nil)
}

func (s *authService) RequestPasswordReset(ctx context.Context, req *pb.PasswordResetRequest) (*pb.PasswordResetResponse, error) {
	span := tracing.TraceService(ctx, "auth", "password.reset.request")
	defer func() { _ = span.End(nil) }()

	var user *repositories.AuthUser
	var err error
	var channel string

	if validation.ValidateEmail(req.Identifier) == nil {
		channel = "email"
		if req.UserType == "AGENT" {
			user, err = s.authRepo.GetAgentByEmail(ctx, req.Identifier)
		} else {
			user, err = s.authRepo.GetRetailerByEmail(ctx, req.Identifier)
		}
	} else if validation.ValidateAgentCode(req.Identifier) == nil {
		channel = "sms"
		if req.UserType == "AGENT" {
			user, err = s.authRepo.GetAgentByCode(ctx, req.Identifier)
		} else {
			user, err = s.authRepo.GetRetailerByCode(ctx, req.Identifier)
		}

	} else if validation.ValidatePhone(req.Identifier) == nil {
		channel = "sms"
		if req.UserType == "AGENT" {
			normalizedPhone := validation.NormalizePhone(req.Identifier)
			user, err = s.authRepo.GetAgentByPhone(ctx, normalizedPhone)
		} else {
			user, err = s.authRepo.GetRetailerByPhone(ctx, req.Identifier)
		}
	} else {
		log.Print("Invalid identifier type")
		return nil, fmt.Errorf("invalid identifier format: must be email, code or phone")
	}

	if err != nil {
		log.Printf("User lookup error: %v\n", err)
	}

	if user == nil {
		return &pb.PasswordResetResponse{
			Message: "If the account exists, an OTP has been sent",
		}, nil
	}

	count, err := s.authRepo.GetRecentResetAttempts(ctx, user.ID, time.Minute, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check reset attempts: %w", err)
	}

	if count >= 3 {
		return &pb.PasswordResetResponse{
			Success: false,
			Message: "Only 3 requests per hour allowed",
		}, nil
	}

	otpNum, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}
	otp := fmt.Sprintf("%06d", otpNum.Int64())

	otpHash, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash OTP: %w", err)
	}

	resetToken := uuid.NewString()

	ttl := 5 * time.Minute
	err = s.tokenRepo.StoreResetToken(ctx, resetToken, user.ID.String(), string(otpHash), ttl)

	if err != nil {
		return nil, fmt.Errorf("failed to store OTP: %w", err)
	}

	message := fmt.Sprintf("Your password reset code is: %s. This code will expire in 5 minutes. For your security, do not share this code with anyone.", otp)
	idempotencyKey := uuid.New().String()

	switch channel {
	case "email":
		err = s.sendEmail(ctx, req.Identifier, message, idempotencyKey)
		if err != nil {
			log.Printf("Failed to send email: %v", err)
		}
	case "sms":
		err = s.sendSMS(ctx, req.Identifier, message, idempotencyKey)
		if err != nil {
			log.Printf("Failed to send sms: %v", err)
		}
	}

	span.AddEvent("otp_sent", attribute.String("method", channel))

	s.logPasswordReset(ctx, user.ID, req.IpAddress, "requested", channel, "", 0)

	return &pb.PasswordResetResponse{
		Success:          true,
		Message:          "If the account exists, an OTP has been sent",
		ResetToken:       resetToken,
		OtpExpirySeconds: 300, // 5 minutes
	}, nil

}

func (s *authService) ValidateResetOTP(ctx context.Context, req *pb.ValidateResetOTPRequest) (*pb.ValidateResetOTPResponse, error) {
	span := tracing.TraceService(ctx, "auth", "password.reset.validate_otp")
	defer func() { _ = span.End(nil) }()

	// Fetch stored OTP hash
	tokenData, err := s.tokenRepo.GetResetToken(ctx, req.ResetToken)
	if err != nil {
		return &pb.ValidateResetOTPResponse{
			Valid:   false,
			Message: "Invalid or expired token",
		}, nil
	}

	agentID, err := uuid.Parse(tokenData.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent ID from token data: %w", err)
	}

	count, err := s.authRepo.GetRecentResetAttempts(ctx, agentID, 24*time.Hour, &req.ResetToken)
	if err != nil {
		return nil, fmt.Errorf("failed to check reset attempts: %w", err)
	}

	if count >= 3 {
		return &pb.ValidateResetOTPResponse{
			Valid:   false,
			Message: "You have exceeded the maximum number of otp attempts. Please request a new otp",
		}, nil
	}

	// // Compare provided OTP with stored hash
	err = bcrypt.CompareHashAndPassword([]byte(tokenData.Token), []byte(req.OtpCode))
	if err != nil {
		// OTP is invalid

		s.logPasswordReset(ctx, agentID, "", "invalid_otp", "", req.ResetToken, int32(count+1))

		remaining := int32(3 - (count + 1))
		if remaining < 0 {
			remaining = 0

		}

		return &pb.ValidateResetOTPResponse{
			Valid:             false,
			Message:           "Invalid OTP",
			RemainingAttempts: remaining,
		}, nil
	}

	s.logPasswordReset(ctx, agentID, "", "validated", "", req.ResetToken, int32(count))

	return &pb.ValidateResetOTPResponse{
		Valid:             true,
		Message:           "OTP is valid",
		RemainingAttempts: int32(3 - count),
	}, nil

}

func (s *authService) ConfirmPasswordReset(ctx context.Context, req *pb.ConfirmPasswordResetRequest) (*pb.ConfirmPasswordResetResponse, error) {

	span := tracing.TraceService(ctx, "auth", "password.reset.confirm")
	defer func() { _ = span.End(nil) }()

	if req.ResetToken == "" || req.NewPassword == "" || req.OtpCode == "" {
		return &pb.ConfirmPasswordResetResponse{
			Success: false,
			Message: "Missing required fields",
		}, nil
	}

	if req.NewPassword != req.ConfirmPassword {
		return &pb.ConfirmPasswordResetResponse{
			Success: false,
			Message: "Passwords do not match",
		}, nil
	}

	if err := validation.ValidatePassword(req.NewPassword); err != nil {
		return &pb.ConfirmPasswordResetResponse{
			Success: false,
			Message: "Password  must be at least 8 characters and include uppercase, lowercase, number, and special character",
		}, nil
	}

	tokenData, err := s.tokenRepo.GetResetToken(ctx, req.ResetToken)
	if err != nil {
		return &pb.ConfirmPasswordResetResponse{
			Success: false,
			Message: "Invalid or expired token",
		}, nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(tokenData.Token), []byte(req.OtpCode))
	if err != nil {
		return &pb.ConfirmPasswordResetResponse{
			Success: false,
			Message: "Invalid token",
		}, nil
	}

	parsedAgentID, err := uuid.Parse(tokenData.AgentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agentID format: %w", err)
	}

	recentPasswords, err := s.authRepo.GetPasswordHistory(ctx, parsedAgentID, 5)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recent passwords: %w", err)
	}

	for _, oldHash := range recentPasswords {
		err := bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(req.NewPassword))
		if err == nil {
			return &pb.ConfirmPasswordResetResponse{
				Success: false,
				Message: "New password matches a recently used password. Please try again.",
			}, nil
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password in database
	err = s.authRepo.UpdateAgentPassword(ctx, parsedAgentID, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to update password: %w", err)
	}

	// Add new password to history
	if err := s.authRepo.AddPasswordToHistory(ctx, parsedAgentID, string(hashedPassword)); err != nil {
		return nil, fmt.Errorf("failed to add password to history: %w", err)
	}

	agentID, err := uuid.Parse(tokenData.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent ID from token data: %w", err)
	}
	s.logPasswordReset(ctx, agentID, "", "confirmed", "", req.ResetToken, 0)

	// delete token from redis
	if err := s.tokenRepo.DeleteResetToken(ctx, req.ResetToken); err != nil {
		fmt.Printf("failed to delete reset token: %v\n", err)
	}

	// revoke all sessions active sessions
	if _, err := s.sessionRepo.RevokeAllForUser(ctx, parsedAgentID); err != nil {
		fmt.Printf("failed to revoke sessions for agent %s: %v\n", parsedAgentID, err)
	}

	return &pb.ConfirmPasswordResetResponse{
		Success: true,
		Message: "Password reset successful",
	}, nil

}

func (s *authService) ResendPasswordResetOTP(ctx context.Context, req *pb.ResendPasswordResetOTPRequest) (*pb.ResendPasswordResetOTPResponse, error) {
	span := tracing.TraceService(ctx, "auth", "password.reset.resend")
	defer func() { _ = span.End(nil) }()

	// validate input
	if req.Identifier == "" || req.Channel == "" || req.ResetToken == "" {
		return &pb.ResendPasswordResetOTPResponse{
			Success: false,
			Message: "Missing required fields",
		}, nil
	}

	var user *repositories.AuthUser
	var err error

	// Find user by identifier
	if validation.ValidateEmail(req.Identifier) == nil {
		// It's a valid email
		user, err = s.authRepo.GetAgentByEmail(ctx, req.Identifier)

	} else if validation.ValidateAgentCode(req.Identifier) == nil {
		// Assume it's a code
		user, err = s.authRepo.GetAgentByCode(ctx, req.Identifier)

	} else if validation.ValidatePhone(req.Identifier) == nil {
		user, err = s.authRepo.GetAgentByPhone(ctx, req.Identifier)
	} else {
		fmt.Println("Invalid identifier type")
		return nil, fmt.Errorf("invalid identifier format: must be email, code or phone")
	}

	if err != nil {
		fmt.Printf("User lookup error: %v\n", err)
	}

	if user == nil {
		// To prevent user enumeration, respond with success even if user not found
		return &pb.ResendPasswordResetOTPResponse{
			Message: "If the account exists, an OTP has been sent",
		}, nil
	}

	// check if token exists in redis

	tokenData, err := s.tokenRepo.GetResetToken(ctx, req.ResetToken)
	if err != nil {
		return &pb.ResendPasswordResetOTPResponse{
			Success: false,
			Message: "invalid or expired reset token",
		}, nil
	}

	agentID, err := uuid.Parse(tokenData.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent ID from token data: %w", err)
	}

	attempts, err := s.authRepo.GetRecentResetAttempts(ctx, agentID, 24*time.Hour, &req.ResetToken)
	if err != nil {
		return nil, err
	}

	if attempts >= 3 {
		return &pb.ResendPasswordResetOTPResponse{
			Success: false,
			Message: "You have exceeded the maximum number of OTP resends. Please request a new password reset otp.",
		}, nil
	}

	// dont allow immediate recent to prevent abuse
	if time.Since(tokenData.CreatedAt) < 60*time.Second {
		return &pb.ResendPasswordResetOTPResponse{
			Success:           false,
			Message:           "An OTP was recently sent. Please wait before requesting a new one",
			NextResendSeconds: 60,
		}, nil
	}

	// generate new otp
	otpNum, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return nil, fmt.Errorf("failed to generate otp: %w", err)
	}
	otp := fmt.Sprintf("%06d", otpNum.Int64())

	// hash before storing
	hashedOTP, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash OTP %w", err)
	}

	// store otp in redis
	if tokenData.AgentID == "" {
		return nil, fmt.Errorf("missing agent ID for reset token")
	}
	if err := s.tokenRepo.StoreResetToken(ctx, req.ResetToken, tokenData.AgentID, string(hashedOTP), 5*time.Minute); err != nil {
		return nil, fmt.Errorf("failed to store OTP %w", err)
	}

	channel := req.Channel
	if channel != "email" && channel != "sms" {
		return &pb.ResendPasswordResetOTPResponse{
			Success: false,
			Message: "Channel must be 'email' or 'sms'",
		}, nil

	}
	// send OTP via chosen channel (email or sms)
	message := fmt.Sprintf("This is your reset otp: %v. Do not share it with anyone", otp)
	idempotencyKey := uuid.New().String()

	if channel == "email" {
		err = s.sendEmail(ctx, req.Identifier, message, idempotencyKey)
		if err != nil {
			log.Printf("Failed to send email: %v", err)
		}
	}

	if channel == "sms" {
		err = s.sendSMS(ctx, req.Identifier, message, idempotencyKey)
		if err != nil {
			log.Printf("Failed to send sms: %v", err)
		}
	}

	span.AddEvent("otp_sent", attribute.String("method", channel))

	// log password reset resend

	s.logPasswordReset(ctx, agentID, "", "resend", channel, req.ResetToken, 0)

	return &pb.ResendPasswordResetOTPResponse{
		Success:           true,
		Message:           "A new OTP has been sent",
		OtpExpirySeconds:  300,
		NextResendSeconds: 60,
	}, nil

}

// RetailerLogin authenticates a retailer
func (s *authService) RetailerLogin(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	span := tracing.TraceService(ctx, "auth", "retailer.login")
	defer func() { _ = span.End(nil) }()

	// Try to find retailer by email, phone, or code
	var user *repositories.AuthUser
	var err error

	// Check identifier type
	if validation.ValidateEmail(req.Identifier) == nil {
		// It's a valid email
		user, err = s.authRepo.GetRetailerByEmail(ctx, req.Identifier)
	} else if validation.ValidatePhone(req.Identifier) == nil {
		// It's a valid phone number, normalize it
		normalizedPhone := validation.NormalizePhone(req.Identifier)
		user, err = s.authRepo.GetRetailerByPhone(ctx, normalizedPhone)
	} else {
		// Assume it's a retailer code
		user, err = s.authRepo.GetRetailerByCode(ctx, req.Identifier)
	}

	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, fmt.Errorf("account locked until %v", user.LockedUntil)
	}

	// Check if account is active
	if !user.IsActive {
		return nil, fmt.Errorf("account is inactive")
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		// Handle failed login attempt in a transaction to ensure atomicity
		var attempts int
		var locked bool

		err := s.authRepo.WithTx(ctx, func(tx *sqlx.Tx) error {
			// Pass the transaction context to ensure operations use the same transaction
			txCtx := context.WithValue(ctx, txKey{}, tx)

			// Increment failed login attempts
			var incrementErr error
			attempts, incrementErr = s.authRepo.IncrementRetailerFailedLogin(txCtx, user.ID)
			if incrementErr != nil {
				return fmt.Errorf("failed to increment login attempts: %w", incrementErr)
			}

			// Lock account if max attempts reached
			if attempts >= s.maxFailedLogins {
				lockUntil := time.Now().Add(s.lockoutDuration)
				if lockErr := s.authRepo.LockRetailerAccount(txCtx, user.ID, lockUntil); lockErr != nil {
					return fmt.Errorf("failed to lock account: %w", lockErr)
				}
				locked = true
			}
			return nil
		})

		if err != nil {
			span.AddEvent("retailer.login.transaction.failed", attribute.String("error", err.Error()))
			// Even if transaction fails, we still reject the login
			return nil, fmt.Errorf("invalid credentials")
		}

		if locked {
			span.AddEvent("retailer.login.locked", attribute.Int("attempts", attempts))
			return nil, fmt.Errorf("account locked due to too many failed attempts")
		}

		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login
	if err := s.authRepo.UpdateRetailerLastLogin(ctx, user.ID); err != nil {
		// Log error but don't fail the login
		span.AddEvent("retailer.last_login_update.failed", attribute.String("error", err.Error()))
	}

	// Generate tokens
	accessToken, err := s.generateAccessToken(user.ID, "RETAILER", user.Code, *user.Phone)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		UserType:     models.UserTypeRetailer,
		RefreshToken: refreshToken,
		UserAgent:    req.UserAgent,
		IPAddress:    req.IPAddress,
		DeviceID:     req.DeviceID,
		IsActive:     true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(s.refreshTokenExpiry),
		LastActivity: time.Now(),
	}

	err = s.sessionRepo.Create(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Get email and phone, handling nil pointers
	email := ""
	if user.Email != nil {
		email = *user.Email
	}
	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTokenExpiry.Seconds()),
		UserID:       user.ID,
		UserType:     "RETAILER",
		UserCode:     user.Code,
		Email:        email,
		Phone:        phone,
	}, nil
}

// RetailerPOSLogin authenticates a retailer with PIN for POS devices
func (s *authService) RetailerPOSLogin(ctx context.Context, retailerCode, pin, deviceIMEI string) (*LoginResponse, error) {
	span := tracing.TraceService(ctx, "auth", "retailer.pos_login")
	defer func() { _ = span.End(nil) }()

	// Get retailer by code
	user, err := s.authRepo.GetRetailerByCode(ctx, retailerCode)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, fmt.Errorf("account locked until %v", user.LockedUntil)
	}

	// Check if account is active
	if !user.IsActive {
		return nil, fmt.Errorf("account is inactive")
	}

	// Verify PIN
	if user.PinHash == nil {
		return nil, fmt.Errorf("PIN not set for this retailer")
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.PinHash), []byte(pin))
	if err != nil {
		// Handle failed login attempt in a transaction to ensure atomicity
		var attempts int
		var locked bool

		err := s.authRepo.WithTx(ctx, func(tx *sqlx.Tx) error {
			// Pass the transaction context to ensure operations use the same transaction
			txCtx := context.WithValue(ctx, txKey{}, tx)

			// Increment failed login attempts
			var incrementErr error
			attempts, incrementErr = s.authRepo.IncrementRetailerFailedLogin(txCtx, user.ID)
			if incrementErr != nil {
				return fmt.Errorf("failed to increment login attempts: %w", incrementErr)
			}

			// Lock account if max attempts reached
			if attempts >= s.maxFailedLogins {
				lockUntil := time.Now().Add(s.lockoutDuration)
				if lockErr := s.authRepo.LockRetailerAccount(txCtx, user.ID, lockUntil); lockErr != nil {
					return fmt.Errorf("failed to lock account: %w", lockErr)
				}
				locked = true
			}
			return nil
		})

		if err != nil {
			span.AddEvent("retailer.pos.login.transaction.failed", attribute.String("error", err.Error()))
			// Even if transaction fails, we still reject the login
			return nil, fmt.Errorf("invalid PIN")
		}

		if locked {
			span.AddEvent("retailer.pos.login.locked", attribute.Int("attempts", attempts))
			return nil, fmt.Errorf("account locked due to too many failed attempts")
		}

		return nil, fmt.Errorf("invalid PIN")
	}

	// Update last login
	if err := s.authRepo.UpdateRetailerLastLogin(ctx, user.ID); err != nil {
		// Log error but don't fail the login
		span.AddEvent("retailer.last_login_update.failed", attribute.String("error", err.Error()))
	}

	// Generate tokens with device ID
	accessToken, err := s.generateAccessTokenWithDevice(user.ID, "RETAILER", user.Code, deviceIMEI, *user.Phone)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create session with device tracking
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		UserType:     models.UserTypeRetailer,
		RefreshToken: refreshToken,
		UserAgent:    "POS Terminal",
		IPAddress:    "127.0.0.1", // Default IP for POS terminals
		DeviceID:     deviceIMEI,
		IsActive:     true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour), // Shorter expiry for POS
		LastActivity: time.Now(),
	}

	err = s.sessionRepo.Create(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTokenExpiry.Seconds()),
		UserID:       user.ID,
		UserType:     "RETAILER",
		UserCode:     user.Code,
	}, nil
}

// RefreshToken generates new tokens from a refresh token
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error) {
	// Get session by refresh token
	session, err := s.sessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Check if session is active
	if !session.IsActive {
		return nil, fmt.Errorf("session is inactive")
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("refresh token expired")
	}

	// Generate new tokens
	accessToken, err := s.generateAccessToken(session.UserID, string(session.UserType), "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	newRefreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Revoke old session
	err = s.sessionRepo.Revoke(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke old session: %w", err)
	}

	// Create new session
	newSession := &models.Session{
		ID:           uuid.New(),
		UserID:       session.UserID,
		UserType:     session.UserType,
		RefreshToken: newRefreshToken,
		UserAgent:    session.UserAgent,
		IPAddress:    session.IPAddress,
		DeviceID:     session.DeviceID,
		IsActive:     true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(s.refreshTokenExpiry),
		LastActivity: time.Now(),
	}

	err = s.sessionRepo.Create(ctx, newSession)
	if err != nil {
		return nil, fmt.Errorf("failed to create new session: %w", err)
	}

	// Fetch user details for the response
	var userCode, email, phone string
	switch session.UserType {
	case models.UserTypeAgent:
		if user, err := s.authRepo.GetAgentByID(ctx, session.UserID); err == nil {
			userCode = user.Code
			if user.Email != nil {
				email = *user.Email
			}
			if user.Phone != nil {
				phone = *user.Phone
			}
		}
	case models.UserTypeRetailer:
		if user, err := s.authRepo.GetRetailerByID(ctx, session.UserID); err == nil {
			userCode = user.Code
			if user.Email != nil {
				email = *user.Email
			}
			if user.Phone != nil {
				phone = *user.Phone
			}
		}
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int(s.accessTokenExpiry.Seconds()),
		UserID:       session.UserID,
		UserType:     string(session.UserType),
		UserCode:     userCode,
		Email:        email,
		Phone:        phone,
	}, nil
}

// Logout revokes a session
func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	session, err := s.sessionRepo.GetByRefreshToken(ctx, refreshToken)
	if err != nil {
		return fmt.Errorf("session not found")
	}

	return s.sessionRepo.Revoke(ctx, session.ID)
}

// ChangePassword changes a user's password
func (s *authService) ChangePassword(ctx context.Context, userID uuid.UUID, userType, oldPassword, newPassword string) error {
	if userType != "AGENT" && userType != "RETAILER" {
		return errors.New("invalid user type")
	}
	var user *repositories.AuthUser
	var err error

	if userType == "AGENT" {
		user, err = s.authRepo.GetAgentByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("Could not get agent: %v", err)
		}
	} else {
		user, err = s.authRepo.GetRetailerByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("Could not get retailer: %v", err)
		}
	}

	// Validate current password
	err = bcrypt.CompareHashAndPassword([]byte(*&user.PasswordHash), []byte(oldPassword)) // retailer_auth table contains both PasswordHash and PinHash with same values
	if err != nil {
		return fmt.Errorf("Current Password is incorrect")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password based on user type
	if userType == "AGENT" {
		err = s.authRepo.UpdateAgentPassword(ctx, userID, string(hashedPassword))
	} else {
		// For retailers, we update their PIN
		err = s.authRepo.UpdateRetailerPin(ctx, userID, string(hashedPassword))
	}

	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Revoke all sessions for security
	_, err = s.sessionRepo.RevokeAllForUser(ctx, userID)
	if err != nil {
		// Log error but don't fail the password change
		return fmt.Errorf("password changed but failed to revoke sessions: %w", err)
	}

	return nil
}

func (s *authService) ChangeRetailerPIN(ctx context.Context, userID uuid.UUID, oldPassword, newPassword, deviceIMEI, retailerCode, ipAddress string) (int64, error) {
	//Validate current pin
	user, err := s.authRepo.GetRetailerByCode(ctx, retailerCode)
	if err != nil {
		return 0, fmt.Errorf("Could not get retailer: %v", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.PinHash), []byte(oldPassword))
	if err != nil {
		return 0, fmt.Errorf("Current PIN is incorrect")
	}

	// Ensure new PIN is different
	if oldPassword == newPassword {
		return 0, errors.New("new PIN must be different from old PIN")
	}

	if err := validation.ValidatePIN(newPassword); err != nil {
		return 0, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("failed to hash password: %w", err)
	}

	err = s.authRepo.UpdateRetailerPin(ctx, userID, string(hashedPassword))
	if err != nil {
		return 0, fmt.Errorf("failed to update password: %w", err)
	}

	// Revoke all sessions for security
	sessionsInvalidated, err := s.sessionRepo.RevokeAllForUser(ctx, userID)
	if err != nil {
		// Log error but don't fail the password change
		return 0, fmt.Errorf("password changed but failed to revoke sessions: %w", err)
	}

	// Add audit log entry
	changeLog := &models.PINChangeLog{
		ID:                  uuid.New(),
		RetailerID:          userID,
		RetailerCode:        retailerCode,
		ChangedBy:           userID,
		ChangeReason:        "",
		DeviceIMEI:          deviceIMEI,
		IPAddress:           ipAddress,
		UserAgent:           "", // what is this field?
		Success:             true,
		FailureReason:       "",
		SessionsInvalidated: int(sessionsInvalidated),
		CreatedAt:           time.Now(),
	}

	err = s.authRepo.CreatePINChangeLog(ctx, changeLog)

	// send notification
	message := "Your PIN has been changed successfully. If you did not initiate this change, please contact support immediately."
	idempotencyKey := uuid.New().String()
	err = s.sendEmail(ctx, *user.Email, message, idempotencyKey)

	return sessionsInvalidated, err
}

// CreateAgentAuth creates authentication credentials for a new agent
func (s *authService) CreateAgentAuth(ctx context.Context, agentID uuid.UUID, agentCode, email, phone, password, createdBy string) error {
	// Validate agent code
	if err := validation.ValidateAgentCode(agentCode); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Validate phone
	if err := validation.ValidatePhone(phone); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	phone = validation.NormalizePhone(phone) // Normalize to international format

	// Validate email if provided
	if email != "" {
		if err := validation.ValidateEmail(email); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	// For agents, allow simple 6-digit passwords generated automatically
	// Skip the strict password validation that requires complexity
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	if len(password) > 128 {
		return fmt.Errorf("password is too long (max 128 characters)")
	}

	// Check if agent already exists
	existingAgent, err := s.authRepo.GetAgentByCode(ctx, agentCode)
	if err == nil && existingAgent != nil {
		return fmt.Errorf("agent with code %s already exists", agentCode)
	}

	// Check if phone number already exists
	existingAgentByPhone, err := s.authRepo.GetAgentByPhone(ctx, phone)
	if err == nil && existingAgentByPhone != nil {
		return fmt.Errorf("agent with phone %s already exists", phone)
	}

	// Check if email already exists (if provided)
	if email != "" {
		existingAgentByEmail, err := s.authRepo.GetAgentByEmail(ctx, email)
		if err == nil && existingAgentByEmail != nil {
			return fmt.Errorf("agent with email %s already exists", email)
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create auth user record
	// Only set email/phone pointers if values are non-empty to avoid empty string duplicates
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	var phonePtr *string
	if phone != "" {
		phonePtr = &phone
	}

	authUser := &repositories.AuthUser{
		ID:           agentID,
		Code:         agentCode,
		Email:        emailPtr,
		Phone:        phonePtr,
		PasswordHash: string(hashedPassword),
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.authRepo.CreateAgent(ctx, authUser)
	if err != nil {
		return fmt.Errorf("failed to create agent auth: %w", err)
	}

	return nil
}

// CreateRetailerAuth creates authentication credentials for a retailer
func (s *authService) CreateRetailerAuth(ctx context.Context, retailerID uuid.UUID, retailerCode, email, phone, pin, createdBy string) error {
	// Log retailer creation start
	fmt.Printf("[AGENT-AUTH] Creating retailer auth - ID: %s, Code: %s, Phone: %s, Email: %s, CreatedBy: %s\n",
		retailerID.String(), retailerCode, phone, email, createdBy)

	// Validate retailer code
	if retailerCode == "" {
		return fmt.Errorf("retailer code is required")
	}

	// Validate phone
	if err := validation.ValidatePhone(phone); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	phone = validation.NormalizePhone(phone) // Normalize to international format

	// Validate email if provided
	if email != "" {
		if err := validation.ValidateEmail(email); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	// For retailers, PIN must be exactly 4 digits
	if len(pin) != 4 {
		return fmt.Errorf("PIN must be exactly 4 digits")
	}
	// Validate that it's numeric
	for _, ch := range pin {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("PIN must contain only digits")
		}
	}

	// Check if retailer already exists
	existingRetailer, err := s.authRepo.GetRetailerByCode(ctx, retailerCode)
	if err == nil && existingRetailer != nil {
		return fmt.Errorf("retailer authentication already exists for code: %s", retailerCode)
	}

	// Hash PIN (using bcrypt same as passwords)
	hashedPin, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash PIN: %w", err)
	}

	// Create auth user record for retailer
	// Only set email/phone pointers if values are non-empty to avoid empty string duplicates
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	var phonePtr *string
	if phone != "" {
		phonePtr = &phone
	}

	authUser := &repositories.AuthUser{
		ID:           retailerID,
		Code:         retailerCode,
		Email:        emailPtr,
		Phone:        phonePtr,
		PasswordHash: string(hashedPin), // For retailers, this stores the PIN hash
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = s.authRepo.CreateRetailer(ctx, authUser)
	if err != nil {
		fmt.Printf("[AGENT-AUTH] Failed to create retailer auth: %v\n", err)
		return fmt.Errorf("failed to create retailer auth: %w", err)
	}

	fmt.Printf("[AGENT-AUTH] Retailer auth created successfully - ID: %s, Code: %s, PIN: %s\n",
		retailerID.String(), retailerCode, pin)

	return nil
}

// Helper functions

func (s *authService) generateAccessToken(userID uuid.UUID, userType, userCode, phone string) (string, error) {
	claims := jwt.Claims{
		UserID:   userID.String(),
		Email:    userCode + "@retailer.local",
		Username: userCode,
		Phone:    phone,
		JTI:      uuid.New().String(),
		Issuer:   "randlotteryltd",
		Roles:    []string{userType},
		Exp:      time.Now().Add(s.accessTokenExpiry).Unix(),
		Iat:      time.Now().Unix(),
	}

	return s.jwtService.GenerateAccessToken(claims)
}

func (s *authService) generateAccessTokenWithDevice(userID uuid.UUID, userType, userCode, deviceID, phone string) (string, error) {
	email := userCode + fmt.Sprintf("@%s.local", userType)
	claims := jwt.Claims{
		UserID:   userID.String(),
		Email:    email,
		Username: userCode,
		Phone:    phone,
		DeviceID: deviceID,
		JTI:      uuid.New().String(),
		Issuer:   "randlotteryltd",
		Roles:    []string{userType},
		Exp:      time.Now().Add(s.accessTokenExpiry).Unix(),
		Iat:      time.Now().Unix(),
	}
	return s.jwtService.GenerateAccessToken(claims)
}

func (s *authService) generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *authService) logPasswordReset(ctx context.Context, agentID uuid.UUID, requestIP, status, channel, resetToken string, attempts int32) error {
	now := time.Now()
	log := &models.PasswordResetLog{
		ID:          uuid.New(),
		AgentID:     agentID,
		ResetToken:  resetToken,
		RequestIP:   requestIP,
		Channel:     channel,
		Status:      status,
		OTPAttempts: int(attempts),
		CompletedAt: &now,
		ExpiresAt:   time.Now().Add(15 * time.Minute),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.authRepo.CreatePasswordResetLog(ctx, log); err != nil {
		return fmt.Errorf("failed to log password reset: %w", err)
	}
	return nil
}

func (s *authService) sendSMS(ctx context.Context, phoneNumber, message, idempotencyKey string) error {

	req := &notificationv1.SendSMSRequest{
		To:             phoneNumber,
		Content:        message,
		IdempotencyKey: idempotencyKey,
	}

	_, err := s.agentNotificationClient.SendSMS(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OTP SMS: %w", err)
	}

	return nil
}

func (s *authService) sendEmail(ctx context.Context, email, message, idempotencyKey string) error {

	req := &notificationv1.SendEmailRequest{
		To:             email,
		Subject:        message,
		IdempotencyKey: idempotencyKey,
	}

	_, err := s.agentNotificationClient.SendEmail(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OTP SMS: %w", err)
	}

	return nil
}

// Validation functions moved to shared/validation package

func (s *authService) ListAgentSessions(ctx context.Context, agentID uuid.UUID) ([]*pb.Session, error) {
	sessions, err := s.sessionRepo.ListAgentSessions(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agent sessions: %w", err)
	}
	pbSessions := make([]*pb.Session, len(sessions))
	for i, session := range sessions {
		pbSessions[i] = &pb.Session{
			Id:        session.ID.String(),
			AgentId:   session.UserID.String(),
			DeviceId:  session.DeviceID,
			UserAgent: session.UserAgent,
			IpAddress: session.IPAddress,
			IsActive:  session.IsActive,
			CreatedAt: timestamppb.New(session.CreatedAt),
			ExpiresAt: timestamppb.New(session.ExpiresAt),
		}
	}
	return pbSessions, nil
}

func (s *authService) CurrentSession(ctx context.Context, agentID uuid.UUID) (*pb.Session, error) {
	session, err := s.sessionRepo.CurrentSession(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current agent session: %w", err)
	}
	pbSession := &pb.Session{
		Id:        session.ID.String(),
		AgentId:   session.UserID.String(),
		DeviceId:  session.DeviceID,
		UserAgent: session.UserAgent,
		IpAddress: session.IPAddress,
		IsActive:  session.IsActive,
		CreatedAt: timestamppb.New(session.CreatedAt),
		ExpiresAt: timestamppb.New(session.ExpiresAt),
	}
	return pbSession, nil
}
