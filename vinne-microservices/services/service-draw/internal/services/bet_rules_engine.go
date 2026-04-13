package services

import (
	"errors"
	"log"
	"os"
	"strings"
)

// BetType constants for VAG MONDAY game
// NOTE: These are normalized versions. Always use NormalizeBetType() when comparing with user input
const (
	BetTypeDirect1   = "DIRECT-1"
	BetTypeDirect2   = "DIRECT-2"
	BetTypeDirect3   = "DIRECT-3"
	BetTypeDirect4   = "DIRECT-4"
	BetTypeDirect5   = "DIRECT-5"
	BetTypePerm2     = "PERM-2"
	BetTypePerm3     = "PERM-3"
	BetTypePerm4     = "PERM-4"
	BetTypePerm5     = "PERM-5"
	BetTypeBankerAll = "BANKER-ALL"
	BetTypeBankerAG  = "BANKER-AG"
)

// Multipliers for each bet type
var betMultipliers = map[string]int64{
	BetTypeDirect1:   40,
	BetTypeDirect2:   240,
	BetTypeDirect3:   1920,
	BetTypeDirect4:   19200,
	BetTypeDirect5:   230400,
	BetTypePerm2:     240,
	BetTypePerm3:     1920,
	BetTypePerm4:     19200,
	BetTypePerm5:     230400,
	BetTypeBankerAll: 240,
	BetTypeBankerAG:  240,
}

// BetLine represents a single bet line from the ticket
type BetLine struct {
	LineNumber int32
	BetType    string

	// For DIRECT and PERM bets
	SelectedNumbers []int32 // Player's chosen numbers

	// For BANKER and AGAINST bets
	Banker  []int32
	Opposed []int32

	// For PERM and Banker bets (compact format)
	NumberOfCombinations int32 // C(n,r) - calculated value
	AmountPerCombination int64 // Amount per combination in pesewas

	// Common fields
	TotalAmount int64 // Total bet amount in pesewas
}

// WinningResult represents the result of checking a bet line against winning numbers
type WinningResult struct {
	BetLine       *BetLine
	IsWinner      bool
	WinningAmount int64 // in pesewas
	MatchedCount  int
}

// BetRulesEngine handles bet calculation logic for VAG MONDAY game
type BetRulesEngine struct {
	logger *log.Logger
}

// NewBetRulesEngine creates a new bet rules engine
func NewBetRulesEngine() *BetRulesEngine {
	return &BetRulesEngine{
		logger: log.New(os.Stdout, "[BetRulesEngine] ", log.LstdFlags),
	}
}

// NormalizeBetType normalizes a bet type string to uppercase for consistent comparison
func NormalizeBetType(betType string) string {
	normalized := strings.ToUpper(strings.TrimSpace(betType))
	// Handle variations: "Direct 1" → "DIRECT-1", "Perm 2" → "PERM-2", etc.
	normalized = strings.ReplaceAll(normalized, " ", "-")
	return normalized
}

// CalculateWinnings checks all bet lines against winning numbers and calculates total winnings
func (e *BetRulesEngine) CalculateWinnings(betLines []*BetLine, winningNumbers []int32) (int64, []*WinningResult, error) {
	if len(winningNumbers) != 5 {
		return 0, nil, errors.New("winning numbers must contain exactly 5 numbers")
	}

	var totalWinnings int64
	var results []*WinningResult

	for _, betLine := range betLines {
		result, err := e.CheckBetLine(betLine, winningNumbers)
		if err != nil {
			return 0, nil, err
		}
		results = append(results, result)
		totalWinnings += result.WinningAmount
	}

	return totalWinnings, results, nil
}

// CheckBetLine checks a single bet line against winning numbers
func (e *BetRulesEngine) CheckBetLine(betLine *BetLine, winningNumbers []int32) (*WinningResult, error) {
	// Normalize bet type for consistent comparison
	normalizedBetType := NormalizeBetType(betLine.BetType)

	switch normalizedBetType {
	case BetTypeDirect1, BetTypeDirect2, BetTypeDirect3, BetTypeDirect4, BetTypeDirect5:
		return e.checkDirectBet(betLine, winningNumbers)
	case BetTypePerm2, BetTypePerm3:
		return e.checkPermBet(betLine, winningNumbers)
	case BetTypeBankerAll:
		return e.checkBankerAll(betLine, winningNumbers)
	case BetTypeBankerAG:
		return e.checkBankerAG(betLine, winningNumbers)
	default:
		return nil, errors.New("unsupported bet type: " + betLine.BetType)
	}
}

