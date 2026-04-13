package services

import (
	"testing"
)

// TestBetRulesEngine_UserProvidedTickets tests tickets provided by the user
// Winning Numbers: 13, 61, 29, 82, 18
func TestBetRulesEngine_UserProvidedTickets(t *testing.T) {
	engine := NewBetRulesEngine()
	winningNumbers := []int32{13, 61, 29, 82, 18}

	tests := []struct {
		ticketNum         int
		betType           string
		selectedNumbers   []int32
		banker            []int32
		opposed           []int32
		stakeAmount       int64
		numCombinations   int32
		expectedWinner    bool
		expectedWinAmount int64
		description       string
	}{
		// ==================== DIRECT-1 BETS ====================
		// Direct-1: Must match position 0 exactly (13)
		{
			ticketNum:       1,
			betType:         "DIRECT-1",
			selectedNumbers: []int32{7},
			stakeAmount:     100,
			expectedWinner:  false,
			description:     "DIRECT-1: 7 doesn't match position 0 (13)",
		},
		{
			ticketNum:         2,
			betType:           "DIRECT-1",
			selectedNumbers:   []int32{13},
			stakeAmount:       200,
			expectedWinner:    true,
			expectedWinAmount: 200 * 80, // DIRECT-1 odds = 80x
			description:       "DIRECT-1: 13 matches position 0 ✅",
		},
		{
			ticketNum:       3,
			betType:         "DIRECT-1",
			selectedNumbers: []int32{61},
			stakeAmount:     150,
			expectedWinner:  false,
			description:     "DIRECT-1: 61 doesn't match position 0 (13)",
		},

		// ==================== DIRECT-2 BETS ====================
		// Direct-2: Must match positions 0,1 exactly (13, 61)
		{
			ticketNum:         4,
			betType:           "DIRECT-2",
			selectedNumbers:   []int32{13, 61},
			stakeAmount:       300,
			expectedWinner:    true,
			expectedWinAmount: 300 * 240, // DIRECT-2 odds = 240x
			description:       "DIRECT-2: [13,61] matches positions 0,1 ✅",
		},
		{
			ticketNum:       5,
			betType:         "DIRECT-2",
			selectedNumbers: []int32{82, 13},
			stakeAmount:     200,
			expectedWinner:  false,
			description:     "DIRECT-2: [82,13] doesn't match positions 0,1 (13,61)",
		},
		{
			ticketNum:       6,
			betType:         "DIRECT-2",
			selectedNumbers: []int32{13, 99},
			stakeAmount:     250,
			expectedWinner:  false,
			description:     "DIRECT-2: [13,99] - 99 not in winning numbers",
		},
		{
			ticketNum:       7,
			betType:         "DIRECT-2",
			selectedNumbers: []int32{29, 50},
			stakeAmount:     180,
			expectedWinner:  false,
			description:     "DIRECT-2: [29,50] doesn't match positions 0,1 (13,61)",
		},

		// ==================== DIRECT-3 BETS ====================
		// Direct-3: Must match positions 0,1,2 exactly (13, 61, 29)
		{
			ticketNum:         8,
			betType:           "DIRECT-3",
			selectedNumbers:   []int32{13, 61, 29},
			stakeAmount:       500,
			expectedWinner:    true,
			expectedWinAmount: 500 * 1920, // DIRECT-3 odds = 1920x
			description:       "DIRECT-3: [13,61,29] matches positions 0,1,2 ✅",
		},
		{
			ticketNum:       9,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{61, 29, 82},
			stakeAmount:     400,
			expectedWinner:  false,
			description:     "DIRECT-3: [61,29,82] doesn't match positions 0,1,2 (13,61,29)",
		},
		{
			ticketNum:       10,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{13, 29, 50},
			stakeAmount:     350,
			expectedWinner:  false,
			description:     "DIRECT-3: [13,29,50] - wrong order and 50 not in winning",
		},
		{
			ticketNum:       11,
			betType:         "DIRECT-3",
			selectedNumbers: []int32{13, 82, 99},
			stakeAmount:     300,
			expectedWinner:  false,
			description:     "DIRECT-3: [13,82,99] doesn't match positions 0,1,2",
		},

		// ==================== PERM-2 BETS ====================
		// Perm-2: Any 2-number combo must have both numbers in winning pool [13,61,29,82,18]
		{
			ticketNum:         12,
			betType:           "PERM-2",
			selectedNumbers:   []int32{13, 18, 99, 16, 82, 14},
			stakeAmount:       3000, // 6 numbers = C(6,2) = 15 combos
			numCombinations:   15,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 3, // 3 winning combos: [13,18], [13,82], [18,82]
			description:       "PERM-2: 3 winning combos from {13,18,82} in pool ✅",
		},
		{
			ticketNum:         13,
			betType:           "PERM-2",
			selectedNumbers:   []int32{29, 40, 61, 17, 88},
			stakeAmount:       2000, // 5 numbers = C(5,2) = 10 combos
			numCombinations:   10,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 1, // 1 winning combo: [29,61]
			description:       "PERM-2: 1 winning combo from {29,61} in pool ✅",
		},
		{
			ticketNum:         14,
			betType:           "PERM-2",
			selectedNumbers:   []int32{61, 82, 15, 19, 23},
			stakeAmount:       2000, // 5 numbers = C(5,2) = 10 combos
			numCombinations:   10,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 1, // 1 winning combo: [61,82]
			description:       "PERM-2: 1 winning combo from {61,82} in pool ✅",
		},
		{
			ticketNum:       15,
			betType:         "PERM-2",
			selectedNumbers: []int32{18, 99, 70, 58},
			stakeAmount:     1200, // 4 numbers = C(4,2) = 6 combos
			numCombinations: 6,
			expectedWinner:  false,
			description:     "PERM-2: Only 1 number (18) in pool, need at least 2 ❌",
		},

		// ==================== PERM-3 BETS ====================
		// Perm-3: Any 3-number combo must have all 3 numbers in winning pool [13,61,29,82,18]
		{
			ticketNum:         16,
			betType:           "PERM-3",
			selectedNumbers:   []int32{67, 13, 61, 29, 18},
			stakeAmount:       2000, // 5 numbers = C(5,3) = 10 combos
			numCombinations:   10,
			expectedWinner:    true,
			expectedWinAmount: 200 * 1920 * 4, // 4 winning combos from {13,61,29,18}
			description:       "PERM-3: 4 winning combos from {13,61,29,18} in pool ✅",
		},
		{
			ticketNum:         17,
			betType:           "PERM-3",
			selectedNumbers:   []int32{16, 29, 82, 18, 99, 12},
			stakeAmount:       4000, // 6 numbers = C(6,3) = 20 combos
			numCombinations:   20,
			expectedWinner:    true,
			expectedWinAmount: 200 * 1920 * 1, // 1 winning combo: [29,82,18]
			description:       "PERM-3: 1 winning combo from {29,82,18} in pool ✅",
		},
		{
			ticketNum:         18,
			betType:           "PERM-3",
			selectedNumbers:   []int32{13, 61, 99, 18, 6, 15, 71, 88},
			stakeAmount:       11200, // 8 numbers = C(8,3) = 56 combos
			numCombinations:   56,
			expectedWinner:    true,
			expectedWinAmount: 200 * 1920 * 1, // 1 winning combo: [13,61,18]
			description:       "PERM-3: 1 winning combo from {13,61,18} in pool ✅",
		},
		{
			ticketNum:         19,
			betType:           "PERM-3",
			selectedNumbers:   []int32{31, 18, 82, 15, 61, 5, 29},
			stakeAmount:       7000, // 7 numbers = C(7,3) = 35 combos
			numCombinations:   35,
			expectedWinner:    true,
			expectedWinAmount: 200 * 1920 * 4, // 4 winning combos from {18,82,61,29}
			description:       "PERM-3: 4 winning combos from {18,82,61,29} in pool ✅",
		},

		// ==================== BANKER-ALL BETS ====================
		// Banker-All: Banker must be in winning pool, pairs with all other winning numbers
		{
			ticketNum:         20,
			betType:           "BANKER-ALL",
			banker:            []int32{29},
			stakeAmount:       17800, // 89 combos (1 banker with 89 other numbers)
			numCombinations:   89,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 4, // 29 pairs with 4 others: (29,13), (29,61), (29,82), (29,18)
			description:       "BANKER-ALL: Banker 29 in pool, 4 winning pairs ✅",
		},
		{
			ticketNum:         21,
			betType:           "BANKER-ALL",
			banker:            []int32{18},
			stakeAmount:       17800,
			numCombinations:   89,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 4, // 18 pairs with 4 others: (18,13), (18,61), (18,29), (18,82)
			description:       "BANKER-ALL: Banker 18 in pool, 4 winning pairs ✅",
		},
		{
			ticketNum:       22,
			betType:         "BANKER-ALL",
			banker:          []int32{17},
			stakeAmount:     17800,
			numCombinations: 89,
			expectedWinner:  false,
			description:     "BANKER-ALL: Banker 17 NOT in winning pool ❌",
		},
		{
			ticketNum:       23,
			betType:         "BANKER-ALL",
			banker:          []int32{66},
			stakeAmount:     17800,
			numCombinations: 89,
			expectedWinner:  false,
			description:     "BANKER-ALL: Banker 66 NOT in winning pool ❌",
		},
		{
			ticketNum:         24,
			betType:           "BANKER-ALL",
			banker:            []int32{13},
			stakeAmount:       17800,
			numCombinations:   89,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 4, // 13 pairs with 4 others: (13,61), (13,29), (13,82), (13,18)
			description:       "BANKER-ALL: Banker 13 in pool, 4 winning pairs ✅",
		},

		// ==================== BANKER-AG BETS ====================
		// Banker-AG: Pairs where one from banker (in pool) and one from opposed (in pool)
		{
			ticketNum:         25,
			betType:           "BANKER-AG",
			banker:            []int32{13, 61, 70, 77},
			opposed:           []int32{29, 82, 10, 11, 53, 67},
			stakeAmount:       4800, // 4 bankers × 6 opposed = 24 combos
			numCombinations:   24,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 4, // 4 pairs: (13,29), (13,82), (61,29), (61,82)
			description:       "BANKER-AG: Bankers {13,61} × Opposed {29,82} = 4 pairs ✅",
		},
		{
			ticketNum:         26,
			betType:           "BANKER-AG",
			banker:            []int32{44, 18},
			opposed:           []int32{31, 82},
			stakeAmount:       800, // 2 bankers × 2 opposed = 4 combos
			numCombinations:   4,
			expectedWinner:    true,
			expectedWinAmount: 200 * 240 * 1, // 1 pair: (18,82)
			description:       "BANKER-AG: Banker {18} × Opposed {82} = 1 pair ✅",
		},
		{
			ticketNum:       27,
			betType:         "BANKER-AG",
			banker:          []int32{55, 71, 21},
			opposed:         []int32{63, 61},
			stakeAmount:     1200, // 3 bankers × 2 opposed = 6 combos
			numCombinations: 6,
			expectedWinner:  false,
			description:     "BANKER-AG: No bankers in pool (55,71,21 not in winning) ❌",
		},
	}

	var totalWinnings int64
	winningCount := 0
	losingCount := 0

	t.Logf("\n╔════════════════════════════════════════════════════════════════╗")
	t.Logf("║         TESTING USER-PROVIDED TICKETS                          ║")
	t.Logf("╚════════════════════════════════════════════════════════════════╝")
	t.Logf("Winning Numbers: %v\n", winningNumbers)

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			// Calculate amount per combination
			var amountPerCombo int64
			if tt.numCombinations > 0 {
				amountPerCombo = tt.stakeAmount / int64(tt.numCombinations)
			} else {
				amountPerCombo = tt.stakeAmount
			}

			betLine := &BetLine{
				BetType:              tt.betType,
				SelectedNumbers:      tt.selectedNumbers,
				Banker:               tt.banker,
				Opposed:              tt.opposed,
				TotalAmount:          tt.stakeAmount,
				NumberOfCombinations: tt.numCombinations,
				AmountPerCombination: amountPerCombo,
			}

			result, err := engine.CheckBetLine(betLine, winningNumbers)
			if err != nil {
				t.Fatalf("Ticket %d: unexpected error: %v", tt.ticketNum, err)
			}

			// Check winner status
			if result.IsWinner != tt.expectedWinner {
				t.Errorf("❌ Ticket %d (%s): expected IsWinner=%v, got %v",
					tt.ticketNum, tt.description, tt.expectedWinner, result.IsWinner)
			}

			// Check winning amount for winners
			if tt.expectedWinner && result.WinningAmount != tt.expectedWinAmount {
				t.Errorf("❌ Ticket %d (%s): expected WinningAmount=%d (%.2f GHS), got %d (%.2f GHS)",
					tt.ticketNum, tt.description,
					tt.expectedWinAmount, float64(tt.expectedWinAmount)/100.0,
					result.WinningAmount, float64(result.WinningAmount)/100.0)
			}

			// Track statistics
			if result.IsWinner {
				winningCount++
				totalWinnings += result.WinningAmount
				t.Logf("✅ Ticket %d WINS: %d pesewas (%.2f GHS) | Stake: %.2f GHS | %s",
					tt.ticketNum,
					result.WinningAmount, float64(result.WinningAmount)/100.0,
					float64(tt.stakeAmount)/100.0,
					tt.description)
			} else {
				losingCount++
				t.Logf("❌ Ticket %d LOSES | Stake: %.2f GHS | %s",
					tt.ticketNum, float64(tt.stakeAmount)/100.0, tt.description)
			}
		})
	}

	// Summary
	t.Logf("\n╔════════════════════════════════════════════════════════════════╗")
	t.Logf("║                         SUMMARY                                ║")
	t.Logf("╚════════════════════════════════════════════════════════════════╝")
	t.Logf("Total tickets tested:  %d", len(tests))
	t.Logf("Winning tickets:       %d", winningCount)
	t.Logf("Losing tickets:        %d", losingCount)
	t.Logf("Total winnings:        %d pesewas (%.2f GHS)", totalWinnings, float64(totalWinnings)/100.0)
	t.Logf("Win rate:              %.2f%%", float64(winningCount)/float64(len(tests))*100)

	// Breakdown by bet type
	t.Logf("\n═══════════════════════ Bet Type Breakdown ═══════════════════════")
	t.Logf("DIRECT-1:    1 winner / 3 total")
	t.Logf("DIRECT-2:    1 winner / 4 total")
	t.Logf("DIRECT-3:    1 winner / 4 total")
	t.Logf("PERM-2:      3 winners / 4 total")
	t.Logf("PERM-3:      4 winners / 4 total")
	t.Logf("BANKER-ALL:  3 winners / 5 total")
	t.Logf("BANKER-AG:   2 winners / 3 total")
}
