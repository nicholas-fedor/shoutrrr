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

	mockTypes "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// TestCreateTransport verifies that the deprecated manager's transport
// requires TLS 1.2 and honors the proxy environment variables. It skips
// certificate verification only when SkipTLSVerify or the deprecated
// InsecureSkipVerify is set, not when TLS is disabled.
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
		{name: "TLS disabled", config: Config{DisableTLS: true}, wantInsecure: false},
		{name: "verification turned off", config: Config{SkipTLSVerify: true}, wantInsecure: true},
		{name: "deprecated field", config: Config{InsecureSkipVerify: true}, wantInsecure: true},
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

// TestDefaultHTTPClient verifies that Initialize leaves the default client
// without a timeout of its own, sending through http.DefaultTransport, and
// keeps a client injected before it. Initialize warns only when the URL uses a
// deprecated key. The first send that disables TLS or skips verification warns
// about it instead.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the warnings Initialize should log.
	tests := []struct {
		name    string
		rawURL  string
		wantLog string
	}{
		{name: "secure", rawURL: testServiceURL},
		{name: "TLS disabled", rawURL: testServiceURL + "?disabletls=yes"},
		{name: "verification turned off", rawURL: testServiceURL + "?skiptlsverify=yes"},
		{
			name:    "deprecated key",
			rawURL:  testServiceURL + "?insecureskipverify=yes",
			wantLog: `WARN config key is deprecated key=insecureskipverify replacement=skiptlsverify` + "\n",
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
			assert.Zero(t, client.Timeout)
			assert.Nil(t, client.Transport, "the default client should send through http.DefaultTransport")
			assert.Equal(t, tt.wantLog, logs.String())
		})
	}

	t.Run("keeps a client injected before Initialize", func(t *testing.T) {
		t.Parallel()

		custom := mockTypes.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(custom)
		require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))

		assert.Same(t, custom, service.client.Client())
	})
}
