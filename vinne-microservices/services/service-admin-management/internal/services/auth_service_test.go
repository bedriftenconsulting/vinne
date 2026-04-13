package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	adminmanagementv1 "github.com/randco/randco-microservices/proto/admin/management/v1"
	"github.com/randco/randco-microservices/services/service-admin-management/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const userIDKey contextKey = "user_id"

type AuthServiceTestSuite struct {
	BaseServiceTestSuite
	authService AuthService
}

func TestAuthServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceTestSuite))
}

func (s *AuthServiceTestSuite) SetupTest() {
	// Call base setup
	s.BaseServiceTestSuite.SetupTest()

	// Initialize auth service with test config
	testConfig := &config.Config{
		Security: config.SecurityConfig{
			MaxFailedLogins: 5,
			LockoutDuration: 30,
		},
	}
	s.authService = NewAuthService(
		s.repos.AdminUser,
		s.repos.AdminUserAuth,
		s.repos.Session,
		s.repos.AuditLog,
		s.jwtManager,
		s.eventBus,
		testConfig,
	)
}

// Test Login
func (s *AuthServiceTestSuite) TestLogin_Success() {
	req := &adminmanagementv1.LoginRequest{
		Email:     "test.admin@example.com",
		Password:  "TestPassword123!",
		IpAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	resp, err := s.authService.Login(s.ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotEmpty(s.T(), resp.AccessToken)
	assert.NotEmpty(s.T(), resp.RefreshToken)
	assert.Equal(s.T(), "test.admin@example.com", resp.User.Email)
	assert.Equal(s.T(), "testadmin", resp.User.Username)

	// Verify session was created
	var sessionCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_sessions 
		WHERE user_id = $1 AND is_active = true
	`, s.testAdminUser.ID).Scan(&sessionCount)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, sessionCount)

	// Verify audit log was created
	var auditCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_audit_logs 
		WHERE admin_user_id = $1 AND action = 'admin.login'
	`, s.testAdminUser.ID).Scan(&auditCount)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, auditCount)
}

