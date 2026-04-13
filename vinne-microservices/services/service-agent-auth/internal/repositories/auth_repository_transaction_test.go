package repositories

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// TestAuthRepository_TransactionAtomicity tests that failed login attempts and account locking are atomic
func TestAuthRepository_TransactionAtomicity(t *testing.T) {
	db, redis, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := NewAuthRepository(db, redis)

	// Create a test agent
	agentID := uuid.New()
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("testpass123"), bcrypt.DefaultCost)
	agent := &AuthUser{
		ID:           agentID,
		Code:         "1001",
		Email:        stringPtr("agent@test.com"),
		Phone:        stringPtr("+233123456789"),
		PasswordHash: string(passwordHash),
		IsActive:     true,
	}
	err := repo.CreateAgent(ctx, agent)
	require.NoError(t, err)

	t.Run("IncrementAndLockAtomic", func(t *testing.T) {
		// Test that increment and lock operations are atomic
		err := repo.WithTx(ctx, func(tx *sqlx.Tx) error {
			// Pass transaction context
			txCtx := context.WithValue(ctx, txKey{}, tx)

			// Increment failed attempts
			attempts, err := repo.IncrementAgentFailedLogin(txCtx, agentID)
			if err != nil {
				return err
			}
			assert.Equal(t, 1, attempts)

			// If we reach max attempts (let's say 3 for this test), lock the account
			if attempts >= 1 { // Using 1 for immediate test
				lockUntil := time.Now().Add(30 * time.Minute)
				err = repo.LockAgentAccount(txCtx, agentID, lockUntil)
				if err != nil {
					return err
				}
			}

			return nil
		})
		require.NoError(t, err)

		// Verify both operations succeeded
		agent, err := repo.GetAgentByID(ctx, agentID)
		require.NoError(t, err)
		assert.Equal(t, 1, agent.FailedLoginAttempts)
		assert.NotNil(t, agent.LockedUntil)
		assert.True(t, agent.LockedUntil.After(time.Now()))
	})
}

// TestAuthRepository_TransactionRollback tests that operations are rolled back on error
func TestAuthRepository_TransactionRollback(t *testing.T) {
	db, redis, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := NewAuthRepository(db, redis)

	// Create a test agent
	agentID := uuid.New()
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("testpass123"), bcrypt.DefaultCost)
	agent := &AuthUser{
		ID:           agentID,
		Code:         "1002",
		Email:        stringPtr("agent2@test.com"),
		Phone:        stringPtr("+233123456790"),
		PasswordHash: string(passwordHash),
		IsActive:     true,
	}
	err := repo.CreateAgent(ctx, agent)
	require.NoError(t, err)

	t.Run("RollbackOnError", func(t *testing.T) {
		// Attempt a transaction that will fail
		err := repo.WithTx(ctx, func(tx *sqlx.Tx) error {
			txCtx := context.WithValue(ctx, txKey{}, tx)

			// Increment failed attempts
			attempts, err := repo.IncrementAgentFailedLogin(txCtx, agentID)
			if err != nil {
				return err
			}
			assert.Equal(t, 1, attempts)

			// Force an error to trigger rollback
			return fmt.Errorf("simulated error")
		})

		// Transaction should have failed
		require.Error(t, err)
		assert.Contains(t, err.Error(), "simulated error")

		// Verify the increment was rolled back
		agent, err := repo.GetAgentByID(ctx, agentID)
		require.NoError(t, err)
		assert.Equal(t, 0, agent.FailedLoginAttempts) // Should still be 0
		assert.Nil(t, agent.LockedUntil)
	})
}

