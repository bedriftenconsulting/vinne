package services

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/randco/service-wallet/internal/models"
	"github.com/randco/service-wallet/internal/repositories"
)

// setupTestContainer creates a PostgreSQL container for testing
func setupTestContainer(t *testing.T) (testcontainers.Container, *sql.DB, *sqlx.DB, func()) {
	ctx := context.Background()

	// Create PostgreSQL container
	req := testcontainers.ContainerRequest{
		Image:        "postgres:17-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "wallet_test",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "Failed to start PostgreSQL container")

	// Get connection details
	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	// Connect to database
	dsn := fmt.Sprintf("host=%s port=%s user=test password=test dbname=wallet_test sslmode=disable", host, port.Port())
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err, "Failed to connect to test database")

	// Wait for connection to be ready
	err = db.Ping()
	require.NoError(t, err, "Failed to ping database")

	// Create sqlx wrapper
	dbx := sqlx.NewDb(db, "postgres")

	// Run migrations
	migrationsPath := filepath.Join("..", "..", "migrations")
	err = goose.Up(db, migrationsPath)
	require.NoError(t, err, "Failed to run migrations")

	cleanup := func() {
		_ = dbx.Close()
		_ = db.Close()
		_ = container.Terminate(ctx)
	}

	return container, db, dbx, cleanup
}

// setupTestRedis creates a mock Redis client for testing
func setupTestRedis() *redis.Client {
	// Use a simple mock or in-memory Redis for testing
	// For this test, we'll use a real Redis client that connects to nothing
	// The cache operations will fail gracefully
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // This might not be available, but won't block tests
	})
}

// TestConcurrentWalletCredits tests that concurrent credit operations don't cause race conditions
func TestConcurrentWalletCredits(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo not needed for this test
		nil, // commissionService not needed for this test
		nil, // eventPublisher not needed for this test
		nil, // agentClient not needed for this test
	)

	// Create a test retailer
	retailerID := uuid.New()

	// Test concurrent credits
	numConcurrentCredits := 10
	creditAmount := int64(100000) // 1000.00 GHS in pesewas
	expectedFinalBalance := creditAmount * int64(numConcurrentCredits)

	var wg sync.WaitGroup
	errors := make(chan error, numConcurrentCredits)

	// Launch concurrent credit operations
	for i := 0; i < numConcurrentCredits; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Test credit #%d", index),
				"", // no idempotency key for this test
				models.CreditSourceAdminDirect,
			)

			if err != nil {
				errors <- err
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Credit operation failed: %v", err)
	}

	// Verify final balance
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err, "Failed to get wallet after concurrent credits")
	assert.Equal(t, expectedFinalBalance, wallet.Balance,
		"Final balance should equal sum of all credits (race condition prevented)")

	// Verify transaction count (exclude audit transactions which have "audit_trace" metadata)
	allTransactions, err := transactionRepo.GetTransactionHistory(ctx, retailerID, models.WalletTypeRetailerWinning, 100, 0)
	require.NoError(t, err, "Failed to get transaction history")

	// Count only business transactions (excluding audit transactions)
	businessTransactions := 0
	auditTransactions := 0
	for _, tx := range allTransactions {
		// Check if this is an audit transaction by looking for audit_trace in metadata
		if tx.Metadata != nil {
			if auditTrace, exists := tx.Metadata["audit_trace"]; exists && auditTrace == true {
				auditTransactions++
				continue
			}
		}
		businessTransactions++
	}

	assert.Equal(t, numConcurrentCredits, businessTransactions,
		"Should have exactly %d business transactions (excluding audit)", numConcurrentCredits)
	t.Logf("Transaction breakdown: %d total (%d business, %d audit)",
		len(allTransactions), businessTransactions, auditTransactions)

	t.Logf("SUCCESS: All %d concurrent credits completed. Final balance: %d pesewas",
		numConcurrentCredits, wallet.Balance)
}

