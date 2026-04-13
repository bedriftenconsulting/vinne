package saga

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	walletv1 "github.com/randco/randco-microservices/proto/wallet/v1"
	"github.com/randco/service-payment/internal/models"
	"github.com/randco/service-payment/internal/providers"
)

var playerLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// PlayerDepositSaga orchestrates the deposit flow: Mobile Money → Player Wallet
// Steps:
// 1. Debit mobile money wallet (Provider)
// 2. Credit player wallet (Wallet Service)
type PlayerDepositSaga struct {
	orchestrator    *Orchestrator
	provider        providers.PaymentProvider
	walletClient    walletv1.WalletServiceClient
	transactionRepo TransactionRepository
}

func NewPlayerDepositSaga(orchestrator *Orchestrator, provider providers.PaymentProvider, walletClient walletv1.WalletServiceClient, transactionRepo TransactionRepository) *PlayerDepositSaga {
	return &PlayerDepositSaga{
		orchestrator:    orchestrator,
		provider:        provider,
		walletClient:    walletClient,
		transactionRepo: transactionRepo,
	}
}

func (pds *PlayerDepositSaga) Execute(ctx context.Context, transaction *models.Transaction) error {
	ctx, span := tracer.Start(ctx, "player_deposit_saga.execute",
		trace.WithAttributes(
			attribute.String("transaction_id", transaction.ID.String()),
			attribute.String("reference", transaction.Reference),
			attribute.Int64("amount", transaction.Amount),
		))
	defer span.End()

	sagaID := fmt.Sprintf("player-deposit-%s", transaction.Reference)

	// Idempotency check: If saga already exists and is COMPLETED, skip execution
	// This prevents duplicate webhooks or retries from crediting wallet multiple times
	existingSaga, getErr := pds.orchestrator.sagaRepo.GetBySagaID(ctx, sagaID)
	if getErr == nil && existingSaga.Status == models.SagaStatusCompleted {
		playerLogger.Info("Saga already completed, skipping execution",
			"transaction_id", transaction.ID.String(),
			"reference", transaction.Reference,
			"saga_status", string(existingSaga.Status))
		span.AddEvent("saga_skipped_already_completed", trace.WithAttributes(
			attribute.String("saga_status", string(existingSaga.Status)),
		))
		return nil // Idempotent - safe to call multiple times
	}

	saga := models.CreateSaga(sagaID, transaction.ID, 2, 3)

	saga.SagaData["transaction_id"] = transaction.ID.String()
	saga.SagaData["reference"] = transaction.Reference
	saga.SagaData["player_id"] = transaction.UserID.String()
	saga.SagaData["wallet_number"] = transaction.SourceIdentifier
	saga.SagaData["wallet_provider"] = transaction.SourceName
	saga.SagaData["amount"] = transaction.Amount
	saga.SagaData["currency"] = transaction.Currency
	saga.SagaData["narration"] = transaction.Narration
	saga.SagaData["customer_name"] = transaction.DestinationName

	if err := pds.orchestrator.sagaRepo.Create(ctx, saga); err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to create saga: %w", err)
	}

	steps := []Step{
		{
			Name:        "DebitMobileMoneyWallet",
			Description: "Debit player's mobile money wallet",
			Execute:     pds.debitMobileMoneyWallet,
			Compensate:  pds.creditMobileMoneyWallet,
		},
		{
			Name:        "CreditPlayerWallet",
			Description: "Credit player's wallet",
			Execute:     pds.creditPlayerWallet,
			Compensate:  pds.debitPlayerWallet,
		},
	}

	err := pds.orchestrator.Execute(ctx, saga, steps)
	if err != nil {
		span.RecordError(err)

		// Refresh transaction to get latest provider data
		freshTxn, getErr := pds.transactionRepo.GetByID(ctx, transaction.ID)
		if getErr != nil {
			playerLogger.Error("Failed to refresh transaction before status update",
				"error", getErr.Error(),
				"transaction_id", transaction.ID.String())
			freshTxn = transaction // Fallback to original if refresh fails
		}

		// Update transaction status to FAILED
		freshTxn.Status = models.StatusFailed
		freshTxn.UpdatedAt = time.Now()
		if updateErr := pds.transactionRepo.Update(ctx, freshTxn); updateErr != nil {
			playerLogger.Error("Failed to update transaction status to FAILED",
				"error", updateErr.Error(),
				"transaction_id", transaction.ID.String(),
				"reference", transaction.Reference)
		}

		return fmt.Errorf("player deposit saga failed: %w", err)
	}

	// Refresh transaction to get latest provider data
	freshTxn, getErr := pds.transactionRepo.GetByID(ctx, transaction.ID)
	if getErr != nil {
		playerLogger.Error("Failed to refresh transaction before status update",
			"error", getErr.Error(),
			"transaction_id", transaction.ID.String())
		freshTxn = transaction // Fallback to original if refresh fails
	}

	// Update transaction status to SUCCESS
	freshTxn.Status = models.StatusSuccess
	freshTxn.CompletedAt = saga.CompletedAt
	freshTxn.UpdatedAt = time.Now()
	if updateErr := pds.transactionRepo.Update(ctx, freshTxn); updateErr != nil {
		playerLogger.Error("Failed to update transaction status to SUCCESS",
			"error", updateErr.Error(),
			"transaction_id", transaction.ID.String(),
			"reference", transaction.Reference)
		return fmt.Errorf("failed to update transaction status: %w", updateErr)
	}

	return nil
}

