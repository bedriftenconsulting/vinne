package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/randco/service-wallet/internal/models"
	redisClient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

type WalletRepositoryTestSuite struct {
	suite.Suite
	db             *sql.DB
	sqlxDB         *sqlx.DB
	redisClient    *redisClient.Client
	pgContainer    *postgres.PostgresContainer
	redisContainer *redis.RedisContainer
	walletRepo     WalletTransactionRepository
	adminRepo      AdminTransactionRepository
}

func TestWalletRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(WalletRepositoryTestSuite))
}

func (s *WalletRepositoryTestSuite) SetupSuite() {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("wallet_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(s.T(), err)
	s.pgContainer = postgresContainer

	// Get PostgreSQL connection details
	dbHost, err := postgresContainer.Host(ctx)
	require.NoError(s.T(), err)
	dbPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(s.T(), err)

	// Connect to PostgreSQL
	dsn := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=wallet_test sslmode=disable",
		dbHost, dbPort.Port())
	sqlxDB, err := sqlx.Connect("postgres", dsn)
	require.NoError(s.T(), err)
	s.sqlxDB = sqlxDB
	s.db = sqlxDB.DB

	// Run migrations
	err = goose.SetDialect("postgres")
	require.NoError(s.T(), err)
	migrationsDir := filepath.Join("..", "..", "migrations")
	err = goose.Up(s.db, migrationsDir)
	require.NoError(s.T(), err)

	// Start Redis container
	redisC, err := redis.Run(ctx, "redis:7-alpine",
		redis.WithSnapshotting(10, 1),
	)
	require.NoError(s.T(), err)
	s.redisContainer = redisC

	// Get Redis connection details
	redisHost, err := redisC.Host(ctx)
	require.NoError(s.T(), err)
	redisPort, err := redisC.MappedPort(ctx, "6379")
	require.NoError(s.T(), err)

	// Connect to Redis
	s.redisClient = redisClient.NewClient(&redisClient.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort.Port()),
		DB:   0,
	})

	// Verify Redis connection
	err = s.redisClient.Ping(ctx).Err()
	require.NoError(s.T(), err)

	// Initialize repositories
	s.walletRepo = NewWalletTransactionRepository(s.db, s.redisClient)
	s.adminRepo = NewAdminTransactionRepository(s.db, s.redisClient)
}

func (s *WalletRepositoryTestSuite) TearDownSuite() {
	if s.db != nil {
		_ = s.db.Close()
	}
	if s.redisClient != nil {
		_ = s.redisClient.Close()
	}
	if s.pgContainer != nil {
		_ = s.pgContainer.Terminate(context.Background())
	}
	if s.redisContainer != nil {
		_ = s.redisContainer.Terminate(context.Background())
	}
}

func (s *WalletRepositoryTestSuite) SetupTest() {
	// Clean up database before each test
	ctx := context.Background()
	_, _ = s.db.ExecContext(ctx, "TRUNCATE wallet_transactions CASCADE")

	// Clear Redis cache
	_ = s.redisClient.FlushDB(ctx).Err()
}

// TestGetTransactionWithMetadata tests that JSONB metadata is properly scanned and unmarshaled
func (s *WalletRepositoryTestSuite) TestGetTransactionWithMetadata() {
	ctx := context.Background()

	// Create a test transaction with JSONB metadata
	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())
	metadata := map[string]interface{}{
		"wallet_owner_name": "Test Agent",
		"wallet_owner_code": "AG123",
		"source":            "test",
		"notes":             "Test transaction with metadata",
	}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(s.T(), err)

	// Insert transaction directly into database
	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())`

	_, err = s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
		100000, 0, 100000, "REF-001", "Test credit",
		models.TransactionStatusCompleted, uuid.NewString(), metadataJSON)
	require.NoError(s.T(), err)

	// Get transaction using repository method
	tx, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx)

	// Verify metadata was properly unmarshaled
	require.NotNil(s.T(), tx.Metadata)
	require.Equal(s.T(), "Test Agent", tx.Metadata["wallet_owner_name"])
	require.Equal(s.T(), "AG123", tx.Metadata["wallet_owner_code"])
	require.Equal(s.T(), "test", tx.Metadata["source"])
	require.Equal(s.T(), "Test transaction with metadata", tx.Metadata["notes"])

	s.T().Log("✅ Successfully scanned and unmarshaled JSONB metadata")
}

// TestGetTransactionByIdempotencyKeyWithMetadata tests metadata scanning in GetTransactionByIdempotencyKey
func (s *WalletRepositoryTestSuite) TestGetTransactionByIdempotencyKeyWithMetadata() {
	ctx := context.Background()

	// Create a test transaction
	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())
	idempotencyKey := uuid.NewString()
	metadata := map[string]interface{}{
		"wallet_owner_name": "Test Retailer",
		"wallet_owner_code": "RT456",
	}
	metadataJSON, err := json.Marshal(metadata)
	require.NoError(s.T(), err)

	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())`

	_, err = s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeRetailerStake, models.TransactionTypeDebit,
		50000, 100000, 50000, "REF-002", "Test debit",
		models.TransactionStatusCompleted, idempotencyKey, metadataJSON)
	require.NoError(s.T(), err)

	// Get transaction by idempotency key
	tx, err := s.walletRepo.GetTransactionByIdempotencyKey(ctx, idempotencyKey)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx)

	// Verify metadata
	require.NotNil(s.T(), tx.Metadata)
	require.Equal(s.T(), "Test Retailer", tx.Metadata["wallet_owner_name"])
	require.Equal(s.T(), "RT456", tx.Metadata["wallet_owner_code"])

	s.T().Log("✅ GetTransactionByIdempotencyKey properly handles JSONB metadata")
}

