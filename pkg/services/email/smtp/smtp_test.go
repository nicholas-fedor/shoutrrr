package smtp

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/textproto"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// fakeSMTP is an SMTP server that answers each client command from a script,
// recording the commands and messages it receives. Connections come from
// dial, which serves each one over an in-memory pipe.
type fakeSMTP struct {
	// replies overrides the reply to a command verb, in order, before the
	// default replies apply. An empty entry uses the default reply, "close"
	// closes the connection, and "hang" stops answering. The AUTH entry is
	// the whole exchange: a 334 line reads one client line before the next.
	replies map[string][]string
	// greeting is the first line the server sends.
	greeting string
	// extensions are the EHLO keywords the server advertises before 8BITMIME.
	extensions []string
	// commands are the client lines received, in order.
	commands []string
	// messages are the DATA contents received, dot-unstuffed.
	messages []string
	// closes counts the client's closes of its connections.
	closes atomic.Int32
	// mu guards replies, commands, and messages.
	mu sync.Mutex
	// startTLS accepts STARTTLS with the test certificate.
	startTLS bool
	// implicitTLS starts every connection with a TLS handshake.
	implicitTLS bool
}

// cancelAfterReset is a connection that ends a send's context once the client
// has read the reply to its RSET command.
type cancelAfterReset struct {
	// Conn is the wrapped connection.
	net.Conn

	// cancel ends the send's context.
	cancel context.CancelFunc
	// reset is set once the client has written RSET.
	reset atomic.Bool
}

// countingConn counts the client's closes of a connection.
type countingConn struct {
	// Conn is the wrapped connection.
	net.Conn

	// closes is incremented on every Close.
	closes *atomic.Int32
}

// Values shared by the SMTP tests.
const (
	// testServiceURL sends from sender@example.com to rec@example.com through
	// a server on localhost, which net/smtp trusts with credentials without
	// TLS, and does not ask for STARTTLS.
	testServiceURL = "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no"
	// testMailFrom is the MAIL command for the test sender on a server that
	// advertises 8BITMIME.
	testMailFrom = "MAIL FROM:<sender@example.com> BODY=8BITMIME"
	// testRcptTo is the RCPT command for the test recipient.
	testRcptTo = "RCPT TO:<rec@example.com>"
)

// testCertificate is a self-signed certificate for localhost, mail.example.com,
// and 127.0.0.1, created once for the tests.
var testCertificate = sync.OnceValues(func() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mock.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost", "mail.example.com"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
})

// newFakeSMTP returns a server that greets clients and advertises the given
// EHLO keywords.
//
// Parameters:
//   - extensions: the EHLO keywords to advertise before 8BITMIME.
//
// Returns:
//   - *fakeSMTP: the server.
func newFakeSMTP(extensions ...string) *fakeSMTP {
	return &fakeSMTP{
		replies:    map[string][]string{},
		greeting:   "220 mock.local ESMTP",
		extensions: extensions,
	}
}

// newTestService returns a service initialized from rawURL that logs to the
// test output and connects to server.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - server: the server to connect to.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string, server *fakeSMTP) *Service {
	t.Helper()

	return newLoggingService(t, rawURL, server, log.New(t.Output(), "", 0))
}

