package smtp

import (
	"context"
	"crypto/tls"
	"net"
	"net/smtp"
	"strconv"
)

// dialClient establishes a connection to the SMTP server using the provided configuration.
//
// Implicit TLS is used when [useImplicitTLS] reports that it is required; otherwise,
// a plain TCP connection is opened. The context deadline is applied to the
// connection when present.
//
// Parameters:
//   - ctx: Context that bounds dialing and the connection deadline.
//   - config: SMTP configuration providing host, port, encryption, and TLS options.
//
// Returns:
//   - An [smtp.Client] wrapping the established connection.
//   - An error if dialing, applying the deadline, or creating the client fails.
func dialClient(ctx context.Context, config *Config) (*smtp.Client, error) {
	var (
		conn net.Conn
		err  error
	)

	addr := net.JoinHostPort(config.Host, strconv.FormatUint(uint64(config.Port), 10))

	if useImplicitTLS(config.Encryption, config.Port) {
		dialer := &tls.Dialer{Config: newTLSConfig(config)}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	} else {
		dialer := &net.Dialer{}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		return nil, fail(FailConnectToServer, err)
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