// checkDirectBet checks direct bet types (Direct 1-5)
// Direct-1: Requires EXACT POSITIONAL MATCHING (first position only)
// Direct-2 to Direct-5: Numbers must appear in the corresponding winning positions in ANY ORDER
func (e *BetRulesEngine) checkDirectBet(betLine *BetLine, winningNumbers []int32) (*WinningResult, error) {
	numbers := betLine.SelectedNumbers
	amount := betLine.TotalAmount

	e.logger.Printf("DEBUG checkDirectBet: BetType=%q, Numbers=%v, Amount=%d", betLine.BetType, numbers, amount)

	result := &WinningResult{
		BetLine:       betLine,
		IsWinner:      false,
		WinningAmount: 0,
		MatchedCount:  0,
	}

	requiredPositions := e.getRequiredMatches(betLine.BetType)
	e.logger.Printf("DEBUG checkDirectBet: RequiredPositions=%d for BetType=%q", requiredPositions, betLine.BetType)

	if requiredPositions == 0 {
		e.logger.Printf("DEBUG checkDirectBet: RequiredPositions is 0, returning no winner")
		return result, nil
	}

	// Direct bet validation: must have exact number of numbers for the bet type
	if len(numbers) != requiredPositions {
		e.logger.Printf("DEBUG checkDirectBet: Invalid number count. Expected=%d, Got=%d", requiredPositions, len(numbers))
		return result, nil
	}

	normalizedBetType := NormalizeBetType(betLine.BetType)

	// Direct-1: Exact positional match required (first position only)
	if normalizedBetType == BetTypeDirect1 {
		e.logger.Printf("  Direct-1: Checking position 0: betNumber=%d vs winningNumber=%d", numbers[0], winningNumbers[0])
		if numbers[0] == winningNumbers[0] {
			result.IsWinner = true
			result.MatchedCount = 1
			multiplier := betMultipliers[BetTypeDirect1]
			result.WinningAmount = amount * multiplier
			e.logger.Printf("DEBUG checkDirectBet: WINNER! Amount=%d, Multiplier=%d, WinningAmount=%d",
				amount, multiplier, result.WinningAmount)
		} else {
			e.logger.Printf("DEBUG checkDirectBet: NOT A WINNER - position 0 did not match")
		}
		return result, nil
	}

	// Direct-2 to Direct-5: Match numbers in ANY ORDER within the corresponding winning positions
	targetWinningNumbers := winningNumbers[:requiredPositions]
	e.logger.Printf("DEBUG checkDirectBet: Checking if %v matches %v in any order", numbers, targetWinningNumbers)

	if e.matchesInAnyOrder(numbers, targetWinningNumbers) {
		result.IsWinner = true
		result.MatchedCount = requiredPositions
		multiplier := betMultipliers[normalizedBetType]
		result.WinningAmount = amount * multiplier
		e.logger.Printf("DEBUG checkDirectBet: WINNER! Numbers match in any order. Amount=%d, Multiplier=%d, WinningAmount=%d",
			amount, multiplier, result.WinningAmount)
	} else {
		e.logger.Printf("DEBUG checkDirectBet: NOT A WINNER - numbers do not match in any order")
	}

	return result, nil
}

