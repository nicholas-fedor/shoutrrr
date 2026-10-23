package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deadlineConn records the deadline set on a connection, failing with err
// when it is set.
type deadlineConn struct {
	// Conn is the wrapped connection.
	net.Conn

	// deadline is the last deadline set.
	deadline time.Time
	// err is the error SetDeadline returns.
	err error
}

// TestDialClient verifies that dialClient connects to the configured host and
// port through the given dialer and reads the greeting, with implicit TLS
// when the encryption and port call for it.
func TestDialClient(t *testing.T) {
	t.Parallel()

	// tests lists configs and servers with the address each should dial.
	tests := []struct {
		name        string
		wantAddress string
		config      Config
		implicitTLS bool
	}{
		{name: "plain", config: Config{Host: "localhost", Port: 2525, Encryption: EncAuto}, wantAddress: "localhost:2525"},
		{name: "IPv6", config: Config{Host: "::1", Port: 587, Encryption: EncNone}, wantAddress: "[::1]:587"},
		{
			name:        "implicit TLS",
			config:      Config{Host: "localhost", Port: 2525, Encryption: EncImplicitTLS, SkipTLSVerify: true},
			implicitTLS: true,
			wantAddress: "localhost:2525",
		},
		{
			name:        "auto on port 465",
			config:      Config{Host: "localhost", Port: ImplicitTLSPort, Encryption: EncAuto, SkipTLSVerify: true},
			implicitTLS: true,
			wantAddress: "localhost:465",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newFakeSMTP()
			server.implicitTLS = tt.implicitTLS

			var dialed string

			client, err := dialClient(t.Context(), &tt.config, func(ctx context.Context, network, address string) (net.Conn, error) {
				dialed = network + " " + address

				return server.dial(ctx, network, address)
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = client.Close() })

			assert.Equal(t, "tcp "+tt.wantAddress, dialed)

			_, isTLS := client.TLSConnectionState()
			assert.Equal(t, tt.implicitTLS, isTLS)
			require.NoError(t, client.Hello("localhost"))
		})
	}
}

// TestDialClientDeadline verifies that the context deadline bounds the
// connection.
func TestDialClientDeadline(t *testing.T) {
	t.Parallel()

	deadline := time.Now().Add(time.Hour)

	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	t.Cleanup(cancel)

	server := newFakeSMTP()

	var conn *deadlineConn

	client, err := dialClient(ctx, &Config{Host: "localhost", Port: 2525}, func(ctx context.Context, network, address string) (net.Conn, error) {
		inner, err := server.dial(ctx, network, address)
		conn = &deadlineConn{Conn: inner}

		return conn, err
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	assert.Equal(t, deadline, conn.deadline)
}

// TestDialClientErrors verifies that a dial error, a failed TLS handshake, a
// failed deadline, or a rejected greeting fails with the matching failure and
// closes the connection.
func TestDialClientErrors(t *testing.T) {
	t.Parallel()

	errBlocked := errors.New("destination blocked")

	t.Run("dial error", func(t *testing.T) {
		t.Parallel()

		_, err := dialClient(t.Context(), &Config{Host: "localhost", Port: 2525}, func(context.Context, string, string) (net.Conn, error) {
			return nil, errBlocked
		})

		require.ErrorIs(t, err, fail(FailConnectToServer, nil))
		require.ErrorIs(t, err, errBlocked)
	})

	t.Run("untrusted certificate", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTP()
		server.implicitTLS = true

		_, err := dialClient(t.Context(), &Config{Host: "localhost", Port: 2525, Encryption: EncImplicitTLS}, server.dial)

		require.ErrorIs(t, err, fail(FailConnectToServer, nil))

		_, isCertErr := errors.AsType[*tls.CertificateVerificationError](err)
		assert.True(t, isCertErr, "error %v is not a certificate verification error", err)
		assert.Equal(t, int32(1), server.closes.Load())
	})

	t.Run("deadline error", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		t.Cleanup(cancel)

		server := newFakeSMTP()

		_, err := dialClient(ctx, &Config{Host: "localhost", Port: 2525}, func(ctx context.Context, network, address string) (net.Conn, error) {
			inner, err := server.dial(ctx, network, address)

			return &deadlineConn{Conn: inner, err: errBlocked}, err
		})

		require.ErrorIs(t, err, fail(FailConnectToServer, nil))
		require.ErrorIs(t, err, errBlocked)
		assert.Equal(t, int32(1), server.closes.Load())
	})

	t.Run("greeting rejected", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTP()
		server.greeting = "554 5.3.2 Not accepting mail"

		_, err := dialClient(t.Context(), &Config{Host: "localhost", Port: 2525}, server.dial)

		require.ErrorIs(t, err, fail(FailCreateSMTPClient, nil))
		assert.Positive(t, server.closes.Load())
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err := dialClient(ctx, &Config{Host: "127.0.0.1", Port: 1}, nil)

		require.ErrorIs(t, err, fail(FailConnectToServer, nil))
		require.ErrorIs(t, err, context.Canceled)
	})
}

// TestDialClientDefaultDialer verifies that a nil dialer connects over TCP, to
// a loopback server.
func TestDialClientDefaultDialer(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	server := newFakeSMTP()

	go func() {
		conn, err := listener.Accept()
		if err == nil {
			server.serve(t.Context(), conn)
		}
	}()

	address := listener.Addr().(*net.TCPAddr)

	client, err := dialClient(t.Context(), &Config{Host: "127.0.0.1", Port: uint16(address.Port)}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Hello("localhost"))
}

// SetDeadline records the deadline and returns the configured error, or sets
// the deadline on the connection.
//
// Parameters:
//   - deadline: the deadline.
//
// Returns:
//   - error: the configured error, or the connection's error.
func (c *deadlineConn) SetDeadline(deadline time.Time) error {
	c.deadline = deadline

	if c.err != nil {
		return c.err
	}

	return c.Conn.SetDeadline(deadline)
}