// newLoggingService returns a service initialized from rawURL that logs to
// logger and connects to server.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - server: the server to connect to.
//   - logger: the service logger.
//
// Returns:
//   - *Service: the service.
func newLoggingService(t *testing.T, rawURL string, server *fakeSMTP, logger *log.Logger) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(parseConfigURL(t, rawURL), logger))
	service.SetDialContext(server.dial)

	return service
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "smtp", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize applies the SMTP defaults under the
// URL settings and infers PLAIN authentication from a username, or none
// without one, unless the URL names a method.
func TestInitialize(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the overrides of the config each should
	// produce.
	tests := []struct {
		override func(*Config)
		name     string
		rawURL   string
	}{
		{
			name:     "defaults",
			rawURL:   "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			override: func(c *Config) { c.Auth = AuthNone },
		},
		{
			name:     "username",
			rawURL:   "smtp://user:pass@mail.example.com:587/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			override: func(c *Config) { c.Username, c.Password, c.Port, c.Auth = "user", "pass", 587, AuthPlain },
		},
		{
			name:     "named method",
			rawURL:   "smtp://user:pass@mail.example.com/?auth=login&fromaddress=sender@example.com&toaddresses=rec@example.com",
			override: func(c *Config) { c.Username, c.Password, c.Auth = "user", "pass", AuthLogin },
		},
		{
			name:     "timeout",
			rawURL:   "smtp://mail.example.com/?fromaddress=sender@example.com&toaddresses=rec@example.com&timeout=30s",
			override: func(c *Config) { c.Auth, c.Timeout = AuthNone, 30*time.Second },
		},
		{
			name:     "port out of range",
			rawURL:   "smtp://mail.example.com:99999/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			override: func(c *Config) { c.Auth = AuthNone },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}
			require.NoError(t, service.Initialize(parseConfigURL(t, tt.rawURL), log.New(t.Output(), "", 0)))

			want := *newInitialConfig()
			want.Host, want.FromAddress, want.ToAddresses = "mail.example.com", "sender@example.com", []string{"rec@example.com"}
			tt.override(&want)
			assert.Equal(t, &want, service.Config)
		})
	}

	t.Run("invalid URL", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseConfigURL(t, "smtp://mail.example.com/?toaddresses=rec@example.com"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrFromAddressMissing)
	})
}

// TestServiceTimeout verifies that the router budget is the session timeout
// for the send: the URL timeout, a timeout param, or the default for a
// non-positive or invalid value.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	// tests lists URLs and params with the budget each should produce.
	tests := []struct {
		params *types.Params
		name   string
		rawURL string
		want   time.Duration
	}{
		{name: "default", rawURL: testServiceURL, want: 10 * time.Second},
		{name: "URL timeout", rawURL: testServiceURL + "&timeout=30s", want: 30 * time.Second},
		{name: "timeout param", rawURL: testServiceURL + "&timeout=30s", params: &types.Params{"timeout": "5s"}, want: 5 * time.Second},
		{name: "invalid param", rawURL: testServiceURL + "&timeout=30s", params: &types.Params{"timeout": "5"}, want: 30 * time.Second},
		{name: "zero timeout", rawURL: testServiceURL + "&timeout=0s", want: 10 * time.Second},
		{name: "negative timeout param", rawURL: testServiceURL, params: &types.Params{"timeout": "-1s"}, want: 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL, newFakeSMTP())

			assert.Equal(t, tt.want, service.ServiceTimeout(tt.params))
		})
	}
}

// TestSendContext verifies that a send connects to the configured address
// through the dialer, with the params applied, and leaves the stored config
// unchanged.
func TestSendContext(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP()

	var dialed []string

	service := newTestService(t, testServiceURL, server)
	service.SetDialContext(func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = append(dialed, network+" "+address)

		return server.dial(ctx, network, address)
	})

	want := service.Config.Clone()

	require.NoError(t, service.SendContext(t.Context(), "Hello", &types.Params{"toaddresses": "other@example.com", "subject": "Backup"}))

	assert.Equal(t, []string{"tcp localhost:2525"}, dialed)
	assert.Equal(t, &want, service.Config)
	assert.Contains(t, server.received(), "RCPT TO:<other@example.com>")
	assert.Contains(t, server.message(t, 0), "Subject: Backup\n")
}

