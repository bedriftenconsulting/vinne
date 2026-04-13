package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionRepository_Create(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSessionRepository(db.DB, rdb)

	t.Run("CreateAgentSession", func(t *testing.T) {
		session := &models.Session{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			UserType:     models.UserTypeAgent,
			RefreshToken: "refresh-token-" + uuid.New().String(),
			UserAgent:    "Mozilla/5.0 Test Agent",
			IPAddress:    "192.168.1.1",
			DeviceID:     "device-123",
			IsActive:     true,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			LastActivity: time.Now(),
		}

		err := repo.Create(context.Background(), session)
		require.NoError(t, err)

		// Verify session was created in database
		createdSession, err := repo.GetByRefreshToken(context.Background(), session.RefreshToken)
		require.NoError(t, err)
		assert.Equal(t, session.ID, createdSession.ID)
		assert.Equal(t, session.UserID, createdSession.UserID)
		assert.Equal(t, session.UserType, createdSession.UserType)
		assert.Equal(t, session.UserAgent, createdSession.UserAgent)
		assert.Equal(t, session.IPAddress, createdSession.IPAddress)
		assert.Equal(t, session.DeviceID, createdSession.DeviceID)
		assert.True(t, createdSession.IsActive)
	})

	t.Run("CreateRetailerSession", func(t *testing.T) {
		session := &models.Session{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			UserType:     models.UserTypeRetailer,
			RefreshToken: "retailer-refresh-token-" + uuid.New().String(),
			UserAgent:    "POS Terminal v1.0",
			IPAddress:    "10.0.0.5",
			DeviceID:     "pos-terminal-456",
			IsActive:     true,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
			LastActivity: time.Now(),
		}

		err := repo.Create(context.Background(), session)
		require.NoError(t, err)

		// Verify session was created
		createdSession, err := repo.GetByRefreshToken(context.Background(), session.RefreshToken)
		require.NoError(t, err)
		assert.Equal(t, session.ID, createdSession.ID)
		assert.Equal(t, models.UserTypeRetailer, createdSession.UserType)
		assert.Equal(t, "pos-terminal-456", createdSession.DeviceID)
	})

	t.Run("CreateDuplicateSession", func(t *testing.T) {
		sessionID := uuid.New()
		refreshToken := "duplicate-refresh-" + uuid.New().String()

		session1 := &models.Session{
			ID:           sessionID,
			UserID:       uuid.New(),
			UserType:     models.UserTypeAgent,
			RefreshToken: refreshToken,
			UserAgent:    "Test Agent",
			IPAddress:    "192.168.1.1",
			DeviceID:     "device-789",
			IsActive:     true,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			LastActivity: time.Now(),
		}

		err := repo.Create(context.Background(), session1)
		require.NoError(t, err)

		// Try to create another session with same ID
		session2 := &models.Session{
			ID:           sessionID, // Same ID
			UserID:       uuid.New(),
			UserType:     models.UserTypeAgent,
			RefreshToken: "different-token",
			UserAgent:    "Test Agent",
			IPAddress:    "192.168.1.2",
			DeviceID:     "device-790",
			IsActive:     true,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			LastActivity: time.Now(),
		}

		err = repo.Create(context.Background(), session2)
		assert.Error(t, err) // Should fail due to duplicate ID
	})
}

func TestSessionRepository_GetByRefreshToken(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSessionRepository(db.DB, rdb)

	// Create a test session
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		UserType:     models.UserTypeAgent,
		RefreshToken: "get-test-refresh-" + uuid.New().String(),
		UserAgent:    "Test User Agent",
		IPAddress:    "192.168.1.100",
		DeviceID:     "test-device-001",
		IsActive:     true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastActivity: time.Now(),
	}

	err := repo.Create(context.Background(), session)
	require.NoError(t, err)

	t.Run("GetActiveSession", func(t *testing.T) {
		retrievedSession, err := repo.GetByRefreshToken(context.Background(), session.RefreshToken)
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrievedSession.ID)
		assert.Equal(t, session.UserID, retrievedSession.UserID)
		assert.Equal(t, session.RefreshToken, retrievedSession.RefreshToken)
		assert.True(t, retrievedSession.IsActive)
	})

	t.Run("GetInactiveSession", func(t *testing.T) {
		// Create an inactive session
		inactiveSession := &models.Session{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			UserType:     models.UserTypeAgent,
			RefreshToken: "inactive-refresh-" + uuid.New().String(),
			UserAgent:    "Test Agent",
			IPAddress:    "192.168.1.101",
			DeviceID:     "device-002",
			IsActive:     false, // Inactive
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			LastActivity: time.Now(),
		}

		err := repo.Create(context.Background(), inactiveSession)
		require.NoError(t, err)

		// Should not return inactive session
		_, err = repo.GetByRefreshToken(context.Background(), inactiveSession.RefreshToken)
		assert.Error(t, err)
	})

	t.Run("GetNonExistentSession", func(t *testing.T) {
		_, err := repo.GetByRefreshToken(context.Background(), "non-existent-token")
		assert.Error(t, err)
	})
}