// TestSequentialWalletCredits tests sequential credit operations as a baseline
func TestSequentialWalletCredits(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	// Create a test retailer
	retailerID := uuid.New()

	// Test sequential credits
	numCredits := 5
	creditAmount := int64(50000) // 500.00 GHS in pesewas

	for i := 0; i < numCredits; i++ {
		_, err := service.CreditRetailerWallet(
			ctx,
			retailerID,
			creditAmount,
			models.WalletTypeRetailerWinning,
			fmt.Sprintf("Sequential credit #%d", i),
			"",
			models.CreditSourceAdminDirect,
		)
		require.NoError(t, err, "Sequential credit %d failed", i)
	}

	// Verify final balance
	expectedFinalBalance := creditAmount * int64(numCredits)
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, wallet.Balance,
		"Final balance should equal sum of all sequential credits")

	t.Logf("SUCCESS: All %d sequential credits completed. Final balance: %d pesewas",
		numCredits, wallet.Balance)
}

// TestIdempotentWalletCredits tests that duplicate requests with same idempotency key don't double-credit
func TestIdempotentWalletCredits(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	// Create a test retailer
	retailerID := uuid.New()
	idempotencyKey := uuid.New().String()
	creditAmount := int64(100000)

	// First credit
	tx1, err := service.CreditRetailerWallet(
		ctx,
		retailerID,
		creditAmount,
		models.WalletTypeRetailerWinning,
		"Test idempotent credit",
		idempotencyKey,
		models.CreditSourceAdminDirect,
	)
	require.NoError(t, err)
	require.NotNil(t, tx1)

	// Duplicate credit with same idempotency key
	tx2, err := service.CreditRetailerWallet(
		ctx,
		retailerID,
		creditAmount,
		models.WalletTypeRetailerWinning,
		"Test idempotent credit",
		idempotencyKey,
		models.CreditSourceAdminDirect,
	)
	require.NoError(t, err)
	require.NotNil(t, tx2)

	// Should return the same transaction
	assert.Equal(t, tx1.ID, tx2.ID, "Duplicate request should return same transaction")

	// Verify final balance (should only be credited once)
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, creditAmount, wallet.Balance,
		"Balance should only be credited once despite duplicate request")

	// Verify transaction count (exclude audit transactions which have "audit_trace" metadata)
	allTransactions, err := transactionRepo.GetTransactionHistory(ctx, retailerID, models.WalletTypeRetailerWinning, 100, 0)
	require.NoError(t, err, "Failed to get transaction history")

	// Count only business transactions (excluding audit transactions)
	businessTransactions := 0
	auditTransactions := 0
	for _, tx := range allTransactions {
		// Check if this is an audit transaction by looking for audit_trace in metadata
		if tx.Metadata != nil {
			if auditTrace, exists := tx.Metadata["audit_trace"]; exists && auditTrace == true {
				auditTransactions++
				continue
			}
		}
		businessTransactions++
	}

	// Should have exactly 1 business transaction (the first request, second was idempotent)
	assert.Equal(t, 1, businessTransactions,
		"Should have exactly 1 business transaction (excluding audit)")
	t.Logf("Transaction breakdown: %d total (%d business, %d audit)",
		len(allTransactions), businessTransactions, auditTransactions)

	t.Logf("SUCCESS: Idempotency key prevented double-credit. Balance: %d pesewas", wallet.Balance)
}

// TestConcurrentCreditsWithStress tests with higher concurrency to stress-test the locking
func TestConcurrentCreditsWithStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	// Create a test retailer
	retailerID := uuid.New()

	// Stress test with many concurrent operations
	numConcurrentCredits := 100
	creditAmount := int64(10000) // 100.00 GHS in pesewas
	expectedFinalBalance := creditAmount * int64(numConcurrentCredits)

	var wg sync.WaitGroup
	errors := make(chan error, numConcurrentCredits)
	startTime := time.Now()

	// Launch concurrent credit operations
	for i := 0; i < numConcurrentCredits; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Stress test credit #%d", index),
				"",
				models.CreditSourceAdminDirect,
			)

			if err != nil {
				errors <- err
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)
	duration := time.Since(startTime)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Errorf("Credit operation failed: %v", err)
	}

	if errorCount > 0 {
		t.Fatalf("Stress test failed with %d errors", errorCount)
	}

	// Verify final balance
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, wallet.Balance,
		"Final balance should equal sum of all credits even under high concurrency")

	t.Logf("SUCCESS: Stress test completed %d concurrent credits in %v. Final balance: %d pesewas",
		numConcurrentCredits, duration, wallet.Balance)
	t.Logf("Average time per operation: %v", duration/time.Duration(numConcurrentCredits))
}

