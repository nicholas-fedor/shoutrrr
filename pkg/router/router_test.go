package router

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCustomURL is a custom URL for a service that supports custom URLs.
const mockCustomURL = "generic+https://example.com/webhook"

// newTestRouter returns a router without services whose log output goes to the
// test's output.
//
// Parameters:
//   - t: the test that receives the router's log output.
//
// Returns:
//   - *ServiceRouter: an empty router with a logger.
func newTestRouter(t *testing.T) *ServiceRouter {
	t.Helper()

	return &ServiceRouter{logger: log.New(t.Output(), "", 0)}
}

// TestNew verifies that the deprecated New still builds a working router, as
// NewWithOptions does with default options.
func TestNew(t *testing.T) {
	t.Parallel()

	r, err := New(log.New(t.Output(), "", 0), "logger://")
	require.NoError(t, err)
	assert.Len(t, r.services, 1)
}

// TestExtractServiceName verifies that ExtractServiceName returns the service
// scheme for plain and custom URLs, keeps a non-empty fragment, and rejects
// URLs without a valid scheme or with an empty fragment.
func TestExtractServiceName(t *testing.T) {
	t.Parallel()

	// tests lists URLs with the service name they should resolve to, or the
	// error they should fail with.
	tests := []struct {
		wantErr  error
		name     string
		rawURL   string
		want     string
		wantFrag string
	}{
		{name: "plain URL", rawURL: "slack://rest/of/url", want: "slack"},
		{name: "custom URL", rawURL: "teams+https://rest/of/url", want: "teams"},
		{name: "non-empty fragment", rawURL: "rocketchat://host/token/a/b#@user", want: "rocketchat", wantFrag: "@user"},
		{name: "missing scheme", rawURL: "://rest/of/url", wantErr: ErrParseURLFailed},
		{name: "scheme with a space", rawURL: "a d://rest/of/url", wantErr: ErrParseURLFailed},
		{name: "empty fragment", rawURL: "mqtt://broker/home/#", wantErr: ErrEmptyFragment},
		{name: "empty fragment after a query", rawURL: "mqtt://broker/home/?qos=1#", wantErr: ErrEmptyFragment},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceName, serviceURL, err := newTestRouter(t).ExtractServiceName(tt.rawURL)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, serviceName)

				return
			}

			assert.Equal(t, tt.wantFrag, serviceURL.Fragment)

			require.NoError(t, err)
			assert.Equal(t, tt.want, serviceName)
		})
	}
}

// TestInitServiceCustomURL verifies that initService accepts a custom URL only
// for a service that supports custom URLs, with or without a logger, and rejects
// a custom URL for an unknown service.
func TestInitServiceCustomURL(t *testing.T) {
	t.Parallel()

	t.Run("unknown service", func(t *testing.T) {
		t.Parallel()

		service, err := newTestRouter(t).initService("log+https://hybr.is")
		require.ErrorIs(t, err, ErrUnknownService)
		assert.Nil(t, service)
	})

	t.Run("unsupported service", func(t *testing.T) {
		t.Parallel()

		service, err := newTestRouter(t).initService("logger+https://hybr.is")
		require.ErrorIs(t, err, ErrCustomURLsNotSupported)
		assert.Nil(t, service)
	})

	t.Run("supported service", func(t *testing.T) {
		t.Parallel()

		service, err := newTestRouter(t).initService(mockCustomURL)
		require.NoError(t, err)
		assert.NotNil(t, service)
	})

	t.Run("router without a logger", func(t *testing.T) {
		t.Parallel()

		service, err := (&ServiceRouter{}).initService(mockCustomURL)
		require.NoError(t, err)
		assert.NotNil(t, service)
	})
}

// TestEnqueue verifies that Enqueue adds messages to the queue, applying any
// format arguments.
func TestEnqueue(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t)

	r.Enqueue("message body")
	r.Enqueue("message with number %d", 5)

	assert.Equal(t, []string{"message body", "message with number 5"}, r.queue)
}

// TestFlushWithoutServices verifies that a deferred Flush on a router without
// services does not panic.
func TestFlushWithoutServices(t *testing.T) {
	t.Parallel()

	r := newTestRouter(t)

	assert.NotPanics(t, func() {
		defer r.Flush(nil)

		r.Enqueue("message")
	})
}