// TestGetTransactionCaching tests that GetTransaction uses Redis cache
func (s *WalletRepositoryTestSuite) TestGetTransactionCaching() {
	ctx := context.Background()

	// Create a test transaction
	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())

	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NOW())`

	_, err := s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
		100000, 0, 100000, "REF-003", "Cache test",
		models.TransactionStatusCompleted, uuid.NewString())
	require.NoError(s.T(), err)

	// First call - should hit database and populate cache
	tx1, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx1)

	// Verify cache was populated
	cacheKey := fmt.Sprintf("transaction:id:%s", transactionID)
	cached, err := s.redisClient.Get(ctx, cacheKey).Result()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), cached)
	s.T().Log("✅ Cache populated after first GetTransaction call")

	// Second call - should hit cache
	tx2, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx2)
	require.Equal(s.T(), tx1.TransactionID, tx2.TransactionID)
	require.Equal(s.T(), tx1.Amount, tx2.Amount)

	s.T().Log("✅ Second GetTransaction call served from cache")

	// Verify TTL is set (30 minutes)
	ttl, err := s.redisClient.TTL(ctx, cacheKey).Result()
	require.NoError(s.T(), err)
	require.Greater(s.T(), ttl.Minutes(), 25.0) // Should be close to 30 minutes
	require.LessOrEqual(s.T(), ttl.Minutes(), 30.0)

	s.T().Log("✅ Cache TTL set correctly to 30 minutes")
}

// TestUpdateTransactionStatusInvalidatesCache tests cache invalidation on status update
func (s *WalletRepositoryTestSuite) TestUpdateTransactionStatusInvalidatesCache() {
	ctx := context.Background()

	// Create a test transaction
	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())

	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NOW())`

	_, err := s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
		100000, 0, 100000, "REF-004", "Status update test",
		models.TransactionStatusPending, uuid.NewString())
	require.NoError(s.T(), err)

	// Cache the transaction
	tx, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.TransactionStatusPending, tx.Status)

	// Verify cache exists
	cacheKey := fmt.Sprintf("transaction:id:%s", transactionID)
	cached, err := s.redisClient.Get(ctx, cacheKey).Result()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), cached)

	// Update transaction status
	err = s.walletRepo.UpdateTransactionStatus(ctx, transactionID, models.TransactionStatusCompleted)
	require.NoError(s.T(), err)

	// Verify specific transaction cache was invalidated
	_, err = s.redisClient.Get(ctx, cacheKey).Result()
	require.Error(s.T(), err)
	require.Equal(s.T(), redisClient.Nil, err)
	s.T().Log("✅ Specific transaction cache invalidated after status update")

	// Verify all admin caches were invalidated
	adminKeys, err := s.redisClient.Keys(ctx, "transactions:all:*").Result()
	require.NoError(s.T(), err)
	require.Empty(s.T(), adminKeys)
	s.T().Log("✅ Admin transaction caches invalidated")

	// Verify owner-specific caches were invalidated
	ownerPattern := fmt.Sprintf("transactions:owner:%s:*", agentID.String())
	ownerKeys, err := s.redisClient.Keys(ctx, ownerPattern).Result()
	require.NoError(s.T(), err)
	require.Empty(s.T(), ownerKeys)
	s.T().Log("✅ Owner-specific caches invalidated")

	// Fetch again - should get updated status from database
	txUpdated, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), models.TransactionStatusCompleted, txUpdated.Status)
	require.NotNil(s.T(), txUpdated.CompletedAt)

	s.T().Log("✅ UpdateTransactionStatus successfully invalidates all relevant caches")
}