// TestConcurrentMixedOperations tests concurrent credits and debits
func TestConcurrentMixedOperations(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	// Create a test retailer and fund the wallet
	retailerID := uuid.New()
	initialBalance := int64(1000000) // 10,000 GHS

	// Initial credit to create wallet
	_, err := service.CreditRetailerWallet(
		ctx,
		retailerID,
		initialBalance,
		models.WalletTypeRetailerWinning,
		"Initial funding",
		"",
		models.CreditSourceAdminDirect,
	)
	require.NoError(t, err)

	// Mix of credits and debits
	numCredits := 5
	numDebits := 3
	creditAmount := int64(50000) // 500 GHS
	debitAmount := int64(30000)  // 300 GHS

	var wg sync.WaitGroup
	errors := make(chan error, numCredits+numDebits)

	// Launch concurrent credits
	for i := 0; i < numCredits; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Mixed test credit #%d", index),
				"",
				models.CreditSourceAdminDirect,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	// Launch concurrent debits
	for i := 0; i < numDebits; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.DebitRetailerWallet(
				ctx,
				retailerID,
				debitAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Mixed test debit #%d", index),
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Mixed operation failed: %v", err)
	}

	// Verify final balance
	expectedFinalBalance := initialBalance + (creditAmount * int64(numCredits)) - (debitAmount * int64(numDebits))
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, wallet.Balance,
		"Final balance should equal initial + credits - debits")

	t.Logf("SUCCESS: Mixed operations completed. Initial: %d, Credits: %d, Debits: %d, Final: %d pesewas",
		initialBalance, numCredits*int(creditAmount), numDebits*int(debitAmount), wallet.Balance)
}

// TestConcurrentSameTimestampOperations tests operations that happen at exact same microsecond
func TestConcurrentSameTimestampOperations(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	retailerID := uuid.New()

	// Fire off operations as close to simultaneously as possible
	numConcurrent := 20
	creditAmount := int64(10000)
	expectedFinalBalance := creditAmount * int64(numConcurrent)

	var wg sync.WaitGroup
	errors := make(chan error, numConcurrent)

	// Use a barrier to make all goroutines start at the same time
	startBarrier := make(chan struct{})

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Wait for barrier to be released
			<-startBarrier

			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Simultaneous credit #%d", index),
				"",
				models.CreditSourceAdminDirect,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	// Release all goroutines at once
	close(startBarrier)

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Simultaneous operation failed: %v", err)
	}

	// Verify final balance
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, wallet.Balance,
		"Final balance should equal sum even with simultaneous execution")

	t.Logf("SUCCESS: %d simultaneous operations completed. Final balance: %d pesewas",
		numConcurrent, wallet.Balance)
}

// TestConcurrentWithWalletCreation tests concurrent operations when wallet doesn't exist yet
func TestConcurrentWithWalletCreation(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	retailerID := uuid.New()

	// All operations try to credit to non-existent wallet
	numConcurrent := 10
	creditAmount := int64(50000)
	expectedFinalBalance := creditAmount * int64(numConcurrent)

	var wg sync.WaitGroup
	errors := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Creation race credit #%d", index),
				"",
				models.CreditSourceAdminDirect,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Wallet creation race operation failed: %v", err)
	}

	// Verify wallet was created and balance is correct
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, wallet.Balance,
		"Final balance should be correct even with concurrent wallet creation")

	t.Logf("SUCCESS: Concurrent wallet creation handled correctly. Final balance: %d pesewas",
		wallet.Balance)
}

// TestConcurrentLargeAmounts tests with very large transaction amounts
func TestConcurrentLargeAmounts(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	retailerID := uuid.New()

	// Very large amounts (e.g., 1 million GHS = 100 million pesewas)
	numConcurrent := 5
	creditAmount := int64(100000000) // 1,000,000 GHS in pesewas
	expectedFinalBalance := creditAmount * int64(numConcurrent)

	var wg sync.WaitGroup
	errors := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Large amount credit #%d", index),
				"",
				models.CreditSourceAdminDirect,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Large amount operation failed: %v", err)
	}

	// Verify final balance
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, wallet.Balance,
		"Final balance should handle large amounts correctly")

	t.Logf("SUCCESS: Large amount operations completed. Final balance: %d pesewas (%.2f GHS)",
		wallet.Balance, float64(wallet.Balance)/100.0)
}

