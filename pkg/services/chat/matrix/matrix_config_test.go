package matrix

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseTestURL parses rawURL and fails the test if it is invalid.
//
// Parameters:
//   - t: the test.
//   - rawURL: the URL to parse.
//
// Returns:
//   - *url.URL: the parsed URL.
func parseTestURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	return parsed
}

// TestConfigGetURL verifies that GetURL encodes the scheme, host, and
// credentials and forces a query separator. Its subtest checks that a
// skiptlsverify from the URL is written back.
func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	result := (&Config{Host: "matrix.example.com", User: "testuser", Password: "testpass"}).GetURL()

	assert.Equal(t, Scheme, result.Scheme)
	assert.Equal(t, "matrix.example.com", result.Host)
	assert.Equal(t, "testuser", result.User.Username())

	password, hasPassword := result.User.Password()
	assert.True(t, hasPassword)
	assert.Equal(t, "testpass", password)
	assert.True(t, result.ForceQuery)

	t.Run("skiptlsverify round trip", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		require.NoError(t, config.SetURL(parseTestURL(t, "matrix://testuser:testpass@matrix.example.com?skiptlsverify=yes")))

		assert.True(t, config.SkipTLSVerify)
		assert.Equal(t, "skiptlsverify=Yes", config.GetURL().RawQuery)
	})
}

// TestConfigSetURL verifies that SetURL reads the host and credentials and
// prefixes room aliases with # unless they already start with # or !.
func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	t.Run("host and credentials", func(t *testing.T) {
		t.Parallel()

		config := &Config{}
		require.NoError(t, config.SetURL(parseTestURL(t, "matrix://testuser:testpass@matrix.example.com?disableTLS=true")))

		assert.Equal(t, "matrix.example.com", config.Host)
		assert.Equal(t, "testuser", config.User)
		assert.Equal(t, "testpass", config.Password)
	})

	// rooms lists room query values with the room each should become.
	rooms := []struct {
		name  string
		query string
		want  string
	}{
		{name: "alias without prefix", query: "testroom", want: "#testroom"},
		{name: "room ID", query: "!testroom:matrix.example.com", want: "!testroom:matrix.example.com"},
		{name: "alias with prefix", query: "%23myroom:matrix.example.com", want: "#myroom:matrix.example.com"},
	}

	for _, tt := range rooms {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &Config{}
			require.NoError(t, config.SetURL(parseTestURL(t, "matrix://testuser:testpass@matrix.example.com?room="+tt.query)))

			require.NotEmpty(t, config.Rooms)
			assert.Equal(t, tt.want, config.Rooms[0])
		})
	}
}

// TestConfigSetURLRejectsIncompleteURLs verifies that SetURL requires a host
// and a password or access token.
func TestConfigSetURLRejectsIncompleteURLs(t *testing.T) {
	t.Parallel()

	// tests lists incomplete URLs with the error each should produce.
	tests := []struct {
		name    string
		rawURL  string
		wantErr error
	}{
		{name: "no host", rawURL: "matrix://testuser:testpass@", wantErr: ErrMissingHost},
		{name: "no password", rawURL: "matrix://testuser@matrix.example.com", wantErr: ErrMissingCredentials},
		{name: "no user info", rawURL: "matrix://matrix.example.com", wantErr: ErrMissingCredentials},
		{name: "empty password", rawURL: "matrix://testuser:@matrix.example.com", wantErr: ErrMissingCredentials},
		{name: "empty access token", rawURL: "matrix://:@matrix.example.com", wantErr: ErrMissingCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, (&Config{}).SetURL(parseTestURL(t, tt.rawURL)), tt.wantErr)
		})
	}
}
