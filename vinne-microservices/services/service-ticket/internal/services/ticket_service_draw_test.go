package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/randco/randco-microservices/services/service-ticket/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateTicketStatusesForDrawProcessing tests the exact scenario that happens during draw processing
func TestUpdateTicketStatusesForDrawProcessing(t *testing.T) {
	// This test simulates what the draw service does when processing draw results
	t.Run("should update winning and losing tickets from issued status", func(t *testing.T) {
		// Create test tickets in "issued" status (as they would be before a draw)
		winningTicketID := uuid.New()
		losingTicketID := uuid.New()
		drawID := uuid.New()

		// Create mock update request as the draw service would send
		req := &UpdateTicketStatusesRequest{
			Updates: []TicketStatusUpdate{
				{
					TicketID:      winningTicketID.String(),
					Status:        "won",
					WinningAmount: 500000, // 5000 GHS in pesewas
					Matches:       3,
					PrizeTier:     "Direct 3",
				},
				{
					TicketID:      losingTicketID.String(),
					Status:        "lost",
					WinningAmount: 0,
					Matches:       0,
					PrizeTier:     "",
				},
			},
			DrawID: drawID.String(),
		}

		// Verify that the status updates would work with our fix
		// In the real implementation, this would go through the database
		// but we can test the business logic here

		// Test winning ticket transition
		winningTicket := &models.Ticket{
			ID:     winningTicketID,
			Status: string(models.TicketStatusIssued),
		}

		// This should now succeed with our fix
		canUpdateToWon := winningTicket.CanTransitionTo(models.TicketStatusWon)
		assert.True(t, canUpdateToWon, "Ticket should be able to transition from 'issued' to 'won'")

		// Test losing ticket transition
		losingTicket := &models.Ticket{
			ID:     losingTicketID,
			Status: string(models.TicketStatusIssued),
		}

		// This should now succeed with our fix
		canUpdateToLost := losingTicket.CanTransitionTo(models.TicketStatusLost)
		assert.True(t, canUpdateToLost, "Ticket should be able to transition from 'issued' to 'lost'")

		// Verify the request structure is valid
		require.Equal(t, 2, len(req.Updates))
		require.Equal(t, "won", req.Updates[0].Status)
		require.Equal(t, "lost", req.Updates[1].Status)
		require.Equal(t, int64(500000), req.Updates[0].WinningAmount)
		require.Equal(t, int64(0), req.Updates[1].WinningAmount)
	})
}

// TestTicketStatusFlowDuringDraw tests the complete flow of ticket status changes during a draw
func TestTicketStatusFlowDuringDraw(t *testing.T) {
	ctx := context.Background()

	t.Run("complete draw processing flow", func(t *testing.T) {
		// Stage 1: Ticket is issued
		ticket := &models.Ticket{
			ID:     uuid.New(),
			Status: string(models.TicketStatusIssued),
		}

		// Stage 2: Draw happens, numbers are selected and verified
		// Stage 3: Results are calculated - ticket is identified as winner or loser

		// Scenario A: Winning ticket
		if ticket.Status == string(models.TicketStatusIssued) {
			// Draw service would call UpdateTicketStatuses with status="won"
			canWin := ticket.CanTransitionTo(models.TicketStatusWon)
			assert.True(t, canWin, "Should be able to mark issued ticket as won")

			if canWin {
				err := ticket.SetStatus(models.TicketStatusWon)
				assert.NoError(t, err, "Should successfully set status to won")
				assert.Equal(t, string(models.TicketStatusWon), ticket.Status)
			}
		}

		// Stage 4: Payout processing
		if ticket.Status == string(models.TicketStatusWon) {
			// Payout service would mark ticket as paid
			canPay := ticket.CanTransitionTo(models.TicketStatusPaid)
			assert.True(t, canPay, "Should be able to mark won ticket as paid")

			if canPay {
				err := ticket.SetStatus(models.TicketStatusPaid)
				assert.NoError(t, err, "Should successfully set status to paid")
				assert.Equal(t, string(models.TicketStatusPaid), ticket.Status)
			}
		}

		// Scenario B: Losing ticket
		ticket2 := &models.Ticket{
			ID:     uuid.New(),
			Status: string(models.TicketStatusIssued),
		}

		if ticket2.Status == string(models.TicketStatusIssued) {
			// Draw service would call UpdateTicketStatuses with status="lost"
			canLose := ticket2.CanTransitionTo(models.TicketStatusLost)
			assert.True(t, canLose, "Should be able to mark issued ticket as lost")

			if canLose {
				err := ticket2.SetStatus(models.TicketStatusLost)
				assert.NoError(t, err, "Should successfully set status to lost")
				assert.Equal(t, string(models.TicketStatusLost), ticket2.Status)
			}
		}

		// Lost tickets are terminal (cannot be paid)
		if ticket2.Status == string(models.TicketStatusLost) {
			cannotPay := ticket2.CanTransitionTo(models.TicketStatusPaid)
			assert.False(t, cannotPay, "Should NOT be able to mark lost ticket as paid")
		}
	})

	_ = ctx // silence unused variable warning
}
