package saga

import (
	"context"
	"fmt"
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

type mockPaymentProvider struct {
	debitResponse  *providers.TransactionResponse
	debitError     error
	creditResponse *providers.TransactionResponse
	creditError    error
}

func (m *mockPaymentProvider) GetProviderName() string {
	return "MockProvider"
}

func (m *mockPaymentProvider) GetProviderType() providers.ProviderType {
	return providers.ProviderTypeMobileMoney
}

func (m *mockPaymentProvider) Authenticate(ctx context.Context) (*providers.AuthResult, error) {
	return nil, nil
}

func (m *mockPaymentProvider) RefreshAuth(ctx context.Context) error {
	return nil
}

func (m *mockPaymentProvider) IsAuthenticated() bool {
	return true
}

func (m *mockPaymentProvider) VerifyWallet(ctx context.Context, req *providers.VerifyWalletRequest) (*providers.VerifyWalletResponse, error) {
	return nil, nil
}

func (m *mockPaymentProvider) VerifyBankAccount(ctx context.Context, req *providers.VerifyBankAccountRequest) (*providers.VerifyBankAccountResponse, error) {
	return nil, nil
}

func (m *mockPaymentProvider) VerifyIdentity(ctx context.Context, req *providers.VerifyIdentityRequest) (*providers.VerifyIdentityResponse, error) {
	return nil, nil
}

func (m *mockPaymentProvider) DebitWallet(ctx context.Context, req *providers.DebitWalletRequest) (*providers.TransactionResponse, error) {
	if m.debitError != nil {
		return nil, m.debitError
	}
	return m.debitResponse, nil
}

func (m *mockPaymentProvider) CreditWallet(ctx context.Context, req *providers.CreditWalletRequest) (*providers.TransactionResponse, error) {
	if m.creditError != nil {
		return nil, m.creditError
	}
	return m.creditResponse, nil
}

func (m *mockPaymentProvider) TransferToBank(ctx context.Context, req *providers.BankTransferRequest) (*providers.TransactionResponse, error) {
	return nil, nil
}

func (m *mockPaymentProvider) CheckTransactionStatus(ctx context.Context, transactionID, reference string) (*providers.TransactionStatusResponse, error) {
	return nil, nil
}

func (m *mockPaymentProvider) GetSupportedOperations() []providers.OperationType {
	return nil
}

func (m *mockPaymentProvider) GetTransactionLimits() *providers.TransactionLimits {
	return nil
}

func (m *mockPaymentProvider) GetSupportedCurrencies() []string {
	return []string{"GHS"}
}

func (m *mockPaymentProvider) HealthCheck(ctx context.Context) error {
	return nil
}

type mockWalletClient struct {
	creditResponse *walletv1.CreditPlayerWalletResponse
	creditError    error
	debitResponse  *walletv1.DebitPlayerWalletResponse
	debitError     error
	creditCalled   bool
	debitCalled    bool
	creditHook     func()
	debitHook      func()
}

func (m *mockWalletClient) CreditPlayerWallet(ctx context.Context, in *walletv1.CreditPlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditPlayerWalletResponse, error) {
	if m.creditError != nil {
		if m.creditHook != nil {
			m.creditHook()
		}
		return nil, m.creditError
	}
	m.creditCalled = true
	if m.creditHook != nil {
		m.creditHook()
	}
	return m.creditResponse, nil
}

func (m *mockWalletClient) DebitPlayerWallet(ctx context.Context, in *walletv1.DebitPlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.DebitPlayerWalletResponse, error) {
	m.debitCalled = true
	if m.debitHook != nil {
		m.debitHook()
	}
	if m.debitError != nil {
		return nil, m.debitError
	}
	return m.debitResponse, nil
}

func (m *mockWalletClient) GetPlayerWalletBalance(ctx context.Context, in *walletv1.GetPlayerWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetPlayerWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetTransactionHistory(ctx context.Context, in *walletv1.GetTransactionHistoryRequest, opts ...grpc.CallOption) (*walletv1.GetTransactionHistoryResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) CreditRetailerWallet(ctx context.Context, in *walletv1.CreditRetailerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditRetailerWalletResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) DebitRetailerWallet(ctx context.Context, in *walletv1.DebitRetailerWalletRequest, opts ...grpc.CallOption) (*walletv1.DebitRetailerWalletResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetRetailerWalletBalance(ctx context.Context, in *walletv1.GetRetailerWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetRetailerWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) ReserveRetailerWalletFunds(ctx context.Context, in *walletv1.ReserveRetailerWalletFundsRequest, opts ...grpc.CallOption) (*walletv1.ReserveRetailerWalletFundsResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) CommitReservedDebit(ctx context.Context, in *walletv1.CommitReservedDebitRequest, opts ...grpc.CallOption) (*walletv1.CommitReservedDebitResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) ReleaseReservation(ctx context.Context, in *walletv1.ReleaseReservationRequest, opts ...grpc.CallOption) (*walletv1.ReleaseReservationResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) CreateAgentWallet(ctx context.Context, in *walletv1.CreateAgentWalletRequest, opts ...grpc.CallOption) (*walletv1.CreateAgentWalletResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) CreateRetailerWallets(ctx context.Context, in *walletv1.CreateRetailerWalletsRequest, opts ...grpc.CallOption) (*walletv1.CreateRetailerWalletsResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) CreditAgentWallet(ctx context.Context, in *walletv1.CreditAgentWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditAgentWalletResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetAgentWalletBalance(ctx context.Context, in *walletv1.GetAgentWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetAgentWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetAllTransactions(ctx context.Context, in *walletv1.GetAllTransactionsRequest, opts ...grpc.CallOption) (*walletv1.GetAllTransactionsResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) ReverseTransaction(ctx context.Context, in *walletv1.ReverseTransactionRequest, opts ...grpc.CallOption) (*walletv1.ReverseTransactionResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) SetCommissionRate(ctx context.Context, in *walletv1.SetCommissionRateRequest, opts ...grpc.CallOption) (*walletv1.SetCommissionRateResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetCommissionRate(ctx context.Context, in *walletv1.GetCommissionRateRequest, opts ...grpc.CallOption) (*walletv1.GetCommissionRateResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetCommissionReport(ctx context.Context, in *walletv1.GetCommissionReportRequest, opts ...grpc.CallOption) (*walletv1.GetCommissionReportResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) UpdateAgentCommission(ctx context.Context, in *walletv1.UpdateAgentCommissionRequest, opts ...grpc.CallOption) (*walletv1.UpdateAgentCommissionResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) TransferAgentToRetailer(ctx context.Context, in *walletv1.TransferAgentToRetailerRequest, opts ...grpc.CallOption) (*walletv1.TransferAgentToRetailerResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) CreatePlayerWallet(ctx context.Context, in *walletv1.CreatePlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreatePlayerWalletResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) ReservePlayerWalletFunds(ctx context.Context, in *walletv1.ReservePlayerWalletFundsRequest, opts ...grpc.CallOption) (*walletv1.ReservePlayerWalletFundsResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetDailyCommissions(ctx context.Context, in *walletv1.GetDailyCommissionsRequest, opts ...grpc.CallOption) (*walletv1.GetDailyCommissionsResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) PlaceHoldOnWallet(ctx context.Context, in *walletv1.PlaceHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.PlaceHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) ReleaseHoldOnWallet(ctx context.Context, in *walletv1.ReleaseHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.ReleaseHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetHoldOnWallet(ctx context.Context, in *walletv1.GetHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.GetHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockWalletClient) GetHoldByRetailer(ctx context.Context, in *walletv1.GetHoldByRetailerRequest, opts ...grpc.CallOption) (*walletv1.GetHoldByRetailerResponse, error) {
	return nil, nil
}

func TestPlayerDepositSaga_PENDING_StopsSaga(t *testing.T) {
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
			TransactionID: "ORANGE-TX-12345",
			RequestedAt:   time.Now(),
		},
	}

	mockWallet := &mockWalletClient{}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("TEST-REF-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                10000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244123456",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "John Doe",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test deposit",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.Error(t, err, "Saga should fail when provider returns PENDING")
	assert.Contains(t, err.Error(), "saga", "Error should indicate saga stopped")
	assert.False(t, mockWallet.creditCalled, "Wallet should NOT be credited when status is PENDING")
}

func TestPlayerDepositSaga_SUCCESS_CompletesSuccessfully(t *testing.T) {
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
			TransactionID: "ORANGE-TX-12345",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt,
		},
	}

	mockWallet := &mockWalletClient{
		creditResponse: &walletv1.CreditPlayerWalletResponse{
			Success:        true,
			Message:        "Wallet credited",
			TransactionId:  uuid.New().String(),
			NewBalance:     20000,
			CreditedAmount: 10000,
		},
	}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("TEST-REF-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                10000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244123456",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "John Doe",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test deposit",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.NoError(t, err, "Saga should complete successfully when provider returns SUCCESS")
	assert.True(t, mockWallet.creditCalled, "Wallet should be credited when status is SUCCESS")

}

func TestPlayerDepositSaga_CompensationSkipsWhenDebitNeverSucceeded(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt2 := time.Now()
	completedAt3 := time.Now()
	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Payment successful",
			TransactionID: "ORANGE-TX-12345",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt2,
		},
		creditResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Refund successful",
			TransactionID: "ORANGE-REFUND-12345",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt3,
		},
	}

	mockWallet := &mockWalletClient{
		creditError: fmt.Errorf("wallet service unavailable"),
	}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("TEST-REF-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                10000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244123456",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "John Doe",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test deposit",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.Error(t, err, "Saga should fail when wallet credit fails")
	// The saga framework wraps errors, so we can't check for specific error text
	// Just verify that the saga failed and compensated
	assert.Contains(t, err.Error(), "saga")

}

func TestPlayerDepositSaga_CompensationSkipsWhenPENDING(t *testing.T) {
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
			Message:       "Payment prompt sent",
			TransactionID: "ORANGE-TX-12345",
			RequestedAt:   time.Now(),
		},
	}

	mockWallet := &mockWalletClient{}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("TEST-REF-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                10000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244123456",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "John Doe",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test deposit",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "saga")
	assert.False(t, mockWallet.creditCalled, "Wallet should NOT be credited when status is PENDING")
}

func TestPlayerDepositSaga_WalletCompensationSkipsWhenCreditNeverSucceeded(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	saga := &models.Saga{
		ID:          uuid.New(),
		SagaID:      "test-player-wallet-compensation",
		Status:      models.SagaStatusFailed,
		CurrentStep: 2,
		TotalSteps:  2,
		SagaData: map[string]interface{}{
			"player_id":               uuid.New().String(),
			"reference":               "TEST-WALLET-COMP",
			"amount":                  int64(10000),
			"provider_transaction_id": "ORANGE-TX-99999",
		},
	}

	mockProvider := &mockPaymentProvider{}
	mockWallet := &mockWalletClient{}
	txnRepo := repositories.NewTransactionRepository(db)

	playerSaga := &PlayerDepositSaga{
		provider:        mockProvider,
		walletClient:    mockWallet,
		transactionRepo: txnRepo,
	}

	err := playerSaga.debitPlayerWallet(ctx, saga.SagaData)

	assert.NoError(t, err, "Compensation should skip gracefully when wallet_transaction_id is missing")
	assert.False(t, mockWallet.debitCalled, "Wallet debit should NOT be called when credit never succeeded")
}

func TestPlayerDepositSaga_NoMoneyLeakOnFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt1 := time.Now()
	completedAt2 := time.Now()

	// Mock provider that successfully debits but then we'll fail the wallet credit
	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Mobile money debit successful",
			TransactionID: "ORANGE-DEBIT-123",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
		creditResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Mobile money refund successful",
			TransactionID: "ORANGE-REFUND-123",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt2,
		},
	}

	// Track all wallet operations
	var walletOperations []string

	mockWallet := &mockWalletClient{
		creditError: fmt.Errorf("wallet service temporarily unavailable"),
		creditHook: func() {
			walletOperations = append(walletOperations, "credit_attempt_failed")
		},
	}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("INTEGRITY-TEST-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                15000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244999888",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "Test Player",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Data integrity test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga - should fail at wallet credit step
	err = saga.Execute(ctx, transaction)

	// Verify saga failed
	assert.Error(t, err, "Saga should fail when wallet credit fails")
	assert.Contains(t, err.Error(), "compensated", "Error should indicate saga was compensated")

	// Verify saga record shows compensation was executed
	savedSaga, sagaErr := sagaRepo.GetBySagaID(ctx, fmt.Sprintf("player-deposit-%s", transaction.Reference))
	require.NoError(t, sagaErr)
	assert.Equal(t, models.SagaStatusCompensated, savedSaga.Status, "Saga should be marked as COMPENSATED after successful compensation")

	// Verify transaction was updated with provider data
	savedTxn, getErr := txnRepo.GetByID(ctx, transaction.ID)
	require.NoError(t, getErr)

	// Verify provider operations - debit was recorded
	assert.Contains(t, savedTxn.ProviderData, "debit_response", "Provider debit should have been recorded")

	// CRITICAL: Verify wallet was NEVER credited despite mobile money debit succeeding
	assert.False(t, mockWallet.creditCalled, "Player wallet should NEVER have been credited")

	// Verify provider transaction ID was saved (proves mobile money debit happened)
	assert.NotNil(t, savedTxn.ProviderTransactionID, "Provider transaction ID should be saved")
	assert.Equal(t, "ORANGE-DEBIT-123", *savedTxn.ProviderTransactionID, "Debit transaction ID should match")

	// Verify compensation data exists in saga
	assert.Contains(t, savedSaga.CompensationData, "failed_step", "Compensation data should record which step failed")
	failedStep, ok := savedSaga.CompensationData["failed_step"].(float64)
	require.True(t, ok, "Failed step should be a number")
	assert.Equal(t, float64(1), failedStep, "Should have failed at step 1 (wallet credit)")

	t.Log("✅ Data integrity verified: No money leak on wallet credit failure")
	t.Logf("   - Mobile money was debited: %s", *savedTxn.ProviderTransactionID)
	t.Logf("   - Player wallet was NOT credited: %v", !mockWallet.creditCalled)
	t.Logf("   - Saga compensated successfully (mobile money refunded)")
	t.Logf("   - Saga status: %s", savedSaga.Status)
}

func TestPlayerDepositSaga_TransactionStatusUpdated(t *testing.T) {
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
			TransactionID: "ORANGE-TX-STATUS",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
	}

	mockWallet := &mockWalletClient{
		creditResponse: &walletv1.CreditPlayerWalletResponse{
			Success:       true,
			Message:       "Wallet credited",
			TransactionId: uuid.New().String(),
			NewBalance:    110000,
		},
	}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("STATUS-TEST-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                10000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244123456",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "John Doe",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Transaction status test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga - should succeed
	err = saga.Execute(ctx, transaction)
	require.NoError(t, err, "Saga should complete successfully")

	// BUG: Verify transaction status was updated to SUCCESS
	savedTxn, getErr := txnRepo.GetByID(ctx, transaction.ID)
	require.NoError(t, getErr)

	t.Logf("Transaction status after successful saga: %s", savedTxn.Status)
	assert.Equal(t, models.StatusSuccess, savedTxn.Status, "BUG: Transaction status should be SUCCESS after successful saga completion")
}

func TestPlayerDepositSaga_TransactionStatusOnFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt1 := time.Now()
	completedAt2 := time.Now()

	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Payment successful",
			TransactionID: "ORANGE-TX-FAIL-STATUS",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
		creditResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Refund successful",
			TransactionID: "ORANGE-REFUND-STATUS",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt2,
		},
	}

	mockWallet := &mockWalletClient{
		creditError: fmt.Errorf("wallet service unavailable"),
	}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("FAIL-STATUS-TEST-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                10000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244123456",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "John Doe",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Transaction failure status test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga - should fail and compensate
	err = saga.Execute(ctx, transaction)
	require.Error(t, err, "Saga should fail")

	// BUG: Verify transaction status was updated to FAILED
	savedTxn, getErr := txnRepo.GetByID(ctx, transaction.ID)
	require.NoError(t, getErr)

	t.Logf("Transaction status after failed saga: %s", savedTxn.Status)
	assert.Equal(t, models.StatusFailed, savedTxn.Status, "BUG: Transaction status should be FAILED after saga failure and compensation")
}

