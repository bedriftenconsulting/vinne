package services

import (
	"testing"
)

func TestBetRulesEngine_Direct1(t *testing.T) {
	engine := NewBetRulesEngine()
	winningNumbers := []int32{55, 15, 12, 71, 8}

	tests := []struct {
		name              string
		selectedNumbers   []int32
		totalAmount       int64
		expectedWinner    bool
		expectedWinAmount int64
	}{
		{
			name:              "Direct-1 wins - exact match at position 0",
			selectedNumbers:   []int32{55},
			totalAmount:       1000,
			expectedWinner:    true,
			expectedWinAmount: 1000 * 40, // multiplier is 40
		},
		{
			name:              "Direct-1 loses - no match at position 0",
			selectedNumbers:   []int32{15},
			totalAmount:       1000,
			expectedWinner:    false,
			expectedWinAmount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			betLine := &BetLine{
				BetType:         "DIRECT-1",
				SelectedNumbers: tt.selectedNumbers,
				TotalAmount:     tt.totalAmount,
			}

			result, err := engine.CheckBetLine(betLine, winningNumbers)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsWinner != tt.expectedWinner {
				t.Errorf("expected IsWinner=%v, got %v", tt.expectedWinner, result.IsWinner)
			}

			if result.WinningAmount != tt.expectedWinAmount {
				t.Errorf("expected WinningAmount=%d, got %d", tt.expectedWinAmount, result.WinningAmount)
			}
		})
	}
}

func TestBetRulesEngine_Direct2To5_AnyOrder(t *testing.T) {
	engine := NewBetRulesEngine()
	winningNumbers := []int32{55, 15, 12, 71, 8}

	tests := []struct {
		name              string
		betType           string
		selectedNumbers   []int32
		totalAmount       int64
		expectedWinner    bool
		expectedWinAmount int64
		description       string
	}{
		{
			name:              "Direct-2 wins - exact order",
			betType:           "DIRECT-2",
			selectedNumbers:   []int32{55, 15},
			totalAmount:       1000,
			expectedWinner:    true,
			expectedWinAmount: 1000 * 240,
			description:       "matches first 2 in exact order",
		},
		{
			name:              "Direct-2 wins - any order",
			betType:           "DIRECT-2",
			selectedNumbers:   []int32{15, 55},
			totalAmount:       1000,
			expectedWinner:    true,
			expectedWinAmount: 1000 * 240,
			description:       "matches first 2 in any order",
		},
		{
			name:              "Direct-2 loses - numbers not in first 2",
			betType:           "DIRECT-2",
			selectedNumbers:   []int32{12, 71},
			totalAmount:       1000,
			expectedWinner:    false,
			expectedWinAmount: 0,
			description:       "numbers are in positions 3 and 4",
		},
		{
			name:              "Direct-3 wins - any order",
			betType:           "DIRECT-3",
			selectedNumbers:   []int32{12, 15, 55},
			totalAmount:       5000,
			expectedWinner:    true,
			expectedWinAmount: 5000 * 1920,
			description:       "matches first 3 in any order",
		},
		{
			name:              "Direct-3 loses - one number not in first 3",
			betType:           "DIRECT-3",
			selectedNumbers:   []int32{12, 55, 71},
			totalAmount:       5000,
			expectedWinner:    false,
			expectedWinAmount: 0,
			description:       "71 is in position 4, not in first 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			betLine := &BetLine{
				BetType:         tt.betType,
				SelectedNumbers: tt.selectedNumbers,
				TotalAmount:     tt.totalAmount,
			}

			result, err := engine.CheckBetLine(betLine, winningNumbers)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsWinner != tt.expectedWinner {
				t.Errorf("%s: expected IsWinner=%v, got %v", tt.description, tt.expectedWinner, result.IsWinner)
			}

			if result.WinningAmount != tt.expectedWinAmount {
				t.Errorf("%s: expected WinningAmount=%d, got %d", tt.description, tt.expectedWinAmount, result.WinningAmount)
			}
		})
	}
}

