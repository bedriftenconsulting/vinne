package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/randco/service-player/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) *sql.DB {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)

	// Run migrations
	err = runMigrations(db)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
		postgresContainer.Terminate(ctx)
	})

	return db
}

func runMigrations(db *sql.DB) error {
	migrationSQL := `
		-- Players table
		CREATE TABLE players (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			phone_number VARCHAR(15) UNIQUE NOT NULL,
			email VARCHAR(255),
			password_hash VARCHAR(255) NOT NULL,
			first_name VARCHAR(100),
			last_name VARCHAR(100),
			date_of_birth DATE,
			national_id VARCHAR(50),
			mobile_money_phone VARCHAR(15),
			status VARCHAR(20) DEFAULT 'ACTIVE',
			email_verified BOOLEAN DEFAULT FALSE,
			phone_verified BOOLEAN DEFAULT FALSE,
			registration_channel VARCHAR(20),
			terms_accepted BOOLEAN DEFAULT FALSE,
			marketing_consent BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			last_login_at TIMESTAMP,
			deleted_at TIMESTAMP
		);

		CREATE INDEX idx_players_phone ON players(phone_number);
		CREATE INDEX idx_players_email ON players(email);
		CREATE INDEX idx_players_status ON players(status);
		CREATE INDEX idx_players_created ON players(created_at);

		-- Player sessions table
		CREATE TABLE player_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			player_id UUID NOT NULL REFERENCES players(id),
			device_id VARCHAR(255) NOT NULL,
			refresh_token VARCHAR(512) UNIQUE NOT NULL,
			access_token_jti VARCHAR(255),
			channel VARCHAR(20) NOT NULL,
			device_type VARCHAR(50),
			app_version VARCHAR(50),
			ip_address INET,
			user_agent TEXT,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP NOT NULL,
			last_used_at TIMESTAMP NOT NULL DEFAULT NOW(),
			revoked_at TIMESTAMP,
			revoked_reason VARCHAR(255)
		);

		CREATE INDEX idx_sessions_player ON player_sessions(player_id);
		CREATE INDEX idx_sessions_device ON player_sessions(device_id);
		CREATE INDEX idx_sessions_token ON player_sessions(refresh_token);
		CREATE INDEX idx_sessions_active ON player_sessions(is_active, expires_at);
		CREATE INDEX idx_sessions_channel ON player_sessions(channel, created_at DESC);

		-- Device registry table
		CREATE TABLE player_devices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			player_id UUID NOT NULL REFERENCES players(id),
			device_id VARCHAR(255) NOT NULL,
			device_type VARCHAR(50),
			device_name VARCHAR(255),
			os VARCHAR(50),
			os_version VARCHAR(50),
			app_version VARCHAR(50),
			push_token VARCHAR(512),
			fingerprint VARCHAR(512),
			is_trusted BOOLEAN DEFAULT FALSE,
			is_blocked BOOLEAN DEFAULT FALSE,
			first_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
			last_seen_at TIMESTAMP NOT NULL DEFAULT NOW(),
			trust_score INTEGER DEFAULT 50,
			UNIQUE(player_id, device_id)
		);

		CREATE INDEX idx_devices_player ON player_devices(player_id);
		CREATE INDEX idx_devices_fingerprint ON player_devices(fingerprint);
		CREATE INDEX idx_devices_trusted ON player_devices(is_trusted);

		-- Player wallet reference
		CREATE TABLE player_wallets (
			player_id UUID PRIMARY KEY REFERENCES players(id),
			wallet_id UUID NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		-- Login attempts table
		CREATE TABLE player_login_attempts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			phone_number VARCHAR(15) NOT NULL,
			player_id UUID,
			device_id VARCHAR(255),
			channel VARCHAR(20) NOT NULL,
			ip_address INET,
			attempt_type VARCHAR(20),
			success BOOLEAN NOT NULL,
			failure_reason VARCHAR(255),
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE INDEX idx_login_phone ON player_login_attempts(phone_number, created_at DESC);
		CREATE INDEX idx_login_player ON player_login_attempts(player_id, created_at DESC);
		CREATE INDEX idx_login_ip ON player_login_attempts(ip_address);
		CREATE INDEX idx_login_channel ON player_login_attempts(channel, created_at DESC);
	`

	_, err := db.Exec(migrationSQL)
	return err
}

func TestPlayerRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPlayerRepository(db)
	ctx := context.Background()

	t.Run("Create and Get Player", func(t *testing.T) {
		email := "test@example.com"
		phoneNumber := "233123456789"
		passwordHash := "hashed_password"
		firstName := "John"
		lastName := "Doe"

		req := models.CreatePlayerRequest{
			PhoneNumber:         phoneNumber,
			Email:               email,
			PasswordHash:        passwordHash,
			FirstName:           firstName,
			LastName:            lastName,
			RegistrationChannel: "mobile",
			TermsAccepted:       true,
			MarketingConsent:    false,
		}

		player, err := repo.Create(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, player.ID)
		assert.Equal(t, phoneNumber, player.PhoneNumber)
		assert.Equal(t, email, player.Email)
		assert.Equal(t, firstName, player.FirstName)
		assert.Equal(t, lastName, player.LastName)
		assert.Equal(t, models.PlayerStatusActive, player.Status)
		assert.Equal(t, "mobile", player.RegistrationChannel)
		assert.True(t, player.TermsAccepted)
		assert.False(t, player.MarketingConsent)

		// Test GetByID
		retrievedPlayer, err := repo.GetByID(ctx, player.ID)
		require.NoError(t, err)
		assert.Equal(t, player.ID, retrievedPlayer.ID)
		assert.Equal(t, phoneNumber, retrievedPlayer.PhoneNumber)

		// Test GetByPhoneNumber
		retrievedByPhone, err := repo.GetByPhoneNumber(ctx, phoneNumber)
		require.NoError(t, err)
		assert.Equal(t, player.ID, retrievedByPhone.ID)

		// Test GetByEmail
		retrievedByEmail, err := repo.GetByEmail(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, player.ID, retrievedByEmail.ID)
	})

	t.Run("Update Player", func(t *testing.T) {
		// Create a player first
		email := "update@example.com"
		phoneNumber := "233987654321"
		req := models.CreatePlayerRequest{
			PhoneNumber:         phoneNumber,
			Email:               email,
			PasswordHash:        "hash",
			RegistrationChannel: "web",
		}

		player, err := repo.Create(ctx, req)
		require.NoError(t, err)

		// Update player
		newFirstName := "Jane"
		newLastName := "Smith"
		newStatus := models.PlayerStatusSuspended
		now := time.Now()

		updateReq := models.UpdatePlayerRequest{
			ID:          player.ID,
			FirstName:   newFirstName,
			LastName:    newLastName,
			Status:      newStatus,
			LastLoginAt: now,
		}

		updatedPlayer, err := repo.Update(ctx, updateReq)
		require.NoError(t, err)
		assert.Equal(t, newFirstName, updatedPlayer.FirstName)
		assert.Equal(t, newLastName, updatedPlayer.LastName)
		assert.Equal(t, newStatus, updatedPlayer.Status)
		assert.Equal(t, now.Unix(), updatedPlayer.LastLoginAt.Unix())
	})

	t.Run("List Players with Filter", func(t *testing.T) {
		// Create multiple players
		phone1 := "233111111111"
		phone2 := "233222222222"
		email1 := "list1@example.com"
		email2 := "list2@example.com"

		req1 := models.CreatePlayerRequest{
			PhoneNumber:         phone1,
			Email:               email1,
			PasswordHash:        "hash1",
			RegistrationChannel: "mobile",
		}

		req2 := models.CreatePlayerRequest{
			PhoneNumber:         phone2,
			Email:               email2,
			PasswordHash:        "hash2",
			RegistrationChannel: "web",
		}

		_, err := repo.Create(ctx, req1)
		require.NoError(t, err)

		_, err = repo.Create(ctx, req2)
		require.NoError(t, err)

		// Test filter by channel
		mobileChannel := "mobile"
		filter := models.PlayerFilter{
			Channel: mobileChannel,
			Limit:   10,
		}

		players, err := repo.List(ctx, filter)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(players), 1)

		// Test count
		count, err := repo.Count(ctx, filter)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(1))
	})

	t.Run("Soft Delete Player", func(t *testing.T) {
		phoneNumber := "233333333333"
		req := models.CreatePlayerRequest{
			PhoneNumber:         phoneNumber,
			PasswordHash:        "hash",
			RegistrationChannel: "mobile",
		}

		player, err := repo.Create(ctx, req)
		require.NoError(t, err)

		// Soft delete
		err = repo.SoftDelete(ctx, player.ID)
		require.NoError(t, err)

		// Should not be found after soft delete
		deletedPlayer, err := repo.GetByID(ctx, player.ID)
		require.NoError(t, err)
		assert.Nil(t, deletedPlayer)
	})
}

func TestPlayerAuthRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPlayerAuthRepository(db)
	ctx := context.Background()

	t.Run("Validate Credentials", func(t *testing.T) {
		phoneNumber := "233444444444"
		passwordHash := "correct_hash"

		// Create player
		playerRepo := NewPlayerRepository(db)
		req := models.CreatePlayerRequest{
			PhoneNumber:         phoneNumber,
			PasswordHash:        passwordHash,
			RegistrationChannel: "mobile",
		}

		player, err := playerRepo.Create(ctx, req)
		require.NoError(t, err)

		// Test valid credentials
		validatedPlayer, err := repo.ValidateCredentials(ctx, phoneNumber, passwordHash)
		require.NoError(t, err)
		assert.NotNil(t, validatedPlayer)
		assert.Equal(t, player.ID, validatedPlayer.ID)

		// Test invalid credentials
		invalidPlayer, err := repo.ValidateCredentials(ctx, phoneNumber, "wrong_hash")
		require.NoError(t, err)
		assert.Nil(t, invalidPlayer)
	})

	t.Run("Update Password", func(t *testing.T) {
		phoneNumber := "233555555555"
		oldHash := "old_hash"
		newHash := "new_hash"

		// Create player
		playerRepo := NewPlayerRepository(db)
		req := models.CreatePlayerRequest{
			PhoneNumber:         phoneNumber,
			PasswordHash:        oldHash,
			RegistrationChannel: "mobile",
		}

		player, err := playerRepo.Create(ctx, req)
		require.NoError(t, err)

		// Update password
		err = repo.UpdatePassword(ctx, player.ID, newHash)
		require.NoError(t, err)

		// Verify password was updated
		hash, err := repo.GetPasswordHash(ctx, player.ID)
		require.NoError(t, err)
		assert.Equal(t, newHash, hash)
	})

	t.Run("Record Login Attempts", func(t *testing.T) {
		phoneNumber := "233666666666"
		deviceID := "device123"
		channel := "mobile"
		ipAddress := "192.168.1.1"

		// Record successful attempt
		err := repo.RecordLoginAttempt(ctx, phoneNumber, nil, deviceID, channel, ipAddress, "password", true, nil)
		require.NoError(t, err)

		// Record failed attempt
		err = repo.RecordLoginAttempt(ctx, phoneNumber, nil, deviceID, channel, ipAddress, "password", false, stringPtr("wrong_password"))
		require.NoError(t, err)

		// Get attempts
		attempts, err := repo.GetLoginAttempts(ctx, phoneNumber, 10)
		require.NoError(t, err)
		assert.Len(t, attempts, 2)
		assert.False(t, attempts[0].Success) // Most recent first
		assert.True(t, attempts[1].Success)
	})

	t.Run("Account Lock Status", func(t *testing.T) {
		phoneNumber := "233777777777"

		// Record multiple failed attempts
		for i := 0; i < 6; i++ {
			err := repo.RecordLoginAttempt(ctx, phoneNumber, nil, "device", "mobile", "192.168.1.1", "password", false, stringPtr("wrong"))
			require.NoError(t, err)
		}

		// Check if account is locked
		isLocked, err := repo.IsAccountLocked(ctx, phoneNumber)
		require.NoError(t, err)
		assert.True(t, isLocked)
	})
}

func TestSessionRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	t.Run("Create and Get Session", func(t *testing.T) {
		// Create a player first
		playerRepo := NewPlayerRepository(db)
		playerReq := models.CreatePlayerRequest{
			PhoneNumber:         "233888888888",
			PasswordHash:        "hash",
			RegistrationChannel: "mobile",
		}

		player, err := playerRepo.Create(ctx, playerReq)
		require.NoError(t, err)

		// Create session
		deviceID := "device123"
		refreshToken := "refresh_token_123"
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		req := models.CreateSessionRequest{
			PlayerID:     player.ID,
			DeviceID:     deviceID,
			RefreshToken: refreshToken,
			Channel:      "mobile",
			DeviceType:   "iOS",
			AppVersion:   "1.0.0",
			IPAddress:    "192.168.1.1",
			UserAgent:    "TestAgent",
			ExpiresAt:    expiresAt,
		}

		session, err := repo.Create(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, session.ID)
		assert.Equal(t, player.ID, session.PlayerID)
		assert.Equal(t, deviceID, session.DeviceID)
		assert.Equal(t, refreshToken, session.RefreshToken)
		assert.True(t, session.IsActive)

		// Test GetByID
		retrievedSession, err := repo.GetByID(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrievedSession.ID)

		// Test GetByRefreshToken
		retrievedByToken, err := repo.GetByRefreshToken(ctx, refreshToken)
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrievedByToken.ID)

		// Test GetByPlayerID
		playerSessions, err := repo.GetByPlayerID(ctx, player.ID)
		require.NoError(t, err)
		assert.Len(t, playerSessions, 1)
		assert.Equal(t, session.ID, playerSessions[0].ID)
	})

	t.Run("Update Session", func(t *testing.T) {
		// Create player and session
		playerRepo := NewPlayerRepository(db)
		playerReq := models.CreatePlayerRequest{
			PhoneNumber:         "233999999999",
			PasswordHash:        "hash",
			RegistrationChannel: "mobile",
		}

		player, err := playerRepo.Create(ctx, playerReq)
		require.NoError(t, err)

		sessionReq := models.CreateSessionRequest{
			PlayerID:     player.ID,
			DeviceID:     "device456",
			RefreshToken: "refresh_token_456",
			Channel:      "web",
			ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		}

		session, err := repo.Create(ctx, sessionReq)
		require.NoError(t, err)

		// Update session
		newJTI := "new_jti_123"
		now := time.Now()
		isActive := false

		updateReq := models.UpdateSessionRequest{
			ID:             session.ID,
			AccessTokenJTI: newJTI,
			LastUsedAt:     now,
			IsActive:       isActive,
		}

		updatedSession, err := repo.Update(ctx, updateReq)
		require.NoError(t, err)
		assert.Equal(t, newJTI, updatedSession.AccessTokenJTI)
		assert.Equal(t, now.Unix(), updatedSession.LastUsedAt.Unix())
		assert.False(t, updatedSession.IsActive)
	})

	t.Run("Revoke Session", func(t *testing.T) {
		// Create player and session
		playerRepo := NewPlayerRepository(db)
		playerReq := models.CreatePlayerRequest{
			PhoneNumber:         "233000000000",
			PasswordHash:        "hash",
			RegistrationChannel: "mobile",
		}

		player, err := playerRepo.Create(ctx, playerReq)
		require.NoError(t, err)

		sessionReq := models.CreateSessionRequest{
			PlayerID:     player.ID,
			DeviceID:     "device789",
			RefreshToken: "refresh_token_789",
			Channel:      "mobile",
			ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
		}

		session, err := repo.Create(ctx, sessionReq)
		require.NoError(t, err)

		// Revoke session
		err = repo.RevokeSession(ctx, session.ID, "user_logout")
		require.NoError(t, err)

		// Verify session is revoked
		retrievedSession, err := repo.GetByID(ctx, session.ID)
		require.NoError(t, err)
		assert.False(t, retrievedSession.IsActive)
		assert.NotNil(t, retrievedSession.RevokedAt)
		assert.Equal(t, "user_logout", retrievedSession.RevokedReason)
	})
}

func TestDeviceRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDeviceRepository(db)
	ctx := context.Background()

	t.Run("Create and Get Device", func(t *testing.T) {
		// Create a player first
		playerRepo := NewPlayerRepository(db)
		playerReq := models.CreatePlayerRequest{
			PhoneNumber:         "233111111111",
			PasswordHash:        "hash",
			RegistrationChannel: "mobile",
		}

		player, err := playerRepo.Create(ctx, playerReq)
		require.NoError(t, err)

		// Create device
		deviceID := "device123"
		deviceType := "mobile"
		deviceName := "iPhone 15"
		os := "iOS"
		osVersion := "17.0"
		fingerprint := "fingerprint123"

		req := models.CreateDeviceRequest{
			PlayerID:    player.ID,
			DeviceID:    deviceID,
			DeviceType:  deviceType,
			DeviceName:  deviceName,
			OS:          os,
			OSVersion:   osVersion,
			Fingerprint: fingerprint,
			TrustScore:  75,
		}

		device, err := repo.Create(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, device.ID)
		assert.Equal(t, player.ID, device.PlayerID)
		assert.Equal(t, deviceID, device.DeviceID)
		assert.Equal(t, deviceType, device.DeviceType)
		assert.Equal(t, deviceName, device.DeviceName)
		assert.Equal(t, os, device.DeviceOS)
		assert.Equal(t, osVersion, device.OSVersion)
		assert.Equal(t, fingerprint, device.Fingerprint)
		assert.Equal(t, 75, device.TrustScore)
		assert.False(t, device.IsTrusted)
		assert.False(t, device.IsBlocked)

		// Test GetByID
		retrievedDevice, err := repo.GetByID(ctx, device.ID)
		require.NoError(t, err)
		assert.Equal(t, device.ID, retrievedDevice.ID)

		// Test GetByPlayerAndDeviceID
		retrievedByPlayerDevice, err := repo.GetByPlayerAndDeviceID(ctx, player.ID, deviceID)
		require.NoError(t, err)
		assert.Equal(t, device.ID, retrievedByPlayerDevice.ID)

		// Test GetByPlayerID
		playerDevices, err := repo.GetByPlayerID(ctx, player.ID)
		require.NoError(t, err)
		assert.Len(t, playerDevices, 1)
		assert.Equal(t, device.ID, playerDevices[0].ID)
	})

	t.Run("Update Device", func(t *testing.T) {
		// Create player and device
		playerRepo := NewPlayerRepository(db)
		playerReq := models.CreatePlayerRequest{
			PhoneNumber:         "233222222222",
			PasswordHash:        "hash",
			RegistrationChannel: "mobile",
		}

		player, err := playerRepo.Create(ctx, playerReq)
		require.NoError(t, err)

		deviceReq := models.CreateDeviceRequest{
			PlayerID:   player.ID,
			DeviceID:   "device456",
			TrustScore: 50,
		}

		device, err := repo.Create(ctx, deviceReq)
		require.NoError(t, err)

		// Update device
		newName := "Updated Device Name"
		newTrustScore := 90
		isTrusted := true
		now := time.Now()

		updateReq := models.UpdateDeviceRequest{
			ID:         device.ID,
			DeviceName: newName,
			TrustScore: newTrustScore,
			IsTrusted:  isTrusted,
			LastSeenAt: now,
		}

		updatedDevice, err := repo.Update(ctx, updateReq)
		require.NoError(t, err)
		assert.Equal(t, newName, updatedDevice.DeviceName)
		assert.Equal(t, newTrustScore, updatedDevice.TrustScore)
		assert.True(t, updatedDevice.IsTrusted)
		assert.Equal(t, now.Unix(), updatedDevice.LastSeenAt.Unix())
	})

	t.Run("Block and Unblock Device", func(t *testing.T) {
		// Create player and device
		playerRepo := NewPlayerRepository(db)
		playerReq := models.CreatePlayerRequest{
			PhoneNumber:         "233333333333",
			PasswordHash:        "hash",
			RegistrationChannel: "mobile",
		}

		player, err := playerRepo.Create(ctx, playerReq)
		require.NoError(t, err)

		deviceReq := models.CreateDeviceRequest{
			PlayerID: player.ID,
			DeviceID: "device789",
		}

		device, err := repo.Create(ctx, deviceReq)
		require.NoError(t, err)

		// Block device
		err = repo.BlockDevice(ctx, player.ID, device.DeviceID, "suspicious_activity")
		require.NoError(t, err)

		// Verify device is blocked
		retrievedDevice, err := repo.GetByID(ctx, device.ID)
		require.NoError(t, err)
		assert.True(t, retrievedDevice.IsBlocked)

		// Unblock device
		err = repo.UnblockDevice(ctx, player.ID, device.DeviceID)
		require.NoError(t, err)

		// Verify device is unblocked
		retrievedDevice, err = repo.GetByID(ctx, device.ID)
		require.NoError(t, err)
		assert.False(t, retrievedDevice.IsBlocked)
	})
}

func stringPtr(s string) *string {
	return &s
}
