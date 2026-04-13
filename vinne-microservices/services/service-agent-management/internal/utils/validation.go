package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{1,14}$`) // E.164 format
)

// ValidateEmail validates an email address
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// ValidatePhoneNumber validates a phone number (E.164 format)
func ValidatePhoneNumber(phone string) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return fmt.Errorf("phone number is required")
	}
	// Remove common separators
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")

	if !phoneRegex.MatchString(phone) {
		return fmt.Errorf("invalid phone number format")
	}
	return nil
}

// ValidateRequired validates that a string field is not empty
func ValidateRequired(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

// ValidateLength validates string length
func ValidateLength(value string, minLength, maxLength int, fieldName string) error {
	length := len(strings.TrimSpace(value))
	if minLength > 0 && length < minLength {
		return fmt.Errorf("%s must be at least %d characters", fieldName, minLength)
	}
	if maxLength > 0 && length > maxLength {
		return fmt.Errorf("%s must not exceed %d characters", fieldName, maxLength)
	}
	return nil
}

// ValidateBusinessEntity validates common business entity fields
func ValidateBusinessEntity(name, email, phone, address string) error {
	if err := ValidateRequired(name, "name"); err != nil {
		return err
	}
	if err := ValidateLength(name, 2, 255, "name"); err != nil {
		return err
	}

	if err := ValidateEmail(email); err != nil {
		return fmt.Errorf("contact email: %w", err)
	}

	if err := ValidatePhoneNumber(phone); err != nil {
		return fmt.Errorf("contact phone: %w", err)
	}

	if err := ValidateRequired(address, "address"); err != nil {
		return err
	}
	if err := ValidateLength(address, 10, 500, "address"); err != nil {
		return err
	}

	return nil
}

// NormalizePhoneNumber normalizes a phone number to E.164 format
func NormalizePhoneNumber(phone, defaultCountryCode string) string {
	// Remove common separators
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	phone = strings.ReplaceAll(phone, ".", "")

	// Add country code if not present
	if !strings.HasPrefix(phone, "+") && !strings.HasPrefix(phone, "00") {
		if defaultCountryCode != "" {
			phone = defaultCountryCode + phone
		}
	}

	// Replace 00 with +
	if strings.HasPrefix(phone, "00") {
		phone = "+" + phone[2:]
	}

	return phone
}

// NormalizeEmail normalizes an email address
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
