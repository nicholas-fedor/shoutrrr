package smtp

import (
	"net/smtp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLoginAuth verifies that the mechanism keeps the credentials and host
// and starts at the username step.
func TestNewLoginAuth(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		&loginAuth{username: "alice", password: "s3cret", host: "mail.example.com"},
		newLoginAuth("alice", "s3cret", "mail.example.com"))
}

// TestLoginAuthStart verifies that LOGIN starts without an initial response
// over TLS or to a loopback host, and refuses an unencrypted remote server or
// another host.
func TestLoginAuthStart(t *testing.T) {
	t.Parallel()

	// tests lists hosts and servers with the error each should produce, or nil.
	tests := []struct {
		wantErr error
		server  smtp.ServerInfo
		name    string
		host    string
	}{
		{name: "TLS", host: "mail.example.com", server: smtp.ServerInfo{Name: "mail.example.com", TLS: true}},
		{name: "localhost without TLS", host: "localhost", server: smtp.ServerInfo{Name: "localhost"}},
		{name: "127.0.0.1 without TLS", host: "127.0.0.1", server: smtp.ServerInfo{Name: "127.0.0.1"}},
		{name: "::1 without TLS", host: "::1", server: smtp.ServerInfo{Name: "::1"}},
		{name: "remote server without TLS", host: "mail.example.com", server: smtp.ServerInfo{Name: "mail.example.com"}, wantErr: errUnencryptedConnection},
		{name: "another host", host: "mail.example.com", server: smtp.ServerInfo{Name: "evil.example.com", TLS: true}, wantErr: errWrongHostName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mech, resp, err := newLoginAuth("alice", "s3cret", tt.host).Start(&tt.server)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, mech)
				assert.Nil(t, resp)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, "LOGIN", mech)
			assert.Nil(t, resp)
		})
	}
}

// TestLoginAuthNext verifies that LOGIN answers the first challenge with the
// username and the second with the password, whatever their text, refuses a
// third, and sends nothing once the exchange is done. Start restarts at the
// username.
func TestLoginAuthNext(t *testing.T) {
	t.Parallel()

	auth := newLoginAuth("alice", "päss word", "localhost")

	resp, err := auth.Next([]byte("User Name\x00"), true)
	require.NoError(t, err)
	assert.Equal(t, "alice", string(resp))

	resp, err = auth.Next([]byte("password:"), true)
	require.NoError(t, err)
	assert.Equal(t, "päss word", string(resp))

	resp, err = auth.Next([]byte("Username:"), true)
	require.ErrorIs(t, err, errUnexpectedServerChallenge)
	require.EqualError(t, err, `unexpected server challenge: "Username:"`)
	assert.Nil(t, resp)

	resp, err = auth.Next(nil, false)
	require.NoError(t, err)
	assert.Nil(t, resp)

	_, _, err = auth.Start(&smtp.ServerInfo{Name: "localhost"})
	require.NoError(t, err)

	resp, err = auth.Next([]byte("Username:"), true)
	require.NoError(t, err)
	assert.Equal(t, "alice", string(resp))
}

// TestIsLocalhost verifies which server names count as loopback hosts.
func TestIsLocalhost(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]bool{
		"localhost": true, "127.0.0.1": true, "::1": true,
		"example.com": false, "": false, "127.0.0.2": false, "LOCALHOST": false,
	} {
		assert.Equal(t, want, isLocalhost(name), name)
	}
}
