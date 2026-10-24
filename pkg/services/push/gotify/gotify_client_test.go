package gotify

import (
	"bytes"
	"crypto/tls"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// TestCreateTransport verifies that the default transport requires TLS 1.2,
// skips certificate verification when TLS is disabled or verification is
// turned off, and honors the proxy environment variables.
func TestCreateTransport(t *testing.T) {
	t.Parallel()

	// tests lists configs with whether the transport should skip
	// certificate verification.
	tests := []struct {
		name         string
		config       Config
		wantInsecure bool
	}{
		{name: "verifies certificates"},
		{name: "TLS disabled", config: Config{DisableTLS: true}, wantInsecure: true},
		{name: "verification turned off", config: Config{InsecureSkipVerify: true}, wantInsecure: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			transport := (&DefaultHTTPClientManager{}).CreateTransport(&tt.config)

			require.NotNil(t, transport.TLSClientConfig)
			assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
			assert.Equal(t, tt.wantInsecure, transport.TLSClientConfig.InsecureSkipVerify)
			assert.NotNil(t, transport.Proxy)
		})
	}
}

// TestCreateClient verifies that the default client sends through the given
// transport with a 10 second timeout.
func TestCreateClient(t *testing.T) {
	t.Parallel()

	transport := &http.Transport{}
	client := (&DefaultHTTPClientManager{}).CreateClient(transport)

	assert.Same(t, transport, client.Transport)
	assert.Equal(t, 10*time.Second, client.Timeout)
}

// TestDefaultHTTPClient verifies that Initialize sets up a default client with
// a 10 second timeout and the URL's TLS settings, logs a warning for each
// insecure TLS setting, and keeps a client injected before it.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the warnings each should log and whether
	// the default client should skip certificate verification.
	tests := []struct {
		name         string
		rawURL       string
		wantLog      string
		wantInsecure bool
	}{
		{name: "secure", rawURL: testServiceURL},
		{
			name:         "TLS disabled",
			rawURL:       testServiceURL + "?disabletls=yes",
			wantLog:      "Warning: TLS is disabled, using insecure HTTP connections\n",
			wantInsecure: true,
		},
		{
			name:         "verification turned off",
			rawURL:       testServiceURL + "?insecureskipverify=yes",
			wantLog:      "Warning: TLS certificate verification is disabled\n",
			wantInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			service := &Service{}
			require.NoError(t, service.Initialize(parseTestURL(t, tt.rawURL), log.New(&logs, "", 0)))

			client, ok := service.client.Client().(*http.Client)
			require.True(t, ok, "default client is %T", service.client.Client())
			assert.Equal(t, 10*time.Second, client.Timeout)

			httpTransport, ok := client.Transport.(*http.Transport)
			require.True(t, ok, "transport is %T", client.Transport)
			require.NotNil(t, httpTransport.TLSClientConfig)
			assert.Equal(t, uint16(tls.VersionTLS12), httpTransport.TLSClientConfig.MinVersion)
			assert.Equal(t, tt.wantInsecure, httpTransport.TLSClientConfig.InsecureSkipVerify)
			assert.Equal(t, tt.wantLog, logs.String())
		})
	}

	t.Run("keeps a client injected before Initialize", func(t *testing.T) {
		t.Parallel()

		custom := typesmocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(custom)
		require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))

		assert.Same(t, custom, service.client.Client())
	})
}