// TestConcurrentVerySmallAmounts tests with very small transaction amounts
func TestConcurrentVerySmallAmounts(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	retailerID := uuid.New()

	// Very small amounts (1 pesewa = 0.01 GHS)
	numConcurrent := 100
	creditAmount := int64(1) // 1 pesewa
	expectedFinalBalance := creditAmount * int64(numConcurrent)

	var wg sync.WaitGroup
	errors := make(chan error, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Small amount credit #%d", index),
				"",
				models.CreditSourceAdminDirect,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Small amount operation failed: %v", err)
	}

	// Verify final balance
	wallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedFinalBalance, wallet.Balance,
		"Final balance should handle small amounts correctly")

	t.Logf("SUCCESS: Small amount operations completed. Final balance: %d pesewas", wallet.Balance)
}

// TestConcurrentBothWalletTypes tests concurrent operations on both stake and winning wallets
func TestConcurrentBothWalletTypes(t *testing.T) {
	_, db, dbx, cleanup := setupTestContainer(t)
	defer cleanup()

	ctx := context.Background()

	// Setup repositories and service
	redisClient := setupTestRedis()
	walletRepo := repositories.NewWalletRepository(db)
	extendedWalletRepo := repositories.NewExtendedWalletRepository(db)
	transactionRepo := repositories.NewWalletTransactionRepository(db, redisClient)
	adminTransactionRepo := repositories.NewAdminTransactionRepository(db, redisClient)
	idempotencyRepo := repositories.NewIdempotencyRepository(dbx)
	reservationRepo := repositories.NewReservationRepository(dbx)

	service := NewWalletService(
		db,
		redisClient,
		walletRepo,
		extendedWalletRepo,
		transactionRepo,
		adminTransactionRepo,
		idempotencyRepo,
		reservationRepo,
		nil, // reversalRepo
		nil, // commissionService
		nil, // eventPublisher
		nil, // agentClient
	)

	retailerID := uuid.New()

	numPerWallet := 10
	creditAmount := int64(50000)
	expectedWinningBalance := creditAmount * int64(numPerWallet)
	// Stake wallet includes 30% commission: grossAmount = baseAmount / 0.7
	// Using math.Ceil: Ceil(50000 / 0.7) = Ceil(71428.571...) = 71429 per transaction
	expectedStakeBalance := int64(71429) * int64(numPerWallet) // 714,290 total

	var wg sync.WaitGroup
	errors := make(chan error, numPerWallet*2)

	// Concurrent operations on winning wallet
	for i := 0; i < numPerWallet; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerWinning,
				fmt.Sprintf("Winning wallet credit #%d", index),
				"",
				models.CreditSourceAdminDirect,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	// Concurrent operations on stake wallet
	for i := 0; i < numPerWallet; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.CreditRetailerWallet(
				ctx,
				retailerID,
				creditAmount,
				models.WalletTypeRetailerStake,
				fmt.Sprintf("Stake wallet credit #%d", index),
				"",
				models.CreditSourceAdminDirect,
			)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Dual wallet operation failed: %v", err)
	}

	// Verify both wallet balances
	winningWallet, err := walletRepo.GetRetailerWinningWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedWinningBalance, winningWallet.Balance,
		"Winning wallet balance should be correct (no commission)")

	stakeWallet, err := walletRepo.GetRetailerStakeWallet(ctx, retailerID)
	require.NoError(t, err)
	assert.Equal(t, expectedStakeBalance, stakeWallet.Balance,
		"Stake wallet balance should include 30%% commission (base / 0.7)")

	t.Logf("SUCCESS: Both wallet types handled correctly. Winning: %d (expected: %d), Stake: %d (expected: %d) pesewas",
		winningWallet.Balance, expectedWinningBalance, stakeWallet.Balance, expectedStakeBalance)
}
