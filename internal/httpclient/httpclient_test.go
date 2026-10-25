package httpclient

import (
	"crypto/tls"
	"net/http"
	"sync"
	"testing"

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

// TestHolderPrefer verifies that Prefer returns a set client over the
// holder's, and the holder's client when the given one is nil or a nil
// *http.Client.
func TestHolderPrefer(t *testing.T) {
	t.Parallel()

	// tests lists the clients passed to Prefer, with whether Prefer should
	// return them.
	tests := []struct {
		client types.HTTPClient
		name   string
		want   bool
	}{
		{name: "set client", client: &http.Client{}, want: true},
		{name: "nil", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var holder Holder

			injected := &http.Client{}
			holder.Set(injected)

			if tt.want {
				assert.Same(t, tt.client, holder.Prefer(tt.client))

				return
			}

			assert.Same(t, injected, holder.Prefer(tt.client))
		})
	}
}

// TestHolderDefaultClient verifies that the default client has no timeout of
// its own and sends through http.DefaultTransport, so it honors the proxy
// environment variables.
func TestHolderDefaultClient(t *testing.T) {
	t.Parallel()

	var holder Holder

	client := requireHTTPClient(t, holder.Client())
	assert.Zero(t, client.Timeout)
	assert.Nil(t, client.Transport)
}

// TestHolderInsecureClient verifies which client InsecureClient returns. It
// returns the injected client when one is set. Otherwise it returns one
// default client that skips verification, separate from the verifying default
// and kept across Set calls.
func TestHolderInsecureClient(t *testing.T) {
	t.Parallel()

	t.Run("insecure default built once and reused", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		first := requireHTTPClient(t, holder.InsecureClient(nil))
		assert.Same(t, first, holder.InsecureClient(nil))
		assert.NotSame(t, holder.Client(), first)
		assert.Zero(t, first.Timeout)

		httpTransport, ok := first.Transport.(*http.Transport)
		require.True(t, ok, "transport is a %T", first.Transport)
		assert.True(t, httpTransport.TLSClientConfig.InsecureSkipVerify)
		assert.Equal(t, uint16(tls.VersionTLS12), httpTransport.TLSClientConfig.MinVersion)
	})

	t.Run("injected client preferred", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		injected := &http.Client{}
		holder.Set(injected)

		assert.Same(t, injected, holder.InsecureClient(nil))
	})

	t.Run("insecure default kept across Set", func(t *testing.T) {
		t.Parallel()

		var holder Holder

		first := holder.InsecureClient(nil)

		holder.Set(&http.Client{})
		holder.Set(nil)
		assert.Same(t, first, holder.InsecureClient(nil))
	})
}

// TestHolderInsecureClientWarns verifies that InsecureClient calls the
// warning callback each time it returns the insecure default client, and never
// for an injected client.
func TestHolderInsecureClientWarns(t *testing.T) {
	t.Parallel()

	var (
		holder Holder
		warned int
	)

	warn := func() { warned++ }

	holder.InsecureClient(warn)
	holder.InsecureClient(warn)
	assert.Equal(t, 2, warned)

	holder.Set(&http.Client{})
	holder.InsecureClient(warn)
	assert.Equal(t, 2, warned, "an injected client must not warn")

	holder.Set((*http.Client)(nil))
	holder.InsecureClient(warn)
	assert.Equal(t, 3, warned)
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

// TestHolderConcurrentUse verifies that every Holder method can run at the
// same time, which the race detector checks. Every Client, Prefer, and
// InsecureClient result is the injected client or a default client.
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
	results := make([][]types.HTTPClient, goroutines)
	insecureResults := make([][]types.HTTPClient, goroutines)

	for g := range goroutines {
		wg.Go(func() {
			for i := range iterations {
				switch (g + i) % 5 {
				case 0:
					holder.Set(injected)
				case 1:
					holder.Set(nil)
				case 2:
					results[g] = append(results[g], holder.Prefer(nil))
				case 3:
					insecureResults[g] = append(insecureResults[g], holder.InsecureClient(func() {}))
				default:
					results[g] = append(results[g], holder.Client())
				}
			}
		})
	}

	wg.Wait()

	for g := range goroutines {
		for _, client := range results[g] {
			if client == injected {
				continue
			}

			httpClient := requireHTTPClient(t, client)
			assert.Zero(t, httpClient.Timeout)
			assert.Nil(t, httpClient.Transport)
		}

		for _, client := range insecureResults[g] {
			if client == injected {
				continue
			}

			httpClient := requireHTTPClient(t, client)
			assert.Zero(t, httpClient.Timeout)
			assert.NotNil(t, httpClient.Transport)
		}
	}
}
