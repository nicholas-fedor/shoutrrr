// Package proxy_test verifies that services send through the proxy named by the
// proxy environment variables when they use their own default HTTP client.
package proxy_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// recordingProxy is a forward proxy that records the host of every request and
// CONNECT tunnel it receives and refuses to forward any of them.
type recordingProxy struct {
	mu    sync.Mutex
	hosts map[string]int
}

// errLookupBlocked is returned for every DNS lookup in this package.
var errLookupBlocked = errors.New("proxy test: DNS lookups are blocked")

// proxy is the recording proxy that TestMain points the environment at.
var proxy = &recordingProxy{hosts: map[string]int{}}

// TestMain points the proxy environment variables at the recording proxy before
// any test runs. net/http reads them once per process, so they must be set before
// the first request. DNS lookups are blocked, so a service that bypasses the
// proxy fails on the host instead of reaching the network. The proxy listens on
// a loopback IP address, so reaching it needs no lookup.
func TestMain(m *testing.M) {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errLookupBlocked
		},
	}

	server := httptest.NewServer(proxy)

	for key, value := range map[string]string{
		"HTTP_PROXY":  server.URL,
		"HTTPS_PROXY": server.URL,
		"NO_PROXY":    "",
		"http_proxy":  server.URL,
		"https_proxy": server.URL,
		"no_proxy":    "",
	} {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "setting %s: %v\n", key, err)
			os.Exit(1)
		}
	}

	code := m.Run()

	server.Close()
	os.Exit(code)
}

// TestDefaultClientsUseProxyEnvironment verifies that each service's default HTTP
// client sends through the proxy from the environment. The targets use the
// reserved .invalid TLD, and the proxy refuses every request, so nothing leaves
// the host and no target name is resolved.
func TestDefaultClientsUseProxyEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		host string
	}{
		{"generic", "generic://generic.example.invalid/webhook", "generic.example.invalid:443"},
		{"gotify", "gotify://gotify.example.invalid/ASECRETgotifyTK", "gotify.example.invalid:443"},
		{"homeassistant", "homeassistant://SECREThassTOKEN@ha.example.invalid", "ha.example.invalid:443"},
		{"mattermost", "mattermost://proxy@mattermost.example.invalid/SECRETmattermostTOKEN", "mattermost.example.invalid:443"},
		{
			"mattermost without TLS",
			"mattermost://proxy@mattermost-http.example.invalid/SECRETmattermostTOKEN?disabletls=yes",
			"mattermost-http.example.invalid",
		},
		{"ntfy", "ntfy://ntfy.example.invalid/topic", "ntfy.example.invalid:443"},
		{"signal", "signal://signal.example.invalid:8080/+15551234567/+15559876543", "signal.example.invalid:8080"},
		{"signalgrid", "signalgrid://SECRETsignalgridKEY@channel", "api.signalgrid.co:443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceRouter, err := router.NewWithOptions(nil, types.SenderOptions{})
			require.NoError(t, err)

			service, err := serviceRouter.Locate(tt.url)
			require.NoError(t, err)

			require.Error(t, service.Send("proxy test", nil), "the proxy refuses every request")
			assert.Positive(t, proxy.count(tt.host), "the request to %s did not go through the proxy", tt.host)
		})
	}
}

// ServeHTTP records the target host and refuses the request. A CONNECT carries
// the host and port of an HTTPS target, and a plain request carries the host of
// an HTTP target.
//
// Parameters:
//   - w: the response writer.
//   - r: the proxied request.
func (p *recordingProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if r.Method != http.MethodConnect && r.URL.Host != "" {
		host = r.URL.Host
	}

	if h, port, err := net.SplitHostPort(host); err == nil && r.Method != http.MethodConnect && port == "80" {
		host = h
	}

	p.mu.Lock()
	p.hosts[host]++
	p.mu.Unlock()

	http.Error(w, "proxy test: forwarding refused", http.StatusForbidden)
}

// count returns how many requests the proxy received for host.
//
// Parameters:
//   - host: the target host, with the port for CONNECT tunnels.
//
// Returns:
//   - int: the number of requests recorded for host.
func (p *recordingProxy) count(host string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.hosts[host]
}