func TestSessionRepository_UpdateLastActivity(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSessionRepository(db.DB, rdb)

	// Create a test session
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		UserType:     models.UserTypeAgent,
		RefreshToken: "activity-test-" + uuid.New().String(),
		UserAgent:    "Test Agent",
		IPAddress:    "192.168.1.200",
		DeviceID:     "device-activity",
		IsActive:     true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastActivity: time.Now().Add(-1 * time.Hour), // Old activity
	}

	err := repo.Create(context.Background(), session)
	require.NoError(t, err)

	t.Run("UpdateActiveSessionActivity", func(t *testing.T) {
		beforeUpdate := session.LastActivity
		time.Sleep(100 * time.Millisecond) // Ensure time difference

		err := repo.UpdateLastActivity(context.Background(), session.ID)
		require.NoError(t, err)

		// Verify last activity was updated
		updatedSession, err := repo.GetByRefreshToken(context.Background(), session.RefreshToken)
		require.NoError(t, err)
		assert.True(t, updatedSession.LastActivity.After(beforeUpdate))
	})

	t.Run("UpdateInactiveSessionActivity", func(t *testing.T) {
		// Create an inactive session
		inactiveSession := &models.Session{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			UserType:     models.UserTypeAgent,
			RefreshToken: "inactive-activity-" + uuid.New().String(),
			UserAgent:    "Test Agent",
			IPAddress:    "192.168.1.201",
			DeviceID:     "device-inactive",
			IsActive:     false,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			LastActivity: time.Now().Add(-1 * time.Hour),
		}

		err := repo.Create(context.Background(), inactiveSession)
		require.NoError(t, err)

		// Update should not affect inactive sessions
		err = repo.UpdateLastActivity(context.Background(), inactiveSession.ID)
		// The update might succeed but won't affect inactive sessions
		// based on the SQL query condition
		assert.NoError(t, err)
	})
}

func TestSessionRepository_Revoke(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSessionRepository(db.DB, rdb)

	t.Run("RevokeActiveSession", func(t *testing.T) {
		session := &models.Session{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			UserType:     models.UserTypeAgent,
			RefreshToken: "revoke-test-" + uuid.New().String(),
			UserAgent:    "Test Agent",
			IPAddress:    "192.168.1.30",
			DeviceID:     "device-revoke",
			IsActive:     true,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			LastActivity: time.Now(),
		}

		err := repo.Create(context.Background(), session)
		require.NoError(t, err)

		// Revoke the session
		err = repo.Revoke(context.Background(), session.ID)
		require.NoError(t, err)

		// Verify session is no longer active
		_, err = repo.GetByRefreshToken(context.Background(), session.RefreshToken)
		assert.Error(t, err) // Should not find inactive session
	})

	t.Run("RevokeNonExistentSession", func(t *testing.T) {
		// Revoking non-existent session should not error
		// (idempotent operation)
		err := repo.Revoke(context.Background(), uuid.New())
		assert.NoError(t, err)
	})
}

func TestSessionRepository_RevokeAllForUser(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSessionRepository(db.DB, rdb)

	userID := uuid.New()
	sessionTokens := make([]string, 3)

	// Create multiple sessions for the same user
	for i := 0; i < 3; i++ {
		session := &models.Session{
			ID:           uuid.New(),
			UserID:       userID,
			UserType:     models.UserTypeAgent,
			RefreshToken: "user-session-" + uuid.New().String(),
			UserAgent:    "Test Agent",
			IPAddress:    "192.168.1.40",
			DeviceID:     "device-" + uuid.New().String(),
			IsActive:     true,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			LastActivity: time.Now(),
		}

		err := repo.Create(context.Background(), session)
		require.NoError(t, err)
		sessionTokens[i] = session.RefreshToken
	}

	// Create a session for a different user
	otherUserSession := &models.Session{
		ID:           uuid.New(),
		UserID:       uuid.New(), // Different user
		UserType:     models.UserTypeAgent,
		RefreshToken: "other-user-session-" + uuid.New().String(),
		UserAgent:    "Test Agent",
		IPAddress:    "192.168.1.41",
		DeviceID:     "device-other",
		IsActive:     true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastActivity: time.Now(),
	}

	err := repo.Create(context.Background(), otherUserSession)
	require.NoError(t, err)

	t.Run("RevokeAllUserSessions", func(t *testing.T) {
		sessionsRevoked, err := repo.RevokeAllForUser(context.Background(), userID)
		require.NoError(t, err)

		// Verify the count of revoked sessions
		assert.Equal(t, 3, sessionsRevoked)

		// Verify all user's sessions are revoked
		for _, token := range sessionTokens {
			_, err := repo.GetByRefreshToken(context.Background(), token)
			assert.Error(t, err) // Should not find inactive sessions
		}

		// Verify other user's session is still active
		otherSession, err := repo.GetByRefreshToken(context.Background(), otherUserSession.RefreshToken)
		require.NoError(t, err)
		assert.True(t, otherSession.IsActive)
	})
}

