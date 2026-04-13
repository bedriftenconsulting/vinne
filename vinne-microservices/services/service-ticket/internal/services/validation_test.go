package services

import (
	"testing"

	"github.com/randco/randco-microservices/services/service-ticket/internal/models"
	"github.com/stretchr/testify/assert"
)

// TestValidatePermBet tests PERM bet validation
func TestValidatePermBet(t *testing.T) {
	service := &ticketService{}

	tests := []struct {
		name    string
		betLine models.BetLine
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid PERM-2 with 5 numbers",
			betLine: models.BetLine{
				BetType:              "PERM-2",
				SelectedNumbers:      []int32{1, 2, 3, 4, 5},
				NumberOfCombinations: 10,
				AmountPerCombination: 100,
				TotalAmount:          1000,
			},
			wantErr: false,
		},
		{
			name: "valid PERM-3 with 7 numbers",
			betLine: models.BetLine{
				BetType:              "PERM-3",
				SelectedNumbers:      []int32{1, 2, 3, 4, 5, 6, 7},
				NumberOfCombinations: 35,
				AmountPerCombination: 100,
				TotalAmount:          3500,
			},
			wantErr: false,
		},
		{
			name: "valid PERM-5 with 10 numbers",
			betLine: models.BetLine{
				BetType:              "PERM-5",
				SelectedNumbers:      []int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				NumberOfCombinations: 252,
				AmountPerCombination: 100,
				TotalAmount:          25200,
			},
			wantErr: false,
		},
		{
			name: "invalid - not enough numbers for PERM-2",
			betLine: models.BetLine{
				BetType:              "PERM-2",
				SelectedNumbers:      []int32{1},
				NumberOfCombinations: 0,
				AmountPerCombination: 100,
				TotalAmount:          100,
			},
			wantErr: true,
			errMsg:  "PERM-2 requires at least 2 numbers",
		},
		{
			name: "invalid - wrong combination count",
			betLine: models.BetLine{
				BetType:              "PERM-2",
				SelectedNumbers:      []int32{1, 2, 3},
				NumberOfCombinations: 5, // Should be 3
				AmountPerCombination: 100,
				TotalAmount:          500,
			},
			wantErr: true,
			errMsg:  "invalid combination count: expected 3, got 5",
		},
		{
			name: "invalid - total amount mismatch",
			betLine: models.BetLine{
				BetType:              "PERM-3",
				SelectedNumbers:      []int32{1, 2, 3, 4},
				NumberOfCombinations: 4,
				AmountPerCombination: 100,
				TotalAmount:          500, // Should be 400
			},
			wantErr: true,
			errMsg:  "total amount mismatch: expected 400, got 500",
		},
		{
			name: "invalid - zero amount per combination",
			betLine: models.BetLine{
				BetType:              "PERM-2",
				SelectedNumbers:      []int32{1, 2, 3},
				NumberOfCombinations: 3,
				AmountPerCombination: 0,
				TotalAmount:          0,
			},
			wantErr: true,
			errMsg:  "amount per combination must be positive",
		},
		{
			name: "legacy format - uses Numbers field",
			betLine: models.BetLine{
				BetType:              "PERM-2",
				Numbers:              []int32{1, 2, 3}, // Legacy field
				NumberOfCombinations: 3,
				AmountPerCombination: 100,
				TotalAmount:          300,
			},
			wantErr: false,
		},
		{
			name: "old format without new fields - should pass basic validation",
			betLine: models.BetLine{
				BetType: "PERM-2",
				Numbers: []int32{1, 2, 3, 4, 5},
				// No NumberOfCombinations - old format
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidatePermBet(&tt.betLine)
			if tt.wantErr {
				assert.Error(t, err, "Expected error but got none")
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err, "Unexpected error: %v", err)
			}
		})
	}
}