// TestSendContextErrors verifies that a send fails with [FailApplySendParams]
// for an invalid param, and with [FailGetSMTPClient] when the dialer fails or
// the caller's context has already ended, before any SMTP command.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid param", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTP()
		err := newTestService(t, testServiceURL, server).Send("Hello", &types.Params{"timeout": "5"})

		require.ErrorIs(t, err, fail(FailApplySendParams, nil))
		require.ErrorIs(t, err, format.ErrParseDurationFailed)
		assert.Empty(t, server.received())
	})

	t.Run("dialer error", func(t *testing.T) {
		t.Parallel()

		errBlocked := errors.New("destination blocked")

		service := newTestService(t, testServiceURL, newFakeSMTP())
		service.SetDialContext(func(context.Context, string, string) (net.Conn, error) {
			return nil, errBlocked
		})

		err := service.Send("Hello", nil)

		require.ErrorIs(t, err, fail(FailGetSMTPClient, nil))
		require.ErrorIs(t, err, errBlocked)
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		service := newTestService(t, testServiceURL, newFakeSMTP())
		service.SetDialContext(func(ctx context.Context, _, _ string) (net.Conn, error) {
			return nil, ctx.Err()
		})

		require.ErrorIs(t, service.SendContext(ctx, "Hello", nil), context.Canceled)
	})
}

// TestSendContextCanceled verifies that canceling a send ends a session that
// waits on the server at once, instead of at its deadline, and that the error
// matches the cancellation. A send canceled during its first recipient tries
// no reset and no later recipient.
func TestSendContextCanceled(t *testing.T) {
	t.Parallel()

	// tests lists the command that the server stops answering, and the
	// number of MAIL commands the server receives.
	tests := []struct {
		name   string
		rawURL string
		verb   string
		want   failures.FailureID
		mails  int
	}{
		{name: "EHLO", rawURL: testServiceURL, verb: "EHLO", want: FailHandshake, mails: 0},
		{name: "DATA", rawURL: testServiceURL, verb: "DATA", want: FailOpenDataStream, mails: 1},
		{
			name:   "DATA of the first of two recipients",
			rawURL: "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com,second@example.com&usestarttls=no",
			verb:   "DATA",
			want:   FailOpenDataStream,
			mails:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				server := newFakeSMTP()
				server.replies[tt.verb] = []string{"hang"}
				service := newTestService(t, tt.rawURL, server)

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				time.AfterFunc(time.Second, cancel)

				start := time.Now()
				err := service.SendContext(ctx, "Hello", nil)

				require.ErrorIs(t, err, fail(tt.want, nil))
				require.ErrorIs(t, err, context.Canceled)
				require.NotErrorIs(t, err, fail(FailResetSession, nil))
				assert.Equal(t, time.Second, time.Since(start))
				assert.Equal(t, tt.mails, strings.Count(strings.Join(server.received(), "\n"), "MAIL FROM"))
			})
		})
	}
}

// TestSendContextCanceledAfterReset verifies that a context that ends while
// the server answers RSET stops the send before the next recipient. The
// second recipient gets no MAIL, and its address is not reported as failed.
func TestSendContextCanceledAfterReset(t *testing.T) {
	t.Parallel()

	const rawURL = "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com,second@example.com&usestarttls=no"

	server := newFakeSMTP()
	service := newTestService(t, rawURL, server)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	service.SetDialContext(func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := server.dial(ctx, network, addr)

		return &cancelAfterReset{Conn: conn, cancel: cancel}, err
	})

	err := service.SendContext(ctx, "Hello", nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), "second@example.com")
	assert.Equal(t, 1, strings.Count(strings.Join(server.received(), "\n"), "MAIL FROM"))
}

// TestWithContextErr verifies that only a network timeout gains the error of a
// context that ended, so a failure with its own cause, such as a rejected
// login, keeps it even when the context ended at the same moment.
func TestWithContextErr(t *testing.T) {
	t.Parallel()

	timeout := fmt.Errorf("reading reply: %w", os.ErrDeadlineExceeded)
	rejected := errors.New("535 authentication failed")

	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	expired, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	t.Cleanup(stop)

	// tests lists errors and contexts with the context error the result
	// should match, if any.
	tests := []struct {
		ctx     context.Context //nolint:containedctx // Each case needs its own context.
		err     error
		wantCtx error
		name    string
	}{
		{name: "timeout after cancel", ctx: canceled, err: timeout, wantCtx: context.Canceled},
		{name: "timeout after deadline", ctx: expired, err: timeout, wantCtx: context.DeadlineExceeded},
		{name: "timeout with a live context", ctx: t.Context(), err: timeout},
		{name: "other error after cancel", ctx: canceled, err: rejected},
		{name: "other error after deadline", ctx: expired, err: rejected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := withContextErr(tt.ctx, tt.err)

			require.ErrorIs(t, got, tt.err)

			if tt.wantCtx != nil {
				require.ErrorIs(t, got, tt.wantCtx)
			} else {
				assert.Equal(t, tt.err, got)
			}
		})
	}

	assert.NoError(t, withContextErr(canceled, nil))
}

