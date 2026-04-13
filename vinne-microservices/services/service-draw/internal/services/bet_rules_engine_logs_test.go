package services

import (
	"testing"
)

// TestBetRulesEngine_LogTickets tests all 24 tickets from the production logs
func TestBetRulesEngine_LogTickets(t *testing.T) {
	engine := NewBetRulesEngine()
	winningNumbers := []int32{55, 72, 69, 63, 44}

	tests := []struct {
		ticketNum         int
		betType           string
		selectedNumbers   []int32
		banker            []int32
		opposed           []int32
		totalAmount       int64
		numCombinations   int32
		expectedWinner    bool
		expectedWinAmount int64
		description       string
	}{
		// Ticket 1 - NOW A WINNER with correct PERM logic
		{
			ticketNum:         1,
			betType:           "PERM-2",
			selectedNumbers:   []int32{33, 44, 55, 63, 71},
			totalAmount:       6000,
			numCombinations:   10,
			expectedWinner:    true,
			expectedWinAmount: 600 * 240 * 3, // 3 combos match: [44,55], [44,63], [55,63]
			description:       "PERM-2: 3 winning combos in ALL winning numbers",
		},
		// Ticket 2
		{
			ticketNum:       2,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{44, 69, 72},
			totalAmount:     200,
			expectedWinner:  false,
			description:     "DIRECT-3: Missing 55 from [55,72,69]",
		},
		// Ticket 3
		{
			ticketNum:       3,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{53, 55, 72},
			totalAmount:     500,
			expectedWinner:  false,
			description:     "DIRECT-3: Missing 69 from [55,72,69]",
		},
		// Ticket 4
		{
			ticketNum:       4,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{10, 69, 72},
			totalAmount:     200,
			expectedWinner:  false,
			description:     "DIRECT-3: Missing 55 from [55,72,69]",
		},
		// Ticket 5 - WINNER with 4 matching combinations
		{
			ticketNum:         5,
			betType:           "PERM-3",
			selectedNumbers:   []int32{14, 18, 44, 55, 63, 72},
			totalAmount:       4000,
			numCombinations:   20,
			expectedWinner:    true,
			expectedWinAmount: 200 * 1920 * 4, // 4 combos match: [44,55,63], [44,55,72], [44,63,72], [55,63,72]
			description:       "PERM-3: 4 winning combos in ALL winning numbers",
		},
		// Ticket 6 - BANKER-AG winner
		{
			ticketNum:         6,
			betType:           "BANKER-AG",
			banker:            []int32{7, 21, 29, 55, 85},
			opposed:           []int32{3, 32, 38, 42, 45, 72},
			totalAmount:       30000,
			numCombinations:   30,
			expectedWinner:    true,
			expectedWinAmount: 1000 * 240 * 1, // 1 winning pair: (55,72)
			description:       "BANKER-AG: Winning pair (55,72)",
		},
		// Ticket 7 - BANKER-AG winner
		{
			ticketNum:         7,
			betType:           "BANKER-AG",
			banker:            []int32{23, 37, 51, 55},
			opposed:           []int32{2, 9, 17, 44, 71},
			totalAmount:       10000,
			numCombinations:   20,
			expectedWinner:    true,
			expectedWinAmount: 500 * 240 * 1, // 1 winning pair: (55,44)
			description:       "BANKER-AG: Winning pair (55,44)",
		},
		// Ticket 8 - BANKER-ALL loser
		{
			ticketNum:       8,
			betType:         "BANKER-ALL",
			banker:          []int32{40},
			totalAmount:     17800,
			numCombinations: 89,
			expectedWinner:  false,
			description:     "BANKER-ALL: Banker 40 not in winning numbers",
		},
		// Ticket 9 - BANKER-ALL loser
		{
			ticketNum:       9,
			betType:         "BANKER-ALL",
			banker:          []int32{20},
			totalAmount:     8900,
			numCombinations: 89,
			expectedWinner:  false,
			description:     "BANKER-ALL: Banker 20 not in winning numbers",
		},
		// Ticket 10 - BANKER-ALL winner
		{
			ticketNum:         10,
			betType:           "BANKER-ALL",
			banker:            []int32{69},
			totalAmount:       17800,
			numCombinations:   89,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 4, // 4 pairs: (69,55), (69,72), (69,63), (69,44)
			description:       "BANKER-ALL: Banker 69 pairs with 4 other winning numbers",
		},
		// Ticket 11
		{
			ticketNum:       11,
			betType:         "PERM-3",
			selectedNumbers: []int32{3, 16, 21, 57},
			totalAmount:     800,
			numCombinations: 4,
			expectedWinner:  false,
			description:     "PERM-3: None in pool [55,72,69,63]",
		},
		// Ticket 12
		{
			ticketNum:       12,
			betType:         "PERM-3",
			selectedNumbers: []int32{9, 12, 37, 54, 68, 72},
			totalAmount:     2000,
			numCombinations: 20,
			expectedWinner:  false,
			description:     "PERM-3: Only 72 in pool, need 3",
		},
		// Ticket 13
		{
			ticketNum:       13,
			betType:         "PERM-3",
			selectedNumbers: []int32{6, 11, 40, 51, 69, 81},
			totalAmount:     4000,
			numCombinations: 20,
			expectedWinner:  false,
			description:     "PERM-3: Only 69 in pool, need 3",
		},
		// Ticket 14 (TKT-40791831) - NOW A WINNER with correct PERM logic
		{
			ticketNum:         14,
			betType:           "PERM-2",
			selectedNumbers:   []int32{2, 28, 31, 55, 63, 88},
			totalAmount:       3000,
			numCombinations:   15,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 1, // 1 combo matches: [55,63]
			description:       "PERM-2: [55,63] matches in ALL winning numbers",
		},
		// Ticket 15
		{
			ticketNum:       15,
			betType:         "PERM-2",
			selectedNumbers: []int32{3, 5, 31, 39, 65},
			totalAmount:     5000,
			numCombinations: 10,
			expectedWinner:  false,
			description:     "PERM-2: None in pool [55,72,69]",
		},
		// Ticket 16
		{
			ticketNum:       16,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{20, 21, 51},
			totalAmount:     200,
			expectedWinner:  false,
			description:     "DIRECT-3: None match [55,72,69]",
		},
		// Ticket 17
		{
			ticketNum:       17,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{1, 29, 44},
			totalAmount:     500,
			expectedWinner:  false,
			description:     "DIRECT-3: None match [55,72,69]",
		},
		// Ticket 18
		{
			ticketNum:       18,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{4, 49, 83},
			totalAmount:     200,
			expectedWinner:  false,
			description:     "DIRECT-3: None match [55,72,69]",
		},
		// Ticket 19
		{
			ticketNum:       19,
			betType:         "DIRECT-2",
			selectedNumbers: []int32{5, 21},
			totalAmount:     100,
			expectedWinner:  false,
			description:     "DIRECT-2: None match [55,72]",
		},
		// Ticket 20
		{
			ticketNum:       20,
			betType:         "DIRECT-2",
			selectedNumbers: []int32{39, 89},
			totalAmount:     500,
			expectedWinner:  false,
			description:     "DIRECT-2: None match [55,72]",
		},
		// Ticket 21
		{
			ticketNum:       21,
			betType:         "DIRECT-2",
			selectedNumbers: []int32{19, 22},
			totalAmount:     200,
			expectedWinner:  false,
			description:     "DIRECT-2: None match [55,72]",
		},
		// Ticket 22
		{
			ticketNum:       22,
			betType:         "DIRECT-1",
			selectedNumbers: []int32{69},
			totalAmount:     200,
			expectedWinner:  false,
			description:     "DIRECT-1: Position 0 is 55, not 69",
		},
		// Ticket 23
		{
			ticketNum:       23,
			betType:         "DIRECT-1",
			selectedNumbers: []int32{53},
			totalAmount:     500,
			expectedWinner:  false,
			description:     "DIRECT-1: Position 0 is 55, not 53",
		},
		// Ticket 24
		{
			ticketNum:       24,
			betType:         "DIRECT-1",
			selectedNumbers: []int32{25},
			totalAmount:     200,
			expectedWinner:  false,
			description:     "DIRECT-1: Position 0 is 55, not 25",
		},
	}

	var totalWinnings int64
	winningCount := 0

	t.Logf("\n=== TESTING ALL 24 TICKETS FROM LOGS ===")
	t.Logf("Winning Numbers: %v\n", winningNumbers)

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			betLine := &BetLine{
				BetType:              tt.betType,
				SelectedNumbers:      tt.selectedNumbers,
				Banker:               tt.banker,
				Opposed:              tt.opposed,
				TotalAmount:          tt.totalAmount,
				NumberOfCombinations: tt.numCombinations,
			}

			result, err := engine.CheckBetLine(betLine, winningNumbers)
			if err != nil {
				t.Fatalf("Ticket %d: unexpected error: %v", tt.ticketNum, err)
			}

			// Check winner status
			if result.IsWinner != tt.expectedWinner {
				t.Errorf("Ticket %d (%s): expected IsWinner=%v, got %v",
					tt.ticketNum, tt.description, tt.expectedWinner, result.IsWinner)
			}

			// Check winning amount
			if tt.expectedWinner && result.WinningAmount != tt.expectedWinAmount {
				t.Errorf("Ticket %d (%s): expected WinningAmount=%d (%.2f GHS), got %d (%.2f GHS)",
					tt.ticketNum, tt.description,
					tt.expectedWinAmount, float64(tt.expectedWinAmount)/100.0,
					result.WinningAmount, float64(result.WinningAmount)/100.0)
			}

			// Track statistics
			if result.IsWinner {
				winningCount++
				totalWinnings += result.WinningAmount
				t.Logf("✅ Ticket %d WINS: %d pesewas (%.2f GHS) - %s",
					tt.ticketNum, result.WinningAmount, float64(result.WinningAmount)/100.0, tt.description)
			} else {
				t.Logf("❌ Ticket %d LOSES - %s", tt.ticketNum, tt.description)
			}
		})
	}

	// Summary
	t.Logf("\n=== SUMMARY ===")
	t.Logf("Total tickets: 24")
	t.Logf("Winning tickets: %d", winningCount)
	t.Logf("Total winnings: %d pesewas (%.2f GHS)", totalWinnings, float64(totalWinnings)/100.0)
}