func (s *AuthServiceTestSuite) TestLogin_InvalidCredentials() {
	req := &adminmanagementv1.LoginRequest{
		Email:     "test.admin@example.com",
		Password:  "WrongPassword",
		IpAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	resp, err := s.authService.Login(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "invalid credentials")
}

func (s *AuthServiceTestSuite) TestLogin_InactiveUser() {
	// Deactivate the user
	_, err := s.db.Exec(`
		UPDATE admin_users SET is_active = false WHERE id = $1
	`, s.testAdminUser.ID)
	require.NoError(s.T(), err)

	req := &adminmanagementv1.LoginRequest{
		Email:     "test.admin@example.com",
		Password:  "TestPassword123!",
		IpAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	resp, err := s.authService.Login(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "account is not active")
}

func (s *AuthServiceTestSuite) TestLogin_NonExistentUser() {
	req := &adminmanagementv1.LoginRequest{
		Email:     "nonexistent@example.com",
		Password:  "SomePassword",
		IpAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	resp, err := s.authService.Login(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
}

// Test Logout
func (s *AuthServiceTestSuite) TestLogout_Success() {
	// First login to create a session
	loginReq := &adminmanagementv1.LoginRequest{
		Email:     "test.admin@example.com",
		Password:  "TestPassword123!",
		IpAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	loginResp, err := s.authService.Login(s.ctx, loginReq)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), loginResp)

	// Now logout
	logoutReq := &adminmanagementv1.LogoutRequest{
		UserId:       s.testAdminUser.ID.String(),
		RefreshToken: loginResp.RefreshToken,
	}

	_, err = s.authService.Logout(s.ctx, logoutReq)
	assert.NoError(s.T(), err)

	// Verify session is invalidated
	var isActive bool
	err = s.db.QueryRow(`
		SELECT is_active FROM admin_sessions 
		WHERE refresh_token = $1
	`, loginResp.RefreshToken).Scan(&isActive)
	assert.NoError(s.T(), err)
	assert.False(s.T(), isActive)

	// Verify audit log was created
	var auditCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_audit_logs 
		WHERE admin_user_id = $1 AND action = 'admin.logout'
	`, s.testAdminUser.ID).Scan(&auditCount)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, auditCount)
}

func (s *AuthServiceTestSuite) TestLogout_InvalidToken() {
	req := &adminmanagementv1.LogoutRequest{
		UserId:       s.testAdminUser.ID.String(),
		RefreshToken: "invalid-token",
	}

	_, err := s.authService.Logout(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "session not found")
}

// Test RefreshToken
func (s *AuthServiceTestSuite) TestRefreshToken_Success() {
	// First login to create a session
	loginReq := &adminmanagementv1.LoginRequest{
		Email:     "test.admin@example.com",
		Password:  "TestPassword123!",
		IpAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	loginResp, err := s.authService.Login(s.ctx, loginReq)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), loginResp)

	// Wait a moment to ensure different token generation
	time.Sleep(time.Millisecond * 100)

	// Refresh the token
	refreshReq := &adminmanagementv1.RefreshTokenRequest{
		RefreshToken: loginResp.RefreshToken,
	}

	refreshResp, err := s.authService.RefreshToken(s.ctx, refreshReq)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), refreshResp)
	assert.NotEmpty(s.T(), refreshResp.AccessToken)
	assert.NotEmpty(s.T(), refreshResp.RefreshToken)
	assert.NotEqual(s.T(), loginResp.AccessToken, refreshResp.AccessToken)
	assert.NotEqual(s.T(), loginResp.RefreshToken, refreshResp.RefreshToken)
}

func (s *AuthServiceTestSuite) TestRefreshToken_ExpiredSession() {
	// Create an expired session
	sessionID := uuid.New()
	userID := s.testAdminUser.ID
	refreshToken := "expired-refresh-token-" + uuid.NewString()

	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sessionID, userID, refreshToken, "Mozilla/5.0", "192.168.1.1",
		time.Now().Add(-24*time.Hour), true) // Expired yesterday
	require.NoError(s.T(), err)

	req := &adminmanagementv1.RefreshTokenRequest{
		RefreshToken: refreshToken,
	}

	resp, err := s.authService.RefreshToken(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "session expired")
}

func (s *AuthServiceTestSuite) TestRefreshToken_InactiveSession() {
	// Create an inactive session
	sessionID := uuid.New()
	userID := s.testAdminUser.ID
	refreshToken := "inactive-refresh-token-" + uuid.NewString()

	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, sessionID, userID, refreshToken, "Mozilla/5.0", "192.168.1.1",
		time.Now().Add(24*time.Hour), false) // Not active
	require.NoError(s.T(), err)

	req := &adminmanagementv1.RefreshTokenRequest{
		RefreshToken: refreshToken,
	}

	resp, err := s.authService.RefreshToken(s.ctx, req)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
	assert.Contains(s.T(), err.Error(), "session inactive")
}

// Test ChangePassword
func (s *AuthServiceTestSuite) TestChangePassword_Success() {
	// First login to create a session
	loginReq := &adminmanagementv1.LoginRequest{
		Email:     "test.admin@example.com",
		Password:  "TestPassword123!",
		IpAddress: "192.168.1.1",
		UserAgent: "Mozilla/5.0",
	}

	_, err := s.authService.Login(s.ctx, loginReq)
	require.NoError(s.T(), err)

	// Create context with user info (normally done by middleware)
	ctx := context.WithValue(s.ctx, userIDKey, s.testAdminUser.ID.String())

	req := &adminmanagementv1.ChangePasswordRequest{
		UserId:          s.testAdminUser.ID.String(),
		CurrentPassword: "TestPassword123!",
		NewPassword:     "NewSecure#Key456!",
	}

	_, err = s.authService.ChangePassword(ctx, req)
	assert.NoError(s.T(), err)

	// Verify password was changed - try to login with new password
	loginReq.Password = "NewSecure#Key456!"
	newLoginResp, err := s.authService.Login(s.ctx, loginReq)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), newLoginResp)

	// Verify old password doesn't work
	loginReq.Password = "TestPassword123!"
	oldLoginResp, err := s.authService.Login(s.ctx, loginReq)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), oldLoginResp)

	// Verify only one active session exists (from the new login)
	var activeSessionCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_sessions 
		WHERE user_id = $1 AND is_active = true
	`, s.testAdminUser.ID).Scan(&activeSessionCount)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, activeSessionCount)
}

func (s *AuthServiceTestSuite) TestChangePassword_WrongOldPassword() {
	ctx := context.WithValue(s.ctx, userIDKey, s.testAdminUser.ID.String())

	req := &adminmanagementv1.ChangePasswordRequest{
		UserId:          s.testAdminUser.ID.String(),
		CurrentPassword: "WrongOldPassword",
		NewPassword:     "NewSecurePassword456!",
	}

	_, err := s.authService.ChangePassword(ctx, req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "current password is incorrect")
}

func (s *AuthServiceTestSuite) TestChangePassword_WeakNewPassword() {
	ctx := context.WithValue(s.ctx, userIDKey, s.testAdminUser.ID.String())

	req := &adminmanagementv1.ChangePasswordRequest{
		UserId:          s.testAdminUser.ID.String(),
		CurrentPassword: "TestPassword123!",
		NewPassword:     "weak", // Too weak
	}

	_, err := s.authService.ChangePassword(ctx, req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "password")
}

// Test MFA operations
func (s *AuthServiceTestSuite) TestEnableMFA_Success() {
	ctx := context.WithValue(s.ctx, userIDKey, s.testAdminUser.ID.String())

	req := &adminmanagementv1.EnableMFARequest{
		UserId: s.testAdminUser.ID.String(),
	}

	resp, err := s.authService.EnableMFA(ctx, req)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.NotEmpty(s.T(), resp.Secret)
	assert.NotEmpty(s.T(), resp.QrCodeUrl)
	assert.Contains(s.T(), resp.QrCodeUrl, "otpauth://totp")

	// Verify MFA secret was stored (encrypted)
	var mfaSecret string
	err = s.db.QueryRow(`
		SELECT mfa_secret FROM admin_users WHERE id = $1
	`, s.testAdminUser.ID).Scan(&mfaSecret)
	assert.NoError(s.T(), err)
	assert.NotEmpty(s.T(), mfaSecret)
}

func (s *AuthServiceTestSuite) TestVerifyMFA_Success() {
	// First enable MFA
	ctx := context.WithValue(s.ctx, userIDKey, s.testAdminUser.ID.String())

	enableReq := &adminmanagementv1.EnableMFARequest{
		UserId: s.testAdminUser.ID.String(),
	}

	enableResp, err := s.authService.EnableMFA(ctx, enableReq)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), enableResp.Secret)

	// Generate a valid TOTP code
	validCode, err := totp.GenerateCode(enableResp.Secret, time.Now())
	require.NoError(s.T(), err)

	// Verify MFA
	verifyReq := &adminmanagementv1.VerifyMFARequest{
		UserId: s.testAdminUser.ID.String(),
		Token:  validCode,
	}

	_, err = s.authService.VerifyMFA(ctx, verifyReq)
	assert.NoError(s.T(), err)

	// Verify MFA is now enabled for the user
	var mfaEnabled bool
	err = s.db.QueryRow(`
		SELECT mfa_enabled FROM admin_users WHERE id = $1
	`, s.testAdminUser.ID).Scan(&mfaEnabled)
	assert.NoError(s.T(), err)
	assert.True(s.T(), mfaEnabled)
}