// TestSendContextTimeout verifies what ends a session with a server that
// stops answering. A send without a deadline is bounded by the session
// timeout from the URL or a param. A caller's deadline applies as it is,
// whether it is earlier or later than that timeout.
func TestSendContextTimeout(t *testing.T) {
	t.Parallel()

	// tests lists sends with the time each should take to fail.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		deadline time.Duration
		want     time.Duration
	}{
		{name: "URL timeout", rawURL: testServiceURL + "&timeout=3s", want: 3 * time.Second},
		{name: "timeout param", rawURL: testServiceURL + "&timeout=3s", params: &types.Params{"timeout": "2s"}, want: 2 * time.Second},
		{name: "default for zero", rawURL: testServiceURL + "&timeout=0s", want: 10 * time.Second},
		{name: "caller deadline", rawURL: testServiceURL + "&timeout=3s", deadline: time.Second, want: time.Second},
		{name: "longer caller deadline", rawURL: testServiceURL + "&timeout=3s", deadline: 5 * time.Second, want: 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				server := newFakeSMTP()
				server.replies["EHLO"] = []string{"hang"}
				service := newTestService(t, tt.rawURL, server)

				ctx := t.Context()

				if tt.deadline > 0 {
					var cancel context.CancelFunc

					ctx, cancel = context.WithTimeout(ctx, tt.deadline)
					defer cancel()
				}

				start := time.Now()
				err := service.SendContext(ctx, "Hello", tt.params)

				require.ErrorIs(t, err, fail(FailHandshake, nil))
				require.ErrorIs(t, err, context.DeadlineExceeded)
				assert.Equal(t, tt.want, time.Since(start))
			})
		})
	}
}

// TestSendLogsConnectionWarnings verifies the warnings about the connection a
// send actually uses, each logged once over two sends and again after the
// service is initialized again. A session without TLS logs the plaintext
// warning, even when skiptlsverify is set, because it skips nothing. An
// implicit TLS session that skips verification, from the URL or a param, logs
// the insecure warning.
func TestSendLogsConnectionWarnings(t *testing.T) {
	t.Parallel()

	const implicitTLSURL = "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&encryption=ImplicitTLS"

	for _, tt := range []struct {
		params      *types.Params
		name        string
		rawURL      string
		implicitTLS bool
		want        string
	}{
		{name: "without TLS", rawURL: testServiceURL, want: "WARN sending without TLS"},
		{name: "skiptlsverify without TLS", rawURL: testServiceURL + "&skiptlsverify=yes", want: "WARN sending without TLS"},
		{
			name:        "skiptlsverify in the URL",
			rawURL:      implicitTLSURL + "&skiptlsverify=yes",
			implicitTLS: true,
			want:        "WARN skipping TLS certificate verification",
		},
		{
			name:        "skiptlsverify param",
			rawURL:      implicitTLSURL,
			params:      &types.Params{"skiptlsverify": "yes"},
			implicitTLS: true,
			want:        "WARN skipping TLS certificate verification",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			server := newFakeSMTP()
			server.implicitTLS = tt.implicitTLS

			service := newLoggingService(t, tt.rawURL, server, log.New(&logs, "", 0))

			require.NoError(t, service.Send("Hello", tt.params))
			require.NoError(t, service.Send("Hello", tt.params))
			assert.Equal(t, []string{tt.want}, warningLines(logs.String()))

			require.NoError(t, service.Initialize(parseConfigURL(t, tt.rawURL), log.New(&logs, "", 0)))
			require.NoError(t, service.Send("Hello", tt.params))
			assert.Equal(t, []string{tt.want, tt.want}, warningLines(logs.String()))
		})
	}
}

