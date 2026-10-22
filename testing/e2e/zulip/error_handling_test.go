package e2e_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Settings for the Zulip error handling tests.
const (
	// unreachableURL names a server at an address that does not answer.
	unreachableURL = "zulip://bot@example.com:secret-key@10.255.255.1:44399"
	// unreachableTimeout bounds the send to the unreachable server.
	unreachableTimeout = 10 * time.Second
)

// TestZulipErrorHandling verifies that sends fail for an unreachable server, a
// canceled context, invalid credentials, and a stream that does not exist.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestZulipErrorHandling(t *testing.T) {
	t.Run("unreachable server", func(t *testing.T) {
		service := newService(t, unreachableURL)

		ctx, cancel := context.WithTimeout(t.Context(), unreachableTimeout)
		defer cancel()

		require.Error(t, service.SendContext(ctx, "E2E Test: unreachable", nil))
	})

	t.Run("already canceled context", func(t *testing.T) {
		service := newService(t, unreachableURL)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.Error(t, service.SendContext(ctx, "E2E Test: canceled", nil))
	})

	t.Run("invalid credentials", func(t *testing.T) {
		parsedURL, err := url.Parse(requireServiceURL(t))
		require.NoError(t, err)

		parsedURL.User = url.UserPassword("bot@example.com", "wrong-key-invalid-key")

		service := newService(t, parsedURL.String())

		err = service.Send("E2E Test: invalid credentials", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
	})

	t.Run("stream that does not exist", func(t *testing.T) {
		service := newService(t, requireServiceURL(t))

		require.Error(t, service.Send("E2E Test: invalid stream", &types.Params{
			"stream": "this-stream-definitely-does-not-exist-" + strings.Repeat("x", 20),
		}))
	})
}