func (pds *PlayerDepositSaga) debitMobileMoneyWallet(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "player_deposit_saga.debit_mobile_money_wallet")
	defer span.End()

	walletNumber := data["wallet_number"].(string)
	walletProvider := data["wallet_provider"].(string)
	amount := data["amount"].(int64)
	currency := data["currency"].(string)
	reference := data["reference"].(string)
	narration := data["narration"].(string)
	customerName := data["customer_name"].(string)

	if narration == "" {
		narration = fmt.Sprintf("Player deposit - GH₵%.2f via %s", float64(amount)/100.0, walletProvider)
	}

	span.SetAttributes(
		attribute.String("wallet_number", walletNumber),
		attribute.String("wallet_provider", walletProvider),
		attribute.Int64("amount", amount),
	)

	req := &providers.DebitWalletRequest{
		WalletNumber:   walletNumber,
		WalletProvider: walletProvider,
		Amount:         float64(amount),
		Currency:       currency,
		Narration:      narration,
		Reference:      reference,
		CustomerName:   customerName,
	}

	playerLogger.Debug("Calling provider DebitWallet",
		"wallet_number", walletNumber,
		"wallet_provider", walletProvider,
		"amount_pesewas", amount,
		"amount_ghs", float64(amount)/100.0,
		"reference", reference,
		"customer_name", customerName)

	txnID := uuid.MustParse(data["transaction_id"].(string))
	txn, getErr := pds.transactionRepo.GetByID(ctx, txnID)
	if getErr != nil {
		playerLogger.Error("Failed to get transaction before provider call",
			"error", getErr.Error(),
			"transaction_id", txnID.String(),
			"reference", reference)
	}

	// CRITICAL: Save provider request details BEFORE calling provider
	// This ensures we have a record of the attempt even if the call fails
	if txn != nil {
		txn.ProviderData["debit_request"] = map[string]interface{}{
			"wallet_number":   walletNumber,
			"wallet_provider": walletProvider,
			"amount":          amount,
			"currency":        currency,
			"narration":       narration,
			"reference":       reference,
			"timestamp":       time.Now(),
		}
		if updateErr := pds.transactionRepo.Update(ctx, txn); updateErr != nil {
			playerLogger.Error("Failed to save provider request (continuing)",
				"error", updateErr.Error(),
				"transaction_id", txnID.String(),
				"reference", reference)
		}
	}

	resp, err := pds.provider.DebitWallet(ctx, req)

	// CRITICAL: Save provider response OR error immediately to transaction record
	// This ensures we have a trace even if saga fails later OR if API call errors
	if txn != nil {
		if err != nil {
			txn.ProviderData["debit_error"] = map[string]interface{}{
				"error":     err.Error(),
				"timestamp": time.Now(),
			}
			if updateErr := pds.transactionRepo.Update(ctx, txn); updateErr != nil {
				playerLogger.Error("Failed to save provider error to transaction (continuing saga)",
					"error", updateErr.Error(),
					"transaction_id", txnID.String(),
					"reference", reference)
			} else {
				playerLogger.Info("Provider error saved to transaction record",
					"transaction_id", txnID.String(),
					"error", err.Error())
			}
		} else if resp != nil {
			txn.ProviderTransactionID = &resp.TransactionID
			txn.ProviderData["debit_response"] = map[string]interface{}{
				"success":        resp.Success,
				"status":         string(resp.Status),
				"message":        resp.Message,
				"transaction_id": resp.TransactionID,
				"timestamp":      time.Now(),
			}
			if updateErr := pds.transactionRepo.Update(ctx, txn); updateErr != nil {
				playerLogger.Error("Failed to save provider response to transaction (continuing saga)",
					"error", updateErr.Error(),
					"transaction_id", txnID.String(),
					"reference", reference)
			} else {
				playerLogger.Info("Provider response saved to transaction record",
					"transaction_id", txnID.String(),
					"provider_transaction_id", resp.TransactionID,
					"status", string(resp.Status))
			}
		}
	}

	if err != nil {
		span.RecordError(err)
		playerLogger.Error("Provider DebitWallet call failed",
			"error", err.Error(),
			"error_type", fmt.Sprintf("%T", err),
			"wallet_number", walletNumber,
			"wallet_provider", walletProvider,
			"amount", amount,
			"reference", reference,
		)
		return nil, fmt.Errorf("failed to debit mobile money wallet: %w", err)
	}

	playerLogger.Info("Provider DebitWallet response received",
		"success", resp.Success,
		"status", string(resp.Status),
		"transaction_id", resp.TransactionID,
		"reference", reference,
		"message", resp.Message)

	// Validate provider response: Check both Success flag AND Status field
	// Bug fix: Provider can return mixed signals (success=true but status=FAILED)
	if !resp.Success || resp.Status == providers.StatusFailed {
		err := fmt.Errorf("mobile money debit failed: success=%v status=%s message=%s",
			resp.Success, string(resp.Status), resp.Message)
		span.RecordError(err)
		playerLogger.Error("Provider DebitWallet returned unsuccessful response",
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
		playerLogger.Error("Provider returned empty transaction ID",
			"success", resp.Success,
			"status", string(resp.Status),
			"wallet_number", walletNumber,
			"reference", reference,
		)
		return nil, err
	}

	if resp.Status == providers.StatusPending {
		playerLogger.Info("Mobile money debit PENDING - stopping saga, waiting for webhook confirmation",
			"transaction_id", resp.TransactionID,
			"reference", reference,
			"status", string(resp.Status),
			"message", resp.Message)

		span.AddEvent("saga_paused_pending_webhook", trace.WithAttributes(
			attribute.String("provider_transaction_id", resp.TransactionID),
			attribute.String("status", string(resp.Status)),
		))

		return nil, fmt.Errorf("saga_pending_confirmation: mobile money debit pending confirmation from provider - transaction will resume via webhook")
	}

	playerLogger.Debug("Mobile money wallet debited successfully",
		"transaction_id", resp.TransactionID,
		"reference", reference,
		"status", string(resp.Status))

	output := map[string]interface{}{
		"provider_transaction_id": resp.TransactionID,
		"provider_status":         string(resp.Status),
		"debit_completed_at":      resp.CompletedAt,
	}

	span.SetAttributes(
		attribute.String("provider_transaction_id", resp.TransactionID),
		attribute.String("status", string(resp.Status)),
	)

	return output, nil
}

