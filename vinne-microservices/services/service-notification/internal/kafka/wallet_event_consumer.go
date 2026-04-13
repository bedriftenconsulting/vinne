package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/randco/randco-microservices/services/service-notification/internal/models"
	"github.com/randco/randco-microservices/services/service-notification/internal/services"
	"github.com/randco/randco-microservices/shared/common/logger"
	"github.com/randco/randco-microservices/shared/events"
)

// WalletEventConsumer consumes wallet events and sends notifications to retailers
type WalletEventConsumer struct {
	eventBus                events.EventBus
	pushNotificationService services.PushNotificationService
	logger                  logger.Logger
	stopChan                chan struct{}
}

// WalletCreditedEvent represents a wallet credit event
type WalletCreditedEvent struct {
	RetailerID    string `json:"retailer_id"`
	Amount        int64  `json:"amount"`         // Amount in pesewas
	TransactionID string `json:"transaction_id"` // Transaction ID
	Type          string `json:"type"`           // Type: "stake", "commission", "winning"
	Description   string `json:"description"`
	Timestamp     string `json:"timestamp"`
}

// WalletDebitedEvent represents a wallet debit event
type WalletDebitedEvent struct {
	RetailerID    string `json:"retailer_id"`
	Amount        int64  `json:"amount"`         // Amount in pesewas
	TransactionID string `json:"transaction_id"` // Transaction ID
	Type          string `json:"type"`           // Type: "payout", "refund"
	Description   string `json:"description"`
	Timestamp     string `json:"timestamp"`
}

// WalletLowBalanceEvent represents a low balance alert event
type WalletLowBalanceEvent struct {
	RetailerID     string `json:"retailer_id"`
	CurrentBalance int64  `json:"current_balance"` // Current balance in pesewas
	Threshold      int64  `json:"threshold"`       // Low balance threshold in pesewas
	Timestamp      string `json:"timestamp"`
}

// NewWalletEventConsumer creates a new wallet event consumer
func NewWalletEventConsumer(
	eventBus events.EventBus,
	pushNotificationService services.PushNotificationService,
	logger logger.Logger,
) *WalletEventConsumer {
	return &WalletEventConsumer{
		eventBus:                eventBus,
		pushNotificationService: pushNotificationService,
		logger:                  logger,
		stopChan:                make(chan struct{}),
	}
}

// truncateString safely truncates a string to maxLen characters
// Returns the original string if it's shorter than maxLen
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// Start starts consuming wallet events
func (c *WalletEventConsumer) Start(ctx context.Context) error {
	// Subscribe to wallet events
	topics := []string{
		"wallet.credited",
		"wallet.debited",
		"wallet.low_balance",
	}

	for _, topic := range topics {
		handler := func(handlerCtx context.Context, envelope *events.EventEnvelope) error {
			return c.handleEvent(handlerCtx, envelope)
		}

		if err := c.eventBus.Subscribe(ctx, topic, handler); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", topic, err)
		}
		c.logger.Info("Subscribed to wallet event topic", "topic", topic)
	}

	return nil
}

// Stop stops consuming wallet events
func (c *WalletEventConsumer) Stop(ctx context.Context) error {
	close(c.stopChan)
	c.logger.Info("Wallet event consumer stopped")
	return nil
}

// handleEvent handles incoming wallet events
func (c *WalletEventConsumer) handleEvent(ctx context.Context, envelope *events.EventEnvelope) error {
	eventType := envelope.Topic

	switch eventType {
	case "wallet.credited":
		return c.handleWalletCredited(ctx, envelope)
	case "wallet.debited":
		return c.handleWalletDebited(ctx, envelope)
	case "wallet.low_balance":
		return c.handleWalletLowBalance(ctx, envelope)
	default:
		c.logger.Warn("Unknown wallet event type", "type", eventType)
		return nil
	}
}

