package gotify

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestValidateToken verifies that legacy tokens of 15 allowed characters
// starting with A and Gotify 3 tokens with the gtfy prefix, an identifier,
// and at least 43 key characters are accepted, and that other values are
// rejected.
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
		{name: "Gotify 3 client token", token: "gtfyc." + strings.Repeat("a", 43), want: true},
		{name: "Gotify 3 signed token", token: enhancedToken + ".1760500000." + strings.Repeat("s", 86), want: true},
		{name: "empty"},
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

// TestValidateDate verifies that RFC 3339 dates with or without a zone, Unix
// seconds, and basic date-times become RFC 3339 dates in UTC, that an empty
// date stays empty, and that other values are rejected.
func TestValidateDate(t *testing.T) {
	t.Parallel()

	// tests lists dates with the RFC 3339 date each should produce.
	tests := []struct {
		date string
		want string
	}{
		{date: "", want: ""},
		{date: "2023-12-25T10:00:00Z", want: "2023-12-25T10:00:00Z"},
		{date: "2023-12-25T10:00:00+02:00", want: "2023-12-25T08:00:00Z"},
		{date: "2023-12-25T10:00:00-08:00", want: "2023-12-25T18:00:00Z"},
		{date: "2023-12-25T10:00:00.123456789Z", want: "2023-12-25T10:00:00Z"},
		{date: "2023-12-25T10:00:00", want: "2023-12-25T10:00:00Z"},
		{date: "1703498400", want: "2023-12-25T10:00:00Z"},
		{date: "0001703498400", want: "2023-12-25T10:00:00Z"},
		{date: "0", want: "1970-01-01T00:00:00Z"},
		{date: "-86400", want: "1969-12-31T00:00:00Z"},
		{date: "2023-12-25 10:00:00", want: "2023-12-25T10:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			t.Parallel()

			date, err := (&DefaultValidator{}).ValidateDate(tt.date)

			require.NoError(t, err)
			assert.Equal(t, tt.want, date)
		})
	}

	for _, invalid := range []string{"tomorrow", "2023-12-25", "10:00:00", "2023/12/25 10:00:00", "2023-12-25T10:00:00Zextra", "1703498400.5", "2023-13-25T10:00:00Z"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			_, err := (&DefaultValidator{}).ValidateDate(invalid)

			require.ErrorIs(t, err, ErrInvalidDate)
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