func (s *AuthServiceTestSuite) TestVerifyMFA_InvalidCode() {
	// First enable MFA
	ctx := context.WithValue(s.ctx, userIDKey, s.testAdminUser.ID.String())

	enableReq := &adminmanagementv1.EnableMFARequest{
		UserId: s.testAdminUser.ID.String(),
	}

	_, err := s.authService.EnableMFA(ctx, enableReq)
	require.NoError(s.T(), err)

	// Try to verify with invalid code
	verifyReq := &adminmanagementv1.VerifyMFARequest{
		UserId: s.testAdminUser.ID.String(),
		Token:  "123456", // Invalid code
	}

	_, err = s.authService.VerifyMFA(ctx, verifyReq)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "invalid MFA code")
}

func (s *AuthServiceTestSuite) TestDisableMFA_Success() {
	// First enable and verify MFA
	ctx := context.WithValue(s.ctx, userIDKey, s.testAdminUser.ID.String())

	// Enable MFA
	enableReq := &adminmanagementv1.EnableMFARequest{
		UserId: s.testAdminUser.ID.String(),
	}
	enableResp, err := s.authService.EnableMFA(ctx, enableReq)
	require.NoError(s.T(), err)

	// Generate valid code and verify
	validCode, err := totp.GenerateCode(enableResp.Secret, time.Now())
	require.NoError(s.T(), err)

	verifyReq := &adminmanagementv1.VerifyMFARequest{
		UserId: s.testAdminUser.ID.String(),
		Token:  validCode,
	}
	_, err = s.authService.VerifyMFA(ctx, verifyReq)
	require.NoError(s.T(), err)

	// Now disable MFA
	// Generate a new valid code for disabling
	disableCode, err := totp.GenerateCode(enableResp.Secret, time.Now())
	require.NoError(s.T(), err)

	disableReq := &adminmanagementv1.DisableMFARequest{
		UserId: s.testAdminUser.ID.String(),
		Token:  disableCode,
	}

	_, err = s.authService.DisableMFA(ctx, disableReq)
	assert.NoError(s.T(), err)

	// Verify MFA is disabled
	var mfaEnabled bool
	var mfaSecret sql.NullString
	err = s.db.QueryRow(`
		SELECT mfa_enabled, mfa_secret FROM admin_users WHERE id = $1
	`, s.testAdminUser.ID).Scan(&mfaEnabled, &mfaSecret)
	assert.NoError(s.T(), err)
	assert.False(s.T(), mfaEnabled)
	assert.False(s.T(), mfaSecret.Valid)
}