func (pds *PlayerDepositSaga) creditPlayerWallet(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "player_deposit_saga.credit_player_wallet")
	defer span.End()

	playerID := data["player_id"].(string)
	amount := data["amount"].(int64)
	reference := data["reference"].(string)

	span.SetAttributes(
		attribute.String("player_id", playerID),
		attribute.Int64("amount", amount),
	)

	walletResp, err := pds.walletClient.CreditPlayerWallet(ctx, &walletv1.CreditPlayerWalletRequest{
		PlayerId:       playerID,
		Amount:         float64(amount),
		Reference:      reference,
		Notes:          "Deposit from mobile money",
		IdempotencyKey: reference,
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to credit player wallet: %w", err)
	}

	if !walletResp.Success {
		err := fmt.Errorf("player wallet credit failed: %s", walletResp.Message)
		span.RecordError(err)
		return nil, err
	}

	output := map[string]interface{}{
		"wallet_transaction_id": walletResp.TransactionId,
		"wallet_balance":        walletResp.NewBalance,
		"credited_amount":       walletResp.CreditedAmount,
	}

	span.SetAttributes(
		attribute.String("wallet_transaction_id", walletResp.TransactionId),
		attribute.Float64("new_balance", walletResp.NewBalance),
	)

	return output, nil
}

