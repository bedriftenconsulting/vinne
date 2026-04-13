package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-payment/internal/models"
	"github.com/randco/service-payment/internal/providers"
)

// WithdrawalSaga orchestrates the withdrawal flow: Winning Wallet → Mobile Money
// Two-Phase Commit Pattern:
// 1. Reserve funds in winning wallet (Phase 1)
// 2. Credit mobile money wallet (Provider)
// 3. Commit wallet debit (Phase 2)
//
// This prevents money loss if the provider fails after we've debited the wallet
type WithdrawalSaga struct {
	orchestrator    *Orchestrator
	provider        providers.PaymentProvider
	walletClient    walletv1.WalletServiceClient
	transactionRepo TransactionRepository
}

func NewWithdrawalSaga(orchestrator *Orchestrator, provider providers.PaymentProvider, walletClient walletv1.WalletServiceClient, transactionRepo TransactionRepository) *WithdrawalSaga {
	return &WithdrawalSaga{
		orchestrator:    orchestrator,
		provider:        provider,
		walletClient:    walletClient,
		transactionRepo: transactionRepo,
	}
}

func (ws *WithdrawalSaga) Execute(ctx context.Context, transaction *models.Transaction) error {
	ctx, span := tracer.Start(ctx, "withdrawal_saga.execute",
		trace.WithAttributes(
			attribute.String("transaction_id", transaction.ID.String()),
			attribute.String("reference", transaction.Reference),
			attribute.Int64("amount", transaction.Amount),
		))
	defer span.End()

	sagaID := fmt.Sprintf("withdrawal-%s", transaction.Reference)

	// Idempotency check: If saga already exists and is COMPLETED, skip execution
	// This prevents duplicate webhooks or retries from processing withdrawal multiple times
	existingSaga, getErr := ws.orchestrator.sagaRepo.GetBySagaID(ctx, sagaID)
	if getErr == nil && existingSaga.Status == models.SagaStatusCompleted {
		logger.Info("Saga already completed, skipping execution",
			"transaction_id", transaction.ID.String(),
			"reference", transaction.Reference,
			"saga_status", string(existingSaga.Status))
		span.AddEvent("saga_skipped_already_completed", trace.WithAttributes(
			attribute.String("saga_status", string(existingSaga.Status)),
		))
		return nil // Idempotent - safe to call multiple times
	}

	saga := models.CreateSaga(sagaID, transaction.ID, 3, 3)

	saga.SagaData["transaction_id"] = transaction.ID.String()
	saga.SagaData["reference"] = transaction.Reference
	saga.SagaData["user_id"] = transaction.UserID.String()
	saga.SagaData["wallet_number"] = transaction.DestinationIdentifier
	saga.SagaData["wallet_provider"] = transaction.DestinationName
	saga.SagaData["amount"] = transaction.Amount
	saga.SagaData["currency"] = transaction.Currency
	saga.SagaData["narration"] = transaction.Narration

	if err := ws.orchestrator.sagaRepo.Create(ctx, saga); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create saga: %w", err)
	}

	steps := []Step{
		{
			Name:        "ReserveWinningWalletFunds",
			Description: "Reserve funds in winning wallet",
			Execute:     ws.reserveWinningWalletFunds,
			Compensate:  ws.releaseWinningWalletReservation,
		},
		{
			Name:        "CreditMobileMoneyWallet",
			Description: "Credit customer's mobile money wallet",
			Execute:     ws.creditMobileMoneyWallet,
			Compensate:  ws.debitMobileMoneyWallet,
		},
		{
			Name:        "CommitWinningWalletDebit",
			Description: "Commit the winning wallet debit",
			Execute:     ws.commitWinningWalletDebit,
			Compensate:  nil,
		},
	}

	err := ws.orchestrator.Execute(ctx, saga, steps)
	if err != nil {
		span.RecordError(err)

		// Refresh transaction to get latest provider data
		freshTxn, getErr := ws.transactionRepo.GetByID(ctx, transaction.ID)
		if getErr != nil {
			logger.Error("Failed to refresh transaction before status update",
				"error", getErr.Error(),
				"transaction_id", transaction.ID.String())
			freshTxn = transaction // Fallback to original if refresh fails
		}

		// Update transaction status to FAILED
		freshTxn.Status = models.StatusFailed
		freshTxn.UpdatedAt = time.Now()
		if updateErr := ws.transactionRepo.Update(ctx, freshTxn); updateErr != nil {
			logger.Error("Failed to update transaction status to FAILED",
				"error", updateErr.Error(),
				"transaction_id", transaction.ID.String(),
				"reference", transaction.Reference)
		}

		return fmt.Errorf("withdrawal saga failed: %w", err)
	}

	// Refresh transaction to get latest provider data
	freshTxn, getErr := ws.transactionRepo.GetByID(ctx, transaction.ID)
	if getErr != nil {
		logger.Error("Failed to refresh transaction before status update",
			"error", getErr.Error(),
			"transaction_id", transaction.ID.String())
		freshTxn = transaction // Fallback to original if refresh fails
	}

	// Update transaction status to SUCCESS
	freshTxn.Status = models.StatusSuccess
	freshTxn.CompletedAt = saga.CompletedAt
	freshTxn.UpdatedAt = time.Now()
	if updateErr := ws.transactionRepo.Update(ctx, freshTxn); updateErr != nil {
		logger.Error("Failed to update transaction status to SUCCESS",
			"error", updateErr.Error(),
			"transaction_id", transaction.ID.String(),
			"reference", transaction.Reference)
		return fmt.Errorf("failed to update transaction status: %w", updateErr)
	}

	return nil
}

