package main

import (
	"fmt"
	"strings"
)

func NormalizePhone(phone string) string {
	// Remove spaces and dashes
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	// If it already starts with '+', assume it's already in international format
	if strings.HasPrefix(phone, "+") {
		return phone
	}

	// If it starts with '0', replace with '+233'
	if strings.HasPrefix(phone, "0") {
		return "+233" + phone[1:]
	}

	// If it starts with '233' (without a leading '+', handled above), prepend '+'
	if strings.HasPrefix(phone, "233") {
		return "+" + phone
	}

	// Otherwise, assume it's a local number without the leading '0' and prepend '+233'
	return "+233" + phone
}

func main() {
	phones := []string{
		"233200000001",
		"+233200000001",
		"0200000001",
	}
	
	for _, phone := range phones {
		normalized := NormalizePhone(phone)
		fmt.Printf("Input: %s -> Normalized: %s\n", phone, normalized)
	}
}