// checkPermBet checks permutation bet types (Perm 2, Perm 3, Perm 4, Perm 5)
// Perm bets match against a POOL of numbers, checking if any combination matches in ANY ORDER
// Perm 2: Check if any 2-number combination matches any 2 from first 3 drawn numbers
// Perm 3: Check if any 3-number combination matches any 3 from first 4 drawn numbers
// Perm N: Check against first (N+1) drawn numbers
// IMPORTANT: If multiple combinations match, the player wins multiple times!
func (e *BetRulesEngine) checkPermBet(betLine *BetLine, winningNumbers []int32) (*WinningResult, error) {
	numbers := betLine.SelectedNumbers

	result := &WinningResult{
		BetLine:       betLine,
		IsWinner:      false,
		WinningAmount: 0,
		MatchedCount:  0,
	}

	requiredPositions := e.getRequiredMatches(betLine.BetType)
	if requiredPositions == 0 {
		return result, nil
	}

	// PERM bets check against ALL winning numbers (position-independent matching)
	// Generate all combinations and check if each combination's numbers all appear in winning numbers
	targetWinningPool := winningNumbers // ALL 5 winning numbers
	e.logger.Printf("DEBUG checkPermBet: BetType=%q, RequiredMatches=%d, TargetPool=%v (ALL winning numbers), PlayerNumbers=%v",
		betLine.BetType, requiredPositions, targetWinningPool, numbers)

	// Count total lines based on combinations (not permutations!)
	// User picks N numbers, generates C(N, requiredPositions) combinations
	// Prefer new format NumberOfCombinations if available, otherwise calculate
	var totalLines int
	if betLine.NumberOfCombinations > 0 {
		totalLines = int(betLine.NumberOfCombinations) // New format
		e.logger.Printf("DEBUG checkPermBet: Using new format NumberOfCombinations=%d", totalLines)
	} else {
		totalLines = e.calculateCombinations(len(numbers), requiredPositions) // Legacy format
		e.logger.Printf("DEBUG checkPermBet: Calculated TotalLines=%d from C(%d, %d)", totalLines, len(numbers), requiredPositions)
	}

	if totalLines == 0 {
		return result, nil
	}

	// Generate all combinations of player's numbers
	// Check ALL combinations - count how many match (player wins once per matching combination)
	combinations := e.generateCombinations(numbers, requiredPositions)
	e.logger.Printf("DEBUG checkPermBet: Generated %d combinations", len(combinations))

	// Check each combination: ALL numbers in the combo must exist in the target pool
	matchingCombinationsCount := 0
	for i, combo := range combinations {
		if e.allNumbersInPool(combo, targetWinningPool) {
			matchingCombinationsCount++
			e.logger.Printf("DEBUG checkPermBet: Combination #%d %v MATCHES - all numbers found in pool %v (Total matches so far: %d)",
				i, combo, targetWinningPool, matchingCombinationsCount)
		}
	}

	if matchingCombinationsCount > 0 {
		result.IsWinner = true
		result.MatchedCount = requiredPositions
		multiplier := betMultipliers[betLine.BetType]

		// Calculate amount per line
		var amountPerLine int64
		if betLine.AmountPerCombination > 0 {
			amountPerLine = betLine.AmountPerCombination
			e.logger.Printf("DEBUG checkPermBet: Using AmountPerCombination=%d", amountPerLine)
		} else if betLine.TotalAmount > 0 {
			amountPerLine = betLine.TotalAmount / int64(totalLines)
			e.logger.Printf("DEBUG checkPermBet: Calculated AmountPerLine=%d (TotalAmount=%d / TotalLines=%d)",
				amountPerLine, betLine.TotalAmount, totalLines)
		}

		// Player wins once for EACH matching combination
		result.WinningAmount = amountPerLine * multiplier * int64(matchingCombinationsCount)
		e.logger.Printf("DEBUG checkPermBet: WINNER! MatchingCombinations=%d, AmountPerLine=%d, Multiplier=%d, TotalWinningAmount=%d",
			matchingCombinationsCount, amountPerLine, multiplier, result.WinningAmount)
	} else {
		e.logger.Printf("DEBUG checkPermBet: NOT A WINNER - no combination matched")
	}

	return result, nil
}