// warningLines returns the logged lines that are warnings, leaving out the
// log of each message sent.
//
// Parameters:
//   - logs: the log output.
//
// Returns:
//   - []string: the warning lines in order.
func warningLines(logs string) []string {
	var lines []string

	for line := range strings.Lines(logs) {
		if strings.HasPrefix(line, "WARN ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}

	return lines
}

// TestSetDialContext verifies that a nil dialer restores the default dialer.
func TestSetDialContext(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL, newFakeSMTP())
	service.SetDialContext(nil)

	assert.Nil(t, service.dialContext)
}

// TestEffectiveTimeout verifies that a non-positive timeout becomes the
// default.
func TestEffectiveTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 3*time.Second, effectiveTimeout(3*time.Second))
	assert.Equal(t, defaultTimeout, effectiveTimeout(0))
	assert.Equal(t, defaultTimeout, effectiveTimeout(-time.Second))
}

// authScript returns the AUTH exchange for a mechanism: the scripted one, or
// the default that accepts the client.
//
// Parameters:
//   - mechanism: the SASL mechanism name.
//
// Returns:
//   - []string: the reply lines.
func (s *fakeSMTP) authScript(mechanism string) []string {
	s.mu.Lock()
	scripted := s.replies["AUTH"]
	delete(s.replies, "AUTH")
	s.mu.Unlock()

	if len(scripted) > 0 {
		return scripted
	}

	switch mechanism {
	case "LOGIN":
		return []string{"334 VXNlcm5hbWU6", "334 UGFzc3dvcmQ6", "235 2.7.0 Accepted"}
	case "CRAM-MD5":
		return []string{"334 " + base64.StdEncoding.EncodeToString([]byte("<1896.697170952@mock.local>")), "235 2.7.0 Accepted"}
	default:
		return []string{"235 2.7.0 Accepted"}
	}
}

// dial serves a new connection to the client over an in-memory pipe, or over
// loopback TCP when the server speaks TLS, because a TLS peer that rejects
// the handshake writes its alert while the other peer is still writing, which
// an unbuffered pipe cannot carry.
//
// Parameters:
//   - ctx: bounds the loopback connection.
//
// Returns:
//   - net.Conn: the client end of the connection.
//   - error: the loopback connection error.
func (s *fakeSMTP) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	if !s.implicitTLS && !s.startTLS {
		client, server := net.Pipe()

		go s.serve(ctx, server)

		return &countingConn{Conn: client, closes: &s.closes}, nil
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	defer func() { _ = listener.Close() }()

	accepted := make(chan net.Conn, 1)

	go func() {
		server, err := listener.Accept()
		if err != nil {
			close(accepted)

			return
		}

		accepted <- server

		s.serve(ctx, server)
	}()

	client, err := (&net.Dialer{}).DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		return nil, err
	}

	<-accepted

	return &countingConn{Conn: client, closes: &s.closes}, nil
}

// message returns the DATA content of the index-th message, failing the test
// when the server has not received it.
//
// Parameters:
//   - t: the test.
//   - index: the message index.
//
// Returns:
//   - string: the message, with LF line endings.
func (s *fakeSMTP) message(t *testing.T, index int) string {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()

	require.Greater(t, len(s.messages), index, "message %d not received", index)

	return s.messages[index]
}

// next returns the scripted reply for verb, or "" for the default reply.
//
// Parameters:
//   - verb: the command verb.
//
// Returns:
//   - string: the reply line, "close", "hang", or "".
func (s *fakeSMTP) next(verb string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue := s.replies[verb]
	if len(queue) == 0 {
		return ""
	}

	s.replies[verb] = queue[1:]

	return queue[0]
}

// received returns the client lines received so far.
//
// Returns:
//   - []string: the lines, in order.
func (s *fakeSMTP) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return slices.Clone(s.commands)
}

// record appends a client line.
//
// Parameters:
//   - line: the line.
func (s *fakeSMTP) record(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.commands = append(s.commands, line)
}

