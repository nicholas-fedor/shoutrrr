package matrix_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

func TestConfigSetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceURL string
	}{
		{
			name:       "basic URL with password",
			serviceURL: "matrix://user:token@matrix.example.com",
		},
		{
			name:       "URL with special characters in password",
			serviceURL: "matrix://user:p%40ssw0rd!@matrix.example.com",
		},
		{
			name:       "URL with port",
			serviceURL: "matrix://user:token@matrix.example.com:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &matrix.Config{}

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			err = config.SetURL(parsedURL)

			require.NoError(t, err, "Expected no error for %s", tt.name)
			require.NotEmpty(t, config.Password, "Password should be set for %s", tt.name)
		})
	}
}

func TestConfigSetURLErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceURL string
		errMsg     string
	}{
		{
			name:       "missing password",
			serviceURL: "matrix://matrix.example.com",
			errMsg:     "password or access token is required",
		},
		{
			name:       "empty host",
			serviceURL: "matrix://user:token@",
			errMsg:     "host is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &matrix.Config{}

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			err = config.SetURL(parsedURL)

			require.Error(t, err, "Expected error for %s", tt.name)
			require.Contains(t, err.Error(), tt.errMsg,
				"Error message should contain %q", tt.errMsg)
		})
	}
}

func TestConfigGetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		serviceURL   string
		expectedHost string
		expectedUser string
		expectedPass string
	}{
		{
			name:         "basic config",
			serviceURL:   "matrix://user:token@matrix.example.com",
			expectedHost: "matrix.example.com",
			expectedUser: "user",
			expectedPass: "token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &matrix.Config{}

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			err = config.SetURL(parsedURL)
			require.NoError(t, err, "Expected no error for %s", tt.name)

			resultURL := config.GetURL()

			require.Equal(t, tt.expectedHost, resultURL.Host,
				"Host mismatch for %s", tt.name)

			require.Equal(t, tt.expectedUser, resultURL.User.Username(),
				"User mismatch for %s", tt.name)

			actualPass, ok := resultURL.User.Password()
			require.True(t, ok, "Password should be present for %s", tt.name)
			require.Equal(t, tt.expectedPass, actualPass,
				"Password mismatch for %s", tt.name)
		})
	}
}

func TestConfigQueryParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceURL string
		wantTitle  string
		wantTLS    bool
	}{
		{
			name:       "with title",
			serviceURL: "matrix://user:token@matrix.example.com?title=MyTitle",
			wantTitle:  "MyTitle",
			wantTLS:    false,
		},
		{
			name:       "with TLS disabled",
			serviceURL: "matrix://user:token@matrix.example.com?disableTLS=true",
			wantTitle:  "",
			wantTLS:    true,
		},
		{
			name:       "with title and TLS",
			serviceURL: "matrix://user:token@matrix.example.com?title=Alert&disableTLS=true",
			wantTitle:  "Alert",
			wantTLS:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &matrix.Config{}

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			err = config.SetURL(parsedURL)
			require.NoError(t, err, "Expected no error for %s", tt.name)

			require.Equal(t, tt.wantTitle, config.Title,
				"Title mismatch for %s", tt.name)
			require.Equal(t, tt.wantTLS, config.DisableTLS,
				"DisableTLS mismatch for %s", tt.name)
		})
	}
}

func TestConfigWithRoomsDirect(t *testing.T) {
	t.Parallel()

	// Test setting rooms directly on config (not via URL)
	config := &matrix.Config{
		Rooms: []string{"#room1:example.com", "!room2:example.com"},
	}

	require.Len(t, config.Rooms, 2)
	require.Equal(t, "#room1:example.com", config.Rooms[0])
	require.Equal(t, "!room2:example.com", config.Rooms[1])
}