// checkBankerAll checks Banker All bet type
// NEW RULE: Banker is paired with all other numbers. A pair wins if BOTH numbers appear in the 5 winning numbers.
// Player wins once for EACH winning pair.
func (e *BetRulesEngine) checkBankerAll(betLine *BetLine, winningNumbers []int32) (*WinningResult, error) {
	result := &WinningResult{
		BetLine:       betLine,
		IsWinner:      false,
		WinningAmount: 0,
		MatchedCount:  0,
	}

	// Validate: Banker All requires exactly 1 banker number
	if len(betLine.Banker) != 1 {
		e.logger.Printf("DEBUG checkBankerAll: Invalid banker count. Expected=1, Got=%d", len(betLine.Banker))
		return result, nil
	}

	bankerNumber := betLine.Banker[0]
	e.logger.Printf("DEBUG checkBankerAll: BankerNumber=%d, WinningNumbers=%v", bankerNumber, winningNumbers)

	// First check: banker number must be in winning numbers
	if !e.contains(winningNumbers, bankerNumber) {
		e.logger.Printf("DEBUG checkBankerAll: Banker number %d NOT in winning numbers - no wins possible", bankerNumber)
		return result, nil
	}

	e.logger.Printf("DEBUG checkBankerAll: Banker number %d IS in winning numbers - checking pairs", bankerNumber)

	// Calculate total combinations
	// For Banker All: always 89 combinations (banker paired with all other 89 numbers from 1-90)
	var totalLines int
	if betLine.NumberOfCombinations > 0 {
		totalLines = int(betLine.NumberOfCombinations) // New format
		e.logger.Printf("DEBUG checkBankerAll: Using NumberOfCombinations=%d", totalLines)
	} else {
		totalLines = 89 // Legacy format: Banker All always has 89 combinations
		e.logger.Printf("DEBUG checkBankerAll: Using default totalLines=89")
	}

	if totalLines == 0 {
		return result, nil
	}

	// Count winning pairs: banker paired with each other winning number
	winningPairsCount := 0
	for _, winNum := range winningNumbers {
		if winNum != bankerNumber {
			// This forms a winning pair: banker + winNum (both are in winning numbers)
			winningPairsCount++
			e.logger.Printf("DEBUG checkBankerAll: Winning pair: [%d, %d]", bankerNumber, winNum)
		}
	}

	e.logger.Printf("DEBUG checkBankerAll: Total winning pairs: %d", winningPairsCount)

	if winningPairsCount > 0 {
		result.IsWinner = true
		result.MatchedCount = 2 // Each winning pair has 2 numbers
		multiplier := betMultipliers[BetTypeBankerAll]

		// Calculate amount per line
		var amountPerLine int64
		if betLine.AmountPerCombination > 0 {
			amountPerLine = betLine.AmountPerCombination
			e.logger.Printf("DEBUG checkBankerAll: Using AmountPerCombination=%d", amountPerLine)
		} else if betLine.TotalAmount > 0 {
			amountPerLine = betLine.TotalAmount / int64(totalLines)
			e.logger.Printf("DEBUG checkBankerAll: Calculated AmountPerLine=%d (TotalAmount=%d / TotalLines=%d)",
				amountPerLine, betLine.TotalAmount, totalLines)
		}

		// Player wins once for EACH winning pair
		result.WinningAmount = amountPerLine * multiplier * int64(winningPairsCount)
		e.logger.Printf("DEBUG checkBankerAll: WINNER! WinningPairs=%d, AmountPerLine=%d, Multiplier=%d, TotalWinningAmount=%d",
			winningPairsCount, amountPerLine, multiplier, result.WinningAmount)
	} else {
		e.logger.Printf("DEBUG checkBankerAll: NOT A WINNER - banker in winning numbers but no other winning numbers to pair with")
	}

	return result, nil
}