func (pds *PlayerDepositSaga) creditMobileMoneyWallet(ctx context.Context, data map[string]interface{}) error {
	ctx, span := tracer.Start(ctx, "player_deposit_saga.credit_mobile_money_wallet_compensation")
	defer span.End()

	reference := data["reference"].(string)

	providerTxID, exists := data["provider_transaction_id"]
	if !exists || providerTxID == nil || providerTxID == "" {
		playerLogger.Info("Skipping mobile money refund - debit never succeeded",
			"reference", reference,
			"reason", "provider_transaction_id not found in saga data")
		span.AddEvent("compensation_skipped", trace.WithAttributes(
			attribute.String("reason", "debit_never_succeeded"),
		))
		return nil
	}

	playerLogger.Info("Executing mobile money refund compensation",
		"reference", reference,
		"provider_transaction_id", providerTxID)

	walletNumber := data["wallet_number"].(string)
	walletProvider := data["wallet_provider"].(string)
	amount := data["amount"].(int64)
	currency := data["currency"].(string)

	span.SetAttributes(
		attribute.String("wallet_number", walletNumber),
		attribute.Int64("amount", amount),
		attribute.String("compensation", "refund"),
	)

	req := &providers.CreditWalletRequest{
		WalletNumber:   walletNumber,
		WalletProvider: walletProvider,
		Amount:         float64(amount),
		Currency:       currency,
		Narration:      "Refund - Deposit failed",
		Reference:      fmt.Sprintf("%s-refund", reference),
	}

	resp, err := pds.provider.CreditWallet(ctx, req)
	if err != nil {
		span.RecordError(err)
		playerLogger.Error("Failed to refund mobile money wallet",
			"error", err.Error(),
			"reference", reference,
			"provider_transaction_id", providerTxID)
		return fmt.Errorf("failed to refund mobile money wallet: %w", err)
	}

	if !resp.Success {
		err := fmt.Errorf("mobile money refund failed: %s", resp.Message)
		span.RecordError(err)
		playerLogger.Error("Mobile money refund returned failure",
			"message", resp.Message,
			"reference", reference)
		return err
	}

	playerLogger.Info("Mobile money refund compensation successful",
		"refund_transaction_id", resp.TransactionID,
		"reference", reference,
		"original_transaction_id", providerTxID)

	span.SetAttributes(attribute.String("refund_transaction_id", resp.TransactionID))

	return nil
}

func (pds *PlayerDepositSaga) debitPlayerWallet(ctx context.Context, data map[string]interface{}) error {
	ctx, span := tracer.Start(ctx, "player_deposit_saga.debit_player_wallet_compensation")
	defer span.End()

	playerID := data["player_id"].(string)
	amount := data["amount"].(int64)
	reference := data["reference"].(string)

	span.SetAttributes(
		attribute.String("player_id", playerID),
		attribute.Int64("amount", amount),
		attribute.String("compensation", "reverse_credit"),
	)

	// CRITICAL: Check if the credit operation actually succeeded
	// If wallet_transaction_id doesn't exist in saga data, it means CreditPlayerWallet never succeeded
	// In that case, skip the compensation (nothing to reverse)
	walletTxID, exists := data["wallet_transaction_id"]
	if !exists || walletTxID == nil || walletTxID == "" {
		playerLogger.Info("Skipping wallet debit compensation - credit never succeeded",
			"player_id", playerID,
			"reference", reference,
			"reason", "wallet_transaction_id not found in saga data")
		span.AddEvent("compensation_skipped", trace.WithAttributes(
			attribute.String("reason", "credit_never_succeeded"),
		))
		return nil
	}

	playerLogger.Info("Executing wallet debit compensation",
		"player_id", playerID,
		"reference", reference,
		"wallet_transaction_id", walletTxID,
		"amount", amount)

	walletResp, err := pds.walletClient.DebitPlayerWallet(ctx, &walletv1.DebitPlayerWalletRequest{
		PlayerId:       playerID,
		Amount:         float64(amount),
		Reference:      fmt.Sprintf("%s-reverse", reference),
		Reason:         "Reversal - Deposit failed",
		IdempotencyKey: fmt.Sprintf("%s-reverse", reference),
	})

	if err != nil {
		span.RecordError(err)
		playerLogger.Error("Failed to debit player wallet for compensation",
			"error", err.Error(),
			"player_id", playerID,
			"reference", reference,
			"wallet_transaction_id", walletTxID)
		return fmt.Errorf("failed to debit player wallet: %w", err)
	}

	if !walletResp.Success {
		err := fmt.Errorf("player wallet debit failed: %s", walletResp.Message)
		span.RecordError(err)
		playerLogger.Error("Wallet debit compensation returned failure",
			"message", walletResp.Message,
			"player_id", playerID,
			"reference", reference)
		return err
	}

	playerLogger.Info("Wallet debit compensation successful",
		"reversal_transaction_id", walletResp.TransactionId,
		"new_balance", walletResp.NewBalance,
		"player_id", playerID,
		"reference", reference)

	span.SetAttributes(
		attribute.String("reversal_transaction_id", walletResp.TransactionId),
		attribute.Float64("new_balance", walletResp.NewBalance),
		attribute.String("reversed", "true"),
	)

	return nil
}
