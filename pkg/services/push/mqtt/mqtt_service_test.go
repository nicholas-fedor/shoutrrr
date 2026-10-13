package mqtt

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"testing"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newServiceWithLogger returns an uninitialized service that logs to the test
// output.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - *Service: the service.
func newServiceWithLogger(t *testing.T) *Service {
	t.Helper()

	service := &Service{}
	service.SetLogger(log.New(t.Output(), "", 0))

	return service
}

// listenLoopback starts a TCP listener on a free loopback port and closes it
// when the test ends.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - net.Listener: the listener.
func listenLoopback(t *testing.T) net.Listener {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	return listener
}

// TestGetID verifies the service identifier and the scheme constants.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "mqtt", (&Service{}).GetID())
	assert.Equal(t, "mqtt", Scheme)
	assert.Equal(t, "mqtts", SchemeTLS)
}

// TestGetDefaultPortForScheme verifies that only the exact mqtts scheme uses the
// TLS port.
func TestGetDefaultPortForScheme(t *testing.T) {
	t.Parallel()

	// tests lists schemes with the default port each should get.
	tests := []struct {
		scheme string
		want   int
	}{
		{scheme: "mqtt", want: 1883},
		{scheme: "mqtts", want: 8883},
		{scheme: "unknown", want: 1883},
		{scheme: "", want: 1883},
		{scheme: "http", want: 1883},
		{scheme: "MQTTS", want: 1883},
	}

	for _, tt := range tests {
		t.Run(tt.scheme, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, (&Service{}).getDefaultPortForScheme(tt.scheme))
		})
	}
}

// TestInitialize verifies that Initialize applies the struct tag defaults, reads
// the URL into the config, and connects lazily.
func TestInitialize(t *testing.T) {
	t.Parallel()

	// defaults is the config for a URL without credentials or query parameters.
	defaults := Config{
		Host: "broker.example.com", Port: 1883, Topic: "test/topic",
		ClientID: "shoutrrr", QoS: QoSAtMostOnce, CleanSession: true,
	}

	// with returns defaults changed by change.
	with := func(change func(*Config)) Config {
		config := defaults
		change(&config)

		return config
	}

	// tests lists service URLs with the config Initialize should produce. The
	// unexported scheme is checked against the URL's scheme on its own.
	tests := []struct {
		name   string
		rawURL string
		want   Config
	}{
		{name: "defaults", rawURL: "mqtt://broker.example.com:1883/test/topic", want: defaults},
		{name: "mqtts port", rawURL: "mqtts://broker.example.com:8883/test/topic", want: with(func(c *Config) { c.Port = 8883 })},
		{name: "custom port", rawURL: "mqtt://broker.example.com:9999/test/topic", want: with(func(c *Config) { c.Port = 9999 })},
		{
			name:   "query parameters",
			rawURL: "mqtt://broker.example.com:1883/test/topic?clientid=myclient&qos=1&retained=yes",
			want: with(func(c *Config) {
				c.ClientID = "myclient"
				c.QoS = QoSAtLeastOnce
				c.Retained = true
			}),
		},
		{
			name:   "credentials",
			rawURL: "mqtt://user:pass@broker.example.com:1883/test/topic",
			want: with(func(c *Config) {
				c.Username = "user"
				c.Password = "pass"
			}),
		},
		{name: "username only", rawURL: "mqtt://user@broker.example.com:1883/test/topic", want: with(func(c *Config) { c.Username = "user" })},
		{name: "TLS disabled", rawURL: "mqtts://broker.example.com:8883/test/topic?disabletls=yes", want: with(func(c *Config) { c.Port, c.DisableTLS = 8883, true })},
		{
			name:   "TLS verification disabled",
			rawURL: "mqtts://broker.example.com:8883/test/topic?disabletlsverification=yes",
			want:   with(func(c *Config) { c.Port, c.DisableTLSVerification = 8883, true }),
		},
		{name: "persistent session", rawURL: "mqtt://broker.example.com:1883/test/topic?cleansession=no", want: with(func(c *Config) { c.CleanSession = false })},
		{name: "deeply nested topic", rawURL: "mqtt://broker.example.com:1883/l1/l2/l3/l4/l5", want: with(func(c *Config) { c.Topic = "l1/l2/l3/l4/l5" })},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL := parseTestURL(t, tt.rawURL)

			service := &Service{}
			require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
			require.NotNil(t, service.Config)
			assert.Equal(t, serviceURL.Scheme, service.Config.scheme, "Initialize must keep the URL scheme")

			got := *service.Config
			got.scheme = ""
			assert.Equal(t, tt.want, got)
			assert.Nil(t, service.connectionManager, "Initialize must not connect")
			assert.False(t, service.ownedConnection)
		})
	}
}

