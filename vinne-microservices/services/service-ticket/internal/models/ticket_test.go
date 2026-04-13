package models

import (
	"testing"
)

func TestTicketStatusTransitions(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus string
		targetStatus  TicketStatus
		shouldAllowed bool
	}{
		// From issued status - these should now work with our fix
		{
			name:          "issued to won - should be allowed after fix",
			currentStatus: string(TicketStatusIssued),
			targetStatus:  TicketStatusWon,
			shouldAllowed: true,
		},
		{
			name:          "issued to lost - should be allowed after fix",
			currentStatus: string(TicketStatusIssued),
			targetStatus:  TicketStatusLost,
			shouldAllowed: true,
		},
		{
			name:          "issued to validated - should still be allowed",
			currentStatus: string(TicketStatusIssued),
			targetStatus:  TicketStatusValidated,
			shouldAllowed: true,
		},
		{
			name:          "issued to cancelled - should be allowed",
			currentStatus: string(TicketStatusIssued),
			targetStatus:  TicketStatusCancelled,
			shouldAllowed: true,
		},
		{
			name:          "issued to paid - should NOT be allowed",
			currentStatus: string(TicketStatusIssued),
			targetStatus:  TicketStatusPaid,
			shouldAllowed: false,
		},
		// From validated status
		{
			name:          "validated to won - should be allowed",
			currentStatus: string(TicketStatusValidated),
			targetStatus:  TicketStatusWon,
			shouldAllowed: true,
		},
		{
			name:          "validated to lost - should be allowed after fix",
			currentStatus: string(TicketStatusValidated),
			targetStatus:  TicketStatusLost,
			shouldAllowed: true,
		},
		// From won status
		{
			name:          "won to paid - should be allowed",
			currentStatus: string(TicketStatusWon),
			targetStatus:  TicketStatusPaid,
			shouldAllowed: true,
		},
		{
			name:          "won to lost - should NOT be allowed",
			currentStatus: string(TicketStatusWon),
			targetStatus:  TicketStatusLost,
			shouldAllowed: false,
		},
		// From lost status (new case we added)
		{
			name:          "lost to void - should be allowed",
			currentStatus: string(TicketStatusLost),
			targetStatus:  TicketStatusVoid,
			shouldAllowed: true,
		},
		{
			name:          "lost to won - should NOT be allowed",
			currentStatus: string(TicketStatusLost),
			targetStatus:  TicketStatusWon,
			shouldAllowed: false,
		},
		{
			name:          "lost to paid - should NOT be allowed",
			currentStatus: string(TicketStatusLost),
			targetStatus:  TicketStatusPaid,
			shouldAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &Ticket{
				Status: tt.currentStatus,
			}

			canTransition := ticket.CanTransitionTo(tt.targetStatus)

			if canTransition != tt.shouldAllowed {
				t.Errorf("CanTransitionTo(%s -> %s) = %v, want %v",
					tt.currentStatus, tt.targetStatus, canTransition, tt.shouldAllowed)
			}
		})
	}
}

// TestDrawProcessingStatusUpdate simulates the exact scenario from draw processing
func TestDrawProcessingStatusUpdate(t *testing.T) {
	// Simulate a ticket that was just issued for a draw
	ticket := &Ticket{
		Status: string(TicketStatusIssued),
	}

	// Test winning scenario - draw service tries to mark ticket as won
	if !ticket.CanTransitionTo(TicketStatusWon) {
		t.Error("Ticket should be able to transition from 'issued' to 'won' during draw processing")
	}

	// Test losing scenario - draw service tries to mark ticket as lost
	ticket2 := &Ticket{
		Status: string(TicketStatusIssued),
	}

	if !ticket2.CanTransitionTo(TicketStatusLost) {
		t.Error("Ticket should be able to transition from 'issued' to 'lost' during draw processing")
	}
}
