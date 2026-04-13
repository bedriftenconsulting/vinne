package constants

import "strings"

// BetType constants for lottery games
// These constants should be used consistently across all services
const (
	// PERM bet types
	BetTypePermPrefix = "PERM-" // Prefix for all PERM bets (PERM-2, PERM-3, etc.)
	BetTypePerm2      = "PERM-2"
	BetTypePerm3      = "PERM-3"
	BetTypePerm4      = "PERM-4"
	BetTypePerm5      = "PERM-5"

	// Direct bet types
	BetTypeDirectPrefix = "DIRECT-" // Prefix for all Direct bets (DIRECT-1, DIRECT-2, etc.)
	BetTypeDirect1      = "DIRECT-1"
	BetTypeDirect2      = "DIRECT-2"
	BetTypeDirect3      = "DIRECT-3"
	BetTypeDirect4      = "DIRECT-4"
	BetTypeDirect5      = "DIRECT-5"

	// Banker bet types
	BetTypeBanker        = "BANKER"     // Alias for Banker All
	BetTypeBankerAll     = "BANKER ALL" // Banker All (1 banker × 89 combinations)
	BetTypeBankerAG      = "BANKER AG"  // Banker Against
	BetTypeBankerAgainst = "AGAINST"    // Alias for Banker Against
)

// NormalizeBetType normalizes a bet type string to uppercase for consistent comparison
func NormalizeBetType(betType string) string {
	return strings.ToUpper(strings.TrimSpace(betType))
}

// IsPermBet checks if a bet type is a PERM bet
func IsPermBet(betType string) bool {
	normalized := NormalizeBetType(betType)
	return strings.HasPrefix(normalized, BetTypePermPrefix)
}

// IsBankerBet checks if a bet type is any kind of Banker bet
func IsBankerBet(betType string) bool {
	normalized := NormalizeBetType(betType)
	return normalized == BetTypeBanker ||
		normalized == BetTypeBankerAll ||
		strings.Contains(normalized, "BANKER AG") ||
		normalized == BetTypeBankerAgainst ||
		strings.Contains(normalized, "AGAINST")
}

// IsBankerAllBet checks if a bet type is specifically Banker All
func IsBankerAllBet(betType string) bool {
	normalized := NormalizeBetType(betType)
	return normalized == BetTypeBanker || normalized == BetTypeBankerAll
}

// IsBankerAgainstBet checks if a bet type is specifically Banker Against
func IsBankerAgainstBet(betType string) bool {
	normalized := NormalizeBetType(betType)
	return strings.Contains(normalized, "BANKER AG") ||
		normalized == BetTypeBankerAgainst ||
		(strings.Contains(normalized, "AGAINST") && !strings.Contains(normalized, "ALL"))
}

// IsDirectBet checks if a bet type is a Direct bet
func IsDirectBet(betType string) bool {
	normalized := NormalizeBetType(betType)
	return strings.HasPrefix(normalized, BetTypeDirectPrefix)
}
