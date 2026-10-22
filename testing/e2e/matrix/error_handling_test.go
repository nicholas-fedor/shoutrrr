package e2e_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sendTimeout bounds a send in the timeout test, above the service's own 10
// second HTTP timeout.
const sendTimeout = 15 * time.Second

// TestMatrixErrorHandling verifies how sends to a real Matrix server handle
// canceled and expired contexts, unreachable servers, missing and malformed
// rooms, unresolvable hosts, empty and very long messages, and the HTTP
// timeout.
//
//nolint:paralleltest // Sends to a live server one message at a time.
func TestMatrixErrorHandling(t *testing.T) {
	t.Run("canceled context", func(t *testing.T) {
		service := getSharedService(t)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, service.SendContext(ctx, "Test message with canceled context", nil), context.Canceled)
	})

	t.Run("deadline exceeded context", func(t *testing.T) {
		service := getSharedService(t)

		ctx, cancel := context.WithTimeout(t.Context(), time.Nanosecond)
		defer cancel()

		<-ctx.Done()

		require.ErrorIs(t, service.SendContext(ctx, "Test message with expired deadline", nil), context.DeadlineExceeded)
	})

	t.Run("timeout when the server is unreachable", func(t *testing.T) {
		parsedURL, err := url.Parse("matrix://user:password@localhost:9999?room=%23nonexistent:invalid&disableTLS=true")
		require.NoError(t, err)

		// Initialization may already fail for an unreachable server.
		if initErr(t, parsedURL) != nil {
			return
		}

		service := newService(t, parsedURL.String())

		require.Error(t, service.Send("This should timeout", nil))
	})

	t.Run("room that does not resolve", func(t *testing.T) {
		parsedURL, err := url.Parse(requireServiceURL(t))
		require.NoError(t, err)

		// A room on a server that does not exist, so the send waits on the
		// server to fail the alias lookup.
		query := parsedURL.Query()
		query.Set("room", "#definitely-does-not-exist-12345:invalid.local")
		parsedURL.RawQuery = query.Encode()

		service := newService(t, parsedURL.String())

		require.Error(t, service.Send("Test timeout handling", nil))
	})

	t.Run("room that does not exist", func(t *testing.T) {
		parsedURL, err := url.Parse(requireServiceURL(t))
		require.NoError(t, err)

		query := parsedURL.Query()
		query.Set("room", "#nonexistent-room-that-does-not-exist-12345:invalid.local")
		parsedURL.RawQuery = query.Encode()

		service := newService(t, parsedURL.String())

		err = service.Send("Test message to non-existent room", nil)
		require.Error(t, err)
		assertContainsAny(t, err, "join", "room", "404", "not found")
	})

	t.Run("invalid room alias format", func(t *testing.T) {
		parsedURL, err := url.Parse(requireServiceURL(t))
		require.NoError(t, err)

		// An alias without a server name.
		query := parsedURL.Query()
		query.Set("room", "invalid-room-alias")
		parsedURL.RawQuery = query.Encode()

		service := newService(t, parsedURL.String())

		// The server may accept or reject the alias, so only the absence of a
		// panic is checked.
		_ = service.Send("Test message with invalid room alias", nil)
	})

	t.Run("unreachable server", func(t *testing.T) {
		parsedURL, err := url.Parse("matrix://user:password@localhost:65432?disableTLS=true")
		require.NoError(t, err)

		if err := initErr(t, parsedURL); err != nil {
			assertContainsAny(t, err, "refused", "no such host", "timeout", "i/o timeout")

			return
		}

		service := newService(t, parsedURL.String())

		require.Error(t, service.Send("Test message", nil))
	})

	t.Run("unresolvable host", func(t *testing.T) {
		service := newService(t, "matrix://user:password@this-host-does-not-exist-12345.invalid:8008?disableTLS=true")

		// The login runs on the first send, which fails with a DNS or network error.
		err := service.Send("E2E Test: invalid host", nil)
		require.Error(t, err)
		assertContainsAny(t, err, "no such host", "dial", "timeout")
	})

	t.Run("invalid host without a domain", func(t *testing.T) {
		service := newService(t, "matrix://user:pass@invalid-host-for-testing:9999?disableTLS=true")

		// The login runs on the first send, which fails on the invalid host.
		require.Error(t, service.Send("E2E Test: invalid host", nil))
	})

	t.Run("empty message", func(t *testing.T) {
		require.NoError(t, getSharedService(t).Send("", nil))
	})

	t.Run("very long message", func(t *testing.T) {
		require.NoError(t, getSharedService(t).Send(strings.Repeat("A very long message. ", 1000), nil))
	})

	t.Run("respects the configured timeout", func(t *testing.T) {
		service := newService(t, requireServiceURL(t))

		// The service's HTTP timeout is 10 seconds, so the send must not hang.
		done := make(chan error, 1)

		go func() {
			done <- service.Send("Timeout test message", nil)
		}()

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(sendTimeout):
			require.Fail(t, "send timed out", "the send took longer than %s, expected under 10s", sendTimeout)
		}
	})
}

// assertContainsAny asserts that an error's message contains at least one of
// the given substrings.
//
// Parameters:
//   - t: the test.
//   - err: the error.
//   - substrings: the accepted substrings.
func assertContainsAny(t *testing.T, err error, substrings ...string) {
	t.Helper()

	for _, substring := range substrings {
		if strings.Contains(err.Error(), substring) {
			return
		}
	}

	assert.Failf(t, "unexpected error", "%q contains none of %q", err.Error(), substrings)
}
