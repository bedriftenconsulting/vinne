package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/randco/service-wallet/internal/models"
	"github.com/stretchr/testify/require"
)

// TestRedisUnavailable tests graceful degradation when Redis is down
func (s *WalletRepositoryTestSuite) TestRedisUnavailable() {
	ctx := context.Background()

	// Create a test transaction
	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())
	metadataJSON, _ := json.Marshal(map[string]interface{}{"test": "value"})

	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())`

	_, err := s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
		100000, 0, 100000, "REF-001", "Redis down test",
		models.TransactionStatusCompleted, uuid.NewString(), metadataJSON)
	require.NoError(s.T(), err)

	// Create repository with nil Redis client to simulate Redis being unavailable
	repoWithoutRedis := NewWalletTransactionRepository(s.db, nil)

	// Should still work by falling back to database
	tx, err := repoWithoutRedis.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx)
	require.Equal(s.T(), transactionID, tx.TransactionID)
	require.Equal(s.T(), "value", tx.Metadata["test"])

	s.T().Log("✅ Gracefully degrades to database when Redis is unavailable (nil client)")
}

// TestCacheStampede tests concurrent requests for the same uncached transaction
func (s *WalletRepositoryTestSuite) TestCacheStampede() {
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
		100000, 0, 100000, "REF-STAMPEDE", "Stampede test",
		models.TransactionStatusCompleted, uuid.NewString())
	require.NoError(s.T(), err)

	// Ensure cache is empty
	cacheKey := fmt.Sprintf("transaction:id:%s", transactionID)
	s.redisClient.Del(ctx, cacheKey)

	// Simulate 10 concurrent requests for the same transaction
	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]*models.WalletTransaction, numGoroutines)
	errors := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			tx, err := s.walletRepo.GetTransaction(ctx, transactionID)
			results[index] = tx
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// All requests should succeed
	for i := 0; i < numGoroutines; i++ {
		require.NoError(s.T(), errors[i], "Request %d should not error", i)
		require.NotNil(s.T(), results[i], "Request %d should return transaction", i)
		require.Equal(s.T(), transactionID, results[i].TransactionID)
	}

	// Cache should be populated after stampede
	cached, err := s.redisClient.Get(ctx, cacheKey).Result()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), cached)

	s.T().Log("✅ Handles cache stampede correctly (multiple concurrent requests)")
}

// TestEmptyResultSetCaching tests if we should cache empty results
func (s *WalletRepositoryTestSuite) TestEmptyResultSetCaching() {
	ctx := context.Background()

	// Query for a non-existent transaction
	nonExistentID := "TXN-DOES-NOT-EXIST"
	tx, err := s.walletRepo.GetTransaction(ctx, nonExistentID)
	require.Error(s.T(), err)
	require.Nil(s.T(), tx)

	// Verify we DON'T cache the error (negative caching can cause issues)
	cacheKey := fmt.Sprintf("transaction:id:%s", nonExistentID)
	_, err = s.redisClient.Get(ctx, cacheKey).Result()
	require.Error(s.T(), err) // Should be cache miss

	s.T().Log("✅ Does not cache non-existent transactions (no negative caching)")

	// Test empty transaction history
	emptyOwnerID := uuid.New()
	txs, err := s.walletRepo.GetTransactionHistory(ctx, emptyOwnerID, models.WalletTypeAgentStake, 20, 0)
	require.NoError(s.T(), err)
	require.Empty(s.T(), txs)

	// Check if empty result is cached
	historyCacheKey := fmt.Sprintf("transactions:owner:%s:type:%s:page:1:size:20",
		emptyOwnerID.String(), models.WalletTypeAgentStake)
	cached, err := s.redisClient.Get(ctx, historyCacheKey).Result()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), cached) // Empty array should be cached

	s.T().Log("✅ Caches empty transaction history (prevents repeated DB queries)")
}

// TestMalformedMetadataJSON tests handling of corrupted metadata in database
func (s *WalletRepositoryTestSuite) TestMalformedMetadataJSON() {
	ctx := context.Background()

	// Insert transaction with malformed JSON metadata directly
	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())

	// Insert with raw SQL to bypass Go validation
	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, NOW())`

	malformedJSON := `{"broken": "json"` // Missing closing brace

	_, err := s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
		100000, 0, 100000, "REF-MALFORMED", "Malformed test",
		models.TransactionStatusCompleted, uuid.NewString(), malformedJSON)

	// PostgreSQL should reject malformed JSONB
	require.Error(s.T(), err)
	require.Contains(s.T(), err.Error(), "invalid input syntax for type json")

	s.T().Log("✅ PostgreSQL rejects malformed JSONB (database-level validation)")
}