// serve answers one client connection until the client quits or closes it.
//
// Parameters:
//   - ctx: bounds the TLS handshakes.
//   - conn: the server end of the connection.
func (s *fakeSMTP) serve(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if certificate, err := testCertificate(); err == nil {
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	tlsActive := false

	if s.implicitTLS {
		tlsConn := tls.Server(conn, tlsConfig)
		if tlsConn.HandshakeContext(ctx) != nil {
			return
		}

		conn, tlsActive = tlsConn, true
	}

	text := textproto.NewConn(conn)

	// reply sends a line, or closes or stops on the special replies. It
	// reports whether the session continues.
	reply := func(line string) bool {
		switch line {
		case "close":
			return false
		case "hang":
			_, _ = io.Copy(io.Discard, conn)

			return false
		default:
			return text.PrintfLine("%s", line) == nil
		}
	}

	if !reply(s.greeting) {
		return
	}

	for {
		line, err := text.ReadLine()
		if err != nil {
			return
		}

		s.record(line)

		verb, argument, _ := strings.Cut(line, " ")
		verb = strings.ToUpper(verb)
		scripted := ""

		if verb != "AUTH" {
			scripted = s.next(verb)
		}

		if scripted != "" {
			if !reply(scripted) {
				return
			}

			continue
		}

		switch verb {
		case "EHLO", "HELO":
			keywords := []string{"mock.local"}
			if s.startTLS && !tlsActive {
				keywords = append(keywords, "STARTTLS")
			}

			keywords = append(append(keywords, s.extensions...), "8BITMIME")

			for i, keyword := range keywords {
				separator := "-"
				if i == len(keywords)-1 {
					separator = " "
				}

				if !reply("250" + separator + keyword) {
					return
				}
			}
		case "STARTTLS":
			if !s.startTLS || tlsActive {
				reply("502 5.5.1 STARTTLS not available")

				continue
			}

			reply("220 2.0.0 Ready to start TLS")

			tlsConn := tls.Server(conn, tlsConfig)
			if tlsConn.HandshakeContext(ctx) != nil {
				return
			}

			conn, tlsActive = tlsConn, true
			text = textproto.NewConn(conn)
		case "AUTH":
			mechanism, _, _ := strings.Cut(argument, " ")

			for _, step := range s.authScript(strings.ToUpper(mechanism)) {
				if !reply(step) {
					return
				}

				if strings.HasPrefix(step, "334") {
					response, err := text.ReadLine()
					if err != nil {
						return
					}

					s.record(response)
				}
			}
		case "DATA":
			reply("354 Start mail input; end with <CRLF>.<CRLF>")

			content, err := io.ReadAll(text.DotReader())
			if err != nil {
				return
			}

			s.mu.Lock()
			s.messages = append(s.messages, string(content))
			s.mu.Unlock()

			end := s.next("DATA-END")
			if end == "" {
				end = "250 2.0.0 Message accepted"
			}

			if !reply(end) {
				return
			}
		case "QUIT":
			reply("221 2.0.0 Bye")

			return
		case "MAIL", "RCPT", "RSET", "NOOP":
			reply("250 2.0.0 OK")
		default:
			reply("502 5.5.1 Unrecognized command")
		}
	}
}

// Close counts the close and closes the connection.
//
// Returns:
// Read reads from the wrapped connection, and ends the send's context once
// RSET has been written, so the read that returns its reply cancels the send.
//
// Parameters:
//   - p: the buffer to read into.
//
// Returns:
//   - int: the number of bytes read.
//   - error: the wrapped connection's error.
func (c *cancelAfterReset) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if c.reset.Load() {
		c.cancel()
	}

	return n, err
}

// Write writes to the wrapped connection, and notes when the client writes
// RSET.
//
// Parameters:
//   - p: the bytes to write.
//
// Returns:
//   - int: the number of bytes written.
//   - error: the wrapped connection's error.
func (c *cancelAfterReset) Write(p []byte) (int, error) {
	if bytes.HasPrefix(p, []byte("RSET")) {
		c.reset.Store(true)
	}

	return c.Conn.Write(p)
}

// - error: the close error.
func (c *countingConn) Close() error {
	c.closes.Add(1)

	return c.Conn.Close()
}