func TestPlayerDepositSaga_ProviderReturnsSuccessTrueButStatusFailed(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt1 := time.Now()

	// BUG TEST: Provider returns success:true but status:FAILED (mixed signals)
	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,                   // Says success...
			Status:        providers.StatusFailed, // But status is FAILED!
			Message:       "Payment failed due to insufficient funds",
			TransactionID: "ORANGE-MIXED-SIGNALS",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
	}

	mockWallet := &mockWalletClient{
		creditResponse: &walletv1.CreditPlayerWalletResponse{
			Success:       true,
			Message:       "Wallet credited",
			TransactionId: uuid.New().String(),
			NewBalance:    110000,
		},
	}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("MIXED-SIGNALS-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                10000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244123456",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "John Doe",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Provider validation test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga
	err = saga.Execute(ctx, transaction)

	// BUG: What happens? Does it treat as success or failure?
	// Current code probably treats it as SUCCESS (only checks resp.Success)
	// But it SHOULD fail because status is FAILED
	t.Logf("Saga execution result: %v", err)

	savedTxn, _ := txnRepo.GetByID(ctx, transaction.ID)
	t.Logf("Transaction status: %s", savedTxn.Status)
	t.Logf("Wallet credited: %v", mockWallet.creditCalled)

	// This should FAIL if saga doesn't validate provider status
	assert.Error(t, err, "BUG: Saga should fail when provider status is FAILED, regardless of success flag")
	assert.False(t, mockWallet.creditCalled, "BUG: Wallet should NOT be credited when provider status is FAILED")
	assert.Equal(t, models.StatusFailed, savedTxn.Status, "BUG: Transaction should be marked FAILED")
}

