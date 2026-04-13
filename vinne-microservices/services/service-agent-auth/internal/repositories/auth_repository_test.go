package repositories

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/randco/randco-microservices/services/service-agent-auth/internal/models"
	redisClient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
)

// setupTestDB creates a test database and Redis using Testcontainers
func setupTestDB(t *testing.T) (*sqlx.DB, *redisClient.Client, func()) {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("test_db"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	// Start Redis container
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		redis.WithSnapshotting(10, 1),
	)
	require.NoError(t, err)

	// Get PostgreSQL connection string
	dbHost, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	dbPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dsn := fmt.Sprintf("host=%s port=%s user=test_user password=test_password dbname=test_db sslmode=disable",
		dbHost, dbPort.Port())

	// Connect to database
	db, err := sqlx.Connect("postgres", dsn)
	require.NoError(t, err)

	// Get Redis connection string
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	// Connect to Redis
	rdb := redisClient.NewClient(&redisClient.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
	})

	// Run migrations using Goose to ensure consistency with real schema
	runMigrations(t, db)

	// Return database, redis and cleanup function
	cleanup := func() {
		_ = db.Close()
		_ = rdb.Close()
		_ = postgresContainer.Terminate(ctx)
		_ = redisContainer.Terminate(ctx)
	}

	return db, rdb, cleanup
}

func runMigrations(t *testing.T, db *sqlx.DB) {
	// Set Goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("Failed to set goose dialect: %v", err)
	}

	// Get the migrations directory path relative to the test file
	migrationsDir := filepath.Join("..", "..", "migrations")

	// Convert sqlx.DB to sql.DB for Goose
	sqlDB := db.DB

	// Run up migrations
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Log("✅ Successfully ran all migrations using Goose")
}

func TestAuthRepository_AgentLogin(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAuthRepository(db, rdb)

	// Create test agent
	agentID := uuid.New()
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	_, err := db.Exec(`
		INSERT INTO agents_auth (id, agent_code, phone, email, password_hash)
		VALUES ($1, $2, $3, $4, $5)`,
		agentID, "AGT-2025-000001", "+233500000001", "agent@test.com", string(hashedPassword))
	require.NoError(t, err)

	// Test GetAgentByPhone
	t.Run("GetAgentByPhone", func(t *testing.T) {
		user, err := repo.GetAgentByPhone(context.Background(), "+233500000001")
		require.NoError(t, err)
		assert.Equal(t, agentID, user.ID)
		assert.Equal(t, "AGT-2025-000001", user.Code)
		assert.NotNil(t, user.Phone)
		assert.Equal(t, "+233500000001", *user.Phone)
		assert.NotNil(t, user.Email)
		assert.Equal(t, "agent@test.com", *user.Email)
		assert.True(t, user.IsActive)
	})

	// Test GetAgentByEmail
	t.Run("GetAgentByEmail", func(t *testing.T) {
		user, err := repo.GetAgentByEmail(context.Background(), "agent@test.com")
		require.NoError(t, err)
		assert.Equal(t, agentID, user.ID)
		assert.Equal(t, "AGT-2025-000001", user.Code)
		assert.NotNil(t, user.Phone)
		assert.Equal(t, "+233500000001", *user.Phone)
	})

	// Test GetAgentByCode
	t.Run("GetAgentByCode", func(t *testing.T) {
		user, err := repo.GetAgentByCode(context.Background(), "AGT-2025-000001")
		require.NoError(t, err)
		assert.Equal(t, agentID, user.ID)
		assert.NotNil(t, user.Phone)
		assert.Equal(t, "+233500000001", *user.Phone)
	})

	// Test phone not found
	t.Run("PhoneNotFound", func(t *testing.T) {
		_, err := repo.GetAgentByPhone(context.Background(), "+233999999999")
		assert.Error(t, err)
	})
}

