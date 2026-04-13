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

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// DepositSaga orchestrates the deposit flow: Mobile Money → Retailer Stake Wallet
// Steps:
// 1. Debit mobile money wallet (Provider)
// 2. Credit retailer stake wallet (Wallet Service)
type DepositSaga struct {
	orchestrator    *Orchestrator
	provider        providers.PaymentProvider
	walletClient    walletv1.WalletServiceClient
	transactionRepo TransactionRepository
}

// TransactionRepository defines the minimal interface needed by saga for transaction updates
type TransactionRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Transaction, error)
	Update(ctx context.Context, tx *models.Transaction) error
}

func NewDepositSaga(orchestrator *Orchestrator, provider providers.PaymentProvider, walletClient walletv1.WalletServiceClient, transactionRepo TransactionRepository) *DepositSaga {
	return &DepositSaga{
		orchestrator:    orchestrator,
		provider:        provider,
		walletClient:    walletClient,
		transactionRepo: transactionRepo,
	}
}

func (ds *DepositSaga) Execute(ctx context.Context, transaction *models.Transaction) error {
	ctx, span := tracer.Start(ctx, "deposit_saga.execute",
		trace.WithAttributes(
			attribute.String("transaction_id", transaction.ID.String()),
			attribute.String("reference", transaction.Reference),
			attribute.Int64("amount", transaction.Amount),
			attribute.String("status", string(transaction.Status)),
		))
	defer span.End()

	skipDebitStep := transaction.ProviderTransactionID != nil && *transaction.ProviderTransactionID != "" && transaction.Status == models.StatusSuccess

	providerTxID := ""
	if transaction.ProviderTransactionID != nil {
		providerTxID = *transaction.ProviderTransactionID
	}

	logger.Info("Deposit saga execution started",
		"transaction_id", transaction.ID.String(),
		"reference", transaction.Reference,
		"status", string(transaction.Status),
		"provider_transaction_id", providerTxID,
		"skip_debit_step", skipDebitStep)

	sagaID := fmt.Sprintf("deposit-%s", transaction.Reference)
	saga, err := ds.orchestrator.sagaRepo.GetBySagaID(ctx, sagaID)

	// Idempotency check: If saga already exists and is COMPLETED, skip execution
	// This prevents duplicate webhooks or retries from crediting wallet multiple times
	if err == nil && saga.Status == models.SagaStatusCompleted {
		logger.Info("Saga already completed, skipping execution",
			"transaction_id", transaction.ID.String(),
			"reference", transaction.Reference,
			"saga_status", string(saga.Status))
		span.AddEvent("saga_skipped_already_completed", trace.WithAttributes(
			attribute.String("saga_status", string(saga.Status)),
		))
		return nil // Idempotent - safe to call multiple times
	}
	if err != nil {
		saga = models.CreateSaga(sagaID, transaction.ID, 2, 3)

		saga.SagaData["transaction_id"] = transaction.ID.String()
		saga.SagaData["reference"] = transaction.Reference
		saga.SagaData["user_id"] = transaction.UserID.String()
		saga.SagaData["wallet_number"] = transaction.SourceIdentifier
		saga.SagaData["wallet_provider"] = transaction.SourceName
		saga.SagaData["amount"] = transaction.Amount
		saga.SagaData["currency"] = transaction.Currency
		saga.SagaData["narration"] = transaction.Narration
		saga.SagaData["customer_name"] = transaction.DestinationName

		if err := ds.orchestrator.sagaRepo.Create(ctx, saga); err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to create saga: %w", err)
		}
	}

	if skipDebitStep {
		saga.SagaData["provider_transaction_id"] = *transaction.ProviderTransactionID
		saga.SagaData["provider_status"] = string(transaction.Status)
		if transaction.CompletedAt != nil {
			saga.SagaData["debit_completed_at"] = transaction.CompletedAt
		}
	}

	var steps []Step

	if skipDebitStep {
		logger.Info("Skipping debit step - payment already confirmed",
			"transaction_id", transaction.ID.String(),
			"reference", transaction.Reference)

		steps = []Step{
			{
				Name:        "CreditStakeWallet",
				Description: "Credit customer's stake wallet",
				Execute:     ds.creditStakeWallet,
				Compensate:  ds.debitStakeWallet,
			},
		}
	} else {
		steps = []Step{
			{
				Name:        "DebitMobileMoneyWallet",
				Description: "Debit customer's mobile money wallet",
				Execute:     ds.debitMobileMoneyWallet,
				Compensate:  ds.creditMobileMoneyWallet,
			},
			{
				Name:        "CreditStakeWallet",
				Description: "Credit customer's stake wallet",
				Execute:     ds.creditStakeWallet,
				Compensate:  ds.debitStakeWallet,
			},
		}
	}

	execErr := ds.orchestrator.Execute(ctx, saga, steps)
	if execErr != nil {
		span.RecordError(execErr)

		// Refresh transaction to get latest provider data
		freshTxn, getErr := ds.transactionRepo.GetByID(ctx, transaction.ID)
		if getErr != nil {
			logger.Error("Failed to refresh transaction before status update",
				"error", getErr.Error(),
				"transaction_id", transaction.ID.String())
			freshTxn = transaction // Fallback to original if refresh fails
		}

		// Update transaction status to FAILED
		freshTxn.Status = models.StatusFailed
		freshTxn.UpdatedAt = time.Now()
		if updateErr := ds.transactionRepo.Update(ctx, freshTxn); updateErr != nil {
			logger.Error("Failed to update transaction status to FAILED",
				"error", updateErr.Error(),
				"transaction_id", transaction.ID.String(),
				"reference", transaction.Reference)
		}

		return fmt.Errorf("deposit saga failed: %w", execErr)
	}

	// Refresh transaction to get latest provider data
	freshTxn, getErr := ds.transactionRepo.GetByID(ctx, transaction.ID)
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
	if updateErr := ds.transactionRepo.Update(ctx, freshTxn); updateErr != nil {
		logger.Error("Failed to update transaction status to SUCCESS",
			"error", updateErr.Error(),
			"transaction_id", transaction.ID.String(),
			"reference", transaction.Reference)
		return fmt.Errorf("failed to update transaction status: %w", updateErr)
	}

	return nil
}

