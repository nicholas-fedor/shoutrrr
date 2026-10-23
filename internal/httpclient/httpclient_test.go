package httpclient

import (
	"crypto/tls"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// requireHTTPClient asserts that client is an *http.Client and returns it.
//
// Parameters:
//   - t: the test.
//   - client: the client to check.
//
// Returns:
//   - *http.Client: the client.
func requireHTTPClient(t *testing.T, client types.HTTPClient) *http.Client {
	t.Helper()

	httpClient, ok := client.(*http.Client)
	require.True(t, ok, "client is a %T", client)

	return httpClient
}

// TestHolderClient verifies which client a Holder returns: the injected one
// when set, and otherwise one default client built on first use and reused.
func TestHolderClient(t *testing.T) {
	t.Parallel()

	t.Run("default built once and reused", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		first := holder.Client()
		assert.Same(t, first, holder.Client())
	})

	t.Run("injected client preferred", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		injected := &http.Client{}
		holder.Set(injected)

		assert.Same(t, injected, holder.Client())
	})

	// A nil client of any kind clears the injected one, so the default is
	// used again.
	nilClients := []struct {
		name   string
		client types.HTTPClient
	}{
		{name: "nil", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range nilClients {
		t.Run(tt.name+" restores the default", func(t *testing.T) {
			t.Parallel()

			var holder Holder

			defaultClient := holder.Client()

			holder.Set(&http.Client{})
			holder.Set(tt.client)

			assert.Same(t, defaultClient, holder.Client())
		})
	}
}

// TestHolderSetDefault verifies that the default client follows the
// SetDefault settings, that new settings replace a default already built, and
// that an injected client is kept.
func TestHolderSetDefault(t *testing.T) {
	t.Parallel()

	t.Run("zero value", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		client := requireHTTPClient(t, holder.Client())
		assert.Zero(t, client.Timeout)
		assert.Nil(t, client.Transport)
	})

	t.Run("timeout keeps the default transport", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		holder.SetDefault(10*time.Second, nil)

		client := requireHTTPClient(t, holder.Client())
		assert.Equal(t, 10*time.Second, client.Timeout)
		assert.Nil(t, client.Transport, "the client should send through http.DefaultTransport")
	})

	t.Run("TLS settings", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		holder.SetDefault(time.Second, tlsConfig)

		client := requireHTTPClient(t, holder.Client())
		httpTransport, ok := client.Transport.(*http.Transport)
		require.True(t, ok, "transport is a %T", client.Transport)
		assert.Same(t, tlsConfig, httpTransport.TLSClientConfig)
	})

	t.Run("new settings rebuild the default", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		holder.SetDefault(time.Second, nil)
		first := holder.Client()

		holder.SetDefault(2*time.Second, nil)
		second := requireHTTPClient(t, holder.Client())

		assert.NotSame(t, first, second)
		assert.Equal(t, 2*time.Second, second.Timeout)
	})

	t.Run("injected client kept", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		injected := &http.Client{}
		holder.Set(injected)
		holder.SetDefault(time.Second, nil)

		assert.Same(t, injected, holder.Client())
	})
}

// TestHolderConcurrentClient verifies that concurrent Client calls share one
// default client, so concurrent sends never build their own.
func TestHolderConcurrentClient(t *testing.T) {
	t.Parallel()

	var (
		holder Holder
		wg     sync.WaitGroup
	)

	clients := make([]types.HTTPClient, 8)

	for i := range clients {
		wg.Go(func() { clients[i] = holder.Client() })
	}

	wg.Wait()

	for _, client := range clients {
		assert.Same(t, clients[0], client)
	}
}

// TestHolderConcurrentUse verifies that Set, SetDefault, and Client can run
// at the same time, which the race detector checks, and that every Client
// result is the injected client or a default built from settings that some
// SetDefault call used.
func TestHolderConcurrentUse(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 8
		iterations = 200
	)

	var (
		holder Holder
		wg     sync.WaitGroup
	)

	injected := &http.Client{}
	timeouts := []time.Duration{time.Second, 2 * time.Second}
	results := make([][]types.HTTPClient, goroutines)

	for g := range goroutines {
		wg.Go(func() {
			for i := range iterations {
				switch (g + i) % 4 {
				case 0:
					holder.Set(injected)
				case 1:
					holder.Set(nil)
				case 2:
					holder.SetDefault(timeouts[i%len(timeouts)], nil)
				default:
					results[g] = append(results[g], holder.Client())
				}
			}
		})
	}

	wg.Wait()

	for _, clients := range results {
		for _, client := range clients {
			if client == injected {
				continue
			}

			httpClient := requireHTTPClient(t, client)
			assert.Contains(t, append(timeouts, 0), httpClient.Timeout)
			assert.Nil(t, httpClient.Transport)
		}
	}
}