func (ws *WithdrawalSaga) reserveWinningWalletFunds(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "withdrawal_saga.reserve_winning_wallet_funds")
	defer span.End()

	retailerID := data["user_id"].(string)
	amount := data["amount"].(int64)
	reference := data["reference"].(string)

	span.SetAttributes(
		attribute.String("retailer_id", retailerID),
		attribute.Int64("amount", amount),
		attribute.String("phase", "reserve"),
	)

	walletResp, err := ws.walletClient.ReserveRetailerWalletFunds(ctx, &walletv1.ReserveRetailerWalletFundsRequest{
		RetailerId: retailerID,
		WalletType: walletv1.WalletType_RETAILER_WINNING,
		Amount:     float64(amount),
		Reference:  reference,
		TtlSeconds: 600,
		Reason:     "Pending withdrawal to mobile money",
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to reserve winning wallet funds: %w", err)
	}

	if !walletResp.Success {
		err := fmt.Errorf("winning wallet funds reservation failed: %s", walletResp.Message)
		span.RecordError(err)
		return nil, err
	}

	output := map[string]interface{}{
		"reservation_id":    walletResp.ReservationId,
		"reserved_amount":   walletResp.ReservedAmount,
		"available_balance": walletResp.AvailableBalance,
		"reserved_at":       walletResp.ReservedAt,
		"expires_at":        walletResp.ExpiresAt,
	}

	span.SetAttributes(
		attribute.String("reservation_id", walletResp.ReservationId),
		attribute.Float64("available_balance", walletResp.AvailableBalance),
	)

	return output, nil
}

