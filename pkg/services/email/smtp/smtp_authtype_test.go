package smtp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAuthTypeString verifies the name of each authentication method, which
// the URL uses as the auth value.
func TestAuthTypeString(t *testing.T) {
	t.Parallel()

	// tests lists methods with their names.
	tests := []struct {
		want string
		auth authType
	}{
		{auth: AuthNone, want: "None"},
		{auth: AuthPlain, want: "Plain"},
		{auth: AuthCRAMMD5, want: "CRAMMD5"},
		{auth: AuthUnknown, want: "Unknown"},
		{auth: AuthOAuth2, want: "OAuth2"},
		{auth: AuthLogin, want: "Login"},
		{auth: authType(99), want: "Invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.auth.String())
		})
	}
}

// TestAuthTypes verifies that the enum helper holds each method and parses
// the names regardless of case.
func TestAuthTypes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, &authTypeVals{
		None: AuthNone, Plain: AuthPlain, CRAMMD5: AuthCRAMMD5, Unknown: AuthUnknown, OAuth2: AuthOAuth2, Login: AuthLogin,
		Enum: AuthTypes.Enum,
	}, AuthTypes)
	assert.Equal(t, int(AuthOAuth2), AuthTypes.Enum.Parse("oauth2"))
	assert.Equal(t, int(AuthCRAMMD5), AuthTypes.Enum.Parse("CRAMMD5"))
}