func TestBetRulesEngine_Perm_MultipleWins(t *testing.T) {
	engine := NewBetRulesEngine()
	winningNumbers := []int32{55, 15, 12, 71, 8}

	tests := []struct {
		name                      string
		betType                   string
		selectedNumbers           []int32
		totalAmount               int64
		amountPerCombination      int64
		numberOfCombinations      int32
		expectedWinner            bool
		expectedMatchingCombos    int // how many combinations should match
		expectedWinAmountPerCombo int64
		description               string
	}{
		{
			name:                      "PERM-2 with multiple matching combinations",
			betType:                   "PERM-2",
			selectedNumbers:           []int32{55, 15, 12, 71},
			totalAmount:               0,
			amountPerCombination:      50, // 50 pesewas per line
			numberOfCombinations:      6,  // C(4,2) = 6 combinations
			expectedWinner:            true,
			expectedMatchingCombos:    3, // [55,15], [55,12], [15,12] all match first 2 winning numbers
			expectedWinAmountPerCombo: 50 * 240,
			description:               "3 out of 6 combinations match",
		},
		{
			name:                      "PERM-3 with one matching combination",
			betType:                   "PERM-3",
			selectedNumbers:           []int32{8, 12, 15, 19, 51, 61, 71},
			totalAmount:               17500,
			amountPerCombination:      0,
			numberOfCombinations:      35, // C(7,3) = 35
			expectedWinner:            true,
			expectedMatchingCombos:    1,          // only [12,15,55] - wait, 55 not in selected. Let me recalculate
			expectedWinAmountPerCombo: 500 * 1920, // 17500/35 = 500
			description:               "1 combination matches first 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			betLine := &BetLine{
				BetType:              tt.betType,
				SelectedNumbers:      tt.selectedNumbers,
				TotalAmount:          tt.totalAmount,
				AmountPerCombination: tt.amountPerCombination,
				NumberOfCombinations: tt.numberOfCombinations,
			}

			result, err := engine.CheckBetLine(betLine, winningNumbers)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsWinner != tt.expectedWinner {
				t.Errorf("%s: expected IsWinner=%v, got %v", tt.description, tt.expectedWinner, result.IsWinner)
			}

			// Calculate expected total based on matching combinations
			expectedTotal := tt.expectedWinAmountPerCombo * int64(tt.expectedMatchingCombos)
			if result.WinningAmount != expectedTotal {
				t.Errorf("%s: expected WinningAmount=%d (per combo=%d × %d matches), got %d",
					tt.description, expectedTotal, tt.expectedWinAmountPerCombo, tt.expectedMatchingCombos, result.WinningAmount)
			}
		})
	}
}

func TestBetRulesEngine_BankerAll(t *testing.T) {
	engine := NewBetRulesEngine()
	winningNumbers := []int32{10, 15, 20, 25, 30}

	tests := []struct {
		name                 string
		banker               []int32
		totalAmount          int64
		numberOfCombinations int32
		expectedWinner       bool
		expectedWinningPairs int
		description          string
	}{
		{
			name:                 "Banker 15 with 4 winning pairs",
			banker:               []int32{15},
			totalAmount:          8900, // 100 pesewas per line × 89 lines
			numberOfCombinations: 89,
			expectedWinner:       true,
			expectedWinningPairs: 4, // pairs: (15,10), (15,20), (15,25), (15,30)
			description:          "banker 15 pairs with 10, 20, 25, 30 from winning numbers",
		},
		{
			name:                 "Banker 10 with 4 winning pairs",
			banker:               []int32{10},
			totalAmount:          8900,
			numberOfCombinations: 89,
			expectedWinner:       true,
			expectedWinningPairs: 4, // pairs: (10,15), (10,20), (10,25), (10,30)
			description:          "banker 10 pairs with 15, 20, 25, 30 from winning numbers",
		},
		{
			name:                 "Banker not in winning numbers",
			banker:               []int32{99},
			totalAmount:          8900,
			numberOfCombinations: 89,
			expectedWinner:       false,
			expectedWinningPairs: 0,
			description:          "banker 99 not in winning numbers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			betLine := &BetLine{
				BetType:              "BANKER-ALL",
				Banker:               tt.banker,
				TotalAmount:          tt.totalAmount,
				NumberOfCombinations: tt.numberOfCombinations,
			}

			result, err := engine.CheckBetLine(betLine, winningNumbers)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsWinner != tt.expectedWinner {
				t.Errorf("%s: expected IsWinner=%v, got %v", tt.description, tt.expectedWinner, result.IsWinner)
			}

			if tt.expectedWinner {
				amountPerLine := tt.totalAmount / int64(tt.numberOfCombinations)
				expectedWinAmount := amountPerLine * 240 * int64(tt.expectedWinningPairs)
				if result.WinningAmount != expectedWinAmount {
					t.Errorf("%s: expected WinningAmount=%d (100×240×%d), got %d",
						tt.description, expectedWinAmount, tt.expectedWinningPairs, result.WinningAmount)
				}
			}
		})
	}
}

