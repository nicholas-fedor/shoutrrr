package gotify

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestValidateToken verifies that application tokens are accepted. Those are
// legacy tokens of 15 allowed characters starting with A, and Gotify 3 tokens
// with the gtfya. prefix and at least 43 key characters. Client tokens, which
// Gotify does not take for a message without an application ID, are rejected
// in both forms, as are other values.
func TestValidateToken(t *testing.T) {
	t.Parallel()

	// tests lists tokens with whether each should be accepted.
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{name: "legacy token", token: testToken, want: true},
		{name: "legacy token with dashes and underscores", token: "Aaa-bbb_ccc.ddd", want: true},
		{name: "Gotify 3 application token", token: enhancedToken, want: true},
		{name: "Gotify 3 signed token", token: enhancedToken + ".1760500000." + strings.Repeat("s", 86), want: true},
		{name: "empty"},
		{name: "Gotify 3 client token", token: clientToken},
		{name: "legacy client token", token: legacyClientToken},
		{name: "Gotify 3 token with another identifier", token: "gtfyx." + strings.Repeat("a", 43)},
		{name: "legacy token too short", token: "Aaa.bbb.ccc.dd"},
		{name: "legacy token too long", token: "Aaa.bbb.ccc.ddde"},
		{name: "legacy token without the A prefix", token: "Baa.bbb.ccc.ddd"},
		{name: "legacy token with an invalid first character", token: "A!a.bbb.ccc.ddd"},
		{name: "legacy token with an invalid middle character", token: "Aaa.bb!.ccc.ddd"},
		{name: "legacy token with an invalid last character", token: "Aaa.bbb.ccc.dd!"},
		{name: "Gotify 3 token without an identifier", token: "gtfy." + strings.Repeat("a", 44)},
		{name: "Gotify 3 token with an empty identifier", token: "gtfy.." + strings.Repeat("a", 43)},
		{name: "Gotify 3 token too short", token: "gtfya." + strings.Repeat("a", 42)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, (&DefaultValidator{}).ValidateToken(tt.token))
		})
	}
}

// TestValidatePriority verifies that priorities from -2 to 10 are accepted.
func TestValidatePriority(t *testing.T) {
	t.Parallel()

	validator := &DefaultValidator{}

	for _, valid := range []int{-2, 0, 10} {
		require.NoError(t, validator.ValidatePriority(valid), valid)
	}

	for _, invalid := range []int{-3, 11} {
		require.ErrorIs(t, validator.ValidatePriority(invalid), ErrInvalidPriority, invalid)
	}
}

// TestValidateMessage verifies that an empty message is rejected.
func TestValidateMessage(t *testing.T) {
	t.Parallel()

	validator := &DefaultValidator{}

	require.NoError(t, validator.ValidateMessage("Hello"))
	require.ErrorIs(t, validator.ValidateMessage(""), ErrEmptyMessage)
}

// TestValidateServiceInitialized verifies that a missing config is rejected.
func TestValidateServiceInitialized(t *testing.T) {
	t.Parallel()

	validator := &DefaultValidator{}

	require.NoError(t, validator.ValidateServiceInitialized(&Config{}))
	require.ErrorIs(t, validator.ValidateServiceInitialized(nil), types.ErrServiceNotInitialized)
}
