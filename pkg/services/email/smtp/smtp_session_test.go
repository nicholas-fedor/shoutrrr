package smtp

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"mime"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/failures"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// failureIDs lists the failure IDs an error should match.
type failureIDs []failures.FailureID

// b64 returns s in standard base64, as SMTP AUTH exchanges carry it.
//
// Parameters:
//   - s: the text to encode.
//
// Returns:
//   - string: the encoded text.
func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// parseMessage parses a message the server received.
//
// Parameters:
//   - t: the test.
//   - raw: the message.
//
// Returns:
//   - *mail.Message: the parsed message.
//   - string: the message body.
func parseMessage(t *testing.T, raw string) (*mail.Message, string) {
	t.Helper()

	message, err := mail.ReadMessage(strings.NewReader(raw))
	require.NoError(t, err)

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(message.Body)
	require.NoError(t, err)

	return message, body.String()
}

// newTestSession returns a session for config over a new connection to
// server, logging to the test output.
//
// Parameters:
//   - t: the test.
//   - server: the server to connect to.
//   - config: the session config.
//
// Returns:
//   - *session: the session.
func newTestSession(t *testing.T, server *fakeSMTP, config *Config) *session {
	t.Helper()

	client, err := dialClient(t.Context(), config, server.dial)
	require.NoError(t, err)

	svc := &Service{}
	svc.SetLogger(log.New(t.Output(), "", 0))

	return &session{client: client, config: config, svc: svc}
}

