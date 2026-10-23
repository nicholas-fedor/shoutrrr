package httpclient

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// holderModel is the expected state of a Holder after a sequence of calls.
type holderModel struct {
	// injected is the injected client, or nil.
	injected types.HTTPClient
	// built is the default client Client returned since the last SetDefault,
	// or nil before Client builds one.
	built types.HTTPClient
	// tlsConfig is the TLS settings of the last SetDefault call.
	tlsConfig *tls.Config
	// timeout is the timeout of the last SetDefault call.
	timeout time.Duration
}

// Holder operations a fuzz input byte encodes in its low two bits.
const (
	// opClient calls Client and checks the result against the model.
	opClient byte = iota
	// opSet calls Set with the client that bits 2 and 3 select.
	opSet
	// opSetDefault calls SetDefault with the timeout that bits 2 and 3 select
	// and the TLS settings that bits 4 and 5 select.
	opSetDefault
	// opClientAgain also calls Client, so reads are as common as writes.
	opClientAgain
)

// fuzzTimeouts are the timeouts opSetDefault chooses from.
var fuzzTimeouts = [...]time.Duration{0, time.Second, 10 * time.Second, 30 * time.Second}

// FuzzHolder runs a sequence of Set, SetDefault, and Client calls on a Holder
// and checks every Client result against a model of the expected state:
//
//   - An injected client is returned whatever the default settings are.
//   - A nil client, or a nil *http.Client, clears the injected client.
//   - The default client has the timeout of the last SetDefault call, and
//     sends through http.DefaultTransport without TLS settings or through a
//     transport with exactly those settings.
//   - The default client is built once and reused until the next SetDefault
//     call, even across Set calls.
//
// Each input byte is one call: the low two bits pick the operation, and the
// higher bits its argument. The seed corpus covers the zero value, an
// injected client set and cleared both ways, default settings replaced before
// and after the default is built, and settings changed while a client is
// injected.
func FuzzHolder(f *testing.F) {
	clients := [...]types.HTTPClient{nil, (*http.Client)(nil), &http.Client{}, &http.Client{}}
	tlsConfigs := [...]*tls.Config{nil, nil, {MinVersion: tls.VersionTLS12}, {MinVersion: tls.VersionTLS13}}

	set := func(client byte) byte { return opSet | client<<2 }
	setDefault := func(timeout, tlsConfig byte) byte { return opSetDefault | timeout<<2 | tlsConfig<<4 }

	for _, seed := range [][]byte{
		{},
		{opClient, opClientAgain},
		{set(2), opClient, set(0), opClient},
		{opClient, set(3), opClient, set(1), opClient},
		{setDefault(2, 0), opClient, setDefault(1, 2), opClient, opClient},
		{opClient, setDefault(3, 3), set(2), opClient, set(0), opClient},
		{set(2), setDefault(2, 2), opClient, set(1), opClient, setDefault(0, 0), opClient},
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
			case opSetDefault:
				timeout, tlsConfig := fuzzTimeouts[op>>2&3], tlsConfigs[op>>4&3]
				holder.SetDefault(timeout, tlsConfig)

				model.timeout, model.tlsConfig, model.built = timeout, tlsConfig, nil
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
	assert.Equal(t, model.timeout, client.Timeout, "step %d: default client timeout", step)

	if model.tlsConfig == nil {
		assert.Nil(t, client.Transport, "step %d: the default client should send through http.DefaultTransport", step)
	} else {
		httpTransport, ok := client.Transport.(*http.Transport)
		require.True(t, ok, "step %d: the default transport is a %T", step, client.Transport)
		assert.Same(t, model.tlsConfig, httpTransport.TLSClientConfig, "step %d: default TLS settings", step)
	}

	model.built = got
}