func (ws *WithdrawalSaga) creditMobileMoneyWallet(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "withdrawal_saga.credit_mobile_money_wallet")
	defer span.End()

	walletNumber := data["wallet_number"].(string)
	walletProvider := data["wallet_provider"].(string)
	amount := data["amount"].(int64)
	currency := data["currency"].(string)
	reference := data["reference"].(string)
	narration := data["narration"].(string)

	span.SetAttributes(
		attribute.String("wallet_number", walletNumber),
		attribute.String("wallet_provider", walletProvider),
		attribute.Int64("amount", amount),
	)

	req := &providers.CreditWalletRequest{
		WalletNumber:   walletNumber,
		WalletProvider: walletProvider,
		Amount:         float64(amount),
		Currency:       currency,
		Narration:      narration,
		Reference:      reference,
	}

	txnID := uuid.MustParse(data["transaction_id"].(string))
	txn, getErr := ws.transactionRepo.GetByID(ctx, txnID)
	if getErr != nil {
		logger.Error("Failed to get transaction before provider call",
			"error", getErr.Error(),
			"transaction_id", txnID.String(),
			"reference", reference)
	}

	// CRITICAL: Save provider request details BEFORE calling provider
	// This ensures we have a record of the attempt even if the call fails
	if txn != nil {
		txn.ProviderData["credit_request"] = map[string]interface{}{
			"wallet_number":   walletNumber,
			"wallet_provider": walletProvider,
			"amount":          amount,
			"currency":        currency,
			"narration":       narration,
			"reference":       reference,
			"timestamp":       time.Now(),
		}
		if updateErr := ws.transactionRepo.Update(ctx, txn); updateErr != nil {
			logger.Error("Failed to save provider request (continuing)",
				"error", updateErr.Error(),
				"transaction_id", txnID.String(),
				"reference", reference)
		}
	}

	resp, err := ws.provider.CreditWallet(ctx, req)

	// CRITICAL: Save provider response OR error immediately to transaction record
	// This ensures we have a trace even if saga fails later OR if API call errors
	if txn != nil {
		if err != nil {
			txn.ProviderData["credit_error"] = map[string]interface{}{
				"error":     err.Error(),
				"timestamp": time.Now(),
			}
			if updateErr := ws.transactionRepo.Update(ctx, txn); updateErr != nil {
				logger.Error("Failed to save provider error to transaction (continuing saga)",
					"error", updateErr.Error(),
					"transaction_id", txnID.String(),
					"reference", reference)
			} else {
				logger.Info("Provider error saved to transaction record",
					"transaction_id", txnID.String(),
					"error", err.Error())
			}
		} else if resp != nil {
			txn.ProviderTransactionID = &resp.TransactionID
			txn.ProviderData["credit_response"] = map[string]interface{}{
				"success":        resp.Success,
				"status":         string(resp.Status),
				"message":        resp.Message,
				"transaction_id": resp.TransactionID,
				"timestamp":      time.Now(),
			}
			if updateErr := ws.transactionRepo.Update(ctx, txn); updateErr != nil {
				logger.Error("Failed to save provider response to transaction (continuing saga)",
					"error", updateErr.Error(),
					"transaction_id", txnID.String(),
					"reference", reference)
			} else {
				logger.Info("Provider response saved to transaction record",
					"transaction_id", txnID.String(),
					"provider_transaction_id", resp.TransactionID,
					"status", string(resp.Status))
			}
		}
	}

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to credit mobile money wallet: %w", err)
	}

	logger.Info("Provider CreditWallet response received",
		"success", resp.Success,
		"status", string(resp.Status),
		"transaction_id", resp.TransactionID,
		"reference", reference,
		"message", resp.Message)

	// Validate provider response: Check both Success flag AND Status field
	// Bug fix: Provider can return mixed signals (success=true but status=FAILED)
	if !resp.Success || resp.Status == providers.StatusFailed {
		err := fmt.Errorf("mobile money credit failed: success=%v status=%s message=%s",
			resp.Success, string(resp.Status), resp.Message)
		span.RecordError(err)
		logger.Error("Provider CreditWallet returned unsuccessful response",
			"success", resp.Success,
			"status", string(resp.Status),
			"message", resp.Message,
			"transaction_id", resp.TransactionID,
			"wallet_number", walletNumber,
			"reference", reference,
		)
		return nil, err
	}

	// Validate transaction ID is not empty
	// Bug fix: Provider must return transaction ID for tracking, refunds, and reconciliation
	if resp.TransactionID == "" {
		err := fmt.Errorf("provider did not return transaction ID")
		span.RecordError(err)
		logger.Error("Provider returned empty transaction ID",
			"success", resp.Success,
			"status", string(resp.Status),
			"wallet_number", walletNumber,
			"reference", reference,
		)
		return nil, err
	}

	if resp.Status == providers.StatusPending {
		logger.Info("Mobile money credit PENDING - stopping saga, waiting for webhook confirmation",
			"transaction_id", resp.TransactionID,
			"reference", reference,
			"status", string(resp.Status),
			"message", resp.Message)

		span.AddEvent("saga_paused_pending_webhook", trace.WithAttributes(
			attribute.String("provider_transaction_id", resp.TransactionID),
			attribute.String("status", string(resp.Status)),
		))

		return nil, fmt.Errorf("saga_pending_confirmation: mobile money credit pending confirmation from provider - transaction will resume via webhook")
	}

	logger.Info("Mobile money credit SUCCESS",
		"transaction_id", resp.TransactionID,
		"reference", reference,
		"status", string(resp.Status))

	output := map[string]interface{}{
		"provider_transaction_id": resp.TransactionID,
		"provider_status":         string(resp.Status),
		"credit_completed_at":     resp.CompletedAt,
	}

	span.SetAttributes(
		attribute.String("provider_transaction_id", resp.TransactionID),
		attribute.String("status", string(resp.Status)),
	)

	return output, nil
}