// TestGetTransactionHistoryCaching tests transaction history caching
func (s *WalletRepositoryTestSuite) TestGetTransactionHistoryCaching() {
	ctx := context.Background()

	// Create multiple transactions for the same owner
	agentID := uuid.New()
	for i := 0; i < 5; i++ {
		transactionID := fmt.Sprintf("TXN-%d-%d", time.Now().UnixNano(), i)
		query := `
			INSERT INTO wallet_transactions (
				transaction_id, wallet_owner_id, wallet_type, transaction_type,
				amount, balance_before, balance_after, reference, description,
				status, idempotency_key, metadata, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NOW())`

		_, err := s.db.ExecContext(ctx, query,
			transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
			10000*(i+1), 0, 10000*(i+1), fmt.Sprintf("REF-%d", i), "History test",
			models.TransactionStatusCompleted, uuid.NewString())
		require.NoError(s.T(), err)
	}

	// First call - should populate cache
	txs1, err := s.walletRepo.GetTransactionHistory(ctx, agentID, models.WalletTypeAgentStake, 20, 0)
	require.NoError(s.T(), err)
	require.Len(s.T(), txs1, 5)

	// Verify cache was populated (5 minute TTL)
	cacheKey := fmt.Sprintf("transactions:owner:%s:type:%s:page:1:size:20",
		agentID.String(), models.WalletTypeAgentStake)
	cached, err := s.redisClient.Get(ctx, cacheKey).Result()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), cached)
	s.T().Log("✅ Transaction history cached")

	// Verify TTL
	ttl, err := s.redisClient.TTL(ctx, cacheKey).Result()
	require.NoError(s.T(), err)
	require.Greater(s.T(), ttl.Minutes(), 4.0)
	require.LessOrEqual(s.T(), ttl.Minutes(), 5.0)

	s.T().Log("✅ Transaction history cache TTL set to 5 minutes")
}

// TestCreateTransactionInvalidatesCache tests that creating a transaction invalidates caches
func (s *WalletRepositoryTestSuite) TestCreateTransactionInvalidatesCache() {
	ctx := context.Background()

	agentID := uuid.New()

	// Create and cache a transaction history
	txs, err := s.walletRepo.GetTransactionHistory(ctx, agentID, models.WalletTypeAgentStake, 20, 0)
	require.NoError(s.T(), err)
	require.Empty(s.T(), txs) // No transactions yet

	// Manually populate some cache keys
	s.redisClient.Set(ctx, "transactions:all:filters:abc123:page:1", "dummy", 5*time.Minute)
	s.redisClient.Set(ctx, "transactions:stats:filters:abc123", "dummy", 5*time.Minute)

	// Verify caches exist
	keys, err := s.redisClient.Keys(ctx, "transactions:*").Result()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), keys)

	// Create a new transaction using CreateTransaction
	reference := "REF-NEW"
	description := "New transaction"
	idempotencyKey := uuid.NewString()
	tx := &models.WalletTransaction{
		TransactionID:   fmt.Sprintf("TXN-%d", time.Now().UnixNano()),
		WalletOwnerID:   agentID,
		WalletType:      models.WalletTypeAgentStake,
		TransactionType: models.TransactionTypeCredit,
		Amount:          100000,
		BalanceBefore:   0,
		BalanceAfter:    100000,
		Reference:       &reference,
		Description:     &description,
		Status:          models.TransactionStatusCompleted,
		CreditSource:    models.CreditSourceAdminDirect,
		IdempotencyKey:  &idempotencyKey,
		Metadata:        map[string]interface{}{"test": "value"},
	}

	err = s.walletRepo.CreateTransaction(ctx, tx)
	require.NoError(s.T(), err)

	// Verify all transaction caches were invalidated
	keys, err = s.redisClient.Keys(ctx, "transactions:*").Result()
	require.NoError(s.T(), err)
	require.Empty(s.T(), keys)

	s.T().Log("✅ CreateTransaction invalidates all relevant caches")
}

// TestEmptyMetadataHandling tests that transactions without metadata don't cause errors
func (s *WalletRepositoryTestSuite) TestEmptyMetadataHandling() {
	ctx := context.Background()

	// Create transaction without metadata (NULL in database)
	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())

	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NULL, NOW())`

	_, err := s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
		100000, 0, 100000, "REF-005", "No metadata",
		models.TransactionStatusCompleted, uuid.NewString())
	require.NoError(s.T(), err)

	// Get transaction - should not error even with NULL metadata
	tx, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx)

	// Metadata should be nil or empty
	if tx.Metadata != nil {
		require.Empty(s.T(), tx.Metadata)
	}

	s.T().Log("✅ Empty/NULL metadata handled correctly")
}
