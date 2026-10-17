package smtp

import (
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewOAuth2Auth verifies that the mechanism keeps the username, access
// token, and host.
func TestNewOAuth2Auth(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		&oauth2Auth{username: "alice", accessToken: "ya29.token", host: "smtp.gmail.com"},
		newOAuth2Auth("alice", "ya29.token", "smtp.gmail.com"))
}

// TestOAuth2AuthStart verifies that XOAUTH2 sends the user and Bearer token as
// its initial response over TLS or to a loopback host, and refuses an
// unencrypted remote server or another host without sending the token.
func TestOAuth2AuthStart(t *testing.T) {
	t.Parallel()

	// tests lists hosts and servers with the error each should produce, or nil.
	tests := []struct {
		wantErr error
		server  smtp.ServerInfo
		name    string
		host    string
	}{
		{name: "TLS", host: "smtp.gmail.com", server: smtp.ServerInfo{Name: "smtp.gmail.com", TLS: true}},
		{name: "localhost without TLS", host: "localhost", server: smtp.ServerInfo{Name: "localhost"}},
		{name: "127.0.0.1 without TLS", host: "127.0.0.1", server: smtp.ServerInfo{Name: "127.0.0.1"}},
		{name: "::1 without TLS", host: "::1", server: smtp.ServerInfo{Name: "::1"}},
		{name: "remote server without TLS", host: "smtp.gmail.com", server: smtp.ServerInfo{Name: "smtp.gmail.com"}, wantErr: errUnencryptedConnection},
		{name: "another host", host: "smtp.gmail.com", server: smtp.ServerInfo{Name: "evil.example.com", TLS: true}, wantErr: errWrongHostName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mech, resp, err := newOAuth2Auth("alice", "SECRETtoken", tt.host).Start(&tt.server)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, mech)
				assert.Nil(t, resp)
				assert.NotContains(t, err.Error(), "SECRET")

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "XOAUTH2", mech)
			assert.Equal(t, "user=alice\x01auth=Bearer SECRETtoken\x01\x01", string(resp))
		})
	}
}

// TestOAuth2AuthNext verifies that XOAUTH2 answers a server challenge, which
// carries an error status, with an empty response so the server can report
// the failure, and sends nothing once the exchange is done.
func TestOAuth2AuthNext(t *testing.T) {
	t.Parallel()

	auth := newOAuth2Auth("alice", "ya29.token", "smtp.gmail.com")

	resp, err := auth.Next([]byte(`{"status":"400","schemes":"Bearer","scope":"https://mail.google.com/"}`), true)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp)

	resp, err = auth.Next(nil, false)
	require.NoError(t, err)
	assert.Nil(t, resp)
}
