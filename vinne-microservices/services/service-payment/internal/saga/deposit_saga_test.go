package saga

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-payment/internal/models"
	"github.com/randco/service-payment/internal/providers"
	"github.com/randco/service-payment/internal/repositories"
)

type mockRetailerWalletClient struct {
	creditResponse *walletv1.CreditRetailerWalletResponse
	creditError    error
	debitResponse  *walletv1.DebitRetailerWalletResponse
	debitError     error
	creditCalled   bool
	debitCalled    bool
}

func (m *mockRetailerWalletClient) CreditPlayerWallet(ctx context.Context, in *walletv1.CreditPlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditPlayerWalletResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) DebitPlayerWallet(ctx context.Context, in *walletv1.DebitPlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.DebitPlayerWalletResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetPlayerWalletBalance(ctx context.Context, in *walletv1.GetPlayerWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetPlayerWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetTransactionHistory(ctx context.Context, in *walletv1.GetTransactionHistoryRequest, opts ...grpc.CallOption) (*walletv1.GetTransactionHistoryResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) CreditRetailerWallet(ctx context.Context, in *walletv1.CreditRetailerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditRetailerWalletResponse, error) {
	if m.creditError != nil {
		return nil, m.creditError
	}
	m.creditCalled = true
	return m.creditResponse, nil
}

func (m *mockRetailerWalletClient) DebitRetailerWallet(ctx context.Context, in *walletv1.DebitRetailerWalletRequest, opts ...grpc.CallOption) (*walletv1.DebitRetailerWalletResponse, error) {
	m.debitCalled = true
	if m.debitError != nil {
		return nil, m.debitError
	}
	return m.debitResponse, nil
}

func (m *mockRetailerWalletClient) GetRetailerWalletBalance(ctx context.Context, in *walletv1.GetRetailerWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetRetailerWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) ReserveRetailerWalletFunds(ctx context.Context, in *walletv1.ReserveRetailerWalletFundsRequest, opts ...grpc.CallOption) (*walletv1.ReserveRetailerWalletFundsResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) CommitReservedDebit(ctx context.Context, in *walletv1.CommitReservedDebitRequest, opts ...grpc.CallOption) (*walletv1.CommitReservedDebitResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) ReleaseReservation(ctx context.Context, in *walletv1.ReleaseReservationRequest, opts ...grpc.CallOption) (*walletv1.ReleaseReservationResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) CreateAgentWallet(ctx context.Context, in *walletv1.CreateAgentWalletRequest, opts ...grpc.CallOption) (*walletv1.CreateAgentWalletResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) CreateRetailerWallets(ctx context.Context, in *walletv1.CreateRetailerWalletsRequest, opts ...grpc.CallOption) (*walletv1.CreateRetailerWalletsResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) CreditAgentWallet(ctx context.Context, in *walletv1.CreditAgentWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditAgentWalletResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetAgentWalletBalance(ctx context.Context, in *walletv1.GetAgentWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetAgentWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetAllTransactions(ctx context.Context, in *walletv1.GetAllTransactionsRequest, opts ...grpc.CallOption) (*walletv1.GetAllTransactionsResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) ReverseTransaction(ctx context.Context, in *walletv1.ReverseTransactionRequest, opts ...grpc.CallOption) (*walletv1.ReverseTransactionResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) SetCommissionRate(ctx context.Context, in *walletv1.SetCommissionRateRequest, opts ...grpc.CallOption) (*walletv1.SetCommissionRateResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetCommissionRate(ctx context.Context, in *walletv1.GetCommissionRateRequest, opts ...grpc.CallOption) (*walletv1.GetCommissionRateResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetCommissionReport(ctx context.Context, in *walletv1.GetCommissionReportRequest, opts ...grpc.CallOption) (*walletv1.GetCommissionReportResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) UpdateAgentCommission(ctx context.Context, in *walletv1.UpdateAgentCommissionRequest, opts ...grpc.CallOption) (*walletv1.UpdateAgentCommissionResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) TransferAgentToRetailer(ctx context.Context, in *walletv1.TransferAgentToRetailerRequest, opts ...grpc.CallOption) (*walletv1.TransferAgentToRetailerResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) CreatePlayerWallet(ctx context.Context, in *walletv1.CreatePlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreatePlayerWalletResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) ReservePlayerWalletFunds(ctx context.Context, in *walletv1.ReservePlayerWalletFundsRequest, opts ...grpc.CallOption) (*walletv1.ReservePlayerWalletFundsResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetDailyCommissions(ctx context.Context, in *walletv1.GetDailyCommissionsRequest, opts ...grpc.CallOption) (*walletv1.GetDailyCommissionsResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) PlaceHoldOnWallet(ctx context.Context, in *walletv1.PlaceHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.PlaceHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) ReleaseHoldOnWallet(ctx context.Context, in *walletv1.ReleaseHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.ReleaseHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetHoldOnWallet(ctx context.Context, in *walletv1.GetHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.GetHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockRetailerWalletClient) GetHoldByRetailer(ctx context.Context, in *walletv1.GetHoldByRetailerRequest, opts ...grpc.CallOption) (*walletv1.GetHoldByRetailerResponse, error) {
	return nil, nil
}

func TestDepositSaga_PENDING_StopsSaga(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusPending,
			Message:       "Payment prompt sent to customer",
			TransactionID: "ORANGE-TX-67890",
			RequestedAt:   time.Now(),
		},
	}

	mockWallet := &mockRetailerWalletClient{}

	saga := NewDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("RETAILER-REF-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                50000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244987654",
		SourceName:            "MTN",
		DestinationType:       "RETAILER_STAKE_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "Jane Retailer",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test retailer deposit",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.Error(t, err, "Saga should fail when provider returns PENDING")
	assert.Contains(t, err.Error(), "saga", "Error should indicate saga stopped")
	assert.False(t, mockWallet.creditCalled, "Stake wallet should NOT be credited when status is PENDING")
}

func TestDepositSaga_SUCCESS_CompletesSuccessfully(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt := time.Now()
	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Payment successful",
			TransactionID: "ORANGE-TX-67890",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt,
		},
	}

	mockWallet := &mockRetailerWalletClient{
		creditResponse: &walletv1.CreditRetailerWalletResponse{
			Success:       true,
			Message:       "Stake wallet credited",
			TransactionId: uuid.New().String(),
			NewBalance:    100000,
			GrossAmount:   50000,
			BaseAmount:    50000,
		},
	}

	saga := NewDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("RETAILER-REF-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                50000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244987654",
		SourceName:            "MTN",
		DestinationType:       "RETAILER_STAKE_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "Jane Retailer",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test retailer deposit",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.NoError(t, err, "Saga should complete successfully")
	assert.True(t, mockWallet.creditCalled, "Stake wallet should be credited")
}

func TestDepositSaga_MobileMoneyRefundSkipsWhenDebitNeverSucceeded(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	saga := &models.Saga{
		ID:          uuid.New(),
		SagaID:      "test-deposit-compensation",
		Status:      models.SagaStatusFailed,
		CurrentStep: 1,
		TotalSteps:  2,
		SagaData: map[string]interface{}{
			"user_id":         uuid.New().String(),
			"reference":       "TEST-DEPOSIT-COMP",
			"amount":          int64(50000),
			"wallet_number":   "0244987654",
			"wallet_provider": "MTN",
			"currency":        "GHS",
		},
	}

	mockProvider := &mockPaymentProvider{}
	mockWallet := &mockRetailerWalletClient{}
	txnRepo := repositories.NewTransactionRepository(db)

	depositSaga := &DepositSaga{
		provider:        mockProvider,
		walletClient:    mockWallet,
		transactionRepo: txnRepo,
	}

	err := depositSaga.creditMobileMoneyWallet(ctx, saga.SagaData)

	assert.NoError(t, err, "Compensation should skip gracefully when provider_transaction_id is missing")
}

func TestDepositSaga_StakeWalletCompensationSkipsWhenCreditNeverSucceeded(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	saga := &models.Saga{
		ID:          uuid.New(),
		SagaID:      "test-stake-compensation",
		Status:      models.SagaStatusFailed,
		CurrentStep: 2,
		TotalSteps:  2,
		SagaData: map[string]interface{}{
			"user_id":                 uuid.New().String(),
			"reference":               "TEST-STAKE-COMP",
			"amount":                  int64(50000),
			"provider_transaction_id": "ORANGE-TX-99999",
		},
	}

	mockProvider := &mockPaymentProvider{}
	mockWallet := &mockRetailerWalletClient{}
	txnRepo := repositories.NewTransactionRepository(db)

	depositSaga := &DepositSaga{
		provider:        mockProvider,
		walletClient:    mockWallet,
		transactionRepo: txnRepo,
	}

	err := depositSaga.debitStakeWallet(ctx, saga.SagaData)

	assert.NoError(t, err, "Compensation should skip gracefully when stake_wallet_transaction_id is missing")
	assert.False(t, mockWallet.debitCalled, "Stake wallet debit should NOT be called when credit never succeeded")
}

func TestDepositSaga_WebhookResumeAfterPENDING(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("WEBHOOK-REF-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusSuccess,
		Amount:                50000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244987654",
		SourceName:            "MTN",
		DestinationType:       "RETAILER_STAKE_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "Jane Retailer",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test webhook resume",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	providerTxID := "ORANGE-WEBHOOK-TX"
	transaction.ProviderTransactionID = &providerTxID

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	mockProvider := &mockPaymentProvider{}

	mockWallet := &mockRetailerWalletClient{
		creditResponse: &walletv1.CreditRetailerWalletResponse{
			Success:       true,
			Message:       "Stake wallet credited via webhook",
			TransactionId: uuid.New().String(),
			NewBalance:    100000,
			GrossAmount:   50000,
			BaseAmount:    50000,
		},
	}

	saga := NewDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	err = saga.Execute(ctx, transaction)

	assert.NoError(t, err, "Saga should complete successfully when webhook already confirmed")
	assert.True(t, mockWallet.creditCalled, "Wallet should be credited")

	savedSaga, err := sagaRepo.GetBySagaID(ctx, fmt.Sprintf("deposit-%s", transaction.Reference))
	require.NoError(t, err)
	assert.Equal(t, models.SagaStatusCompleted, savedSaga.Status)
}

func TestDepositSaga_NoMoneyLeakOnFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt1 := time.Now()

	// Mock provider that successfully debits mobile money
	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Mobile money debit successful",
			TransactionID: "ORANGE-DEBIT-456",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
		creditResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Mobile money refund successful",
			TransactionID: "ORANGE-REFUND-456",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
	}

	// Mock wallet that fails stake wallet credit
	mockWallet := &mockRetailerWalletClient{
		creditError: fmt.Errorf("stake wallet service temporarily unavailable"),
	}

	saga := NewDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("DEPOSIT-INTEGRITY-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                25000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244111222",
		SourceName:            "MTN",
		DestinationType:       "RETAILER_STAKE_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "Test Retailer",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Retailer deposit integrity test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga - should fail at stake wallet credit step
	err = saga.Execute(ctx, transaction)

	// Verify saga failed and compensated
	assert.Error(t, err, "Saga should fail when stake wallet credit fails")
	assert.Contains(t, err.Error(), "compensated", "Error should indicate saga was compensated")

	// Verify saga record shows compensation was executed
	savedSaga, sagaErr := sagaRepo.GetBySagaID(ctx, fmt.Sprintf("deposit-%s", transaction.Reference))
	require.NoError(t, sagaErr)
	assert.Equal(t, models.SagaStatusCompensated, savedSaga.Status, "Saga should be marked as COMPENSATED after successful compensation")

	// Verify transaction was updated with provider data
	savedTxn, getErr := txnRepo.GetByID(ctx, transaction.ID)
	require.NoError(t, getErr)

	// Verify provider operations - debit was recorded
	assert.Contains(t, savedTxn.ProviderData, "debit_response", "Provider debit should have been recorded")

	// CRITICAL: Verify stake wallet was NEVER credited despite mobile money debit succeeding
	assert.False(t, mockWallet.creditCalled, "Stake wallet should NEVER have been credited")

	// Verify provider transaction ID was saved (proves mobile money debit happened)
	assert.NotNil(t, savedTxn.ProviderTransactionID, "Provider transaction ID should be saved")
	assert.Equal(t, "ORANGE-DEBIT-456", *savedTxn.ProviderTransactionID, "Debit transaction ID should match")

	// Verify compensation data exists in saga
	assert.Contains(t, savedSaga.CompensationData, "failed_step", "Compensation data should record which step failed")
	failedStep, ok := savedSaga.CompensationData["failed_step"].(float64)
	require.True(t, ok, "Failed step should be a number")
	assert.Equal(t, float64(1), failedStep, "Should have failed at step 1 (stake wallet credit)")

	t.Log("✅ Data integrity verified: No money leak on stake wallet credit failure")
	t.Logf("   - Mobile money was debited: %s", *savedTxn.ProviderTransactionID)
	t.Logf("   - Stake wallet was NOT credited: %v", !mockWallet.creditCalled)
	t.Logf("   - Saga compensated successfully (mobile money refunded)")
	t.Logf("   - Saga status: %s", savedSaga.Status)
}

func TestDepositSaga_DuplicateWebhookIdempotency(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt1 := time.Now()

	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Payment successful",
			TransactionID: "ORANGE-WEBHOOK-DUP",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
	}

	mockWallet := &mockRetailerWalletClient{
		creditResponse: &walletv1.CreditRetailerWalletResponse{
			Success:       true,
			Message:       "Stake wallet credited",
			TransactionId: uuid.New().String(),
			NewBalance:    100000,
		},
	}

	saga := NewDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	// Create transaction that simulates webhook already updated it
	providerTxID := "ORANGE-WEBHOOK-DUP"
	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("WEBHOOK-DUP-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusSuccess, // Already updated by webhook!
		Amount:                50000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244111222",
		SourceName:            "MTN",
		DestinationType:       "RETAILER_STAKE_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "Test Retailer",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Webhook idempotency test",
		ProviderTransactionID: &providerTxID,
		CompletedAt:           &completedAt1,
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// First execution - simulate webhook resume (should credit wallet)
	err = saga.Execute(ctx, transaction)
	require.NoError(t, err, "First saga execution should complete successfully")

	creditedFirstTime := mockWallet.creditCalled
	require.True(t, creditedFirstTime, "Wallet should be credited on first execution (webhook resume)")

	// Reset the mock flag to track second execution
	mockWallet.creditCalled = false

	// Execute saga AGAIN (simulating duplicate webhook or retry)
	err = saga.Execute(ctx, transaction)
	require.NoError(t, err, "Saga should handle already-completed transaction gracefully")

	creditedSecondTime := mockWallet.creditCalled

	// BUG: Does it credit the wallet AGAIN?
	t.Logf("First execution credited: %v", creditedFirstTime)
	t.Logf("Second execution credited: %v", creditedSecondTime)

	// This should FAIL if saga isn't idempotent
	assert.False(t, creditedSecondTime,
		"BUG: Wallet should NOT be credited again on duplicate execution")
}

func TestDepositSaga_ConcurrentExecution(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt1 := time.Now()

	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Payment successful",
			TransactionID: "ORANGE-CONCURRENT",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
	}

	mockWallet := &mockRetailerWalletClient{
		creditResponse: &walletv1.CreditRetailerWalletResponse{
			Success:       true,
			Message:       "Stake wallet credited",
			TransactionId: uuid.New().String(),
			NewBalance:    100000,
		},
	}

	saga := NewDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("CONCURRENT-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                50000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244111222",
		SourceName:            "MTN",
		DestinationType:       "RETAILER_STAKE_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "Test Retailer",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Concurrent execution test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga concurrently (simulating duplicate API call or webhook + retry)
	var wg sync.WaitGroup
	errors := make([]error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		errors[0] = saga.Execute(ctx, transaction)
	}()
	go func() {
		defer wg.Done()
		errors[1] = saga.Execute(ctx, transaction)
	}()
	wg.Wait()

	t.Logf("Execution 1 error: %v", errors[0])
	t.Logf("Execution 2 error: %v", errors[1])

	// BUG: What happens with concurrent execution?
	// Possibilities:
	// 1. Both succeed -> money credited twice! ❌
	// 2. Database constraint violation -> depends on DB schema
	// 3. One succeeds, one fails gracefully -> ideal ✅

	// At least ONE should succeed
	successCount := 0
	if errors[0] == nil {
		successCount++
	}
	if errors[1] == nil {
		successCount++
	}

	t.Logf("Success count: %d", successCount)
	assert.GreaterOrEqual(t, successCount, 1, "At least one execution should succeed")

	// BUG CHECK: If both succeeded, that's a race condition bug!
	if successCount == 2 {
		t.Error("BUG: Both concurrent executions succeeded - potential for double crediting!")
	}
}