// checkBankerAG checks Banker Against bet type
// NEW RULE: Each banker is paired with each opposed number. A pair wins if BOTH numbers appear in the 5 winning numbers.
// Player wins once for EACH winning pair.
func (e *BetRulesEngine) checkBankerAG(betLine *BetLine, winningNumbers []int32) (*WinningResult, error) {
	result := &WinningResult{
		BetLine:       betLine,
		IsWinner:      false,
		WinningAmount: 0,
		MatchedCount:  0,
	}

	// Validate: Banker AG requires at least 1 banker and 1 opposed number
	if len(betLine.Banker) == 0 || len(betLine.Opposed) == 0 {
		e.logger.Printf("DEBUG checkBankerAG: Invalid banker/opposed count. Banker=%d, Opposed=%d",
			len(betLine.Banker), len(betLine.Opposed))
		return result, nil
	}

	e.logger.Printf("DEBUG checkBankerAG: Banker=%v, Opposed=%v, WinningNumbers=%v",
		betLine.Banker, betLine.Opposed, winningNumbers)

	// Calculate total combinations
	// For Banker AG: combinations = banker_count × opposed_count
	var totalLines int
	if betLine.NumberOfCombinations > 0 {
		totalLines = int(betLine.NumberOfCombinations) // New format
		e.logger.Printf("DEBUG checkBankerAG: Using NumberOfCombinations=%d", totalLines)
	} else {
		// Legacy format: calculate combinations
		totalLines = len(betLine.Banker) * len(betLine.Opposed)
		e.logger.Printf("DEBUG checkBankerAG: Calculated totalLines=%d (%d bankers × %d opposed)",
			totalLines, len(betLine.Banker), len(betLine.Opposed))
	}

	if totalLines == 0 {
		return result, nil
	}

	// Count winning pairs: each (banker, opposed) pair where BOTH are in winning numbers
	winningPairsCount := 0
	for _, banker := range betLine.Banker {
		for _, opposed := range betLine.Opposed {
			// Check if BOTH banker AND opposed are in winning numbers
			if e.contains(winningNumbers, banker) && e.contains(winningNumbers, opposed) {
				winningPairsCount++
				e.logger.Printf("DEBUG checkBankerAG: Winning pair: [%d, %d] (both in draw)", banker, opposed)
			} else {
				e.logger.Printf("DEBUG checkBankerAG: Losing pair: [%d, %d] (banker_in_draw=%v, opposed_in_draw=%v)",
					banker, opposed, e.contains(winningNumbers, banker), e.contains(winningNumbers, opposed))
			}
		}
	}

	e.logger.Printf("DEBUG checkBankerAG: Total winning pairs: %d out of %d", winningPairsCount, totalLines)

	if winningPairsCount > 0 {
		result.IsWinner = true
		result.MatchedCount = 2 // Each winning pair has 2 numbers
		multiplier := betMultipliers[BetTypeBankerAG]

		// Calculate amount per line
		var amountPerLine int64
		if betLine.AmountPerCombination > 0 {
			amountPerLine = betLine.AmountPerCombination
			e.logger.Printf("DEBUG checkBankerAG: Using AmountPerCombination=%d", amountPerLine)
		} else if betLine.TotalAmount > 0 {
			amountPerLine = betLine.TotalAmount / int64(totalLines)
			e.logger.Printf("DEBUG checkBankerAG: Calculated AmountPerLine=%d (TotalAmount=%d / TotalLines=%d)",
				amountPerLine, betLine.TotalAmount, totalLines)
		}

		// Player wins once for EACH winning pair
		result.WinningAmount = amountPerLine * multiplier * int64(winningPairsCount)
		e.logger.Printf("DEBUG checkBankerAG: WINNER! WinningPairs=%d, AmountPerLine=%d, Multiplier=%d, TotalWinningAmount=%d",
			winningPairsCount, amountPerLine, multiplier, result.WinningAmount)
	} else {
		e.logger.Printf("DEBUG checkBankerAG: NOT A WINNER - no pairs with both numbers in winning draw")
	}

	return result, nil
}

// Helper functions

func (e *BetRulesEngine) getRequiredMatches(betType string) int {
	normalized := NormalizeBetType(betType)
	switch normalized {
	case BetTypeDirect1:
		return 1
	case BetTypeDirect2, BetTypePerm2:
		return 2
	case BetTypeDirect3, BetTypePerm3:
		return 3
	case BetTypeDirect4:
		return 4
	case BetTypeDirect5:
		return 5
	default:
		return 0
	}
}

func (e *BetRulesEngine) contains(haystack []int32, needle int32) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// matchesInAnyOrder checks if two slices contain the same numbers in any order
func (e *BetRulesEngine) matchesInAnyOrder(slice1, slice2 []int32) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	// Create frequency maps
	freq1 := make(map[int32]int)
	freq2 := make(map[int32]int)

	for _, num := range slice1 {
		freq1[num]++
	}
	for _, num := range slice2 {
		freq2[num]++
	}

	// Compare frequency maps
	for num, count := range freq1 {
		if freq2[num] != count {
			return false
		}
	}

	return true
}

// allNumbersInPool checks if all numbers in the combination exist within the pool
func (e *BetRulesEngine) allNumbersInPool(combination []int32, pool []int32) bool {
	for _, num := range combination {
		if !e.contains(pool, num) {
			return false
		}
	}
	return true
}

// generateCombinations generates all C(n,r) combinations from the given numbers
func (e *BetRulesEngine) generateCombinations(numbers []int32, r int) [][]int32 {
	var result [][]int32
	var combination []int32

	var generate func(start int)
	generate = func(start int) {
		if len(combination) == r {
			// Make a copy of the combination
			combo := make([]int32, r)
			copy(combo, combination)
			result = append(result, combo)
			return
		}

		for i := start; i < len(numbers); i++ {
			combination = append(combination, numbers[i])
			generate(i + 1)
			combination = combination[:len(combination)-1]
		}
	}

	generate(0)
	return result
}

