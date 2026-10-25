package telegram

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientFallback verifies that a client without an HTTP client of its own
// reuses one fallback client with the request timeout, so its requests share
// connections.
func TestClientFallback(t *testing.T) {
	t.Parallel()

	client := &Client{}

	fallback, ok := client.httpClientOrDefault().(*http.Client)
	require.True(t, ok, "the fallback should be an *http.Client")
	assert.Equal(t, defaultHTTPTimeout, fallback.Timeout)
	assert.Same(t, fallback, client.httpClientOrDefault())
	assert.Same(t, fallback, (&Client{}).httpClientOrDefault())
}
