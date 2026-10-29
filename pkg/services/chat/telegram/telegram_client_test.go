package telegram

import (
	"io"
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

// TestClientGetUpdates verifies the getUpdates request body. Nil allowed
// updates leave the parameter out, which the Bot API reads as keeping the
// previous setting. An empty list is sent, because it asks for the default
// update types.
func TestClientGetUpdates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		allowedUpdates []string
		wantBody       string
	}{
		{
			name:           "nil keeps the previous setting",
			allowedUpdates: nil,
			wantBody:       `{"offset":3,"limit":10,"timeout":5}`,
		},
		{
			name:           "empty list asks for the default types",
			allowedUpdates: []string{},
			wantBody:       `{"allowed_updates":[],"offset":3,"limit":10,"timeout":5}`,
		},
		{
			name:           "listed types",
			allowedUpdates: []string{"message"},
			wantBody:       `{"allowed_updates":["message"],"offset":3,"limit":10,"timeout":5}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var body string

			client := &Client{
				httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					data, err := io.ReadAll(req.Body)
					if err != nil {
						return nil, err
					}

					body = string(data)

					return newResponse(req, http.StatusOK, `{"ok":true,"result":[]}`), nil
				})},
				token:  "12345:mock-token",
				apiURL: "",
			}

			updates, err := client.GetUpdates(3, 10, 5, tt.allowedUpdates)
			require.NoError(t, err)
			assert.Empty(t, updates)
			assert.JSONEq(t, tt.wantBody, body)
		})
	}
}