// TestAuthRepository_ConcurrentTransactions tests that concurrent login attempts are handled correctly
func TestAuthRepository_ConcurrentTransactions(t *testing.T) {
	db, redis, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := NewAuthRepository(db, redis)

	// Create a test agent
	agentID := uuid.New()
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("testpass123"), bcrypt.DefaultCost)
	agent := &AuthUser{
		ID:           agentID,
		Code:         "1003",
		Email:        stringPtr("agent3@test.com"),
		Phone:        stringPtr("+233123456791"),
		PasswordHash: string(passwordHash),
		IsActive:     true,
	}
	err := repo.CreateAgent(ctx, agent)
	require.NoError(t, err)

	t.Run("ConcurrentFailedLogins", func(t *testing.T) {
		const numGoroutines = 5
		const maxAttempts = 3

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		successCount := 0
		var mu sync.Mutex

		// Simulate concurrent failed login attempts
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()

				err := repo.WithTx(ctx, func(tx *sqlx.Tx) error {
					txCtx := context.WithValue(ctx, txKey{}, tx)

					// Get current state within transaction
					agent, err := repo.GetAgentByID(txCtx, agentID)
					if err != nil {
						return err
					}

					// Check if already locked
					if agent.LockedUntil != nil && agent.LockedUntil.After(time.Now()) {
						return fmt.Errorf("account already locked")
					}

					// Increment failed attempts
					attempts, err := repo.IncrementAgentFailedLogin(txCtx, agentID)
					if err != nil {
						return err
					}

					// Lock if max attempts reached
					if attempts >= maxAttempts {
						lockUntil := time.Now().Add(30 * time.Minute)
						err = repo.LockAgentAccount(txCtx, agentID, lockUntil)
						if err != nil {
							return err
						}
					}

					mu.Lock()
					successCount++
					mu.Unlock()

					return nil
				})

				// Some transactions might fail due to conflicts, which is expected
				if err != nil {
					t.Logf("Transaction failed (expected for some): %v", err)
				}
			}()
		}

		wg.Wait()

		// Verify final state
		agent, err := repo.GetAgentByID(ctx, agentID)
		require.NoError(t, err)

		// At least maxAttempts should have been recorded
		assert.GreaterOrEqual(t, agent.FailedLoginAttempts, maxAttempts)

		// Account should be locked after reaching max attempts
		if agent.FailedLoginAttempts >= maxAttempts {
			assert.NotNil(t, agent.LockedUntil)
			assert.True(t, agent.LockedUntil.After(time.Now()))
		}

		t.Logf("Final state: attempts=%d, locked=%v, successful_transactions=%d",
			agent.FailedLoginAttempts, agent.LockedUntil != nil, successCount)
	})
}

// TestAuthRepository_RetailerTransactionAtomicity tests retailer login transaction atomicity
func TestAuthRepository_RetailerTransactionAtomicity(t *testing.T) {
	db, redis, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := NewAuthRepository(db, redis)

	// Create a test retailer
	retailerID := uuid.New()
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte("testpass123"), bcrypt.DefaultCost)
	pinHash, _ := bcrypt.GenerateFromPassword([]byte("1234"), bcrypt.DefaultCost)

	// Insert retailer directly into database
	query := `
		INSERT INTO retailers_auth (id, retailer_code, email, phone, password_hash, pin_hash, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`
	_, err := db.ExecContext(ctx, query, retailerID, "RT1001", "retailer@test.com", "+233123456792",
		string(passwordHash), string(pinHash), true)
	require.NoError(t, err)

	t.Run("RetailerIncrementAndLockAtomic", func(t *testing.T) {
		// Test that increment and lock operations are atomic for retailers
		err := repo.WithTx(ctx, func(tx *sqlx.Tx) error {
			txCtx := context.WithValue(ctx, txKey{}, tx)

			// Increment failed attempts
			attempts, err := repo.IncrementRetailerFailedLogin(txCtx, retailerID)
			if err != nil {
				return err
			}
			assert.Equal(t, 1, attempts)

			// Lock the account
			if attempts >= 1 {
				lockUntil := time.Now().Add(30 * time.Minute)
				err = repo.LockRetailerAccount(txCtx, retailerID, lockUntil)
				if err != nil {
					return err
				}
			}

			return nil
		})
		require.NoError(t, err)

		// Verify both operations succeeded
		retailer, err := repo.GetRetailerByID(ctx, retailerID)
		require.NoError(t, err)
		assert.Equal(t, 1, retailer.FailedLoginAttempts)
		assert.NotNil(t, retailer.LockedUntil)
		assert.True(t, retailer.LockedUntil.After(time.Now()))
	})
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
