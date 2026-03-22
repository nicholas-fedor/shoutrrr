package matrix_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
)

// TestServiceInitializeWithTokenOnly tests the token-only authentication flow.
// This tests the URL format matrix://:syt_xxx@matrix.example.com where
// the username is empty and the password is the access token.
// We test via Config directly to avoid network calls.
func TestServiceInitializeWithTokenOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceURL string
		wantUser   string
		wantPass   string
	}{
		{
			name:       "token-only auth with access token",
			serviceURL: "matrix://:syt_xxx_token@matrix.example.com",
			wantUser:   "",
			wantPass:   "syt_xxx_token",
		},
		{
			name:       "token-only auth with longer token",
			serviceURL: "matrix://:syt_1234567890abcdefghijklmnopqrstuvwxyz@matrix.example.com",
			wantUser:   "",
			wantPass:   "syt_1234567890abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:       "token-only auth with special chars in token",
			serviceURL: "matrix://:syt_test-token_123@matrix.example.com",
			wantUser:   "",
			wantPass:   "syt_test-token_123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			// Use Config directly to avoid network calls
			config := &matrix.Config{}
			err = config.SetURL(parsedURL)

			require.NoError(t, err, "Expected no error for %s", tt.name)
			require.Equal(t, tt.wantUser, config.User,
				"User should be empty for token-only auth in %s", tt.name)
			require.Equal(t, tt.wantPass, config.Password,
				"Password should be set to token for %s", tt.name)
		})
	}
}

// TestServiceInitializeWithUserAndToken tests that when both user and token
// are provided via config (not via service), the config is set correctly.
func TestServiceInitializeWithUserAndToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serviceURL string
		wantUser   string
		wantPass   string
	}{
		{
			name:       "user and token provided",
			serviceURL: "matrix://user:syt_xxx@matrix.example.com",
			wantUser:   "user",
			wantPass:   "syt_xxx",
		},
		{
			name:       "user with token as password",
			serviceURL: "matrix://myuser:$ecret_t0k3n@matrix.example.com",
			wantUser:   "myuser",
			wantPass:   "$ecret_t0k3n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			// Use Config directly to avoid network calls
			config := &matrix.Config{}
			err = config.SetURL(parsedURL)

			require.NoError(t, err, "Expected no error for %s", tt.name)
			require.Equal(t, tt.wantUser, config.User,
				"User mismatch for %s", tt.name)
			require.Equal(t, tt.wantPass, config.Password,
				"Password mismatch for %s", tt.name)
		})
	}
}

// TestServiceInitializeTokenOnlyWithRooms tests token-only auth with room configuration.
// Rooms are set directly on the config struct, not via URL params (since URL only supports one room).
func TestServiceInitializeTokenOnlyWithRooms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serviceURL    string
		rooms         []string
		expectedRooms []string
	}{
		{
			name:          "token-only with room set directly",
			serviceURL:    "matrix://:syt_xxx@matrix.example.com",
			rooms:         []string{"#test:example.com"},
			expectedRooms: []string{"#test:example.com"},
		},
		{
			name:          "token-only with multiple rooms set directly",
			serviceURL:    "matrix://:syt_xxx@matrix.example.com",
			rooms:         []string{"#room1:example.com", "#room2:example.com"},
			expectedRooms: []string{"#room1:example.com", "#room2:example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURL, err := url.Parse(tt.serviceURL)
			require.NoError(t, err, "URL should be parseable for %s", tt.name)

			// Set rooms directly on config
			config := &matrix.Config{
				Rooms: tt.rooms,
			}
			err = config.SetURL(parsedURL)

			require.NoError(t, err, "Expected no error for %s", tt.name)
			require.Empty(t, config.User,
				"User should be empty for token-only auth in %s", tt.name)
			require.Equal(t, tt.expectedRooms, config.Rooms,
				"Rooms mismatch for %s", tt.name)
		})
	}
}