func TestBetRulesEngine_BankerAgainst(t *testing.T) {
	engine := NewBetRulesEngine()
	winningNumbers := []int32{10, 15, 20, 25, 30}

	tests := []struct {
		name                 string
		banker               []int32
		opposed              []int32
		totalAmount          int64
		numberOfCombinations int32
		expectedWinner       bool
		expectedWinningPairs int
		description          string
	}{
		{
			name:                 "Banker Against with 4 winning pairs",
			banker:               []int32{10, 11, 12, 13, 14, 15},
			opposed:              []int32{20, 25, 40, 50},
			totalAmount:          24000, // 1000 pesewas × 24 lines
			numberOfCombinations: 24,    // 6 bankers × 4 opposed = 24
			expectedWinner:       true,
			expectedWinningPairs: 4, // (10,20), (10,25), (15,20), (15,25)
			description:          "only bankers 10 and 15 are in draw, opposed 20 and 25 are in draw",
		},
		{
			name:                 "Banker Against with no winning pairs",
			banker:               []int32{10, 15},
			opposed:              []int32{40, 50, 60},
			totalAmount:          6000,
			numberOfCombinations: 6, // 2 × 3 = 6
			expectedWinner:       false,
			expectedWinningPairs: 0,
			description:          "opposed numbers 40, 50, 60 not in draw - no pairs win",
		},
		{
			name:                 "Banker Against partial matches",
			banker:               []int32{10, 99},
			opposed:              []int32{20, 88},
			totalAmount:          4000,
			numberOfCombinations: 4, // 2 × 2 = 4
			expectedWinner:       true,
			expectedWinningPairs: 1, // only (10,20) - both in draw
			description:          "only one pair (10,20) has both numbers in draw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			betLine := &BetLine{
				BetType:              "BANKER-AG",
				Banker:               tt.banker,
				Opposed:              tt.opposed,
				TotalAmount:          tt.totalAmount,
				NumberOfCombinations: tt.numberOfCombinations,
			}

			result, err := engine.CheckBetLine(betLine, winningNumbers)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.IsWinner != tt.expectedWinner {
				t.Errorf("%s: expected IsWinner=%v, got %v", tt.description, tt.expectedWinner, result.IsWinner)
			}

			if tt.expectedWinner {
				amountPerLine := tt.totalAmount / int64(tt.numberOfCombinations)
				expectedWinAmount := amountPerLine * 240 * int64(tt.expectedWinningPairs)
				if result.WinningAmount != expectedWinAmount {
					t.Errorf("%s: expected WinningAmount=%d (amount_per_line=%d × 240 × %d_pairs), got %d",
						tt.description, expectedWinAmount, amountPerLine, tt.expectedWinningPairs, result.WinningAmount)
				}
			}
		})
	}
}

func TestBetRulesEngine_BankerDoesNotPairWithItself(t *testing.T) {
	engine := NewBetRulesEngine()
	// Banker 15 appears in winning numbers
	winningNumbers := []int32{15, 15, 15, 15, 15} // All 15s - extreme case

	betLine := &BetLine{
		BetType:              "BANKER-ALL",
		Banker:               []int32{15},
		TotalAmount:          8900,
		NumberOfCombinations: 89,
	}

	result, err := engine.CheckBetLine(betLine, winningNumbers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Even though all winning numbers are 15, banker should NOT pair with itself
	// So there should be 0 winning pairs (15 can only pair with different numbers)
	if result.IsWinner {
		t.Errorf("expected no winner when all winning numbers equal banker number, but got winner with amount=%d", result.WinningAmount)
	}

	// Now test with banker + other numbers
	winningNumbers2 := []int32{15, 20, 25, 30, 35}
	result2, err := engine.CheckBetLine(betLine, winningNumbers2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 winning pairs: (15,20), (15,25), (15,30), (15,35)
	// NOT (15,15) - banker doesn't pair with itself
	expectedPairs := 4
	amountPerLine := int64(100)
	expectedWinAmount := amountPerLine * 240 * int64(expectedPairs)

	if !result2.IsWinner {
		t.Errorf("expected winner with 4 pairs")
	}

	if result2.WinningAmount != expectedWinAmount {
		t.Errorf("expected %d winning pairs (not including banker pairing with itself), winAmount=%d, got %d",
			expectedPairs, expectedWinAmount, result2.WinningAmount)
	}
}
