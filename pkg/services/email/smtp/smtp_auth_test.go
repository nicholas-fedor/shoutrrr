package smtp

import (
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewAuth verifies the mechanism each authentication method selects, by
// the AUTH exchange it starts with a TLS server at the configured host.
func TestNewAuth(t *testing.T) {
	t.Parallel()

	server := &smtp.ServerInfo{Name: "mail.example.com", TLS: true}

	// tests lists methods with the mechanism name and initial response each
	// should start with.
	tests := []struct {
		name     string
		wantMech string
		wantResp string
		auth     authType
	}{
		{name: "Plain", auth: AuthPlain, wantMech: "PLAIN", wantResp: "\x00user\x00pass"},
		{name: "CRAMMD5", auth: AuthCRAMMD5, wantMech: "CRAM-MD5"},
		{name: "OAuth2", auth: AuthOAuth2, wantMech: "XOAUTH2", wantResp: "user=user\x01auth=Bearer pass\x01\x01"},
		{name: "Login", auth: AuthLogin, wantMech: "LOGIN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			auth, err := newAuth(&Config{Auth: tt.auth, Username: "user", Password: "pass", Host: "mail.example.com"})
			require.NoError(t, err)
			require.NotNil(t, auth)

			mech, resp, err := auth.Start(server)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMech, mech)
			assert.Equal(t, tt.wantResp, string(resp))
		})
	}

	t.Run("None", func(t *testing.T) {
		t.Parallel()

		auth, err := newAuth(&Config{Auth: AuthNone, Username: "user", Password: "pass"})

		require.NoError(t, err)
		assert.Nil(t, auth)
	})
}

// TestNewAuthErrors verifies that an unset or unknown authentication method
// fails with [FailAuthType], naming the method.
func TestNewAuthErrors(t *testing.T) {
	t.Parallel()

	// tests lists methods with the error each should produce.
	tests := []struct {
		name      string
		wantError string
		auth      authType
	}{
		{name: "Unknown", auth: AuthUnknown, wantError: "invalid authentication method 'Unknown'"},
		{name: "out of range", auth: authType(99), wantError: "invalid authentication method 'Invalid'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			auth, err := newAuth(&Config{Auth: tt.auth})

			assert.Nil(t, auth)
			require.ErrorIs(t, err, fail(FailAuthType, nil))
			assert.EqualError(t, err, tt.wantError)
		})
	}
}
