package validation

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	// Email validation regex (simplified but reasonable)
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// Phone validation regex (international format)
	// Accepts formats: +233XXXXXXXXX, 0XXXXXXXXX, XXXXXXXXX
	phoneRegex = regexp.MustCompile(`^(\+233|0)?[0-9]{9,10}$`)

	// Agent code format (numeric only: 1001, 1002, etc.) - matches agent-management service
	agentCodeRegex = regexp.MustCompile(`^\d+$`)

	// Retailer code format (8-digit numeric only) - matches agent-management service
	retailerCodeRegex = regexp.MustCompile(`^\d{8}$`)

	// Common weak passwords to reject
	weakPasswords = []string{
		"password", "12345678", "123456789", "qwerty", "abc123",
		"password123", "admin", "letmein", "welcome", "monkey",
		"1234567890", "password1", "123123", "qwertyuiop",
	}
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// MultiValidationError represents multiple validation errors
type MultiValidationError struct {
	Errors []ValidationError
}

func (e MultiValidationError) Error() string {
	var messages []string
	for _, err := range e.Errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// ValidateEmail validates an email address
func ValidateEmail(email string) error {
	if email == "" {
		return ValidationError{Field: "email", Message: "email is required"}
	}

	email = strings.TrimSpace(strings.ToLower(email))

	if len(email) > 255 {
		return ValidationError{Field: "email", Message: "email is too long (max 255 characters)"}
	}

	if !emailRegex.MatchString(email) {
		return ValidationError{Field: "email", Message: "invalid email format"}
	}

	return nil
}

// ValidatePhone validates a phone number (Ghana format)
func ValidatePhone(phone string) error {
	if phone == "" {
		return ValidationError{Field: "phone", Message: "phone number is required"}
	}

	// Remove spaces and dashes
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	if !phoneRegex.MatchString(phone) {
		return ValidationError{Field: "phone", Message: "invalid phone number format (expected Ghana format)"}
	}

	return nil
}

// NormalizePhone normalizes a phone number to international format
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

// ValidatePassword validates password strength
func ValidatePassword(password string) error {
	if password == "" {
		return ValidationError{Field: "password", Message: "password is required"}
	}

	if len(password) < 8 {
		return ValidationError{Field: "password", Message: "password must be at least 8 characters long"}
	}

	if len(password) > 128 {
		return ValidationError{Field: "password", Message: "password is too long (max 128 characters)"}
	}

	// Check for weak passwords
	lowerPassword := strings.ToLower(password)
	for _, weak := range weakPasswords {
		if lowerPassword == weak {
			return ValidationError{Field: "password", Message: "password is too weak"}
		}
	}

	// Check password complexity
	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// Require at least 3 out of 4 character types
	complexity := 0
	if hasUpper {
		complexity++
	}
	if hasLower {
		complexity++
	}
	if hasNumber {
		complexity++
	}
	if hasSpecial {
		complexity++
	}

	if complexity < 3 {
		return ValidationError{Field: "password", Message: "password must contain at least 3 of: uppercase, lowercase, numbers, special characters"}
	}

	return nil
}

// ValidatePIN validates a 4-6 digit PIN
func ValidatePIN(pin string) error {
	if pin == "" {
		return ValidationError{Field: "pin", Message: "PIN is required"}
	}

	if len(pin) < 4 || len(pin) > 6 {
		return ValidationError{Field: "pin", Message: "PIN must be 4-6 digits"}
	}

	// Check if all characters are digits
	for _, char := range pin {
		if !unicode.IsDigit(char) {
			return ValidationError{Field: "pin", Message: "PIN must contain only digits"}
		}
	}

	// Check for weak PINs
	weakPINs := []string{"0000", "1111", "1234", "4321", "2222", "3333", "4444", "5555", "6666", "7777", "8888", "9999", "000000", "111111", "123456", "654321"}
	for _, weak := range weakPINs {
		if pin == weak {
			return ValidationError{Field: "pin", Message: "PIN is too weak"}
		}
	}

	return nil
}

// ValidateAgentCode validates an agent code format
func ValidateAgentCode(code string) error {
	if code == "" {
		return ValidationError{Field: "agent_code", Message: "agent code is required"}
	}

	if !agentCodeRegex.MatchString(code) {
		return ValidationError{Field: "agent_code", Message: "invalid agent code format (expected format: 1001)"}
	}

	return nil
}

// ValidateRetailerCode validates a retailer code format
func ValidateRetailerCode(code string) error {
	if code == "" {
		return ValidationError{Field: "retailer_code", Message: "retailer code is required"}
	}

	if !retailerCodeRegex.MatchString(code) {
		return ValidationError{Field: "retailer_code", Message: "invalid retailer code format (expected format: 12345678)"}
	}

	return nil
}

// ValidateUsername validates a username
func ValidateUsername(username string) error {
	if username == "" {
		return ValidationError{Field: "username", Message: "username is required"}
	}

	if len(username) < 3 {
		return ValidationError{Field: "username", Message: "username must be at least 3 characters"}
	}

	if len(username) > 50 {
		return ValidationError{Field: "username", Message: "username is too long (max 50 characters)"}
	}

	// Check if username contains only allowed characters
	for _, char := range username {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' && char != '-' {
			return ValidationError{Field: "username", Message: "username can only contain letters, numbers, underscore and dash"}
		}
	}

	return nil
}

// ValidateName validates a person's name
func ValidateName(name, fieldName string) error {
	if name == "" {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("%s is required", fieldName)}
	}

	if len(name) < 2 {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("%s must be at least 2 characters", fieldName)}
	}

	if len(name) > 100 {
		return ValidationError{Field: fieldName, Message: fmt.Sprintf("%s is too long (max 100 characters)", fieldName)}
	}

	return nil
}

// ValidateBusinessName validates a business name
func ValidateBusinessName(name string) error {
	if name == "" {
		return ValidationError{Field: "business_name", Message: "business name is required"}
	}

	if len(name) < 2 {
		return ValidationError{Field: "business_name", Message: "business name must be at least 2 characters"}
	}

	if len(name) > 255 {
		return ValidationError{Field: "business_name", Message: "business name is too long (max 255 characters)"}
	}

	return nil
}
