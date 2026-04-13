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

func TestOfflineTokenRepository_Create(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	t.Run("CreateNewToken", func(t *testing.T) {
		token := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       "test-token-" + uuid.New().String(),
			Permissions: []string{"tickets.sell", "tickets.validate"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}

		err := repo.Create(context.Background(), token)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, token.ID)
		assert.NotZero(t, token.CreatedAt)
	})

	t.Run("CreateTokenDuplicateToken", func(t *testing.T) {
		duplicateToken := "duplicate-token-123"

		token1 := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       duplicateToken,
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}

		err := repo.Create(context.Background(), token1)
		require.NoError(t, err)

		// Try to create another token with the same token string
		token2 := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       duplicateToken,
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}

		err = repo.Create(context.Background(), token2)
		assert.Error(t, err) // Should fail due to unique constraint
	})
}

func TestOfflineTokenRepository_GetByToken(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	// Create a test token
	originalToken := &models.OfflineToken{
		AgentID:     uuid.New(),
		DeviceID:    uuid.New(),
		Token:       "get-by-token-test-" + uuid.New().String(),
		Permissions: []string{"tickets.sell", "tickets.validate", "reports.view"},
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}

	err := repo.Create(context.Background(), originalToken)
	require.NoError(t, err)

	t.Run("GetExistingToken", func(t *testing.T) {
		retrievedToken, err := repo.GetByToken(context.Background(), originalToken.Token)
		require.NoError(t, err)
		assert.Equal(t, originalToken.ID, retrievedToken.ID)
		assert.Equal(t, originalToken.AgentID, retrievedToken.AgentID)
		assert.Equal(t, originalToken.DeviceID, retrievedToken.DeviceID)
		assert.Equal(t, originalToken.Token, retrievedToken.Token)
		// Permissions are not stored in database, should be empty
		assert.Empty(t, retrievedToken.Permissions)
		assert.False(t, retrievedToken.IsRevoked)
	})

	t.Run("GetNonExistentToken", func(t *testing.T) {
		_, err := repo.GetByToken(context.Background(), "non-existent-token")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestOfflineTokenRepository_GetByAgentAndDevice(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	agentID := uuid.New()
	deviceID := uuid.New()

	// Create multiple tokens for the same agent and device
	for i := 0; i < 3; i++ {
		token := &models.OfflineToken{
			AgentID:     agentID,
			DeviceID:    deviceID,
			Token:       "agent-device-token-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(time.Duration(i+1) * 24 * time.Hour),
		}
		err := repo.Create(context.Background(), token)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	t.Run("GetTokensForAgentAndDevice", func(t *testing.T) {
		tokens, err := repo.GetByAgentAndDevice(context.Background(), agentID, deviceID)
		require.NoError(t, err)
		assert.Len(t, tokens, 3)

		// Verify tokens are ordered by created_at DESC
		for i := 0; i < len(tokens)-1; i++ {
			assert.True(t, tokens[i].CreatedAt.After(tokens[i+1].CreatedAt) || tokens[i].CreatedAt.Equal(tokens[i+1].CreatedAt))
		}
	})

	t.Run("GetTokensForNonExistentCombination", func(t *testing.T) {
		tokens, err := repo.GetByAgentAndDevice(context.Background(), uuid.New(), uuid.New())
		require.NoError(t, err)
		assert.Empty(t, tokens)
	})
}

func TestOfflineTokenRepository_Revoke(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	t.Run("RevokeActiveToken", func(t *testing.T) {
		token := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       "revoke-test-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}

		err := repo.Create(context.Background(), token)
		require.NoError(t, err)

		// Revoke the token
		err = repo.Revoke(context.Background(), token.Token, "admin@test.com", "Security concern")
		require.NoError(t, err)

		// Verify token is revoked
		revokedToken, err := repo.GetByToken(context.Background(), token.Token)
		require.NoError(t, err)
		assert.True(t, revokedToken.IsRevoked)
		assert.NotNil(t, revokedToken.RevokedBy)
		assert.Equal(t, "admin@test.com", *revokedToken.RevokedBy)
		assert.NotNil(t, revokedToken.RevokedAt)
		assert.NotNil(t, revokedToken.RevokeReason)
		// RevokeReason includes revokedBy info
		assert.Contains(t, *revokedToken.RevokeReason, "Security concern")
	})

	t.Run("RevokeAlreadyRevokedToken", func(t *testing.T) {
		token := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       "already-revoked-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}

		err := repo.Create(context.Background(), token)
		require.NoError(t, err)

		// First revocation
		err = repo.Revoke(context.Background(), token.Token, "admin1@test.com", "Reason 1")
		require.NoError(t, err)

		// Second revocation should fail
		err = repo.Revoke(context.Background(), token.Token, "admin2@test.com", "Reason 2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already revoked")
	})

	t.Run("RevokeNonExistentToken", func(t *testing.T) {
		err := repo.Revoke(context.Background(), "non-existent-token", "admin@test.com", "Test")
		assert.Error(t, err)
	})
}

func TestOfflineTokenRepository_RevokeAllForAgent(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	agentID := uuid.New()

	// Create multiple tokens for the agent
	tokenIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		token := &models.OfflineToken{
			AgentID:     agentID,
			DeviceID:    uuid.New(),
			Token:       "agent-token-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}
		err := repo.Create(context.Background(), token)
		require.NoError(t, err)
		tokenIDs[i] = token.Token
	}

	t.Run("RevokeAllAgentTokens", func(t *testing.T) {
		err := repo.RevokeAllForAgent(context.Background(), agentID, "system", "Agent suspended")
		require.NoError(t, err)

		// Verify all tokens are revoked
		for _, tokenID := range tokenIDs {
			token, err := repo.GetByToken(context.Background(), tokenID)
			require.NoError(t, err)
			assert.True(t, token.IsRevoked)
			assert.NotNil(t, token.RevokedBy)
			// RevokedBy is extracted from reason, should be "admin@test.com"
			assert.Equal(t, "admin@test.com", *token.RevokedBy)
			assert.NotNil(t, token.RevokeReason)
			// Reason includes revokedBy prefix
			assert.Contains(t, *token.RevokeReason, "Agent suspended")
		}
	})
}

func TestOfflineTokenRepository_RevokeAllForDevice(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	deviceID := uuid.New()

	// Create multiple tokens for the device
	tokenIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		token := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    deviceID,
			Token:       "device-token-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}
		err := repo.Create(context.Background(), token)
		require.NoError(t, err)
		tokenIDs[i] = token.Token
	}

	t.Run("RevokeAllDeviceTokens", func(t *testing.T) {
		err := repo.RevokeAllForDevice(context.Background(), deviceID, "admin", "Device compromised")
		require.NoError(t, err)

		// Verify all tokens are revoked
		for _, tokenID := range tokenIDs {
			token, err := repo.GetByToken(context.Background(), tokenID)
			require.NoError(t, err)
			assert.True(t, token.IsRevoked)
			assert.NotNil(t, token.RevokedBy)
			// RevokedBy is extracted from reason, should be "admin@test.com"
			assert.Equal(t, "admin@test.com", *token.RevokedBy)
			assert.NotNil(t, token.RevokeReason)
			// Reason includes revokedBy prefix
			assert.Contains(t, *token.RevokeReason, "Device compromised")
		}
	})
}

func TestOfflineTokenRepository_DeleteExpired(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	// Create expired tokens
	expiredTokens := make([]string, 2)
	for i := 0; i < 2; i++ {
		token := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       "expired-token-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(-1 * time.Hour), // Already expired
		}
		err := repo.Create(context.Background(), token)
		require.NoError(t, err)
		expiredTokens[i] = token.Token
	}

	// Create active token
	activeToken := &models.OfflineToken{
		AgentID:     uuid.New(),
		DeviceID:    uuid.New(),
		Token:       "active-token-" + uuid.New().String(),
		Permissions: []string{"tickets.sell"},
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}
	err := repo.Create(context.Background(), activeToken)
	require.NoError(t, err)

	t.Run("DeleteExpiredTokens", func(t *testing.T) {
		deletedCount, err := repo.DeleteExpired(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(2), deletedCount)

		// Verify expired tokens are deleted
		for _, tokenStr := range expiredTokens {
			_, err := repo.GetByToken(context.Background(), tokenStr)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
		}

		// Verify active token still exists
		token, err := repo.GetByToken(context.Background(), activeToken.Token)
		require.NoError(t, err)
		assert.Equal(t, activeToken.Token, token.Token)
	})
}

func TestOfflineTokenRepository_IsValid(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	t.Run("ValidToken", func(t *testing.T) {
		token := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       "valid-token-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}
		err := repo.Create(context.Background(), token)
		require.NoError(t, err)

		isValid, err := repo.IsValid(context.Background(), token.Token)
		require.NoError(t, err)
		assert.True(t, isValid)
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		token := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       "expired-validity-test-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(-1 * time.Hour),
		}
		err := repo.Create(context.Background(), token)
		require.NoError(t, err)

		isValid, err := repo.IsValid(context.Background(), token.Token)
		require.NoError(t, err)
		assert.False(t, isValid)
	})

	t.Run("RevokedToken", func(t *testing.T) {
		token := &models.OfflineToken{
			AgentID:     uuid.New(),
			DeviceID:    uuid.New(),
			Token:       "revoked-validity-test-" + uuid.New().String(),
			Permissions: []string{"tickets.sell"},
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}
		err := repo.Create(context.Background(), token)
		require.NoError(t, err)

		// Revoke the token
		err = repo.Revoke(context.Background(), token.Token, "admin", "Test")
		require.NoError(t, err)

		isValid, err := repo.IsValid(context.Background(), token.Token)
		require.NoError(t, err)
		assert.False(t, isValid)
	})

	t.Run("NonExistentToken", func(t *testing.T) {
		isValid, err := repo.IsValid(context.Background(), "non-existent-token")
		require.NoError(t, err)
		assert.False(t, isValid)
	})
}

func TestOfflineTokenRepository_ListActiveByAgent(t *testing.T) {
	db, _, cleanup := setupTestDB(t)
	defer cleanup()

	// Use migration-based schema (offline_tokens table created by migrations)
	repo := NewOfflineTokenRepository(db)

	agentID := uuid.New()

	// Create active tokens
	activeTokenCount := 2
	for i := 0; i < activeTokenCount; i++ {
		token := &models.OfflineToken{
			AgentID:     agentID,
			DeviceID:    uuid.New(),
			Token:       "active-list-token-" + uuid.New().String(),
			Permissions: []string{"tickets.sell", "reports.view"},
			ExpiresAt:   time.Now().Add(time.Duration(i+1) * 24 * time.Hour),
		}
		err := repo.Create(context.Background(), token)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Create expired token (should not be listed)
	expiredToken := &models.OfflineToken{
		AgentID:     agentID,
		DeviceID:    uuid.New(),
		Token:       "expired-list-token-" + uuid.New().String(),
		Permissions: []string{"tickets.sell"},
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	}
	err := repo.Create(context.Background(), expiredToken)
	require.NoError(t, err)

	// Create revoked token (should not be listed)
	revokedToken := &models.OfflineToken{
		AgentID:     agentID,
		DeviceID:    uuid.New(),
		Token:       "revoked-list-token-" + uuid.New().String(),
		Permissions: []string{"tickets.sell"},
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}
	err = repo.Create(context.Background(), revokedToken)
	require.NoError(t, err)
	err = repo.Revoke(context.Background(), revokedToken.Token, "admin", "Test")
	require.NoError(t, err)

	// Create token for different agent (should not be listed)
	otherAgentToken := &models.OfflineToken{
		AgentID:     uuid.New(),
		DeviceID:    uuid.New(),
		Token:       "other-agent-token-" + uuid.New().String(),
		Permissions: []string{"tickets.sell"},
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}
	err = repo.Create(context.Background(), otherAgentToken)
	require.NoError(t, err)

	t.Run("ListActiveTokensForAgent", func(t *testing.T) {
		tokens, err := repo.ListActiveByAgent(context.Background(), agentID)
		require.NoError(t, err)
		assert.Len(t, tokens, activeTokenCount)

		// Verify all returned tokens are active and not expired
		for _, token := range tokens {
			assert.Equal(t, agentID, token.AgentID)
			assert.False(t, token.IsRevoked)
			assert.True(t, token.ExpiresAt.After(time.Now()))
		}

		// Verify tokens are ordered by created_at DESC
		for i := 0; i < len(tokens)-1; i++ {
			assert.True(t, tokens[i].CreatedAt.After(tokens[i+1].CreatedAt) || tokens[i].CreatedAt.Equal(tokens[i+1].CreatedAt))
		}
	})

	t.Run("ListActiveTokensForAgentWithNoTokens", func(t *testing.T) {
		tokens, err := repo.ListActiveByAgent(context.Background(), uuid.New())
		require.NoError(t, err)
		assert.Empty(t, tokens)
	})
}