func (e *BetRulesEngine) calculatePermutationLines(n, r int) int {
	if r > n || r <= 0 {
		return 0
	}
	return int(factorial(n) / factorial(n-r))
}

func (e *BetRulesEngine) calculateCombinations(n, r int) int {
	if r > n || r < 0 {
		return 0
	}
	if r == 0 || r == n {
		return 1
	}
	return int(factorial(n) / (factorial(r) * factorial(n-r)))
}

func factorial(n int) int64 {
	if n <= 1 {
		return 1
	}
	result := int64(1)
	for i := 2; i <= n; i++ {
		result *= int64(i)
	}
	return result
}

// GetBetTypeMultiplier returns the multiplier for a given bet type
func (e *BetRulesEngine) GetBetTypeMultiplier(betType string) int64 {
	normalized := NormalizeBetType(betType)
	if multiplier, ok := betMultipliers[normalized]; ok {
		return multiplier
	}
	return 0
}

// CalculateTotalLines calculates the total number of lines for a bet
func (e *BetRulesEngine) CalculateTotalLines(betLine *BetLine) int {
	// Prefer NumberOfCombinations if available
	if betLine.NumberOfCombinations > 0 {
		return int(betLine.NumberOfCombinations)
	}

	numbers := betLine.SelectedNumbers

	normalized := NormalizeBetType(betLine.BetType)
	switch normalized {
	case BetTypeDirect1, BetTypeDirect2, BetTypeDirect3, BetTypeDirect4, BetTypeDirect5:
		return 1
	case BetTypePerm2:
		return e.calculatePermutationLines(len(numbers), 2)
	case BetTypePerm3:
		return e.calculatePermutationLines(len(numbers), 3)
	case BetTypeBankerAll, BetTypeBankerAG:
		bankerCount := len(betLine.Banker)
		numbersCount := len(numbers)
		remainingSlots := 2 - bankerCount
		if remainingSlots <= 0 || numbersCount < remainingSlots {
			return 0
		}
		return e.calculateCombinations(numbersCount, remainingSlots)
	default:
		return 0
	}
}

// ValidateBetLine validates a bet line structure
func (e *BetRulesEngine) ValidateBetLine(betLine *BetLine) error {
	amount := betLine.TotalAmount

	if amount <= 0 {
		return errors.New("bet amount must be positive")
	}

	numbers := betLine.SelectedNumbers

	normalized := NormalizeBetType(betLine.BetType)
	switch normalized {
	case BetTypeDirect1:
		if len(numbers) < 1 {
			return errors.New("direct 1 requires at least 1 number")
		}
	case BetTypeDirect2:
		if len(numbers) < 2 {
			return errors.New("direct 2 requires at least 2 numbers")
		}
	case BetTypeDirect3:
		if len(numbers) < 3 {
			return errors.New("direct 3 requires at least 3 numbers")
		}
	case BetTypePerm2:
		if len(numbers) < 2 {
			return errors.New("perm 2 requires at least 2 numbers")
		}
	case BetTypePerm3:
		if len(numbers) < 3 {
			return errors.New("perm 3 requires at least 3 numbers")
		}
	case BetTypeBankerAll, BetTypeBankerAG:
		if len(betLine.Banker) == 0 {
			return errors.New("banker bet requires at least 1 banker number")
		}
		if len(betLine.Banker) >= 2 {
			return errors.New("banker bet cannot have 2 or more banker numbers for 2-number match")
		}
		if len(numbers) < (2 - len(betLine.Banker)) {
			return errors.New("insufficient numbers for banker bet")
		}
		if normalized == BetTypeBankerAG && len(betLine.Opposed) == 0 {
			return errors.New("banker AG requires at least 1 opposed number")
		}
	default:
		return errors.New("unsupported bet type: " + betLine.BetType)
	}

	// Validate all numbers are in valid range (1-90 for 5/90 game)
	allNumbers := append([]int32{}, numbers...)
	allNumbers = append(allNumbers, betLine.Banker...)
	allNumbers = append(allNumbers, betLine.Opposed...)

	for _, num := range allNumbers {
		if num < 1 || num > 90 {
			return errors.New("all numbers must be between 1 and 90")
		}
	}

	return nil
}
