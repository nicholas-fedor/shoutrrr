package smtp

import (
	"context"
	"crypto/tls"
	"net"
	"net/smtp"
	"strconv"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// dialClient establishes a connection to the SMTP server using the provided configuration.
//
// A custom dial function is used when non-nil; otherwise a default [net.Dialer]
// opens the TCP connection. Implicit TLS is applied after the TCP dial when
// [useImplicitTLS] reports that it is required. The context deadline is applied
// to the connection when present.
//
// Parameters:
//   - ctx: Context that bounds dialing and the connection deadline.
//   - config: SMTP configuration providing host, port, encryption, and TLS options.
//   - dial: Optional custom TCP dial function. Nil uses [net.Dialer.DialContext].
//
// Returns:
//   - An [smtp.Client] wrapping the established connection.
//   - An error if dialing, applying the deadline, or creating the client fails.
func dialClient(ctx context.Context, config *Config, dial types.DialContextFunc) (*smtp.Client, error) {
	addr := net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.Port), 10))

	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}

	conn, err := dial(ctx, "tcp", addr)
	if err != nil {
		return nil, fail(FailConnectToServer, err)
	}

	if useImplicitTLS(config.Encryption, config.Port) {
		tlsConn := tls.Client(conn, newTLSConfig(config))
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()

			return nil, fail(FailConnectToServer, err)
		}

		conn = tlsConn
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()

			return nil, fail(FailConnectToServer, err)
		}
	}

	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		_ = conn.Close()

		return nil, fail(FailCreateSMTPClient, err)
	}

	return client, nil
}