func TestSessionRepository_CleanupExpired(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSessionRepository(db.DB, rdb)

	// Create expired session
	expiredSession := &models.Session{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		UserType:     models.UserTypeAgent,
		RefreshToken: "expired-session-" + uuid.New().String(),
		UserAgent:    "Test Agent",
		IPAddress:    "192.168.1.50",
		DeviceID:     "device-expired",
		IsActive:     true,
		CreatedAt:    time.Now().Add(-48 * time.Hour),
		ExpiresAt:    time.Now().Add(-24 * time.Hour), // Already expired
		LastActivity: time.Now().Add(-48 * time.Hour),
	}

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO auth_sessions (
			id, user_id, user_type, refresh_token, user_agent, ip_address,
			device_id, is_active, created_at, expires_at, last_activity
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		expiredSession.ID,
		expiredSession.UserID,
		expiredSession.UserType,
		expiredSession.RefreshToken,
		expiredSession.UserAgent,
		expiredSession.IPAddress,
		expiredSession.DeviceID,
		expiredSession.IsActive,
		expiredSession.CreatedAt,
		expiredSession.ExpiresAt,
		expiredSession.LastActivity,
	)
	require.NoError(t, err)

	// Create old inactive session (>30 days)
	oldInactiveSession := &models.Session{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		UserType:     models.UserTypeAgent,
		RefreshToken: "old-inactive-" + uuid.New().String(),
		UserAgent:    "Test Agent",
		IPAddress:    "192.168.1.51",
		DeviceID:     "device-old",
		IsActive:     false,
		CreatedAt:    time.Now().Add(-35 * 24 * time.Hour), // >30 days old
		ExpiresAt:    time.Now().Add(24 * time.Hour),       // Not expired yet
		LastActivity: time.Now().Add(-35 * 24 * time.Hour),
	}

	_, err = db.ExecContext(context.Background(), `
		INSERT INTO auth_sessions (
			id, user_id, user_type, refresh_token, user_agent, ip_address,
			device_id, is_active, created_at, expires_at, last_activity
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		oldInactiveSession.ID,
		oldInactiveSession.UserID,
		oldInactiveSession.UserType,
		oldInactiveSession.RefreshToken,
		oldInactiveSession.UserAgent,
		oldInactiveSession.IPAddress,
		oldInactiveSession.DeviceID,
		oldInactiveSession.IsActive,
		oldInactiveSession.CreatedAt,
		oldInactiveSession.ExpiresAt,
		oldInactiveSession.LastActivity,
	)
	require.NoError(t, err)

	// Create recent inactive session (<30 days)
	recentInactiveSession := &models.Session{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		UserType:     models.UserTypeAgent,
		RefreshToken: "recent-inactive-" + uuid.New().String(),
		UserAgent:    "Test Agent",
		IPAddress:    "192.168.1.52",
		DeviceID:     "device-recent",
		IsActive:     false,
		CreatedAt:    time.Now().Add(-5 * 24 * time.Hour), // <30 days old
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastActivity: time.Now().Add(-5 * 24 * time.Hour),
	}

	err = repo.Create(context.Background(), recentInactiveSession)
	require.NoError(t, err)

	// Create active session
	activeSession := &models.Session{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		UserType:     models.UserTypeAgent,
		RefreshToken: "active-cleanup-" + uuid.New().String(),
		UserAgent:    "Test Agent",
		IPAddress:    "192.168.1.53",
		DeviceID:     "device-active",
		IsActive:     true,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		LastActivity: time.Now(),
	}

	err = repo.Create(context.Background(), activeSession)
	require.NoError(t, err)

	t.Run("CleanupExpiredAndOldInactiveSessions", func(t *testing.T) {
		err := repo.CleanupExpired(context.Background())
		require.NoError(t, err)

		// Verify expired session is deleted
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM auth_sessions WHERE id = $1", expiredSession.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Expired session should be deleted")

		// Verify old inactive session is deleted
		err = db.QueryRow("SELECT COUNT(*) FROM auth_sessions WHERE id = $1", oldInactiveSession.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Old inactive session should be deleted")

		// Verify recent inactive session still exists
		err = db.QueryRow("SELECT COUNT(*) FROM auth_sessions WHERE id = $1", recentInactiveSession.ID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "Recent inactive session should still exist")

		// Verify active session still exists
		activeSessionRetrieved, err := repo.GetByRefreshToken(context.Background(), activeSession.RefreshToken)
		require.NoError(t, err)
		assert.Equal(t, activeSession.ID, activeSessionRetrieved.ID)
	})
}

func TestSessionRepository_ConcurrentSessions(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSessionRepository(db.DB, rdb)

	userID := uuid.New()

	t.Run("MultipleConcurrentSessionsPerUser", func(t *testing.T) {
		// Create multiple concurrent sessions for the same user
		// (e.g., user logged in from multiple devices)
		sessions := make([]*models.Session, 3)
		for i := 0; i < 3; i++ {
			sessions[i] = &models.Session{
				ID:           uuid.New(),
				UserID:       userID,
				UserType:     models.UserTypeAgent,
				RefreshToken: "concurrent-" + uuid.New().String(),
				UserAgent:    "Device " + string(rune(i+1)),
				IPAddress:    "192.168.1.100",
				DeviceID:     "device-concurrent-" + string(rune(i+1)),
				IsActive:     true,
				CreatedAt:    time.Now(),
				ExpiresAt:    time.Now().Add(24 * time.Hour),
				LastActivity: time.Now(),
			}

			err := repo.Create(context.Background(), sessions[i])
			require.NoError(t, err)
		}

		// Verify all sessions can be retrieved
		for _, session := range sessions {
			retrievedSession, err := repo.GetByRefreshToken(context.Background(), session.RefreshToken)
			require.NoError(t, err)
			assert.Equal(t, session.ID, retrievedSession.ID)
			assert.True(t, retrievedSession.IsActive)
		}

		// Revoke one session should not affect others
		err := repo.Revoke(context.Background(), sessions[0].ID)
		require.NoError(t, err)

		// First session should be inactive
		_, err = repo.GetByRefreshToken(context.Background(), sessions[0].RefreshToken)
		assert.Error(t, err)

		// Other sessions should still be active
		for i := 1; i < 3; i++ {
			retrievedSession, err := repo.GetByRefreshToken(context.Background(), sessions[i].RefreshToken)
			require.NoError(t, err)
			assert.True(t, retrievedSession.IsActive)
		}
	})
}

func TestSessionRepository_SessionValidation(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewSessionRepository(db.DB, rdb)

	t.Run("ValidSession", func(t *testing.T) {
		session := &models.Session{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			UserType:     models.UserTypeAgent,
			RefreshToken: "valid-session-" + uuid.New().String(),
			UserAgent:    "Test Agent",
			IPAddress:    "192.168.1.200",
			DeviceID:     "device-valid",
			IsActive:     true,
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
			LastActivity: time.Now(),
		}

		err := repo.Create(context.Background(), session)
		require.NoError(t, err)

		retrievedSession, err := repo.GetByRefreshToken(context.Background(), session.RefreshToken)
		require.NoError(t, err)
		assert.True(t, retrievedSession.IsValid())
		assert.False(t, retrievedSession.IsExpired())
	})

	t.Run("ExpiredSession", func(t *testing.T) {
		// We can't directly create an expired session through the normal Create method
		// So we'll insert it directly for testing
		expiredSession := &models.Session{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			UserType:     models.UserTypeAgent,
			RefreshToken: "expired-validation-" + uuid.New().String(),
			UserAgent:    "Test Agent",
			IPAddress:    "192.168.1.201",
			DeviceID:     "device-expired-val",
			IsActive:     true,
			CreatedAt:    time.Now().Add(-48 * time.Hour),
			ExpiresAt:    time.Now().Add(-24 * time.Hour), // Expired
			LastActivity: time.Now().Add(-48 * time.Hour),
		}

		_, err := db.ExecContext(context.Background(), `
			INSERT INTO auth_sessions (
				id, user_id, user_type, refresh_token, user_agent, ip_address,
				device_id, is_active, created_at, expires_at, last_activity
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			expiredSession.ID,
			expiredSession.UserID,
			expiredSession.UserType,
			expiredSession.RefreshToken,
			expiredSession.UserAgent,
			expiredSession.IPAddress,
			expiredSession.DeviceID,
			expiredSession.IsActive,
			expiredSession.CreatedAt,
			expiredSession.ExpiresAt,
			expiredSession.LastActivity,
		)
		require.NoError(t, err)

		// We can still retrieve it since it's marked as active in DB
		retrievedSession, err := repo.GetByRefreshToken(context.Background(), expiredSession.RefreshToken)
		require.NoError(t, err)

		// But it should be considered invalid due to expiration
		assert.False(t, retrievedSession.IsValid())
		assert.True(t, retrievedSession.IsExpired())
	})
}