func TestAuthRepository_RetailerLogin(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAuthRepository(db, rdb)

	// Create test retailer
	retailerID := uuid.New()
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	hashedPIN, _ := bcrypt.GenerateFromPassword([]byte("1234"), bcrypt.DefaultCost)

	_, err := db.Exec(`
		INSERT INTO retailers_auth (id, retailer_code, email, phone, password_hash, pin_hash)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		retailerID, "12345678", "retailer@test.com", "+233123456789", string(hashedPassword), string(hashedPIN))
	require.NoError(t, err)

	// Test GetRetailerByEmail
	t.Run("GetRetailerByEmail", func(t *testing.T) {
		user, err := repo.GetRetailerByEmail(context.Background(), "retailer@test.com")
		require.NoError(t, err)
		assert.Equal(t, retailerID, user.ID)
		assert.Equal(t, "12345678", user.Code)
		assert.NotNil(t, user.Email)
		assert.Equal(t, "retailer@test.com", *user.Email)
		assert.NotNil(t, user.Phone)
		assert.Equal(t, "+233123456789", *user.Phone)
		assert.NotNil(t, user.PinHash)
		assert.True(t, user.IsActive)
	})

	// Test GetRetailerByPhone
	t.Run("GetRetailerByPhone", func(t *testing.T) {
		user, err := repo.GetRetailerByPhone(context.Background(), "+233123456789")
		require.NoError(t, err)
		assert.Equal(t, retailerID, user.ID)
		assert.Equal(t, "12345678", user.Code)
	})

	// Test GetRetailerByCode
	t.Run("GetRetailerByCode", func(t *testing.T) {
		user, err := repo.GetRetailerByCode(context.Background(), "12345678")
		require.NoError(t, err)
		assert.Equal(t, retailerID, user.ID)
		assert.NotNil(t, user.PinHash)
	})

	// Test GetRetailerByID
	t.Run("GetRetailerByID", func(t *testing.T) {
		user, err := repo.GetRetailerByID(context.Background(), retailerID)
		require.NoError(t, err)
		assert.Equal(t, "12345678", user.Code)
	})

	// Test retailer not found
	t.Run("RetailerNotFound", func(t *testing.T) {
		_, err := repo.GetRetailerByEmail(context.Background(), "notfound@test.com")
		assert.Error(t, err)
	})
}

func TestAuthRepository_AgentAccountOperations(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAuthRepository(db, rdb)

	// Create test agent
	agentID := uuid.New()
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	_, err := db.Exec(`
		INSERT INTO agents_auth (id, agent_code, phone, email, password_hash)
		VALUES ($1, $2, $3, $4, $5)`,
		agentID, "AGT-2025-000002", "+233500000002", "agent2@test.com", string(hashedPassword))
	require.NoError(t, err)

	// Test UpdateAgentLastLogin
	t.Run("UpdateAgentLastLogin", func(t *testing.T) {
		err := repo.UpdateAgentLastLogin(context.Background(), agentID)
		require.NoError(t, err)

		// Verify last_login_at was updated
		var lastLogin *time.Time
		var failedAttempts int
		err = db.QueryRow("SELECT last_login_at, failed_login_attempts FROM agents_auth WHERE id = $1", agentID).Scan(&lastLogin, &failedAttempts)
		require.NoError(t, err)
		assert.NotNil(t, lastLogin)
		assert.Equal(t, 0, failedAttempts) // Should reset failed attempts
	})

	// Test IncrementAgentFailedLogin
	t.Run("IncrementAgentFailedLogin", func(t *testing.T) {
		attempts, err := repo.IncrementAgentFailedLogin(context.Background(), agentID)
		require.NoError(t, err)
		assert.Equal(t, 1, attempts)

		attempts, err = repo.IncrementAgentFailedLogin(context.Background(), agentID)
		require.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})

	// Test LockAgentAccount
	t.Run("LockAgentAccount", func(t *testing.T) {
		lockUntil := time.Now().Add(30 * time.Minute)
		err := repo.LockAgentAccount(context.Background(), agentID, lockUntil)
		require.NoError(t, err)

		// Verify account is locked
		var lockedUntil *time.Time
		err = db.QueryRow("SELECT locked_until FROM agents_auth WHERE id = $1", agentID).Scan(&lockedUntil)
		require.NoError(t, err)
		assert.NotNil(t, lockedUntil)
		assert.True(t, lockedUntil.After(time.Now()))
	})

	// Test UnlockAgentAccount
	t.Run("UnlockAgentAccount", func(t *testing.T) {
		err := repo.UnlockAgentAccount(context.Background(), agentID)
		require.NoError(t, err)

		// Verify account is unlocked
		var lockedUntil *time.Time
		var failedAttempts int
		err = db.QueryRow("SELECT locked_until, failed_login_attempts FROM agents_auth WHERE id = $1", agentID).Scan(&lockedUntil, &failedAttempts)
		require.NoError(t, err)
		assert.Nil(t, lockedUntil)
		assert.Equal(t, 0, failedAttempts) // Should reset failed attempts
	})

	// Test UpdateAgentPassword
	t.Run("UpdateAgentPassword", func(t *testing.T) {
		newHashedPassword, _ := bcrypt.GenerateFromPassword([]byte("newpassword123"), bcrypt.DefaultCost)
		err := repo.UpdateAgentPassword(context.Background(), agentID, string(newHashedPassword))
		require.NoError(t, err)

		// Verify password was updated
		var passwordHash string
		var passwordChangedAt *time.Time
		err = db.QueryRow("SELECT password_hash, password_changed_at FROM agents_auth WHERE id = $1", agentID).Scan(&passwordHash, &passwordChangedAt)
		require.NoError(t, err)
		assert.Equal(t, string(newHashedPassword), passwordHash)
		assert.NotNil(t, passwordChangedAt)
	})
}

func TestAuthRepository_CreateAgent(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAuthRepository(db, rdb)

	t.Run("CreateNewAgent", func(t *testing.T) {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		email := "newagent@test.com"
		phone := "+233500000003"

		agent := &AuthUser{
			ID:           uuid.New(),
			Code:         "AGT-2025-000003",
			Email:        &email,
			Phone:        &phone,
			PasswordHash: string(hashedPassword),
			IsActive:     true,
		}

		err := repo.CreateAgent(context.Background(), agent)
		require.NoError(t, err)

		// Verify agent was created
		createdAgent, err := repo.GetAgentByCode(context.Background(), "AGT-2025-000003")
		require.NoError(t, err)
		assert.Equal(t, agent.ID, createdAgent.ID)
		assert.Equal(t, agent.Code, createdAgent.Code)
		assert.NotNil(t, createdAgent.Email)
		assert.Equal(t, email, *createdAgent.Email)
		assert.NotNil(t, createdAgent.Phone)
		assert.Equal(t, phone, *createdAgent.Phone)
	})

	t.Run("CreateAgentDuplicateCode", func(t *testing.T) {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		email := "duplicate@test.com"
		phone := "+233500000004"

		agent := &AuthUser{
			ID:           uuid.New(),
			Code:         "AGT-2025-000003", // Same code as above
			Email:        &email,
			Phone:        &phone,
			PasswordHash: string(hashedPassword),
			IsActive:     true,
		}

		err := repo.CreateAgent(context.Background(), agent)
		assert.Error(t, err) // Should fail due to duplicate code
	})
}

func TestAuthRepository_RetailerAccountOperations(t *testing.T) {
	db, rdb, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewAuthRepository(db, rdb)

	// Create test retailer
	retailerID := uuid.New()
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	_, err := db.Exec(`
		INSERT INTO retailers_auth (id, retailer_code, email, phone, password_hash)
		VALUES ($1, $2, $3, $4, $5)`,
		retailerID, "87654321", "retailer2@test.com", "+233123456780", string(hashedPassword))
	require.NoError(t, err)

	// Test UpdateRetailerLastLogin
	t.Run("UpdateRetailerLastLogin", func(t *testing.T) {
		err := repo.UpdateRetailerLastLogin(context.Background(), retailerID)
		require.NoError(t, err)

		// Verify last_login_at was updated
		var lastLogin *time.Time
		var failedAttempts int
		err = db.QueryRow("SELECT last_login_at, failed_login_attempts FROM retailers_auth WHERE id = $1", retailerID).Scan(&lastLogin, &failedAttempts)
		require.NoError(t, err)
		assert.NotNil(t, lastLogin)
		assert.Equal(t, 0, failedAttempts)
	})

	// Test IncrementRetailerFailedLogin
	t.Run("IncrementRetailerFailedLogin", func(t *testing.T) {
		attempts, err := repo.IncrementRetailerFailedLogin(context.Background(), retailerID)
		require.NoError(t, err)
		assert.Equal(t, 1, attempts)

		attempts, err = repo.IncrementRetailerFailedLogin(context.Background(), retailerID)
		require.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})

	// Test LockRetailerAccount
	t.Run("LockRetailerAccount", func(t *testing.T) {
		lockUntil := time.Now().Add(30 * time.Minute)
		err := repo.LockRetailerAccount(context.Background(), retailerID, lockUntil)
		require.NoError(t, err)

		// Verify account is locked
		var lockedUntil *time.Time
		err = db.QueryRow("SELECT locked_until FROM retailers_auth WHERE id = $1", retailerID).Scan(&lockedUntil)
		require.NoError(t, err)
		assert.NotNil(t, lockedUntil)
		assert.True(t, lockedUntil.After(time.Now()))
	})

	// Test UnlockRetailerAccount
	t.Run("UnlockRetailerAccount", func(t *testing.T) {
		err := repo.UnlockRetailerAccount(context.Background(), retailerID)
		require.NoError(t, err)

		// Verify account is unlocked
		var lockedUntil *time.Time
		var failedAttempts int
		err = db.QueryRow("SELECT locked_until, failed_login_attempts FROM retailers_auth WHERE id = $1", retailerID).Scan(&lockedUntil, &failedAttempts)
		require.NoError(t, err)
		assert.Nil(t, lockedUntil)
		assert.Equal(t, 0, failedAttempts)
	})

	// Test UpdateRetailerPassword
	t.Run("UpdateRetailerPassword", func(t *testing.T) {
		newHashedPassword, _ := bcrypt.GenerateFromPassword([]byte("newpassword123"), bcrypt.DefaultCost)
		err := repo.UpdateRetailerPassword(context.Background(), retailerID, string(newHashedPassword))
		require.NoError(t, err)

		// Verify password was updated
		var passwordHash string
		err = db.QueryRow("SELECT password_hash FROM retailers_auth WHERE id = $1", retailerID).Scan(&passwordHash)
		require.NoError(t, err)
		assert.Equal(t, string(newHashedPassword), passwordHash)
	})

	// Test UpdateRetailerPin
	t.Run("UpdateRetailerPin", func(t *testing.T) {
		newHashedPin, _ := bcrypt.GenerateFromPassword([]byte("5678"), bcrypt.DefaultCost)
		err := repo.UpdateRetailerPin(context.Background(), retailerID, string(newHashedPin))
		require.NoError(t, err)

		// Verify PIN was updated
		var pinHash *string
		err = db.QueryRow("SELECT pin_hash FROM retailers_auth WHERE id = $1", retailerID).Scan(&pinHash)
		require.NoError(t, err)
		assert.NotNil(t, pinHash)
		assert.Equal(t, string(newHashedPin), *pinHash)
	})

	// Test CreatePinChangeLog
	t.Run("CreatePINChangeLog", func (t *testing.T)  {
		log:= &models.PINChangeLog{
			ID: uuid.New(),
			RetailerID: retailerID,
			RetailerCode: "87654321",
			DeviceIMEI: "867400020220040",
			IPAddress: "192.168.1.100",
			SessionsInvalidated: 3,
			ChangedBy: retailerID,
			Success: true,
			ChangeReason: "pin security compromised",
			FailureReason: "",
			CreatedAt: time.Now(),
		}
		err := repo.CreatePINChangeLog(context.Background(),log)
		require.NoError(t, err)
	})
}
