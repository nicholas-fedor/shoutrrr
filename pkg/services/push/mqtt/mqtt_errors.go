// Package mqtt provides a notification service for MQTT message brokers.
package mqtt

import (
	"errors"
	"fmt"
)

// Error definitions for the MQTT service.
var (
	// ErrPublishTimeout is returned when message publication exceeds the configured timeout.
	// This error indicates the broker did not acknowledge the message within the expected time.
	ErrPublishTimeout = errors.New("publish timeout exceeded")

	// ErrConnectionNotInitialized is returned when the connection manager fails to initialize.
	ErrConnectionNotInitialized = errors.New("MQTT connection manager not initialized")

	// ErrTopicRequired is returned when a configuration URL lacks a required topic path.
	// The topic is mandatory for publishing messages and must be provided in the URL path.
	ErrTopicRequired = errors.New("topic is required")

	// ErrInvalidQoS is returned when a QoS value is outside the valid range (0-2).
	// MQTT protocol only supports QoS levels 0, 1, and 2.
	ErrInvalidQoS = errors.New("invalid QoS value: must be 0, 1, or 2")

	// ErrPasswordWithoutUsername is returned when a password is provided without a username.
	// Password credentials require a username to be included in the URL.
	ErrPasswordWithoutUsername = errors.New(
		"password provided without username: username is required when using password authentication",
	)
)

// reasonCodeFailureThreshold is the threshold at which MQTT v5 reason codes
// indicate failure. Codes >= 0x80 are failure/error codes per the MQTT v5 specification.
// Codes 0x00-0x7F are success or non-fatal warning codes.
const reasonCodeFailureThreshold = 0x80

// PublishError represents an MQTT v5 publish failure with a reason code.
// MQTT v5 reason codes >= 0x80 indicate failures, while codes < 0x80 indicate
// success or non-fatal conditions that should be logged but not treated as errors.
type PublishError struct {
	// ReasonCode is the MQTT v5 reason code returned by the broker.
	ReasonCode byte
	// ReasonString is the optional human-readable reason description from the broker.
	ReasonString string
}

// Error implements the error interface for PublishError.
// It returns a formatted error message including the reason code in hex format
// and the optional reason string if provided.
func (e PublishError) Error() string {
	// Build the base error message with hex-formatted reason code
	msg := fmt.Sprintf("MQTT publish failed: reason code 0x%02X", e.ReasonCode)

	// Append the reason string if provided by the broker
	if e.ReasonString != "" {
		msg = fmt.Sprintf("%s - %s", msg, e.ReasonString)
	}

	return msg
}

// IsFailureCode checks if an MQTT v5 reason code indicates a failure.
// Reason codes >= 0x80 are failure codes per the MQTT v5 specification.
//
// Returns true if the code indicates a failure that should be returned as an error,
// false if the code indicates success or a non-fatal condition.
func IsFailureCode(code byte) bool {
	return code >= reasonCodeFailureThreshold
}