// handleWalletCredited handles wallet credited events
func (c *WalletEventConsumer) handleWalletCredited(ctx context.Context, envelope *events.EventEnvelope) error {
	var walletEvent WalletCreditedEvent
	if err := json.Unmarshal(envelope.Payload, &walletEvent); err != nil {
		c.logger.Error("Failed to unmarshal wallet credited event", "error", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	c.logger.Info("Processing wallet credited event",
		"retailer_id", walletEvent.RetailerID,
		"amount", walletEvent.Amount,
		"type", walletEvent.Type,
	)

	// Determine notification type based on credit type
	var notifType models.RetailerNotificationType
	var title, body string

	switch walletEvent.Type {
	case "stake":
		notifType = models.RetailerNotificationTypeStake
		title = "Stake Placed Successfully"
		body = fmt.Sprintf("Your stake of GHS %.2f has been placed. Transaction ID: %s",
			float64(walletEvent.Amount)/100, truncateString(walletEvent.TransactionID, 8))

	case "winning":
		notifType = models.RetailerNotificationTypeWinning
		title = "🎉 Congratulations! You Won!"
		body = fmt.Sprintf("You won GHS %.2f! Amount has been credited to your wallet.",
			float64(walletEvent.Amount)/100)

	case "commission":
		notifType = models.RetailerNotificationTypeCommission
		title = "Commission Earned"
		body = fmt.Sprintf("You earned GHS %.2f in commission. Keep up the great work!",
			float64(walletEvent.Amount)/100)

	default:
		notifType = models.RetailerNotificationTypeGeneral
		title = "Wallet Credited"
		body = fmt.Sprintf("Your wallet has been credited with GHS %.2f",
			float64(walletEvent.Amount)/100)
	}

	// Create notification request
	req := &models.CreateRetailerNotificationRequest{
		RetailerID:    walletEvent.RetailerID,
		Type:          notifType,
		Title:         title,
		Body:          body,
		Amount:        &walletEvent.Amount,
		TransactionID: &walletEvent.TransactionID,
	}

	// Send push notification with history
	if err := c.pushNotificationService.SendPushNotificationWithHistory(ctx, req); err != nil {
		c.logger.Error("Failed to send wallet credited notification",
			"retailer_id", walletEvent.RetailerID,
			"error", err,
		)
		return err
	}

	return nil
}

// handleWalletDebited handles wallet debited events
func (c *WalletEventConsumer) handleWalletDebited(ctx context.Context, envelope *events.EventEnvelope) error {
	var walletEvent WalletDebitedEvent
	if err := json.Unmarshal(envelope.Payload, &walletEvent); err != nil {
		c.logger.Error("Failed to unmarshal wallet debited event", "error", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	c.logger.Info("Processing wallet debited event",
		"retailer_id", walletEvent.RetailerID,
		"amount", walletEvent.Amount,
		"type", walletEvent.Type,
	)

	// Create notification
	title := "Wallet Debited"
	body := fmt.Sprintf("GHS %.2f has been debited from your wallet. Transaction ID: %s",
		float64(walletEvent.Amount)/100, truncateString(walletEvent.TransactionID, 8))

	req := &models.CreateRetailerNotificationRequest{
		RetailerID:    walletEvent.RetailerID,
		Type:          models.RetailerNotificationTypeGeneral,
		Title:         title,
		Body:          body,
		Amount:        &walletEvent.Amount,
		TransactionID: &walletEvent.TransactionID,
	}

	// Send push notification with history
	if err := c.pushNotificationService.SendPushNotificationWithHistory(ctx, req); err != nil {
		c.logger.Error("Failed to send wallet debited notification",
			"retailer_id", walletEvent.RetailerID,
			"error", err,
		)
		return err
	}

	return nil
}

// handleWalletLowBalance handles wallet low balance events
func (c *WalletEventConsumer) handleWalletLowBalance(ctx context.Context, envelope *events.EventEnvelope) error {
	var walletEvent WalletLowBalanceEvent
	if err := json.Unmarshal(envelope.Payload, &walletEvent); err != nil {
		c.logger.Error("Failed to unmarshal wallet low balance event", "error", err)
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	c.logger.Info("Processing wallet low balance event",
		"retailer_id", walletEvent.RetailerID,
		"current_balance", walletEvent.CurrentBalance,
		"threshold", walletEvent.Threshold,
	)

	// Create low balance alert
	title := "⚠️ Low Wallet Balance"
	body := fmt.Sprintf("Your wallet balance is low (GHS %.2f). Please top up to continue placing stakes.",
		float64(walletEvent.CurrentBalance)/100)

	req := &models.CreateRetailerNotificationRequest{
		RetailerID: walletEvent.RetailerID,
		Type:       models.RetailerNotificationTypeLowBalance,
		Title:      title,
		Body:       body,
		Amount:     &walletEvent.CurrentBalance,
	}

	// Send push notification with history
	if err := c.pushNotificationService.SendPushNotificationWithHistory(ctx, req); err != nil {
		c.logger.Error("Failed to send low balance notification",
			"retailer_id", walletEvent.RetailerID,
			"error", err,
		)
		return err
	}

	return nil
}