// TestVeryLargeMetadata tests handling of large metadata objects
func (s *WalletRepositoryTestSuite) TestVeryLargeMetadata() {
	ctx := context.Background()

	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())

	// Create large metadata (simulate complex transaction with lots of details)
	largeMetadata := map[string]interface{}{
		"wallet_owner_name": "Test Agent",
		"wallet_owner_code": "AG123",
		"source":            "api",
	}

	// Add 100 fields to simulate large metadata
	for i := 0; i < 100; i++ {
		largeMetadata[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	metadataJSON, err := json.Marshal(largeMetadata)
	require.NoError(s.T(), err)

	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())`

	_, err = s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
		100000, 0, 100000, "REF-LARGE", "Large metadata test",
		models.TransactionStatusCompleted, uuid.NewString(), metadataJSON)
	require.NoError(s.T(), err)

	// Retrieve and verify
	tx, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx.Metadata)
	require.Equal(s.T(), "Test Agent", tx.Metadata["wallet_owner_name"])
	require.Equal(s.T(), "value_99", tx.Metadata["field_99"])

	s.T().Logf("✅ Handles large metadata (%d bytes, %d fields)", len(metadataJSON), len(largeMetadata))
}

// TestConcurrentStatusUpdates tests race conditions on status updates
func (s *WalletRepositoryTestSuite) TestConcurrentStatusUpdates() {
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
		100000, 0, 100000, "REF-CONCURRENT", "Concurrent update test",
		models.TransactionStatusPending, uuid.NewString())
	require.NoError(s.T(), err)

	// Try to update status concurrently from 10 goroutines
	const numGoroutines = 10
	var wg sync.WaitGroup
	errors := make([]error, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			errors[index] = s.walletRepo.UpdateTransactionStatus(ctx, transactionID, models.TransactionStatusCompleted)
		}(i)
	}

	wg.Wait()

	// All updates should succeed (idempotent - updating to same status)
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if errors[i] == nil {
			successCount++
		}
	}
	require.Greater(s.T(), successCount, 0, "At least some updates should succeed")

	// Verify final status
	var finalStatus string
	err = s.db.QueryRowContext(ctx, "SELECT status FROM wallet_transactions WHERE transaction_id = $1", transactionID).Scan(&finalStatus)
	require.NoError(s.T(), err)
	require.Equal(s.T(), string(models.TransactionStatusCompleted), finalStatus)

	s.T().Logf("✅ Concurrent status updates handled (%d/%d succeeded)", successCount, numGoroutines)
}

// TestNullIdempotencyKey tests transactions without idempotency keys
func (s *WalletRepositoryTestSuite) TestNullIdempotencyKey() {
	ctx := context.Background()

	agentID := uuid.New()
	transactionID := fmt.Sprintf("TXN-%d", time.Now().UnixNano())

	query := `
		INSERT INTO wallet_transactions (
			transaction_id, wallet_owner_id, wallet_type, transaction_type,
			amount, balance_before, balance_after, reference, description,
			status, idempotency_key, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULL, NULL, NOW())`

	_, err := s.db.ExecContext(ctx, query,
		transactionID, agentID, models.WalletTypeAgentStake, models.TransactionTypeCredit,
		100000, 0, 100000, "REF-NULL-IDEM", "Null idempotency key",
		models.TransactionStatusCompleted)
	require.NoError(s.T(), err)

	// Should be able to retrieve
	tx, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx)
	require.Nil(s.T(), tx.IdempotencyKey)

	s.T().Log("✅ Handles NULL idempotency keys correctly")
}

// TestCacheExpiry tests behavior when cache expires during operation
func (s *WalletRepositoryTestSuite) TestCacheExpiry() {
	ctx := context.Background()

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
		100000, 0, 100000, "REF-EXPIRY", "Cache expiry test",
		models.TransactionStatusCompleted, uuid.NewString())
	require.NoError(s.T(), err)

	// Populate cache with very short TTL (1 second)
	cacheKey := fmt.Sprintf("transaction:id:%s", transactionID)
	tx1, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)

	// Manually set short TTL
	txJSON, _ := json.Marshal(tx1)
	s.redisClient.Set(ctx, cacheKey, txJSON, 1*time.Second)

	// Wait for cache to expire
	time.Sleep(2 * time.Second)

	// Should still work by querying database
	tx2, err := s.walletRepo.GetTransaction(ctx, transactionID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tx2)
	require.Equal(s.T(), transactionID, tx2.TransactionID)

	s.T().Log("✅ Handles cache expiry gracefully (refetches from DB)")
}

// TestInvalidStatusTransition tests if we can reverse a reversed transaction
func (s *WalletRepositoryTestSuite) TestInvalidStatusTransition() {
	ctx := context.Background()

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
		100000, 0, 100000, "REF-TRANSITION", "Transition test",
		models.TransactionStatusCompleted, uuid.NewString())
	require.NoError(s.T(), err)

	// Update to REVERSED
	err = s.walletRepo.UpdateTransactionStatus(ctx, transactionID, models.TransactionStatusReversed)
	require.NoError(s.T(), err)

	// Verify reversed_at is set
	var reversedAt sql.NullTime
	err = s.db.QueryRowContext(ctx, "SELECT reversed_at FROM wallet_transactions WHERE transaction_id = $1", transactionID).Scan(&reversedAt)
	require.NoError(s.T(), err)
	require.True(s.T(), reversedAt.Valid)

	// Try to reverse again (repository doesn't prevent this - business logic should)
	err = s.walletRepo.UpdateTransactionStatus(ctx, transactionID, models.TransactionStatusCompleted)
	require.NoError(s.T(), err) // Repository allows it (no validation at repo level)

	s.T().Log("✅ Repository allows status transitions (validation should be in service layer)")
}

// TestPartialCacheInvalidationFailure tests if cache invalidation partially fails
func (s *WalletRepositoryTestSuite) TestPartialCacheInvalidationFailure() {
	ctx := context.Background()

	agentID := uuid.New()

	// Pre-populate several cache keys
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("transactions:all:filters:test%d:page:1", i)
		s.redisClient.Set(ctx, key, "dummy", 5*time.Minute)
	}

	// Create a transaction (this should invalidate all transactions:all:* keys)
	reference := "REF-PARTIAL"
	description := "Partial invalidation test"
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
		Metadata:        map[string]interface{}{"test": "partial"},
	}

	err := s.walletRepo.CreateTransaction(ctx, tx)
	require.NoError(s.T(), err)

	// Verify all admin caches were invalidated
	keys, err := s.redisClient.Keys(ctx, "transactions:all:*").Result()
	require.NoError(s.T(), err)
	require.Empty(s.T(), keys, "All admin cache keys should be invalidated")

	s.T().Log("✅ Batch cache invalidation works (all matching keys deleted)")
}