func (ws *WithdrawalSaga) commitWinningWalletDebit(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "withdrawal_saga.commit_winning_wallet_debit")
	defer span.End()

	reservationID := data["reservation_id"].(string)
	retailerID := data["user_id"].(string)

	span.SetAttributes(
		attribute.String("retailer_id", retailerID),
		attribute.String("reservation_id", reservationID),
		attribute.String("phase", "commit"),
	)

	walletResp, err := ws.walletClient.CommitReservedDebit(ctx, &walletv1.CommitReservedDebitRequest{
		RetailerId:    retailerID,
		ReservationId: reservationID,
		Reference:     data["reference"].(string),
		Notes:         "Withdrawal completed successfully",
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to commit debit: %w", err)
	}

	if !walletResp.Success {
		err := fmt.Errorf("debit commit failed: %s", walletResp.Message)
		span.RecordError(err)
		return nil, err
	}

	output := map[string]interface{}{
		"winning_wallet_transaction_id": walletResp.TransactionId,
		"debited_amount":                walletResp.DebitedAmount,
		"new_balance":                   walletResp.NewBalance,
		"committed_at":                  walletResp.CommittedAt,
	}

	span.SetAttributes(
		attribute.String("transaction_id", walletResp.TransactionId),
		attribute.Float64("new_balance", walletResp.NewBalance),
		attribute.String("committed", "true"),
	)

	return output, nil
}

func (ws *WithdrawalSaga) releaseWinningWalletReservation(ctx context.Context, data map[string]interface{}) error {
	ctx, span := tracer.Start(ctx, "withdrawal_saga.release_winning_wallet_reservation_compensation")
	defer span.End()

	reservationID, ok := data["reservation_id"].(string)
	if !ok {
		span.SetAttributes(attribute.String("skipped", "no_reservation"))
		return nil
	}

	retailerID := data["user_id"].(string)

	span.SetAttributes(
		attribute.String("retailer_id", retailerID),
		attribute.String("reservation_id", reservationID),
		attribute.String("compensation", "release_reservation"),
	)

	walletResp, err := ws.walletClient.ReleaseReservation(ctx, &walletv1.ReleaseReservationRequest{
		RetailerId:    retailerID,
		ReservationId: reservationID,
		Reason:        "External transfer failed",
	})

	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to release reservation: %w", err)
	}

	if !walletResp.Success {
		err := fmt.Errorf("reservation release failed: %s", walletResp.Message)
		span.RecordError(err)
		return err
	}

	span.SetAttributes(
		attribute.Float64("released_amount", walletResp.ReleasedAmount),
		attribute.String("released", "true"),
	)

	return nil
}

func (ws *WithdrawalSaga) debitMobileMoneyWallet(ctx context.Context, data map[string]interface{}) error {
	ctx, span := tracer.Start(ctx, "withdrawal_saga.debit_mobile_money_wallet_compensation")
	defer span.End()

	reference := data["reference"].(string)

	providerTxID, exists := data["provider_transaction_id"]
	if !exists || providerTxID == nil || providerTxID == "" {
		logger.Info("Skipping mobile money reversal - credit never succeeded",
			"reference", reference,
			"reason", "provider_transaction_id not found in saga data")
		span.AddEvent("compensation_skipped", trace.WithAttributes(
			attribute.String("reason", "credit_never_succeeded"),
		))
		return nil
	}

	logger.Info("Executing mobile money reversal compensation",
		"reference", reference,
		"provider_transaction_id", providerTxID)

	walletNumber := data["wallet_number"].(string)
	walletProvider := data["wallet_provider"].(string)
	amount := data["amount"].(int64)
	currency := data["currency"].(string)

	span.SetAttributes(
		attribute.String("wallet_number", walletNumber),
		attribute.Int64("amount", amount),
		attribute.String("compensation", "reverse_credit"),
	)

	req := &providers.DebitWalletRequest{
		WalletNumber:   walletNumber,
		WalletProvider: walletProvider,
		Amount:         float64(amount),
		Currency:       currency,
		Narration:      "Reversal - Withdrawal failed",
		Reference:      fmt.Sprintf("%s-reverse", reference),
	}

	resp, err := ws.provider.DebitWallet(ctx, req)
	if err != nil {
		span.RecordError(err)
		logger.Error("Failed to reverse mobile money credit",
			"error", err.Error(),
			"reference", reference,
			"provider_transaction_id", providerTxID)
		return fmt.Errorf("failed to reverse mobile money credit: %w", err)
	}

	if !resp.Success {
		err := fmt.Errorf("mobile money reversal failed: %s", resp.Message)
		span.RecordError(err)
		logger.Error("Mobile money reversal returned failure",
			"message", resp.Message,
			"reference", reference)
		return err
	}

	logger.Info("Mobile money reversal compensation successful",
		"reversal_transaction_id", resp.TransactionID,
		"reference", reference,
		"original_transaction_id", providerTxID)

	span.SetAttributes(attribute.String("reversal_transaction_id", resp.TransactionID))

	return nil
}