// Test concurrent login sessions
func (s *AuthServiceTestSuite) TestMultipleConcurrentSessions() {
	// Create multiple login sessions
	for i := 0; i < 3; i++ {
		req := &adminmanagementv1.LoginRequest{
			Email:     "test.admin@example.com",
			Password:  "TestPassword123!",
			IpAddress: "192.168.1." + string(rune(i+1)),
			UserAgent: "Device-" + string(rune(i+1)),
		}

		resp, err := s.authService.Login(s.ctx, req)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), resp)
	}

	// Verify 3 active sessions exist
	var sessionCount int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_sessions 
		WHERE user_id = $1 AND is_active = true
	`, s.testAdminUser.ID).Scan(&sessionCount)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 3, sessionCount)
}

// Test session cleanup
func (s *AuthServiceTestSuite) TestExpiredSessionCleanup() {
	// Create some expired sessions
	for i := 0; i < 3; i++ {
		sessionID := uuid.New()
		_, err := s.db.Exec(`
			INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, sessionID, s.testAdminUser.ID, "expired-token-"+sessionID.String(),
			"Mozilla/5.0", "192.168.1.1", time.Now().Add(-48*time.Hour), true)
		require.NoError(s.T(), err)
	}

	// Create a valid session
	validSession := uuid.New()
	_, err := s.db.Exec(`
		INSERT INTO admin_sessions (id, user_id, refresh_token, user_agent, ip_address, expires_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, validSession, s.testAdminUser.ID, "valid-token-"+validSession.String(),
		"Mozilla/5.0", "192.168.1.1", time.Now().Add(24*time.Hour), true)
	require.NoError(s.T(), err)

	// Run cleanup
	err = s.repos.Session.DeleteExpired(s.ctx)
	assert.NoError(s.T(), err)

	// Verify only valid session remains
	var sessionCount int
	err = s.db.QueryRow(`
		SELECT COUNT(*) FROM admin_sessions WHERE user_id = $1
	`, s.testAdminUser.ID).Scan(&sessionCount)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, sessionCount)
}