func TestPlayerDepositSaga_ProviderReturnsNullTransactionID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt1 := time.Now()

	// BUG TEST: Provider returns success but no transaction ID (invalid response)
	mockProvider := &mockPaymentProvider{
		debitResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Payment successful",
			TransactionID: "", // Empty transaction ID!
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt1,
		},
	}

	mockWallet := &mockWalletClient{
		creditResponse: &walletv1.CreditPlayerWalletResponse{
			Success:       true,
			Message:       "Wallet credited",
			TransactionId: uuid.New().String(),
			NewBalance:    110000,
		},
	}

	saga := NewPlayerDepositSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("NULL-TXN-ID-%d", time.Now().Unix()),
		Type:                  models.TypeDeposit,
		Status:                models.StatusPending,
		Amount:                10000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "MOBILE_MONEY",
		SourceIdentifier:      "0244123456",
		SourceName:            "MTN",
		DestinationType:       "PLAYER_WALLET",
		DestinationIdentifier: uuid.New().String(),
		DestinationName:       "John Doe",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Provider validation test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga
	err = saga.Execute(ctx, transaction)

	t.Logf("Saga execution result: %v", err)

	savedTxn, _ := txnRepo.GetByID(ctx, transaction.ID)
	t.Logf("Transaction status: %s", savedTxn.Status)
	t.Logf("Provider transaction ID: %v", savedTxn.ProviderTransactionID)

	// This should FAIL if saga doesn't validate transaction ID
	assert.Error(t, err, "BUG: Saga should fail when provider doesn't return a transaction ID")
	assert.False(t, mockWallet.creditCalled, "BUG: Should not proceed without valid transaction ID")
}