// TestValidateBankerBet tests Banker bet validation
func TestValidateBankerBet(t *testing.T) {
	service := &ticketService{}

	tests := []struct {
		name    string
		betLine models.BetLine
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid Banker All",
			betLine: models.BetLine{
				BetType:              "BANKER ALL",
				Banker:               []int32{12},
				NumberOfCombinations: 89,
				AmountPerCombination: 100,
				TotalAmount:          8900,
			},
			wantErr: false,
		},
		{
			name: "valid Banker All - lowercase",
			betLine: models.BetLine{
				BetType:              "banker all",
				Banker:               []int32{5},
				NumberOfCombinations: 89,
				AmountPerCombination: 50,
				TotalAmount:          4450,
			},
			wantErr: false,
		},
		{
			name: "valid Banker Against",
			betLine: models.BetLine{
				BetType:              "BANKER AG",
				Banker:               []int32{12, 22},
				Opposed:              []int32{33, 44, 55},
				NumberOfCombinations: 6,
				AmountPerCombination: 100,
				TotalAmount:          600,
			},
			wantErr: false,
		},
		{
			name: "valid Banker Against - single banker",
			betLine: models.BetLine{
				BetType:              "AGAINST",
				Banker:               []int32{10},
				Opposed:              []int32{20, 30, 40, 50},
				NumberOfCombinations: 4,
				AmountPerCombination: 100,
				TotalAmount:          400,
			},
			wantErr: false,
		},
		{
			name: "invalid - Banker All with multiple bankers",
			betLine: models.BetLine{
				BetType:              "BANKER ALL",
				Banker:               []int32{12, 22},
				NumberOfCombinations: 89,
				AmountPerCombination: 100,
				TotalAmount:          8900,
			},
			wantErr: true,
			errMsg:  "Banker All requires exactly 1 banker number, got 2",
		},
		{
			name: "invalid - Banker All with wrong combination count",
			betLine: models.BetLine{
				BetType:              "BANKER",
				Banker:               []int32{12},
				NumberOfCombinations: 90, // Should be 89
				AmountPerCombination: 100,
				TotalAmount:          9000,
			},
			wantErr: true,
			errMsg:  "Banker All must have 89 combinations, got 90",
		},
		{
			name: "invalid - Banker AG with no opposed",
			betLine: models.BetLine{
				BetType:              "BANKER AG",
				Banker:               []int32{12},
				Opposed:              []int32{},
				NumberOfCombinations: 0,
				AmountPerCombination: 100,
				TotalAmount:          0,
			},
			wantErr: true,
			errMsg:  "Banker Against requires at least 1 opposed number",
		},
		{
			name: "invalid - Banker AG with no banker",
			betLine: models.BetLine{
				BetType:              "BANKER AG",
				Banker:               []int32{},
				Opposed:              []int32{33, 44},
				NumberOfCombinations: 0,
				AmountPerCombination: 100,
				TotalAmount:          0,
			},
			wantErr: true,
			errMsg:  "Banker Against requires at least 1 banker number",
		},
		{
			name: "invalid - Banker AG wrong combination count",
			betLine: models.BetLine{
				BetType:              "BANKER AG",
				Banker:               []int32{12, 22, 33},
				Opposed:              []int32{44, 55},
				NumberOfCombinations: 5, // Should be 6 (3 × 2)
				AmountPerCombination: 100,
				TotalAmount:          500,
			},
			wantErr: true,
			errMsg:  "invalid combination count: expected 6, got 5",
		},
		{
			name: "invalid - total amount mismatch",
			betLine: models.BetLine{
				BetType:              "BANKER ALL",
				Banker:               []int32{12},
				NumberOfCombinations: 89,
				AmountPerCombination: 100,
				TotalAmount:          9000, // Should be 8900
			},
			wantErr: true,
			errMsg:  "total amount mismatch: expected 8900, got 9000",
		},
		{
			name: "old format without new fields - should pass",
			betLine: models.BetLine{
				BetType: "BANKER ALL",
				Banker:  []int32{12},
				// No NumberOfCombinations - old format
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateBankerBet(&tt.betLine)
			if tt.wantErr {
				assert.Error(t, err, "Expected error but got none")
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err, "Unexpected error: %v", err)
			}
		})
	}
}

// TestExtractPermSize tests PERM size extraction
func TestExtractPermSize(t *testing.T) {
	tests := []struct {
		name     string
		betType  string
		expected int
		wantErr  bool
	}{
		{"PERM-2", "PERM-2", 2, false},
		{"PERM-3", "PERM-3", 3, false},
		{"PERM-5", "PERM-5", 5, false},
		{"lowercase perm-2", "perm-2", 2, false},
		{"invalid - not PERM", "DIRECT-2", 0, true},
		{"invalid - PERM-1", "PERM-1", 0, true},
		{"invalid - PERM-6", "PERM-6", 0, true},
		{"invalid - no number", "PERM-", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractPermSize(tt.betType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestCalculateCombinations tests combination calculation
func TestCalculateCombinations(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		r        int
		expected int
	}{
		{"C(5,2)", 5, 2, 10},
		{"C(7,3)", 7, 3, 35},
		{"C(10,5)", 10, 5, 252},
		{"C(10,2)", 10, 2, 45},
		{"C(6,3)", 6, 3, 20},
		{"C(4,4)", 4, 4, 1},
		{"C(5,0)", 5, 0, 1},
		{"C(3,5) - invalid", 3, 5, 0},
		{"C(5,-1) - invalid", 5, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateCombinations(tt.n, tt.r)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsPermBet tests PERM bet type detection
func TestIsPermBet(t *testing.T) {
	tests := []struct {
		betType  string
		expected bool
	}{
		{"PERM-2", true},
		{"PERM-3", true},
		{"perm-2", true},
		{"Perm-5", true},
		{"DIRECT-2", false},
		{"BANKER", false},
		{"BANKER ALL", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.betType, func(t *testing.T) {
			result := IsPermBet(tt.betType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsBankerBet tests Banker bet type detection
func TestIsBankerBet(t *testing.T) {
	tests := []struct {
		betType  string
		expected bool
	}{
		{"BANKER", true},
		{"BANKER ALL", true},
		{"banker all", true},
		{"BANKER AG", true},
		{"AGAINST", true},
		{"banker ag", true},
		{"PERM-2", false},
		{"DIRECT-2", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.betType, func(t *testing.T) {
			result := IsBankerBet(tt.betType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsDirectBet tests Direct bet type detection
func TestIsDirectBet(t *testing.T) {
	tests := []struct {
		betType  string
		expected bool
	}{
		{"DIRECT-1", true},
		{"DIRECT-2", true},
		{"direct-5", true},
		{"Direct-3", true},
		{"PERM-2", false},
		{"BANKER", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.betType, func(t *testing.T) {
			result := IsDirectBet(tt.betType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFactorial tests factorial calculation
func TestFactorial(t *testing.T) {
	tests := []struct {
		name     string
		n        int
		expected int64
	}{
		{"0!", 0, 1},
		{"1!", 1, 1},
		{"5!", 5, 120},
		{"10!", 10, 3628800},
		{"negative", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := factorial(tt.n)
			assert.Equal(t, tt.expected, result)
		})
	}
}