func (ds *DepositSaga) debitMobileMoneyWallet(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "deposit_saga.debit_mobile_money_wallet")
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

	req := &providers.DebitWalletRequest{
		WalletNumber:   walletNumber,
		WalletProvider: walletProvider,
		Amount:         float64(amount),
		Currency:       currency,
		Narration:      narration,
		Reference:      reference,
		CustomerName:   data["customer_name"].(string),
	}

	txnID := uuid.MustParse(data["transaction_id"].(string))
	txn, getErr := ds.transactionRepo.GetByID(ctx, txnID)
	if getErr != nil {
		logger.Error("Failed to get transaction before provider call",
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
		if updateErr := ds.transactionRepo.Update(ctx, txn); updateErr != nil {
			logger.Error("Failed to save provider request (continuing)",
				"error", updateErr.Error(),
				"transaction_id", txnID.String(),
				"reference", reference)
		}
	}

	resp, err := ds.provider.DebitWallet(ctx, req)

	// CRITICAL: Save provider response OR error immediately to transaction record
	// This ensures we have a trace even if saga fails later OR if API call errors
	if txn != nil {
		if err != nil {
			txn.ProviderData["debit_error"] = map[string]interface{}{
				"error":     err.Error(),
				"timestamp": time.Now(),
			}
			if updateErr := ds.transactionRepo.Update(ctx, txn); updateErr != nil {
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
			txn.ProviderData["debit_response"] = map[string]interface{}{
				"success":        resp.Success,
				"status":         string(resp.Status),
				"message":        resp.Message,
				"transaction_id": resp.TransactionID,
				"timestamp":      time.Now(),
			}
			if updateErr := ds.transactionRepo.Update(ctx, txn); updateErr != nil {
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
		logger.Error("Provider DebitWallet call failed",
			"error", err.Error(),
			"wallet_number", walletNumber,
			"wallet_provider", walletProvider,
			"amount", amount,
			"reference", reference,
		)
		return nil, fmt.Errorf("failed to debit mobile money wallet: %w", err)
	}

	logger.Info("Provider DebitWallet response received",
		"success", resp.Success,
		"status", string(resp.Status),
		"message", resp.Message,
		"transaction_id", resp.TransactionID,
		"wallet_number", walletNumber,
		"reference", reference)

	// Validate provider response: Check both Success flag AND Status field
	// Bug fix: Provider can return mixed signals (success=true but status=FAILED)
	if !resp.Success || resp.Status == providers.StatusFailed {
		err := fmt.Errorf("mobile money debit failed: success=%v status=%s message=%s",
			resp.Success, string(resp.Status), resp.Message)
		span.RecordError(err)
		logger.Error("Provider DebitWallet returned unsuccessful response",
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
		logger.Info("Mobile money debit PENDING - stopping saga, waiting for webhook confirmation",
			"transaction_id", resp.TransactionID,
			"wallet_number", walletNumber,
			"reference", reference,
			"status", string(resp.Status),
			"message", resp.Message)

		span.AddEvent("saga_paused_pending_webhook", trace.WithAttributes(
			attribute.String("provider_transaction_id", resp.TransactionID),
			attribute.String("status", string(resp.Status)),
		))

		return nil, fmt.Errorf("saga_pending_confirmation: mobile money debit pending confirmation from provider - transaction will resume via webhook")
	}

	logger.Info("Mobile money debit SUCCESS",
		"transaction_id", resp.TransactionID,
		"wallet_number", walletNumber,
		"reference", reference)

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

func (ds *DepositSaga) creditStakeWallet(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := tracer.Start(ctx, "deposit_saga.credit_stake_wallet")
	defer span.End()

	retailerID := data["user_id"].(string)
	amount := data["amount"].(int64)
	reference := data["reference"].(string)

	span.SetAttributes(
		attribute.String("retailer_id", retailerID),
		attribute.Int64("amount", amount),
	)

	walletResp, err := ds.walletClient.CreditRetailerWallet(ctx, &walletv1.CreditRetailerWalletRequest{
		RetailerId:     retailerID,
		WalletType:     walletv1.WalletType_RETAILER_STAKE,
		Amount:         float64(amount),
		Reference:      reference,
		Notes:          "Deposit from mobile money",
		IdempotencyKey: reference,
		CreditSource:   walletv1.CreditSource_CREDIT_SOURCE_MOBILE_MONEY, // Mark as mobile money deposit
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to credit stake wallet: %w", err)
	}

	if !walletResp.Success {
		err := fmt.Errorf("stake wallet credit failed: %s", walletResp.Message)
		span.RecordError(err)
		return nil, err
	}

	output := map[string]interface{}{
		"stake_wallet_transaction_id": walletResp.TransactionId,
		"stake_wallet_balance":        walletResp.NewBalance,
		"gross_amount":                walletResp.GrossAmount,
		"base_amount":                 walletResp.BaseAmount,
	}

	span.SetAttributes(
		attribute.String("stake_wallet_transaction_id", walletResp.TransactionId),
		attribute.Float64("new_balance", walletResp.NewBalance),
	)

	return output, nil
}

func (ds *DepositSaga) creditMobileMoneyWallet(ctx context.Context, data map[string]interface{}) error {
	ctx, span := tracer.Start(ctx, "deposit_saga.credit_mobile_money_wallet_compensation")
	defer span.End()

	reference := data["reference"].(string)

	providerTxID, exists := data["provider_transaction_id"]
	if !exists || providerTxID == nil || providerTxID == "" {
		logger.Info("Skipping mobile money refund - debit never succeeded",
			"reference", reference,
			"reason", "provider_transaction_id not found in saga data")
		span.AddEvent("compensation_skipped", trace.WithAttributes(
			attribute.String("reason", "debit_never_succeeded"),
		))
		return nil
	}

	logger.Info("Executing mobile money refund compensation",
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

	resp, err := ds.provider.CreditWallet(ctx, req)
	if err != nil {
		span.RecordError(err)
		logger.Error("Failed to refund mobile money wallet",
			"error", err.Error(),
			"reference", reference,
			"provider_transaction_id", providerTxID)
		return fmt.Errorf("failed to refund mobile money wallet: %w", err)
	}

	if !resp.Success {
		err := fmt.Errorf("mobile money refund failed: %s", resp.Message)
		span.RecordError(err)
		logger.Error("Mobile money refund returned failure",
			"message", resp.Message,
			"reference", reference)
		return err
	}

	logger.Info("Mobile money refund compensation successful",
		"refund_transaction_id", resp.TransactionID,
		"reference", reference,
		"original_transaction_id", providerTxID)

	span.SetAttributes(attribute.String("refund_transaction_id", resp.TransactionID))

	return nil
}

func (ds *DepositSaga) debitStakeWallet(ctx context.Context, data map[string]interface{}) error {
	ctx, span := tracer.Start(ctx, "deposit_saga.debit_stake_wallet_compensation")
	defer span.End()

	retailerID := data["user_id"].(string)
	amount := data["amount"].(int64)
	reference := data["reference"].(string)

	walletTxID, exists := data["stake_wallet_transaction_id"]
	if !exists || walletTxID == nil || walletTxID == "" {
		logger.Info("Skipping stake wallet debit compensation - credit never succeeded",
			"retailer_id", retailerID,
			"reference", reference,
			"reason", "stake_wallet_transaction_id not found in saga data")
		span.AddEvent("compensation_skipped", trace.WithAttributes(
			attribute.String("reason", "credit_never_succeeded"),
		))
		return nil
	}

	logger.Info("Executing stake wallet debit compensation",
		"retailer_id", retailerID,
		"reference", reference,
		"wallet_transaction_id", walletTxID,
		"amount", amount)

	span.SetAttributes(
		attribute.String("retailer_id", retailerID),
		attribute.Int64("amount", amount),
		attribute.String("compensation", "reverse_credit"),
	)

	walletResp, err := ds.walletClient.DebitRetailerWallet(ctx, &walletv1.DebitRetailerWalletRequest{
		RetailerId:     retailerID,
		WalletType:     walletv1.WalletType_RETAILER_STAKE,
		Amount:         float64(amount),
		Reference:      fmt.Sprintf("%s-reverse", reference),
		Reason:         "Reversal - Deposit failed",
		IdempotencyKey: fmt.Sprintf("%s-reverse", reference),
	})

	if err != nil {
		span.RecordError(err)
		logger.Error("Failed to debit stake wallet for compensation",
			"error", err.Error(),
			"retailer_id", retailerID,
			"reference", reference,
			"wallet_transaction_id", walletTxID)
		return fmt.Errorf("failed to debit stake wallet: %w", err)
	}

	if !walletResp.Success {
		err := fmt.Errorf("stake wallet debit failed: %s", walletResp.Message)
		span.RecordError(err)
		logger.Error("Stake wallet debit compensation returned failure",
			"message", walletResp.Message,
			"retailer_id", retailerID,
			"reference", reference)
		return err
	}

	logger.Info("Stake wallet debit compensation successful",
		"reversal_transaction_id", walletResp.TransactionId,
		"new_balance", walletResp.NewBalance,
		"retailer_id", retailerID,
		"reference", reference)

	span.SetAttributes(
		attribute.String("reversal_transaction_id", walletResp.TransactionId),
		attribute.Float64("new_balance", walletResp.NewBalance),
		attribute.String("reversed", "true"),
	)

	return nil
}
