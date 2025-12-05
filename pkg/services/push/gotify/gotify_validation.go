package gotify

import (
	"strconv"
	"strings"
	"time"
)

const (
	// TokenLength defines the expected length of a Gotify token, which must be exactly 15 characters and start with 'A'.
	TokenLength = 15
	// TokenChars specifies the valid characters for a Gotify token.
	TokenChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_"
)

// Validator handles input validation for the Gotify service.
type Validator interface {
	ValidateMessage(message string) error
	ValidateServiceInitialized(config *Config) error
	ValidatePriority(priority int) error
	ValidateDate(date string) (string, error)
	ValidateToken(token string) bool
}

// DefaultValidator provides the default implementation of Validator.
type DefaultValidator struct{}

// ValidateMessage checks if the message is not empty.
func (v *DefaultValidator) ValidateMessage(message string) error {
	if message == "" {
		return ErrEmptyMessage
	}

	return nil
}

// ValidateServiceInitialized checks if the service configuration is initialized.
func (v *DefaultValidator) ValidateServiceInitialized(config *Config) error {
	if config == nil {
		return ErrServiceNotInitialized
	}

	return nil
}

// ValidatePriority checks if the priority is within the valid range (-2 to 10).
func (v *DefaultValidator) ValidatePriority(priority int) error {
	if priority < -2 || priority > 10 {
		return ErrInvalidPriority
	}

	return nil
}

// ValidateDate validates and converts the provided date string to RFC3339 format.
// It attempts parsing multiple formats in order of preference: RFC3339, RFC3339 without timezone,
// Unix timestamp (seconds), and basic date-time formats.
// Returns the converted date string in RFC3339 format and nil error on success,
// or empty string and an error if all parsing attempts fail.
func (v *DefaultValidator) ValidateDate(date string) (string, error) {
	if date == "" {
		return "", nil
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, date); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}

	// Try RFC3339 without timezone
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", date, time.UTC); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}

	// Try Unix timestamp seconds
	if unix, err := strconv.ParseInt(date, 10, 64); err == nil {
		t := time.Unix(unix, 0)

		return t.UTC().Format(time.RFC3339), nil
	}

	// Try basic date-time "2006-01-02 15:04:05"
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", date, time.UTC); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}

	return "", ErrInvalidDate
}

// ValidateToken checks if a Gotify token meets length and character requirements.
// This function implements Gotify's token validation rules to ensure tokens are properly formatted
// before attempting API calls. Tokens must be exactly 15 characters, start with 'A', and contain
// only valid characters from the allowed set.
// Parameters:
//   - token: The token string to validate
//
// Returns: true if token is valid according to Gotify's rules, false otherwise.
func (v *DefaultValidator) ValidateToken(token string) bool {
	// First check: token must be exactly 15 characters long and start with 'A'
	if len(token) != TokenLength || token[0] != 'A' {
		return false
	}

	// Second check: iterate through each character to ensure only valid characters are used
	for _, c := range token {
		// Check if current character exists in the allowed character set
		if !strings.ContainsRune(TokenChars, c) {
			return false
		}
	}

	// All validation checks passed
	return true
}
