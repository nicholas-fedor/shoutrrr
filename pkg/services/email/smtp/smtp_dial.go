package smtp

import (
	"context"
	"crypto/tls"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// dialClient establishes a connection to the SMTP server using the provided configuration.
//
// A custom dial function is used when non-nil; otherwise a default [net.Dialer]
// opens the TCP connection. Implicit TLS is applied after the TCP dial when
// [useImplicitTLS] reports that it is required. The context deadline is applied
// to the connection when present. When the context ends before the returned
// stop function is called, the connection's blocked reads and writes return at
// once.
//
// Parameters:
//   - ctx: Context that bounds dialing and the connection deadline.
//   - config: SMTP configuration providing host, port, encryption, and TLS options.
//   - dial: Optional custom TCP dial function. Nil uses [net.Dialer.DialContext].
//
// Returns:
//   - *smtp.Client: the client wrapping the established connection.
//   - func() bool: the [context.AfterFunc] stop function of that watch. Call it
//     once the session is over.
//   - error: an error if dialing, applying the deadline, or creating the client
//     fails.
func dialClient(ctx context.Context, config *Config, dial types.DialContextFunc) (*smtp.Client, func() bool, error) {
	addr := net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.Port), 10))

	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}

	conn, err := dial(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, fail(FailConnectToServer, err)
	}

	if useImplicitTLS(config.Encryption, config.Port) {
		tlsConn := tls.Client(conn, newTLSConfig(config))
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()

			return nil, nil, fail(FailConnectToServer, err)
		}

		conn = tlsConn
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()

			return nil, nil, fail(FailConnectToServer, err)
		}
	}

	// A deadline in the past ends the reads and writes the session is blocked
	// in, so a canceled send stops at once instead of at its deadline. The
	// connection is safe to change from another goroutine.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) }) //nolint:errcheck // Fails only on a closed connection.

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		stop()

		_ = conn.Close()

		return nil, nil, fail(FailCreateSMTPClient, err)
	}

	return client, stop, nil
}