// TestCreateTLSConfig verifies that the TLS config requires TLS 1.2, verifies
// certificates unless disabled, and is new on every call.
func TestCreateTLSConfig(t *testing.T) {
	t.Parallel()

	service := newServiceWithLogger(t)

	verified := service.createTLSConfig(&Config{})
	require.NotNil(t, verified)
	assert.False(t, verified.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS12), verified.MinVersion)

	unverified := service.createTLSConfig(&Config{DisableTLSVerification: true})
	require.NotNil(t, unverified)
	assert.True(t, unverified.InsecureSkipVerify)

	assert.NotSame(t, verified, service.createTLSConfig(&Config{}))
}

// TestCloseWithoutConnection verifies that Close succeeds, repeatedly, when no
// connection was opened.
func TestCloseWithoutConnection(t *testing.T) {
	t.Parallel()

	service := newServiceWithLogger(t)

	for range 3 {
		require.NoError(t, service.Close())
	}
}

// TestAttemptConnection verifies that a connection needs a dial function, dials
// TCP at the broker address, returns the plain connection without TLS, and
// fails the TLS handshake against a plaintext listener.
func TestAttemptConnection(t *testing.T) {
	t.Parallel()

	t.Run("no dial function", func(t *testing.T) {
		t.Parallel()

		_, err := (&Service{}).attemptConnection(t.Context(), autopaho.ClientConfig{}, parseTestURL(t, "mqtt://broker.example.com:1883"))
		require.ErrorIs(t, err, ErrNoDialContext)
	})

	t.Run("dials the broker address", func(t *testing.T) {
		t.Parallel()

		errBlocked := errors.New("destination blocked")

		var gotNetwork, gotAddr string

		service := &Service{}
		service.SetDialContext(func(_ context.Context, network, addr string) (net.Conn, error) {
			gotNetwork, gotAddr = network, addr

			return nil, errBlocked
		})

		_, err := service.attemptConnection(t.Context(), autopaho.ClientConfig{}, parseTestURL(t, "mqtt://broker.example.com:1883"))
		require.ErrorIs(t, err, errBlocked)
		assert.Equal(t, "tcp", gotNetwork)
		assert.Equal(t, "broker.example.com:1883", gotAddr)
	})

	t.Run("plain connection", func(t *testing.T) {
		t.Parallel()

		listener := listenLoopback(t)
		accepted := make(chan net.Conn, 1)

		// Closing the channel when Accept fails keeps the receive below from
		// blocking.
		go func() {
			defer close(accepted)

			conn, err := listener.Accept()
			if err == nil {
				accepted <- conn
			}
		}()

		service := &Service{}
		service.SetDialContext((&net.Dialer{}).DialContext)

		conn, err := service.attemptConnection(t.Context(), autopaho.ClientConfig{}, parseTestURL(t, "mqtt://"+listener.Addr().String()))
		require.NoError(t, err)
		require.NotNil(t, conn)
		require.NoError(t, conn.Close())

		peer, ok := <-accepted
		require.True(t, ok, "the listener did not accept the connection")
		require.NoError(t, peer.Close())
	})

	t.Run("TLS handshake against plaintext", func(t *testing.T) {
		t.Parallel()

		listener := listenLoopback(t)

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			defer func() { _ = conn.Close() }()

			_, _ = conn.Read(make([]byte, 1))
		}()

		service := &Service{}
		service.SetDialContext((&net.Dialer{}).DialContext)

		_, err := service.attemptConnection(
			t.Context(),
			autopaho.ClientConfig{TlsCfg: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}},
			parseTestURL(t, "mqtts://"+listener.Addr().String()),
		)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS handshake")
	})
}
