package mqtt

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublishErrorMessage verifies that a publish error names its reason code
// in hex and appends a non-empty reason string.
func TestPublishErrorMessage(t *testing.T) {
	t.Parallel()

	// tests lists publish errors with the message each should produce.
	tests := []struct {
		name string
		err  PublishError
		want string
	}{
		{name: "code only", err: PublishError{ReasonCode: 0x80}, want: "MQTT publish failed: reason code 0x80"},
		{name: "code and reason", err: PublishError{ReasonCode: 0x81, ReasonString: "Client identifier not valid"}, want: "MQTT publish failed: reason code 0x81 - Client identifier not valid"},
		{name: "success code", err: PublishError{ReasonCode: 0x00, ReasonString: "Success"}, want: "MQTT publish failed: reason code 0x00 - Success"},
		{name: "highest code", err: PublishError{ReasonCode: 0xFF}, want: "MQTT publish failed: reason code 0xFF"},
		{name: "single hex digit", err: PublishError{ReasonCode: 0x01}, want: "MQTT publish failed: reason code 0x01"},
		{
			name: "special characters",
			err:  PublishError{ReasonCode: 0x8C, ReasonString: "Topic Name invalid: /test/# (wildcards not allowed)"},
			want: "MQTT publish failed: reason code 0x8C - Topic Name invalid: /test/# (wildcards not allowed)",
		},
		{name: "whitespace reason", err: PublishError{ReasonCode: 0x20, ReasonString: "   "}, want: "MQTT publish failed: reason code 0x20 -    "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

// TestIsFailureCode verifies that reason codes from 0x80 up are failures and
// lower codes are not.
func TestIsFailureCode(t *testing.T) {
	t.Parallel()

	// tests lists reason codes on both sides of the 0x80 boundary.
	tests := []struct {
		code byte
		want bool
	}{
		{code: 0x00},
		{code: 0x01},
		{code: 0x02},
		{code: 0x10},
		{code: 0x7F},
		{code: 0x80, want: true},
		{code: 0x81, want: true},
		{code: 0x82, want: true},
		{code: 0x87, want: true},
		{code: 0x8C, want: true},
		{code: 0x90, want: true},
		{code: 0xFF, want: true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("0x%02X", tt.code), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, IsFailureCode(tt.code))
		})
	}
}

// TestSentinelErrors verifies the sentinel error messages and that the
// sentinels are distinct.
func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	require.EqualError(t, ErrPublishTimeout, "publish timeout exceeded")
	require.EqualError(t, ErrConnectionNotInitialized, "MQTT connection manager not initialized")
	require.EqualError(t, ErrNoDialContext, "custom dial context is not configured")
	assert.NotErrorIs(t, ErrPublishTimeout, ErrConnectionNotInitialized)
}
