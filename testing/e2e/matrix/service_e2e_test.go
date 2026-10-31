package e2e_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/matrix"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// TestMatrixInitialization verifies that a service initializes from valid
// credentials, that a real Matrix server rejects invalid ones on the first
// send, and that a URL without a host or credentials is rejected.
//
//nolint:paralleltest // Talks to a live server one test at a time.
func TestMatrixInitialization(t *testing.T) {
	t.Run("valid credentials from environment", func(t *testing.T) {
		service := getSharedService(t)

		require.NotNil(t, service.Config)
		assert.NotEmpty(t, service.Config.Host)
	})

	t.Run("invalid credentials", func(t *testing.T) {
		parsedURL, err := url.Parse(requireServiceURL(t))
		require.NoError(t, err)

		// The configured server, with credentials it rejects.
		parsedURL.User = url.UserPassword("invaliduser", "invalidpassword")

		service := newService(t, parsedURL.String())

		// The login runs on the first send, where the server rejects the credentials.
		err = service.Send("E2E Test: invalid credentials", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid username or password")
	})

	t.Run("missing host", func(t *testing.T) {
		parsedURL, err := url.Parse("matrix://user:password@")
		require.NoError(t, err)

		require.Error(t, initErr(t, parsedURL))
	})

	t.Run("missing credentials", func(t *testing.T) {
		parsedURL, err := url.Parse("matrix://localhost:8008")
		require.NoError(t, err)

		require.Error(t, initErr(t, parsedURL))
	})
}

// TestMatrixSend verifies that messages reach a real Matrix room, with or
// without a title, and that a service without a homeserver fails to send.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestMatrixSend(t *testing.T) {
	t.Run("basic text message", func(t *testing.T) {
		require.NoError(t, getSharedService(t).Send("E2E Test: Basic text message", nil))
	})

	t.Run("custom title", func(t *testing.T) {
		require.NoError(t, getSharedService(t).Send(
			"E2E Test: Message with custom title",
			&types.Params{"title": "TestNotification"},
		))
	})

	t.Run("fails when the client is not initialized", func(t *testing.T) {
		// A service given only a config has no homeserver to send to.
		service := &matrix.Service{Config: &matrix.Config{Host: "localhost"}}

		require.ErrorIs(t, service.Send("This should fail", nil), matrix.ErrClientNotInitialized)
	})
}

// TestMatrixConfiguration verifies that a room given without its # prefix is
// stored as a room alias.
func TestMatrixConfiguration(t *testing.T) {
	t.Parallel()

	service := newService(t, "matrix://user:pass@localhost:8008?room=testroom&disableTLS=true")

	assert.Equal(t, []string{"#testroom"}, service.Config.Rooms)
}
