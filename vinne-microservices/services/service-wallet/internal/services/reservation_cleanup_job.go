package services

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/randco/service-wallet/internal/models"
	"github.com/randco/service-wallet/internal/repositories"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var jobLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
var tracer = otel.Tracer("wallet-service")

// ReservationCleanupJob handles automatic release of expired wallet reservations
// This prevents funds from being locked permanently if a saga crashes before compensation runs
type ReservationCleanupJob struct {
	reservationRepo repositories.ReservationRepository
	walletRepo      repositories.WalletRepository
	interval        time.Duration
	stopChan        chan struct{}
}

// NewReservationCleanupJob creates a new reservation cleanup background job
func NewReservationCleanupJob(
	reservationRepo repositories.ReservationRepository,
	walletRepo repositories.WalletRepository,
	intervalMinutes int,
) *ReservationCleanupJob {
	if intervalMinutes <= 0 {
		intervalMinutes = 1 // Default: run every minute
	}

	return &ReservationCleanupJob{
		reservationRepo: reservationRepo,
		walletRepo:      walletRepo,
		interval:        time.Duration(intervalMinutes) * time.Minute,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the background job
func (j *ReservationCleanupJob) Start(ctx context.Context) {
	jobLogger.Info("Starting reservation cleanup job",
		"interval_minutes", int(j.interval.Minutes()))

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Run immediately on startup
	j.runCleanup(ctx)

	for {
		select {
		case <-ticker.C:
			j.runCleanup(ctx)
		case <-j.stopChan:
			jobLogger.Info("Stopping reservation cleanup job")
			return
		case <-ctx.Done():
			jobLogger.Info("Context cancelled, stopping reservation cleanup job")
			return
		}
	}
}

// Stop stops the background job
func (j *ReservationCleanupJob) Stop() {
	close(j.stopChan)
}

// runCleanup executes a single cleanup cycle
func (j *ReservationCleanupJob) runCleanup(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "reservation_cleanup_job.run_cleanup")
	defer span.End()

	startTime := time.Now()

	jobLogger.Info("Starting reservation cleanup cycle")

	// Get all expired but still ACTIVE reservations
	expiredReservations, err := j.reservationRepo.GetExpiredReservations(ctx)
	if err != nil {
		span.RecordError(err)
		jobLogger.Error("Failed to get expired reservations",
			"error", err.Error())
		return
	}

	if len(expiredReservations) == 0 {
		jobLogger.Info("No expired reservations to process")
		return
	}

	jobLogger.Info("Found expired reservations to release",
		"count", len(expiredReservations))

	span.SetAttributes(attribute.Int("expired_count", len(expiredReservations)))

	releasedCount := 0
	failedCount := 0

	for _, reservation := range expiredReservations {
		// Get the wallet to extract wallet ID
		walletInterface, getErr := j.walletRepo.GetByOwnerAndType(ctx, reservation.WalletOwnerID, reservation.WalletType)
		if getErr != nil {
			jobLogger.Error("Failed to get wallet for expired reservation",
				"error", getErr.Error(),
				"reservation_id", reservation.ReservationID,
				"wallet_owner_id", reservation.WalletOwnerID.String(),
				"wallet_type", string(reservation.WalletType))
			failedCount++
			continue
		}

		// Extract wallet ID based on wallet type
		var walletID uuid.UUID
		switch reservation.WalletType {
		case models.WalletTypeAgentStake:
			if wallet, ok := walletInterface.(*models.AgentStakeWallet); ok {
				walletID = wallet.ID
			}
		case models.WalletTypeRetailerStake:
			if wallet, ok := walletInterface.(*models.RetailerStakeWallet); ok {
				walletID = wallet.ID
			}
		case models.WalletTypeRetailerWinning:
			if wallet, ok := walletInterface.(*models.RetailerWinningWallet); ok {
				walletID = wallet.ID
			}
		case models.WalletTypePlayerWallet:
			if wallet, ok := walletInterface.(*models.PlayerWallet); ok {
				walletID = wallet.ID
			}
		}

		if walletID == uuid.Nil {
			jobLogger.Error("Failed to extract wallet ID from wallet interface",
				"reservation_id", reservation.ReservationID,
				"wallet_type", string(reservation.WalletType))
			failedCount++
			continue
		}

		// Release the reservation in the wallet (restore funds to available balance)
		releaseErr := j.walletRepo.ReleaseReservation(ctx, walletID, reservation.WalletType, reservation.ReservationID, reservation.Amount)
		if releaseErr != nil {
			jobLogger.Error("Failed to release funds for expired reservation",
				"error", releaseErr.Error(),
				"reservation_id", reservation.ReservationID,
				"wallet_id", walletID.String(),
				"amount", reservation.Amount)
			failedCount++
			continue
		}

		// Mark reservation as released in the database
		markErr := j.reservationRepo.MarkAsReleased(ctx, reservation.ReservationID)
		if markErr != nil {
			jobLogger.Error("Failed to mark reservation as released",
				"error", markErr.Error(),
				"reservation_id", reservation.ReservationID)
			// Note: Funds were already released, so this is just a status update failure
			// We still count it as released since the critical operation succeeded
		}

		releasedCount++

		jobLogger.Info("Successfully released expired reservation",
			"reservation_id", reservation.ReservationID,
			"wallet_owner_id", reservation.WalletOwnerID.String(),
			"wallet_type", string(reservation.WalletType),
			"amount_pesewas", reservation.Amount,
			"amount_ghs", float64(reservation.Amount)/100.0,
			"reference", reservation.Reference,
			"expired_at", reservation.ExpiresAt.Format(time.RFC3339))
	}

	duration := time.Since(startTime)

	jobLogger.Info("Reservation cleanup cycle completed",
		"duration_ms", duration.Milliseconds(),
		"expired_count", len(expiredReservations),
		"released_count", releasedCount,
		"failed_count", failedCount)

	span.SetAttributes(
		attribute.Int("released_count", releasedCount),
		attribute.Int("failed_count", failedCount),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)
}
