package httpclient

import (
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// clientFunc adapts a function to [types.HTTPClient], so a nil function is a
// nil client that is not a pointer.
type clientFunc func(req *http.Request) (*http.Response, error)

// derefClient sends through its inner client, so a nil *derefClient panics
// with a nil pointer dereference.
type derefClient struct {
	// inner is the client that sends the requests.
	inner types.HTTPClient
}

// nilSafeClient answers every request itself, so a nil *nilSafeClient works.
type nilSafeClient struct{}

// panicClient panics with an error that is not a runtime error.
type panicClient struct{}

// headerClient records the request method, so a nil headerClient panics with a
// write to a nil map.
type headerClient map[string]string

// sliceClient records the request method, so a nil sliceClient panics with an
// index out of range.
type sliceClient []string

// chanClient closes its channel, so a nil chanClient panics with a close of a
// nil channel.
type chanClient chan struct{}

// trackedBody is a request body that records whether it was closed.
type trackedBody struct {
	io.Reader

	// closed reports whether Close was called.
	closed bool
}

// errCustomPanic is the value panicClient panics with.
var errCustomPanic = errors.New("custom panic")

// Do calls f.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f clientFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

// Do sends req through the inner client.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: the inner client's response.
//   - error: the inner client's error.
func (c *derefClient) Do(req *http.Request) (*http.Response, error) { return c.inner.Do(req) }

// Do answers req with an empty 200 response without reading the receiver.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: an empty 200 response.
//   - error: always nil.
func (*nilSafeClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
}

// Do panics with errCustomPanic.
//
// Parameters:
//   - *http.Request: unused.
//
// Returns:
//   - *http.Response: never returns.
//   - error: never returns.
func (*panicClient) Do(*http.Request) (*http.Response, error) { panic(errCustomPanic) }

// Do records the request method in the map.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: always nil.
//   - error: always nil.
func (c headerClient) Do(req *http.Request) (*http.Response, error) {
	c["method"] = req.Method

	return nil, nil //nolint:nilnil // The tests only reach this with a nil map, which panics first.
}

// Do records the request method in the first element.
//
// Parameters:
//   - req: the request.
//
// Returns:
//   - *http.Response: always nil.
//   - error: always nil.
func (c sliceClient) Do(req *http.Request) (*http.Response, error) {
	c[0] = req.Method

	return nil, nil //nolint:nilnil // The tests only reach this with a nil slice, which panics first.
}

// Do closes the channel.
//
// Parameters:
//   - *http.Request: unused.
//
// Returns:
//   - *http.Response: always nil.
//   - error: always nil.
func (c chanClient) Do(*http.Request) (*http.Response, error) {
	close(c)

	return nil, nil //nolint:nilnil // The tests only reach this with a nil channel, which panics first.
}

// Close records that the body was closed.
//
// Returns:
//   - error: always nil.
func (b *trackedBody) Close() error {
	b.closed = true

	return nil
}

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

// TestHolderNilPointerClients verifies how a holder sends through an injected
// client that is a nil value of a type other than *http.Client. Requests go
// to the client, so a nil receiver that works keeps working. A runtime panic
// from the client fails the request with [types.ErrNilHTTPClient] and closes
// its body. Any other panic is raised again. The default client never takes
// over, so a caller's mistaken nil client cannot route around the client it
// meant to inject.
func TestHolderNilPointerClients(t *testing.T) {
	t.Parallel()

	// clients lists nil clients of each kind that can be nil, with the type
	// name a failure carries, or whether the request succeeds or panics.
	clients := []struct {
		client    types.HTTPClient
		name      string
		typeName  string
		wantOK    bool
		wantPanic bool
	}{
		{name: "nil pointer dereferenced", client: (*derefClient)(nil), typeName: "*httpclient.derefClient"},
		{name: "nil mock", client: (*mocks.MockHTTPClient)(nil), typeName: "*mocks.MockHTTPClient"},
		{name: "nil func", client: clientFunc(nil), typeName: "httpclient.clientFunc"},
		{name: "nil map written", client: headerClient(nil), typeName: "httpclient.headerClient"},
		{name: "nil slice indexed", client: sliceClient(nil), typeName: "httpclient.sliceClient"},
		{name: "nil channel closed", client: chanClient(nil), typeName: "httpclient.chanClient"},
		{name: "nil receiver that works", client: (*nilSafeClient)(nil), wantOK: true},
		{name: "nil receiver with another panic", client: (*panicClient)(nil), wantPanic: true},
	}

	// paths lists how a service obtains the client it sends through.
	paths := []struct {
		get  func(t *testing.T, holder *Holder, client types.HTTPClient) types.HTTPClient
		name string
	}{
		{name: "Set then Client", get: func(_ *testing.T, holder *Holder, client types.HTTPClient) types.HTTPClient {
			holder.Set(client)

			return holder.Client()
		}},
		{name: "Set then InsecureClient", get: func(t *testing.T, holder *Holder, client types.HTTPClient) types.HTTPClient {
			t.Helper()

			holder.Set(client)

			return holder.InsecureClient(func() { t.Error("the insecure warning must not be logged") })
		}},
		{name: "Prefer", get: func(_ *testing.T, holder *Holder, client types.HTTPClient) types.HTTPClient {
			return holder.Prefer(client)
		}},
	}

	for _, tt := range clients {
		for _, path := range paths {
			t.Run(tt.name+"/"+path.name, func(t *testing.T) {
				t.Parallel()

				var holder Holder

				client := path.get(t, &holder, tt.client)

				body := &trackedBody{Reader: strings.NewReader("payload"), closed: false}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.invalid/", body)
				require.NoError(t, err)

				if tt.wantPanic {
					assert.PanicsWithValue(t, errCustomPanic, func() { _, _ = client.Do(req) })

					return
				}

				var res *http.Response

				require.NotPanics(t, func() { res, err = client.Do(req) })

				if tt.wantOK {
					require.NoError(t, err)
					assert.Equal(t, http.StatusOK, res.StatusCode)

					return
				}

				require.ErrorIs(t, err, types.ErrNilHTTPClient)
				assert.Contains(t, err.Error(), tt.typeName)
				assert.Nil(t, res)
				assert.True(t, body.closed, "the request body must be closed")
			})
		}
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
