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
	"google.golang.org/protobuf/types/known/timestamppb"

	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-payment/internal/models"
	"github.com/randco/service-payment/internal/providers"
	"github.com/randco/service-payment/internal/repositories"
)

type mockWithdrawalWalletClient struct {
	reserveResponse *walletv1.ReserveRetailerWalletFundsResponse
	reserveError    error
	commitResponse  *walletv1.CommitReservedDebitResponse
	commitError     error
	releaseResponse *walletv1.ReleaseReservationResponse
	releaseError    error
	reserveCalled   bool
	commitCalled    bool
	releaseCalled   bool
}

func (m *mockWithdrawalWalletClient) CreditPlayerWallet(ctx context.Context, in *walletv1.CreditPlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditPlayerWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) DebitPlayerWallet(ctx context.Context, in *walletv1.DebitPlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.DebitPlayerWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetPlayerWalletBalance(ctx context.Context, in *walletv1.GetPlayerWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetPlayerWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetTransactionHistory(ctx context.Context, in *walletv1.GetTransactionHistoryRequest, opts ...grpc.CallOption) (*walletv1.GetTransactionHistoryResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) CreditRetailerWallet(ctx context.Context, in *walletv1.CreditRetailerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditRetailerWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) DebitRetailerWallet(ctx context.Context, in *walletv1.DebitRetailerWalletRequest, opts ...grpc.CallOption) (*walletv1.DebitRetailerWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetRetailerWalletBalance(ctx context.Context, in *walletv1.GetRetailerWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetRetailerWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) ReserveRetailerWalletFunds(ctx context.Context, in *walletv1.ReserveRetailerWalletFundsRequest, opts ...grpc.CallOption) (*walletv1.ReserveRetailerWalletFundsResponse, error) {
	m.reserveCalled = true
	if m.reserveError != nil {
		return nil, m.reserveError
	}
	return m.reserveResponse, nil
}

func (m *mockWithdrawalWalletClient) CommitReservedDebit(ctx context.Context, in *walletv1.CommitReservedDebitRequest, opts ...grpc.CallOption) (*walletv1.CommitReservedDebitResponse, error) {
	m.commitCalled = true
	if m.commitError != nil {
		return nil, m.commitError
	}
	return m.commitResponse, nil
}

func (m *mockWithdrawalWalletClient) ReleaseReservation(ctx context.Context, in *walletv1.ReleaseReservationRequest, opts ...grpc.CallOption) (*walletv1.ReleaseReservationResponse, error) {
	m.releaseCalled = true
	if m.releaseError != nil {
		return nil, m.releaseError
	}
	return m.releaseResponse, nil
}

func (m *mockWithdrawalWalletClient) CreateAgentWallet(ctx context.Context, in *walletv1.CreateAgentWalletRequest, opts ...grpc.CallOption) (*walletv1.CreateAgentWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) CreateRetailerWallets(ctx context.Context, in *walletv1.CreateRetailerWalletsRequest, opts ...grpc.CallOption) (*walletv1.CreateRetailerWalletsResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) CreditAgentWallet(ctx context.Context, in *walletv1.CreditAgentWalletRequest, opts ...grpc.CallOption) (*walletv1.CreditAgentWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetAgentWalletBalance(ctx context.Context, in *walletv1.GetAgentWalletBalanceRequest, opts ...grpc.CallOption) (*walletv1.GetAgentWalletBalanceResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetAllTransactions(ctx context.Context, in *walletv1.GetAllTransactionsRequest, opts ...grpc.CallOption) (*walletv1.GetAllTransactionsResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) ReverseTransaction(ctx context.Context, in *walletv1.ReverseTransactionRequest, opts ...grpc.CallOption) (*walletv1.ReverseTransactionResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) SetCommissionRate(ctx context.Context, in *walletv1.SetCommissionRateRequest, opts ...grpc.CallOption) (*walletv1.SetCommissionRateResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetCommissionRate(ctx context.Context, in *walletv1.GetCommissionRateRequest, opts ...grpc.CallOption) (*walletv1.GetCommissionRateResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetCommissionReport(ctx context.Context, in *walletv1.GetCommissionReportRequest, opts ...grpc.CallOption) (*walletv1.GetCommissionReportResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) UpdateAgentCommission(ctx context.Context, in *walletv1.UpdateAgentCommissionRequest, opts ...grpc.CallOption) (*walletv1.UpdateAgentCommissionResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) TransferAgentToRetailer(ctx context.Context, in *walletv1.TransferAgentToRetailerRequest, opts ...grpc.CallOption) (*walletv1.TransferAgentToRetailerResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) CreatePlayerWallet(ctx context.Context, in *walletv1.CreatePlayerWalletRequest, opts ...grpc.CallOption) (*walletv1.CreatePlayerWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) ReservePlayerWalletFunds(ctx context.Context, in *walletv1.ReservePlayerWalletFundsRequest, opts ...grpc.CallOption) (*walletv1.ReservePlayerWalletFundsResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetDailyCommissions(ctx context.Context, in *walletv1.GetDailyCommissionsRequest, opts ...grpc.CallOption) (*walletv1.GetDailyCommissionsResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) PlaceHoldOnWallet(ctx context.Context, in *walletv1.PlaceHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.PlaceHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) ReleaseHoldOnWallet(ctx context.Context, in *walletv1.ReleaseHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.ReleaseHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetHoldOnWallet(ctx context.Context, in *walletv1.GetHoldOnWalletRequest, opts ...grpc.CallOption) (*walletv1.GetHoldOnWalletResponse, error) {
	return nil, nil
}

func (m *mockWithdrawalWalletClient) GetHoldByRetailer(ctx context.Context, in *walletv1.GetHoldByRetailerRequest, opts ...grpc.CallOption) (*walletv1.GetHoldByRetailerResponse, error) {
	return nil, nil
}

func TestWithdrawalSaga_PENDING_StopsSaga(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	mockProvider := &mockPaymentProvider{
		creditResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusPending,
			Message:       "Withdrawal initiated",
			TransactionID: "ORANGE-WITHDRAW-12345",
			RequestedAt:   time.Now(),
		},
	}

	mockWallet := &mockWithdrawalWalletClient{
		reserveResponse: &walletv1.ReserveRetailerWalletFundsResponse{
			Success:          true,
			Message:          "Funds reserved",
			ReservationId:    uuid.New().String(),
			ReservedAmount:   30000,
			AvailableBalance: 70000,
			ReservedAt:       timestamppb.Now(),
			ExpiresAt:        timestamppb.New(time.Now().Add(10 * time.Minute)),
		},
		releaseResponse: &walletv1.ReleaseReservationResponse{
			Success:             true,
			Message:             "Reservation released",
			ReleasedAmount:      30000,
			NewAvailableBalance: 100000,
		},
	}

	saga := NewWithdrawalSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("WITHDRAW-REF-%d", time.Now().Unix()),
		Type:                  models.TypeWithdrawal,
		Status:                models.StatusPending,
		Amount:                30000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "RETAILER_WINNING_WALLET",
		SourceIdentifier:      uuid.New().String(),
		SourceName:            "Retailer Winning Wallet",
		DestinationType:       "MOBILE_MONEY",
		DestinationIdentifier: "0244567890",
		DestinationName:       "MTN",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test withdrawal",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.Error(t, err, "Saga should fail when provider returns PENDING")
	assert.Contains(t, err.Error(), "saga")
	assert.True(t, mockWallet.reserveCalled, "Funds should be reserved")
	assert.False(t, mockWallet.commitCalled, "Wallet debit should NOT be committed when status is PENDING")
}

func TestWithdrawalSaga_SUCCESS_CompletesSuccessfully(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	completedAt := time.Now()
	mockProvider := &mockPaymentProvider{
		creditResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Withdrawal successful",
			TransactionID: "ORANGE-WITHDRAW-12345",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt,
		},
	}

	mockWallet := &mockWithdrawalWalletClient{
		reserveResponse: &walletv1.ReserveRetailerWalletFundsResponse{
			Success:          true,
			Message:          "Funds reserved",
			ReservationId:    uuid.New().String(),
			ReservedAmount:   30000,
			AvailableBalance: 70000,
			ReservedAt:       timestamppb.Now(),
			ExpiresAt:        timestamppb.New(time.Now().Add(10 * time.Minute)),
		},
		commitResponse: &walletv1.CommitReservedDebitResponse{
			Success:       true,
			Message:       "Debit committed",
			TransactionId: uuid.New().String(),
			DebitedAmount: 30000,
			NewBalance:    70000,
			CommittedAt:   timestamppb.Now(),
		},
	}

	saga := NewWithdrawalSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("WITHDRAW-REF-%d", time.Now().Unix()),
		Type:                  models.TypeWithdrawal,
		Status:                models.StatusPending,
		Amount:                30000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "RETAILER_WINNING_WALLET",
		SourceIdentifier:      uuid.New().String(),
		SourceName:            "Retailer Winning Wallet",
		DestinationType:       "MOBILE_MONEY",
		DestinationIdentifier: "0244567890",
		DestinationName:       "MTN",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test withdrawal",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.NoError(t, err, "Saga should complete successfully")
	assert.True(t, mockWallet.reserveCalled, "Funds should be reserved")
	assert.True(t, mockWallet.commitCalled, "Wallet debit should be committed")
	assert.False(t, mockWallet.releaseCalled, "Reservation should NOT be released on success")
}

func TestWithdrawalSaga_MobileMoneyReversalSkipsWhenCreditNeverSucceeded(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	saga := &models.Saga{
		ID:          uuid.New(),
		SagaID:      "test-withdrawal-compensation",
		Status:      models.SagaStatusFailed,
		CurrentStep: 2,
		TotalSteps:  3,
		SagaData: map[string]interface{}{
			"reference":       "TEST-WITHDRAW-COMP",
			"amount":          int64(30000),
			"wallet_number":   "0244567890",
			"wallet_provider": "MTN",
			"currency":        "GHS",
			"reservation_id":  uuid.New().String(),
		},
	}

	mockProvider := &mockPaymentProvider{}
	mockWallet := &mockWithdrawalWalletClient{}
	txnRepo := repositories.NewTransactionRepository(db)

	withdrawalSaga := &WithdrawalSaga{
		provider:        mockProvider,
		walletClient:    mockWallet,
		transactionRepo: txnRepo,
	}

	err := withdrawalSaga.debitMobileMoneyWallet(ctx, saga.SagaData)

	assert.NoError(t, err, "Compensation should skip gracefully when provider_transaction_id is missing")
}

func TestWithdrawalSaga_ReservationReleasedOnFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	mockProvider := &mockPaymentProvider{
		creditError: fmt.Errorf("provider service unavailable"),
	}

	reservationID := uuid.New().String()

	mockWallet := &mockWithdrawalWalletClient{
		reserveResponse: &walletv1.ReserveRetailerWalletFundsResponse{
			Success:          true,
			Message:          "Funds reserved",
			ReservationId:    reservationID,
			ReservedAmount:   30000,
			AvailableBalance: 70000,
			ReservedAt:       timestamppb.Now(),
			ExpiresAt:        timestamppb.New(time.Now().Add(10 * time.Minute)),
		},
		releaseResponse: &walletv1.ReleaseReservationResponse{
			Success:        true,
			Message:        "Reservation released",
			ReleasedAmount: 30000,
		},
	}

	saga := NewWithdrawalSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("WITHDRAW-FAIL-%d", time.Now().Unix()),
		Type:                  models.TypeWithdrawal,
		Status:                models.StatusPending,
		Amount:                30000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "RETAILER_WINNING_WALLET",
		SourceIdentifier:      uuid.New().String(),
		SourceName:            "Retailer Winning Wallet",
		DestinationType:       "MOBILE_MONEY",
		DestinationIdentifier: "0244567890",
		DestinationName:       "MTN",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Test withdrawal failure",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.Error(t, err, "Saga should fail when provider fails")
	assert.Contains(t, err.Error(), "saga")
	assert.True(t, mockWallet.reserveCalled, "Funds should be reserved")
	assert.True(t, mockWallet.releaseCalled, "Reservation should be released on failure")
}

func TestWithdrawalSaga_ReservationReleaseSkipsWhenReserveNeverSucceeded(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	saga := &models.Saga{
		ID:          uuid.New(),
		SagaID:      "test-reserve-compensation",
		Status:      models.SagaStatusFailed,
		CurrentStep: 1,
		TotalSteps:  3,
		SagaData: map[string]interface{}{
			"user_id":   uuid.New().String(),
			"reference": "TEST-RESERVE-COMP",
			"amount":    int64(30000),
		},
	}

	mockProvider := &mockPaymentProvider{}
	mockWallet := &mockWithdrawalWalletClient{}
	txnRepo := repositories.NewTransactionRepository(db)

	withdrawalSaga := &WithdrawalSaga{
		provider:        mockProvider,
		walletClient:    mockWallet,
		transactionRepo: txnRepo,
	}

	err := withdrawalSaga.releaseWinningWalletReservation(ctx, saga.SagaData)

	assert.NoError(t, err, "Compensation should skip gracefully when reservation_id is missing")
	assert.False(t, mockWallet.releaseCalled, "Reservation release should NOT be called when reserve never succeeded")
}

func TestWithdrawalSaga_TwoPhaseCommitProtectsFromMoneyLoss(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	reservationID := uuid.New().String()

	completedAt2 := time.Now()
	mockProvider := &mockPaymentProvider{
		creditResponse: &providers.TransactionResponse{
			Success:       true,
			Status:        providers.StatusSuccess,
			Message:       "Withdrawal successful",
			TransactionID: "ORANGE-WITHDRAW-99999",
			RequestedAt:   time.Now(),
			CompletedAt:   &completedAt2,
		},
	}

	mockWallet := &mockWithdrawalWalletClient{
		reserveResponse: &walletv1.ReserveRetailerWalletFundsResponse{
			Success:          true,
			Message:          "Funds reserved",
			ReservationId:    reservationID,
			ReservedAmount:   30000,
			AvailableBalance: 70000,
			ReservedAt:       timestamppb.Now(),
			ExpiresAt:        timestamppb.New(time.Now().Add(10 * time.Minute)),
		},
		commitResponse: &walletv1.CommitReservedDebitResponse{
			Success:       true,
			Message:       "Debit committed",
			TransactionId: uuid.New().String(),
			DebitedAmount: 30000,
			NewBalance:    70000,
			CommittedAt:   timestamppb.Now(),
		},
	}

	saga := NewWithdrawalSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("2PC-TEST-%d", time.Now().Unix()),
		Type:                  models.TypeWithdrawal,
		Status:                models.StatusPending,
		Amount:                30000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "RETAILER_WINNING_WALLET",
		SourceIdentifier:      uuid.New().String(),
		SourceName:            "Retailer Winning Wallet",
		DestinationType:       "MOBILE_MONEY",
		DestinationIdentifier: "0244567890",
		DestinationName:       "MTN",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Two-phase commit test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	err = saga.Execute(ctx, transaction)

	assert.NoError(t, err, "Two-phase commit should complete successfully")
	assert.True(t, mockWallet.reserveCalled, "Phase 1: Reserve called")
	assert.True(t, mockWallet.commitCalled, "Phase 2: Commit called")
	assert.False(t, mockWallet.releaseCalled, "Reservation should NOT be released on success")
}

func TestWithdrawalSaga_NoMoneyLeakOnFailure(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	// Mock provider that fails to credit mobile money
	mockProvider := &mockPaymentProvider{
		creditError: fmt.Errorf("mobile money provider temporarily unavailable"),
	}

	reservationID := uuid.New().String()

	// Mock wallet that successfully reserves funds
	mockWallet := &mockWithdrawalWalletClient{
		reserveResponse: &walletv1.ReserveRetailerWalletFundsResponse{
			Success:          true,
			Message:          "Funds reserved",
			ReservationId:    reservationID,
			ReservedAmount:   40000,
			AvailableBalance: 60000,
			ReservedAt:       timestamppb.Now(),
			ExpiresAt:        timestamppb.New(time.Now().Add(10 * time.Minute)),
		},
		releaseResponse: &walletv1.ReleaseReservationResponse{
			Success:             true,
			Message:             "Reservation released",
			ReleasedAmount:      40000,
			NewAvailableBalance: 100000,
		},
	}

	saga := NewWithdrawalSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("WITHDRAW-INTEGRITY-%d", time.Now().Unix()),
		Type:                  models.TypeWithdrawal,
		Status:                models.StatusPending,
		Amount:                40000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "RETAILER_WINNING_WALLET",
		SourceIdentifier:      uuid.New().String(),
		SourceName:            "Retailer Winning Wallet",
		DestinationType:       "MOBILE_MONEY",
		DestinationIdentifier: "0244777888",
		DestinationName:       "MTN",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Withdrawal integrity test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga - should fail at mobile money credit step
	err = saga.Execute(ctx, transaction)

	// Verify saga failed and compensated
	assert.Error(t, err, "Saga should fail when mobile money credit fails")
	assert.Contains(t, err.Error(), "compensated", "Error should indicate saga was compensated")

	// Verify saga record shows compensation was executed
	savedSaga, sagaErr := sagaRepo.GetBySagaID(ctx, fmt.Sprintf("withdrawal-%s", transaction.Reference))
	require.NoError(t, sagaErr)
	assert.Equal(t, models.SagaStatusCompensated, savedSaga.Status, "Saga should be marked as COMPENSATED after successful compensation")

	// CRITICAL: Verify two-phase commit protected funds
	assert.True(t, mockWallet.reserveCalled, "Funds should have been reserved (phase 1)")
	assert.False(t, mockWallet.commitCalled, "Debit should NEVER have been committed (phase 2 skipped)")
	assert.True(t, mockWallet.releaseCalled, "Reservation should have been released (compensation)")

	// Verify compensation data exists in saga
	assert.Contains(t, savedSaga.CompensationData, "failed_step", "Compensation data should record which step failed")
	failedStep, ok := savedSaga.CompensationData["failed_step"].(float64)
	require.True(t, ok, "Failed step should be a number")
	assert.Equal(t, float64(1), failedStep, "Should have failed at step 1 (mobile money credit)")

	// Verify reservation ID was saved (proves funds were reserved)
	assert.Contains(t, savedSaga.SagaData, "reservation_id", "Reservation ID should be saved in saga data")
	assert.Equal(t, reservationID, savedSaga.SagaData["reservation_id"], "Reservation ID should match")

	t.Log("✅ Data integrity verified: No money leak on mobile money credit failure")
	t.Logf("   - Funds reserved in winning wallet: %s", reservationID)
	t.Logf("   - Mobile money was NOT credited (failed)")
	t.Logf("   - Winning wallet debit was NOT committed: %v", !mockWallet.commitCalled)
	t.Logf("   - Reservation was released (compensation): %v", mockWallet.releaseCalled)
	t.Logf("   - Saga status: %s", savedSaga.Status)
	t.Logf("   - Two-phase commit prevented money loss!")
}

func TestWithdrawalSaga_OrphanedReservationTimeout(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	txnRepo := repositories.NewTransactionRepository(db)
	sagaRepo := repositories.NewSagaRepository(db)
	orchestrator := NewOrchestrator(sagaRepo, &mockPublisher{})

	// Mock provider that fails AFTER funds are reserved
	mockProvider := &mockPaymentProvider{
		creditError: fmt.Errorf("provider timeout - network error"),
	}

	reservationID := uuid.New().String()

	mockWallet := &mockWithdrawalWalletClient{
		reserveResponse: &walletv1.ReserveRetailerWalletFundsResponse{
			Success:          true,
			Message:          "Funds reserved",
			ReservationId:    reservationID,
			ReservedAmount:   50000,
			AvailableBalance: 50000,
			ReservedAt:       timestamppb.Now(),
			ExpiresAt:        timestamppb.New(time.Now().Add(10 * time.Minute)), // 10 min TTL
		},
		releaseResponse: &walletv1.ReleaseReservationResponse{
			Success:             true,
			Message:             "Reservation released",
			ReleasedAmount:      50000,
			NewAvailableBalance: 100000,
		},
	}

	saga := NewWithdrawalSaga(orchestrator, mockProvider, mockWallet, txnRepo)

	transaction := &models.Transaction{
		ID:                    uuid.New(),
		Reference:             fmt.Sprintf("ORPHAN-RESERVE-%d", time.Now().Unix()),
		Type:                  models.TypeWithdrawal,
		Status:                models.StatusPending,
		Amount:                50000,
		Currency:              "GHS",
		UserID:                uuid.New(),
		SourceType:            "RETAILER_WINNING_WALLET",
		SourceIdentifier:      uuid.New().String(),
		SourceName:            "Retailer Winning Wallet",
		DestinationType:       "MOBILE_MONEY",
		DestinationIdentifier: "0244777888",
		DestinationName:       "MTN",
		ProviderData:          make(map[string]interface{}),
		Narration:             "Orphaned reservation test",
		RequestedAt:           time.Now(),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	err := txnRepo.Create(ctx, transaction)
	require.NoError(t, err)

	// Execute saga - should fail at mobile money credit and compensate
	err = saga.Execute(ctx, transaction)

	// Verify saga failed and compensated
	require.Error(t, err, "Saga should fail when provider times out")
	assert.True(t, mockWallet.reserveCalled, "Funds should have been reserved")
	assert.True(t, mockWallet.releaseCalled, "Reservation should have been released via compensation")

	savedSaga, sagaErr := sagaRepo.GetBySagaID(ctx, fmt.Sprintf("withdrawal-%s", transaction.Reference))
	require.NoError(t, sagaErr)
	assert.Equal(t, models.SagaStatusCompensated, savedSaga.Status, "Saga should be compensated")

	// BUG CHECK: What if saga crashed BEFORE compensation?
	// Scenario: Reserved funds at 10:00, saga crashes, reservation expires at 10:10
	// Question: Is there a background job to release expired reservations?
	// If not, funds are locked forever!

	t.Log("✅ Compensation released reservation successfully")
	t.Logf("   Reservation ID: %s", reservationID)
	t.Logf("   Reservation released: %v", mockWallet.releaseCalled)

	// This test PASSES because compensation works
	// But real bug: What if process crashes BEFORE compensation runs?
	t.Log("⚠️  WARNING: If saga process crashes before compensation, reservation becomes orphaned!")
	t.Log("⚠️  RECOMMENDATION: Wallet service needs background job to auto-release expired reservations")
}
