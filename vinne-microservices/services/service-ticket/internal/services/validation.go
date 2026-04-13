package services

import (
	"errors"
	"fmt"
	"math"

	"github.com/randco/randco-microservices/services/service-ticket/internal/models"
	"github.com/randco/randco-microservices/shared/constants"
)

// ValidatePermBet validates a PERM bet line
func (s *ticketService) ValidatePermBet(betLine *models.BetLine) error {
	// Extract PERM size from bet_type (e.g., "PERM-2" → 2)
	permSize, err := extractPermSize(betLine.BetType)
	if err != nil {
		return err
	}

	// Get numbers from compact format
	numbers := betLine.SelectedNumbers

	// Validate selected numbers count
	n := len(numbers)
	if n < permSize {
		return fmt.Errorf("PERM-%d requires at least %d numbers, got %d",
			permSize, permSize, n)
	}

	// For new format, validate combinations and amounts
	if betLine.NumberOfCombinations > 0 {
		// Calculate expected combinations
		expectedCombinations := calculateCombinations(n, permSize)
		if betLine.NumberOfCombinations != int32(expectedCombinations) {
			return fmt.Errorf("invalid combination count: expected %d, got %d",
				expectedCombinations, betLine.NumberOfCombinations)
		}

		// Validate amounts
		if betLine.AmountPerCombination <= 0 {
			return errors.New("amount per combination must be positive")
		}

		calculatedTotal := betLine.AmountPerCombination * int64(betLine.NumberOfCombinations)
		if calculatedTotal != betLine.TotalAmount {
			return fmt.Errorf("total amount mismatch: expected %d, got %d",
				calculatedTotal, betLine.TotalAmount)
		}
	}

	return nil
}

// ValidateBankerBet validates a Banker bet line
func (s *ticketService) ValidateBankerBet(betLine *models.BetLine) error {
	// Validate no overlaps between banker, opposed, and selected numbers
	if err := validateBankerNumberOverlaps(betLine); err != nil {
		return err
	}

	if constants.IsBankerAllBet(betLine.BetType) {
		// Banker All: Must have exactly 1 banker number
		if len(betLine.Banker) != 1 {
			return fmt.Errorf("banker all requires exactly 1 banker number, got %d",
				len(betLine.Banker))
		}

		// For new format, validate combinations
		if betLine.NumberOfCombinations > 0 {
			// Banker All always has 89 combinations
			if betLine.NumberOfCombinations != 89 {
				return fmt.Errorf("banker all must have 89 combinations, got %d",
					betLine.NumberOfCombinations)
			}

			// Validate amounts
			if betLine.AmountPerCombination <= 0 {
				return errors.New("amount per combination must be positive")
			}

			calculatedTotal := betLine.AmountPerCombination * int64(betLine.NumberOfCombinations)
			if calculatedTotal != betLine.TotalAmount {
				return fmt.Errorf("total amount mismatch: expected %d, got %d",
					calculatedTotal, betLine.TotalAmount)
			}
		}
	} else if constants.IsBankerAgainstBet(betLine.BetType) {
		// Banker Against: Must have at least 1 banker and 1 opposed
		if len(betLine.Banker) == 0 {
			return errors.New("banker against requires at least 1 banker number")
		}
		if len(betLine.Opposed) == 0 {
			return errors.New("banker against requires at least 1 opposed number")
		}

		// For new format, validate combinations
		if betLine.NumberOfCombinations > 0 {
			// Calculate expected combinations
			expectedCombinations := len(betLine.Banker) * len(betLine.Opposed)
			if betLine.NumberOfCombinations != int32(expectedCombinations) {
				return fmt.Errorf("invalid combination count: expected %d, got %d",
					expectedCombinations, betLine.NumberOfCombinations)
			}

			// Validate amounts
			if betLine.AmountPerCombination <= 0 {
				return errors.New("amount per combination must be positive")
			}

			calculatedTotal := betLine.AmountPerCombination * int64(betLine.NumberOfCombinations)
			if calculatedTotal != betLine.TotalAmount {
				return fmt.Errorf("total amount mismatch: expected %d, got %d",
					calculatedTotal, betLine.TotalAmount)
			}
		}
	}

	return nil
}

// validateBankerNumberOverlaps validates that banker, opposed, and selected numbers don't overlap
func validateBankerNumberOverlaps(betLine *models.BetLine) error {
	// Create sets for efficient lookup
	bankerSet := make(map[int32]bool)
	opposedSet := make(map[int32]bool)
	selectedSet := make(map[int32]bool)

	// Build banker set
	for _, num := range betLine.Banker {
		bankerSet[num] = true
	}

	// Build opposed set and check for banker/opposed overlap
	for _, num := range betLine.Opposed {
		if bankerSet[num] {
			return fmt.Errorf("number %d cannot be both banker and opposed", num)
		}
		opposedSet[num] = true
	}

	// Get selected numbers from compact format
	selectedNumbers := betLine.SelectedNumbers

	// Build selected set and check for overlaps with banker/opposed
	for _, num := range selectedNumbers {
		if bankerSet[num] {
			return fmt.Errorf("number %d cannot be both banker and selected", num)
		}
		if opposedSet[num] {
			return fmt.Errorf("number %d cannot be both opposed and selected", num)
		}
		selectedSet[num] = true
	}

	return nil
}

// extractPermSize extracts the PERM size from bet type string
// Examples: "PERM-2" → 2, "Perm-3" → 3
func extractPermSize(betType string) (int, error) {
	normalized := constants.NormalizeBetType(betType)
	if !constants.IsPermBet(normalized) {
		return 0, fmt.Errorf("invalid PERM bet type: %s", betType)
	}

	var size int
	_, err := fmt.Sscanf(normalized, "PERM-%d", &size)
	if err != nil {
		return 0, fmt.Errorf("failed to parse PERM size from %s: %w", betType, err)
	}

	if size < 2 || size > 5 {
		return 0, fmt.Errorf("invalid PERM size: %d (must be 2-5)", size)
	}

	return size, nil
}

// calculateCombinations calculates C(n,r) = n! / (r! × (n-r)!)
func calculateCombinations(n, r int) int {
	if r > n || r < 0 {
		return 0
	}
	if r == 0 || r == n {
		return 1
	}
	if r > n-r {
		r = n - r // Optimization: C(n,r) = C(n,n-r)
	}

	result := 1
	for i := 0; i < r; i++ {
		result *= (n - i)
		result /= (i + 1)
	}
	return result
}

// factorial calculates n!
func factorial(n int) int64 {
	if n < 0 {
		return 0
	}
	if n == 0 || n == 1 {
		return 1
	}

	// Factorial grows very fast:
	// 20! = 2,432,902,008,176,640,000 (fits in int64)
	// 21! = 51,090,942,171,709,440,000 (overflows int64)
	if n > 20 {
		return math.MaxInt64 // Prevent overflow
	}

	result := int64(1)
	for i := 2; i <= n; i++ {
		// Check BEFORE multiplication to prevent overflow
		if result > math.MaxInt64/int64(i) {
			return math.MaxInt64
		}
		result *= int64(i)
	}
	return result
}

// IsPermBet checks if a bet type is a PERM bet
func IsPermBet(betType string) bool {
	return constants.IsPermBet(betType)
}

// IsBankerBet checks if a bet type is a Banker bet
func IsBankerBet(betType string) bool {
	return constants.IsBankerBet(betType)
}

// IsDirectBet checks if a bet type is a Direct bet
func IsDirectBet(betType string) bool {
	return constants.IsDirectBet(betType)
}
