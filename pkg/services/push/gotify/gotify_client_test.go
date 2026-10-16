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

// TestInitClient verifies that initClient creates the default client with a
// warning for each insecure TLS setting, and keeps a client that is already
// set.
func TestInitClient(t *testing.T) {
	t.Parallel()

	// tests lists configs with the warnings each should log.
	tests := []struct {
		name    string
		wantLog string
		config  Config
	}{
		{name: "secure"},
		{
			name:    "TLS disabled",
			config:  Config{DisableTLS: true},
			wantLog: "Warning: TLS is disabled, using insecure HTTP connections\n",
		},
		{
			name:    "verification turned off",
			config:  Config{InsecureSkipVerify: true},
			wantLog: "Warning: TLS certificate verification is disabled\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			service := &Service{Config: &tt.config}
			service.SetLogger(log.New(&logs, "", 0))

			initClient(service, &DefaultHTTPClientManager{})

			client, ok := service.httpClient.(*http.Client)
			require.True(t, ok, "default client is %T", service.httpClient)
			assert.Equal(t, 10*time.Second, client.Timeout)
			assert.NotNil(t, service.client)
			assert.Equal(t, tt.wantLog, logs.String())

			initClient(service, &DefaultHTTPClientManager{})

			assert.Same(t, client, service.httpClient)
			assert.Equal(t, tt.wantLog, logs.String())
		})
	}

	t.Run("keeps a custom client", func(t *testing.T) {
		t.Parallel()

		custom := typesmocks.NewMockHTTPClient(t)
		service := &Service{Config: &Config{}}
		service.SetHTTPClient(custom)

		initClient(service, &DefaultHTTPClientManager{})

		assert.Same(t, custom, service.httpClient)
	})
}