// TestSessionRun verifies the SMTP conversation of a send: the greeting,
// STARTTLS when the server offers it and the URL asks for it, AUTH with the
// configured mechanism, and one MAIL, RCPT, and DATA transaction per
// recipient, separated by RSET and followed by QUIT.
func TestSessionRun(t *testing.T) {
	t.Parallel()

	cramResponse := func() string {
		mac := hmac.New(md5.New, []byte("pass"))
		mac.Write([]byte("<1896.697170952@mock.local>"))

		return b64("user " + hex.EncodeToString(mac.Sum(nil)))
	}()

	// tests lists URLs and servers with the client lines each send should
	// produce.
	tests := []struct {
		server func() *fakeSMTP
		params *types.Params
		name   string
		rawURL string
		want   []string
	}{
		{
			name:   "no authentication",
			rawURL: testServiceURL,
			want:   []string{"EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:   "client host",
			rawURL: testServiceURL + "&clienthost=client.example.com",
			want:   []string{"EHLO client.example.com", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:   "several recipients",
			rawURL: "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=a@example.com,b@example.com&usestarttls=no",
			want: []string{
				"EHLO localhost", testMailFrom, "RCPT TO:<a@example.com>", "DATA", "RSET",
				testMailFrom, "RCPT TO:<b@example.com>", "DATA", "QUIT",
			},
		},
		{
			name:   "plus-tagged addresses",
			rawURL: "smtp://localhost:2525/?fromaddress=sender+alerts@example.com&toaddresses=rec+a@example.com&usestarttls=no",
			want:   []string{"EHLO localhost", "MAIL FROM:<sender+alerts@example.com> BODY=8BITMIME", "RCPT TO:<rec+a@example.com>", "DATA", "QUIT"},
		},
		{
			name:   "spaces around addresses",
			rawURL: "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=a@example.com,%20b+x@example.com%20&usestarttls=no",
			want: []string{
				"EHLO localhost", testMailFrom, "RCPT TO:<a@example.com>", "DATA", "RSET",
				testMailFrom, "RCPT TO:<b+x@example.com>", "DATA", "QUIT",
			},
		},
		{
			name:   "spaces around param addresses",
			rawURL: testServiceURL,
			params: &types.Params{"toaddresses": "a@example.com, b+x@example.com "},
			want: []string{
				"EHLO localhost", testMailFrom, "RCPT TO:<a@example.com>", "DATA", "RSET",
				testMailFrom, "RCPT TO:<b+x@example.com>", "DATA", "QUIT",
			},
		},
		{
			name:   "PLAIN",
			server: func() *fakeSMTP { return newFakeSMTP("AUTH PLAIN") },
			rawURL: "smtp://user:pass@localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no",
			want:   []string{"EHLO localhost", "AUTH PLAIN " + b64("\x00user\x00pass"), testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:   "PLAIN without credentials",
			server: func() *fakeSMTP { return newFakeSMTP("AUTH PLAIN") },
			rawURL: testServiceURL + "&auth=plain",
			want:   []string{"EHLO localhost", "AUTH PLAIN " + b64("\x00\x00"), testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:   "LOGIN",
			server: func() *fakeSMTP { return newFakeSMTP("AUTH LOGIN") },
			rawURL: "smtp://user:pass@localhost:2525/?auth=login&fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no",
			want:   []string{"EHLO localhost", "AUTH LOGIN", b64("user"), b64("pass"), testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:   "CRAM-MD5",
			server: func() *fakeSMTP { return newFakeSMTP("AUTH CRAM-MD5") },
			rawURL: "smtp://user:pass@localhost:2525/?auth=crammd5&fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no",
			want:   []string{"EHLO localhost", "AUTH CRAM-MD5", cramResponse, testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:   "XOAUTH2",
			server: func() *fakeSMTP { return newFakeSMTP("AUTH XOAUTH2") },
			rawURL: "smtp://user:token@localhost:2525/?auth=oauth2&fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no",
			want: []string{
				"EHLO localhost", "AUTH XOAUTH2 " + b64("user=user\x01auth=Bearer token\x01\x01"), testMailFrom, testRcptTo, "DATA", "QUIT",
			},
		},
		{
			name: "STARTTLS",
			server: func() *fakeSMTP {
				server := newFakeSMTP("AUTH PLAIN")
				server.startTLS = true

				return server
			},
			rawURL: "smtp://user:pass@localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&skiptlsverify=yes",
			want:   []string{"EHLO localhost", "STARTTLS", "EHLO localhost", "AUTH PLAIN " + b64("\x00user\x00pass"), testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name: "STARTTLS with encryption None",
			server: func() *fakeSMTP {
				server := newFakeSMTP()
				server.startTLS = true

				return server
			},
			rawURL: "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&encryption=none&skiptlsverify=yes",
			want:   []string{"EHLO localhost", "STARTTLS", "EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name: "STARTTLS turned off",
			server: func() *fakeSMTP {
				server := newFakeSMTP()
				server.startTLS = true

				return server
			},
			rawURL: testServiceURL,
			want:   []string{"EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:   "STARTTLS not offered",
			rawURL: "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			want:   []string{"EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name: "implicit TLS",
			server: func() *fakeSMTP {
				server := newFakeSMTP("AUTH PLAIN")
				server.implicitTLS = true

				return server
			},
			rawURL: "smtp://user:pass@localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&encryption=implicittls&skiptlsverify=yes",
			want:   []string{"EHLO localhost", "AUTH PLAIN " + b64("\x00user\x00pass"), testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name: "implicit TLS with STARTTLS offered",
			server: func() *fakeSMTP {
				server := newFakeSMTP("STARTTLS")
				server.implicitTLS = true

				return server
			},
			rawURL: "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&encryption=implicittls&skiptlsverify=yes",
			want:   []string{"EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name: "skiptlsverify param",
			server: func() *fakeSMTP {
				server := newFakeSMTP()
				server.implicitTLS = true

				return server
			},
			rawURL: "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&encryption=implicittls",
			params: &types.Params{"skiptlsverify": "yes"},
			want:   []string{"EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newFakeSMTP()
			if tt.server != nil {
				server = tt.server()
			}

			require.NoError(t, newTestService(t, tt.rawURL, server).Send("Hello", tt.params))
			assert.Equal(t, tt.want, server.received())
		})
	}
}

// TestSessionRunMessage verifies the message each recipient receives: the
// headers addressed to that recipient alone, the subject and sender name, and
// the body, through the plain template when one is set.
func TestSessionRunMessage(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP()
	service := newTestService(t,
		"smtp://localhost:2525/?fromaddress=sender@example.com&fromname=Alerts&toaddresses=a@example.com,b@example.com&subject=Disk%20full&usestarttls=no",
		server)
	require.NoError(t, service.SetTemplateString(templatePlain, "Note: {{.message}}"))

	require.NoError(t, service.Send("90% used", nil))

	for i, recipient := range []string{"<a@example.com>", "<b@example.com>"} {
		message, body := parseMessage(t, server.message(t, i))

		assert.Equal(t, recipient, message.Header.Get("To"))
		assert.Equal(t, `"Alerts" <sender@example.com>`, message.Header.Get("From"))
		assert.Equal(t, "Disk full", message.Header.Get("Subject"))
		assert.Equal(t, `text/plain; charset="UTF-8"`, message.Header.Get("Content-Type"))
		assert.Empty(t, message.Header.Get("Cc"))
		assert.Empty(t, message.Header.Get("Bcc"))
		assert.Equal(t, "Note: 90% used\n", body)
	}
}

// TestSessionRunHTML verifies that an HTML send carries a plain and an HTML
// part, each through its template, between the boundary that the
// Content-Type header names.
func TestSessionRunHTML(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP()
	service := newTestService(t, testServiceURL+"&usehtml=yes", server)
	require.NoError(t, service.SetTemplateString(templatePlain, "plain: {{.message}}"))
	require.NoError(t, service.SetTemplateString(templateHTML, "<p>{{.message}}</p>"))

	require.NoError(t, service.Send("Hello", nil))

	message, body := parseMessage(t, server.message(t, 0))

	contentType, boundary, found := strings.Cut(message.Header.Get("Content-Type"), "boundary=")
	require.True(t, found, "no boundary in %q", message.Header.Get("Content-Type"))
	assert.Equal(t, "multipart/alternative; ", contentType)
	assert.Len(t, boundary, 16)
	assert.Equal(t,
		"\n\n--"+boundary+"\nContent-Type: text/plain; charset=\"UTF-8\"\nContent-Transfer-Encoding: 8bit\n\nplain: Hello"+
			"\n\n--"+boundary+"\nContent-Type: text/html; charset=\"UTF-8\"\nContent-Transfer-Encoding: 8bit\n\n<p>Hello</p>"+
			"\n\n--"+boundary+"--\n",
		body)
}

// TestSessionRunLongSubject verifies that a long subject reaches the server
// folded within the recommended line lengths and parses back to the original
// text, whether plain ASCII or RFC 2047 encoded.
func TestSessionRunLongSubject(t *testing.T) {
	t.Parallel()

	// tests lists subjects with the line limit each folded header should keep.
	tests := []struct {
		name    string
		subject string
		limit   int
	}{
		{name: "ASCII", subject: strings.TrimSpace(strings.Repeat("Backup of the database finished ", 8)), limit: 78},
		{name: "UTF-8", subject: strings.TrimSpace(strings.Repeat("Sicherung der Datenbank abgeschlossen: Größe geprüft ", 8)), limit: 76},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newFakeSMTP()
			require.NoError(t, newTestService(t, testServiceURL+"&subject="+url.QueryEscape(tt.subject), server).Send("Hello", nil))

			raw := server.message(t, 0)
			header, _, _ := strings.Cut(raw, "\n\n")

			for line := range strings.SplitSeq(header, "\n") {
				assert.LessOrEqual(t, len(line), tt.limit, line)
			}

			message, _ := parseMessage(t, raw)

			subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
			require.NoError(t, err)
			assert.Equal(t, tt.subject, subject)
		})
	}
}

// TestSessionRunEmptyMessage verifies that an empty message is sent with an
// empty body.
func TestSessionRunEmptyMessage(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP()
	require.NoError(t, newTestService(t, testServiceURL, server).Send("", nil))

	_, body := parseMessage(t, server.message(t, 0))
	assert.Empty(t, body)
}

// TestSessionRunLogsStartTLS verifies that a send warns when the server does
// not offer STARTTLS and continues without it, along with the shared warning
// about sending without TLS. Each warning is logged once over two sends.
func TestSessionRunLogsStartTLS(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	server := newFakeSMTP()
	service := newLoggingService(t, "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com", server, log.New(&logs, "", 0))

	require.NoError(t, service.Send("Hello", nil))
	require.NoError(t, service.Send("Hello", nil))
	assert.Equal(t,
		"Warning: StartTLS enabled, but server does not support it. Connection is unencrypted\n"+
			"WARN sending without TLS\n"+
			"Mail successfully sent to \"rec@example.com\"!\n"+
			"Mail successfully sent to \"rec@example.com\"!\n",
		logs.String())
}

// TestSessionRunErrors verifies the failure of each rejected step, that a
// send stops at a missing sender or a rejected greeting, STARTTLS, or AUTH
// without sending mail, and that it closes the connection. After a failed AUTH, net/smtp itself
// cancels the exchange with "*" and sends QUIT.
func TestSessionRunErrors(t *testing.T) {
	t.Parallel()

	// tests lists URLs, servers, and params with the failure each send
	// should produce and the client lines the server should receive.
	tests := []struct {
		wantCause error
		params    *types.Params
		server    func() *fakeSMTP
		name      string
		rawURL    string
		want      []string
		wantFail  failureIDs
	}{
		{
			name:     "greeting rejected",
			server:   withReplies(map[string][]string{"EHLO": {"500 5.5.1 Not now"}, "HELO": {"500 5.5.1 Not now"}}),
			rawURL:   testServiceURL,
			wantFail: failureIDs{FailHandshake},
			want:     []string{"EHLO localhost", "HELO localhost"},
		},
		{
			name:      "STARTTLS required but not offered",
			rawURL:    "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&requirestarttls=yes",
			wantFail:  failureIDs{FailEnableStartTLS},
			wantCause: ErrServerNoStartTLS,
			want:      []string{"EHLO localhost"},
		},
		{
			name: "STARTTLS rejected",
			server: func() *fakeSMTP {
				server := newFakeSMTP("STARTTLS")
				server.replies["STARTTLS"] = []string{"454 4.7.0 TLS not available"}

				return server
			},
			rawURL:   "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			wantFail: failureIDs{FailEnableStartTLS},
			want:     []string{"EHLO localhost", "STARTTLS"},
		},
		{
			name: "STARTTLS with an untrusted certificate",
			server: func() *fakeSMTP {
				server := newFakeSMTP()
				server.startTLS = true

				return server
			},
			rawURL:   "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com",
			wantFail: failureIDs{FailEnableStartTLS},
			want:     []string{"EHLO localhost", "STARTTLS"},
		},
		{
			name:      "blank sender param",
			rawURL:    testServiceURL,
			params:    &types.Params{"fromaddress": " "},
			wantFail:  failureIDs{FailSetSender},
			wantCause: ErrFromAddressMissing,
		},
		{
			name:      "empty sender param",
			rawURL:    testServiceURL,
			params:    &types.Params{"fromaddress": ""},
			wantFail:  failureIDs{FailSetSender},
			wantCause: ErrFromAddressMissing,
		},
		{
			name:     "authentication method unknown",
			rawURL:   testServiceURL,
			params:   &types.Params{"auth": "unknown"},
			wantFail: failureIDs{FailAuthType},
			want:     []string{"EHLO localhost"},
		},
		{
			name:     "PLAIN rejected",
			server:   withReplies(map[string][]string{"AUTH": {"535 5.7.8 Authentication credentials invalid"}}),
			rawURL:   "smtp://user:pass@localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no",
			wantFail: failureIDs{FailAuthenticating},
			want:     []string{"EHLO localhost", "AUTH PLAIN " + b64("\x00user\x00pass"), "*", "QUIT"},
		},
		{
			name:     "LOGIN rejected after the password",
			server:   withReplies(map[string][]string{"AUTH": {"334 VXNlcm5hbWU6", "334 UGFzc3dvcmQ6", "535 5.7.8 Authentication credentials invalid"}}),
			rawURL:   "smtp://user:pass@localhost:2525/?auth=login&fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no",
			wantFail: failureIDs{FailAuthenticating},
			want:     []string{"EHLO localhost", "AUTH LOGIN", b64("user"), b64("pass"), "*", "QUIT"},
		},
		{
			name: "XOAUTH2 rejected with an error status",
			server: withReplies(map[string][]string{"AUTH": {
				"334 " + b64(`{"status":"400","schemes":"Bearer","scope":"https://mail.google.com/"}`), "535 5.7.8 Username and Password not accepted",
			}}),
			rawURL:   "smtp://user:token@localhost:2525/?auth=oauth2&fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no",
			wantFail: failureIDs{FailAuthenticating},
			want:     []string{"EHLO localhost", "AUTH XOAUTH2 " + b64("user=user\x01auth=Bearer token\x01\x01"), "", "*", "QUIT"},
		},
		{
			name:     "PLAIN to a remote server without TLS",
			rawURL:   "smtp://user:pass@mail.example.com:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com&usestarttls=no",
			wantFail: failureIDs{FailAuthenticating},
			want:     []string{"EHLO localhost", "QUIT"},
		},
		{
			name:     "sender rejected",
			server:   withReplies(map[string][]string{"MAIL": {"550 5.1.0 Sender rejected"}}),
			rawURL:   testServiceURL,
			wantFail: failureIDs{FailSendRecipient, FailSetSender},
			want:     []string{"EHLO localhost", testMailFrom, "QUIT"},
		},
		{
			name:     "recipient rejected",
			server:   withReplies(map[string][]string{"RCPT": {"550 5.1.1 User unknown"}}),
			rawURL:   testServiceURL,
			wantFail: failureIDs{FailSendRecipient, FailSetRecipient},
			want:     []string{"EHLO localhost", testMailFrom, testRcptTo, "QUIT"},
		},
		{
			name:     "DATA rejected",
			server:   withReplies(map[string][]string{"DATA": {"554 5.5.1 No valid recipients"}}),
			rawURL:   testServiceURL,
			wantFail: failureIDs{FailSendRecipient, FailOpenDataStream},
			want:     []string{"EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:     "message rejected",
			server:   withReplies(map[string][]string{"DATA-END": {"554 5.7.1 Message rejected as spam"}}),
			rawURL:   testServiceURL,
			wantFail: failureIDs{FailSendRecipient, FailCloseDataStream},
			want:     []string{"EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
		{
			name:      "sender with a line break",
			rawURL:    testServiceURL,
			params:    &types.Params{"fromaddress": "sender@example.com\r\nBcc: victim@example.com"},
			wantFail:  failureIDs{FailSendRecipient, FailSetSender},
			wantCause: errHeaderBreak,
			want:      []string{"EHLO localhost", "QUIT"},
		},
		{
			name:      "recipient with a line break",
			rawURL:    testServiceURL,
			params:    &types.Params{"toaddresses": "rec@example.com\nBcc: victim@example.com"},
			wantFail:  failureIDs{FailSendRecipient, FailSetRecipient},
			wantCause: errHeaderBreak,
			want:      []string{"EHLO localhost", "QUIT"},
		},
		{
			name:     "QUIT rejected",
			server:   withReplies(map[string][]string{"QUIT": {"502 5.5.1 You cannot quit"}}),
			rawURL:   testServiceURL,
			wantFail: failureIDs{FailSendRecipient, FailClosingSession},
			want:     []string{"EHLO localhost", testMailFrom, testRcptTo, "DATA", "QUIT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newFakeSMTP()
			if tt.server != nil {
				server = tt.server()
			}

			err := newTestService(t, tt.rawURL, server).Send("Hello", tt.params)

			for _, id := range tt.wantFail {
				require.ErrorIs(t, err, fail(id, nil))
			}

			if tt.wantCause != nil {
				require.ErrorIs(t, err, tt.wantCause)
			}

			assert.Equal(t, tt.want, server.received())
			assert.Positive(t, server.closes.Load())
		})
	}
}

// TestSessionRunUntrustedCertificate verifies that a STARTTLS upgrade to a
// server with an untrusted certificate fails certificate verification.
func TestSessionRunUntrustedCertificate(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP()
	server.startTLS = true

	err := newTestService(t, "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=rec@example.com", server).Send("Hello", nil)

	_, isCertErr := errors.AsType[*tls.CertificateVerificationError](err)
	assert.True(t, isCertErr, "error %v is not a certificate verification error", err)
}

// TestSessionRunRecipientFailures verifies that a send delivers to the other
// recipients after one is rejected and reports the rejected one, and stops
// when the server rejects RSET between recipients.
func TestSessionRunRecipientFailures(t *testing.T) {
	t.Parallel()

	const rawURL = "smtp://localhost:2525/?fromaddress=sender@example.com&toaddresses=a@example.com,b@example.com,c@example.com&usestarttls=no"

	t.Run("rejected recipient", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTP()
		server.replies["RCPT"] = []string{"", "550 5.1.1 User unknown"}

		err := newTestService(t, rawURL, server).Send("Hello", nil)

		require.ErrorIs(t, err, fail(FailSetRecipient, nil))
		assert.Contains(t, err.Error(), `error sending message to recipient "b@example.com"`)
		assert.NotContains(t, err.Error(), "a@example.com")
		assert.NotContains(t, err.Error(), "c@example.com")

		first, _ := parseMessage(t, server.message(t, 0))
		second, _ := parseMessage(t, server.message(t, 1))
		assert.Equal(t, "<a@example.com>", first.Header.Get("To"))
		assert.Equal(t, "<c@example.com>", second.Header.Get("To"))
	})

	t.Run("RSET rejected", func(t *testing.T) {
		t.Parallel()

		server := newFakeSMTP()
		server.replies["RSET"] = []string{"421 4.3.0 Service shutting down"}

		err := newTestService(t, rawURL, server).Send("Hello", nil)

		require.ErrorIs(t, err, fail(FailResetSession, nil))
		assert.Equal(t, []string{"EHLO localhost", testMailFrom, "RCPT TO:<a@example.com>", "DATA", "RSET", "QUIT"}, server.received())
	})
}

// TestSessionQuit verifies that a successful QUIT closes the connection once,
// and that a short reply to QUIT, after the mail was accepted, is logged
// without failing the send.
func TestSessionQuit(t *testing.T) {
	t.Parallel()

	t.Run("successful QUIT", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer

		server := newFakeSMTP()
		require.NoError(t, newLoggingService(t, testServiceURL, server, log.New(&logs, "", 0)).Send("Hello", nil))

		assert.Equal(t, int32(1), server.closes.Load())
		assert.NotContains(t, logs.String(), "Failed to close")
	})

	t.Run("short reply", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer

		server := newFakeSMTP()
		server.replies["QUIT"] = []string{"22"}

		require.NoError(t, newLoggingService(t, testServiceURL, server, log.New(&logs, "", 0)).Send("Hello", nil))
		assert.Contains(t, logs.String(), `Warning: Ignoring session closure error (delivery succeeded): short response: "22"`+"\n")
	})
}

// TestSessionWithoutRecipients verifies that a session without recipients
// fails before the greeting.
func TestSessionWithoutRecipients(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP()
	config := newInitialConfig()
	config.Host, config.FromAddress = "localhost", "sender@example.com"

	err := newTestSession(t, server, config).run(t.Context(), "Hello")

	require.ErrorIs(t, err, fail(FailSendRecipient, nil))
	require.ErrorIs(t, err, ErrToAddressMissing)
	assert.Empty(t, server.received())
	assert.Equal(t, int32(1), server.closes.Load())
}

// TestSessionAuth verifies that no authentication sends no AUTH command.
func TestSessionAuth(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP("AUTH PLAIN")
	config := newInitialConfig()
	config.Host, config.Auth = "localhost", AuthNone

	session := newTestSession(t, server, config)
	t.Cleanup(session.closeIfOpen)

	require.NoError(t, session.auth())
	assert.Empty(t, server.received())
}

// TestSessionCloseIfOpen verifies that a session closes its connection once.
func TestSessionCloseIfOpen(t *testing.T) {
	t.Parallel()

	server := newFakeSMTP()
	config := newInitialConfig()
	config.Host = "localhost"

	session := newTestSession(t, server, config)

	session.closeIfOpen()
	session.closeIfOpen()

	assert.True(t, session.closed)
	assert.Equal(t, int32(1), server.closes.Load())
}

// TestSessionEhloName verifies the EHLO name: the client host, or the local
// host name for auto.
func TestSessionEhloName(t *testing.T) {
	t.Parallel()

	hostname, err := os.Hostname()
	require.NoError(t, err)

	assert.Equal(t, "client.example.com", (&session{config: &Config{ClientHost: "client.example.com"}}).ehloName())
	assert.Equal(t, hostname, (&session{config: &Config{ClientHost: "auto"}}).ehloName())
}

// withReplies returns a server constructor with the given scripted replies.
//
// Parameters:
//   - replies: the scripted replies by command verb.
//
// Returns:
//   - func() *fakeSMTP: the constructor.
func withReplies(replies map[string][]string) func() *fakeSMTP {
	return func() *fakeSMTP {
		server := newFakeSMTP()
		server.replies = replies

		return server
	}
}
