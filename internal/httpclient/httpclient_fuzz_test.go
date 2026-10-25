package httpclient

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// holderModel is the expected state of a Holder after a sequence of calls.
type holderModel struct {
	// injected is the injected client, or nil.
	injected types.HTTPClient
	// built is the default client Client returned, or nil before Client
	// builds one.
	built types.HTTPClient
	// insecure is the default client InsecureClient returned, or nil before
	// InsecureClient builds one.
	insecure types.HTTPClient
}

// Holder operations a fuzz input byte encodes in its low two bits.
const (
	// opClient calls Client and checks the result against the model.
	opClient byte = iota
	// opSet calls Set with the client that bits 2 and 3 select.
	opSet
	// opPrefer calls Prefer with the client that bits 2 and 3 select and
	// checks the result against the model.
	opPrefer
	// opInsecureClient calls InsecureClient and checks the result against the
	// model, so reads are as common as writes.
	opInsecureClient
)

// FuzzHolder runs a sequence of Set, Prefer, Client, and InsecureClient calls
// on a Holder and checks every result against a model of the expected state:
//
//   - An injected client is returned by both getters, and InsecureClient calls
//     its warning callback only when it returns the insecure default client.
//   - A nil client, or a nil *http.Client, clears the injected client.
//   - Prefer returns a client it is given, unless that client is nil or a nil
//     *http.Client, and returns what Client returns otherwise.
//   - The default client has no timeout and sends through
//     http.DefaultTransport.
//   - The insecure default client has no timeout and skips certificate
//     verification.
//   - Each default client is built once and reused, even across Set calls.
//
// Each input byte is one call. The low two bits pick the operation, and the
// higher bits its argument. The seed corpus covers the zero value, an
// injected client set and cleared both ways, and Prefer with a set client, a
// nil client, and a nil *http.Client before and after the default is built.
func FuzzHolder(f *testing.F) {
	clients := [...]types.HTTPClient{nil, (*http.Client)(nil), &http.Client{}, &http.Client{}}

	set := func(client byte) byte { return opSet | client<<2 }
	prefer := func(client byte) byte { return opPrefer | client<<2 }

	for _, seed := range [][]byte{
		{},
		{opClient, opInsecureClient, opInsecureClient},
		{set(2), opClient, set(0), opClient},
		{opClient, set(3), opClient, set(1), opClient},
		{prefer(0), opClient, prefer(2), prefer(1), opClient},
		{set(2), prefer(1), prefer(3), set(0), prefer(0), opClient},
		{opInsecureClient, set(2), opInsecureClient, set(0), opInsecureClient},
		{prefer(3), set(3), prefer(2), prefer(0), set(1), prefer(1)},
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, ops []byte) {
		var (
			holder Holder
			model  holderModel
		)

		for i, op := range ops {
			switch op & 3 {
			case opSet:
				client := clients[op>>2&3]
				holder.Set(client)

				model.injected = client
				if c, ok := client.(*http.Client); ok && c == nil {
					model.injected = nil
				}
			case opPrefer:
				client := clients[op>>2&3]
				got := holder.Prefer(client)

				if c, ok := client.(*http.Client); client != nil && (!ok || c != nil) {
					require.Same(t, client, got, "step %d: Prefer should return the client it is given", i)

					continue
				}

				checkClient(t, i, got, &model)
			case opInsecureClient:
				warned := false
				got := holder.InsecureClient(func() { warned = true })
				checkInsecureClient(t, i, got, warned, &model)
			default:
				checkClient(t, i, holder.Client(), &model)
			}
		}
	})
}

// checkClient checks a Client result against the model and records a newly
// built default client in it.
//
// Parameters:
//   - t: the test.
//   - step: the index of the call in the input, for failure messages.
//   - got: the client that Client returned.
//   - model: the expected state, updated when got is a new default client.
func checkClient(t *testing.T, step int, got types.HTTPClient, model *holderModel) {
	t.Helper()

	if model.injected != nil {
		require.Same(t, model.injected, got, "step %d: Client should return the injected client", step)

		return
	}

	if model.built != nil {
		require.Same(t, model.built, got, "step %d: Client should reuse the default client", step)

		return
	}

	client, ok := got.(*http.Client)
	require.True(t, ok, "step %d: the default client is a %T", step, got)
	assert.Zero(t, client.Timeout, "step %d: default client timeout", step)
	assert.Nil(t, client.Transport, "step %d: the default client should send through http.DefaultTransport", step)

	model.built = got
}

// checkInsecureClient checks an InsecureClient result and its warning
// against the model, and records a newly built insecure default client in it.
//
// Parameters:
//   - t: the test.
//   - step: the index of the call in the input, for failure messages.
//   - got: the client that InsecureClient returned.
//   - warned: whether InsecureClient called its warning callback.
//   - model: the expected state, updated when got is a new insecure client.
func checkInsecureClient(t *testing.T, step int, got types.HTTPClient, warned bool, model *holderModel) {
	t.Helper()

	if model.injected != nil {
		require.Same(t, model.injected, got, "step %d: InsecureClient should return the injected client", step)
		require.False(t, warned, "step %d: an injected client must not warn", step)

		return
	}

	require.True(t, warned, "step %d: the insecure default client should warn", step)

	if model.insecure != nil {
		require.Same(t, model.insecure, got, "step %d: InsecureClient should reuse the insecure client", step)

		return
	}

	client, ok := got.(*http.Client)
	require.True(t, ok, "step %d: the insecure client is a %T", step, got)
	assert.Zero(t, client.Timeout, "step %d: insecure client timeout", step)

	httpTransport, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "step %d: the insecure transport is a %T", step, client.Transport)
	require.NotNil(t, httpTransport.TLSClientConfig, "step %d: insecure TLS settings", step)
	assert.True(t, httpTransport.TLSClientConfig.InsecureSkipVerify, "step %d: verification should be skipped", step)
	assert.Equal(t, uint16(tls.VersionTLS12), httpTransport.TLSClientConfig.MinVersion, "step %d: minimum TLS version", step)

	model.insecure = got
}
