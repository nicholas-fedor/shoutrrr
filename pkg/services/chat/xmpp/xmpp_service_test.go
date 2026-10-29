package xmpp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// roomSend records one SendToRoom call.
type roomSend struct {
	// room is the room JID.
	room string
	// nick is the nickname used to join.
	nick string
	// password is the room password.
	password string
	// body is the message body.
	body string
}

// recordingSession is a [Session] that records the messages it is asked to
// send and fails the targets listed in failures.
type recordingSession struct {
	// failures maps a target JID to the error sending to it returns.
	failures map[string]error
	// chats holds the recipient and body of each SendChat call.
	chats [][2]string
	// rooms holds each SendToRoom call.
	rooms []roomSend
	// closed reports whether Close was called.
	closed bool
}

// Values shared by the service tests.
const (
	// chatURL is a service URL with one chat recipient.
	chatURL = "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"
)

// Errors returned by the failing test doubles.
var (
	// errChat is the error a failing chat send returns.
	errChat = errors.New("chat failed")
	// errRoom is the error a failing room send returns.
	errRoom = errors.New("room failed")
	// errNegotiate is the error a failing session factory returns.
	errNegotiate = errors.New("negotiate failed")
)

// Close records that the session was closed.
//
// Returns:
//   - error: nil.
func (s *recordingSession) Close() error {
	s.closed = true

	return nil
}

// SendChat records the chat message and returns the failure for to, if any.
//
// Parameters:
//   - ctx: unused.
//   - to: the recipient JID.
//   - body: the message body.
//
// Returns:
//   - error: the failure configured for to, or nil.
func (s *recordingSession) SendChat(_ context.Context, to, body string) error {
	s.chats = append(s.chats, [2]string{to, body})

	return s.failures[to]
}

// SendToRoom records the room message and returns the failure for room, if
// any.
//
// Parameters:
//   - ctx: unused.
//   - room: the room JID.
//   - nick: the nickname used to join.
//   - password: the room password.
//   - body: the message body.
//
// Returns:
//   - error: the failure configured for room, or nil.
func (s *recordingSession) SendToRoom(_ context.Context, room, nick, password, body string) error {
	s.rooms = append(s.rooms, roomSend{room: room, nick: nick, password: password, body: body})

	return s.failures[room]
}

// pipeDialer returns a dial function that connects to a [net.Pipe] whose
// server end discards everything it receives.
//
// Returns:
//   - types.DialContextFunc: the dial function.
func pipeDialer() types.DialContextFunc {
	return func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()

		go func() {
			_, _ = io.Copy(io.Discard, server)
			_ = server.Close()
		}()

		return client, nil
	}
}

// newTestService returns a service initialized from rawURL that logs to the
// test output, dials a pipe, and uses session for every send.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - session: the session every send uses.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string, session Session) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(t.Output(), "", 0)))
	service.SetDialContext(pipeDialer())
	service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
		return session, nil
	})

	return service
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "xmpp", (&Service{}).GetID())
}

// TestServiceTimeout verifies that the router budget for a send is the
// session timeout.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, defaultTimeout, (&Service{}).ServiceTimeout(nil))
}

// TestInitialize verifies that Initialize parses the service URL and selects
// the mellium session factory, and rejects a URL without targets.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		require.NoError(t, service.Initialize(parseTestURL(t, chatURL), log.New(t.Output(), "", 0)))

		assert.Equal(t, "xmpp.example.com", service.Config.Host)
		assert.Equal(t, []string{"bob@example.com"}, service.Config.To)
		assert.NotNil(t, service.newSession)
	})

	t.Run("missing targets", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "xmpp://alice:secret@xmpp.example.com/"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrMissingTargets)
	})
}

// TestSend verifies that a send delivers the message, with the title on its
// own line, to every chat recipient and then every room, joining rooms with
// the configured nick or the auth localpart and the room password.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the calls the session should
	// receive.
	tests := []struct {
		params    *types.Params
		name      string
		rawURL    string
		wantChats [][2]string
		wantRooms []roomSend
	}{
		{
			name:      "chat recipient",
			rawURL:    chatURL,
			wantChats: [][2]string{{"bob@example.com", "hello"}},
		},
		{
			name:      "room with title and room password",
			rawURL:    "xmpp://alice:secret@xmpp.example.com/?rooms=alerts@conference.example.com&title=Alert&roompassword=pw",
			wantRooms: []roomSend{{room: "alerts@conference.example.com", nick: "alice", password: "pw", body: "Alert\nhello"}},
		},
		{
			name:   "chats and rooms with a nick",
			rawURL: "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com,carol@example.com&rooms=alerts@conference.example.com&nick=bot",
			wantChats: [][2]string{
				{"bob@example.com", "hello"},
				{"carol@example.com", "hello"},
			},
			wantRooms: []roomSend{{room: "alerts@conference.example.com", nick: "bot", body: "hello"}},
		},
		{
			name:      "params",
			rawURL:    chatURL,
			params:    &types.Params{"to": "dave@example.com", "rooms": "ops@conference.example.com", "nick": "pager", "title": "Alert"},
			wantChats: [][2]string{{"dave@example.com", "Alert\nhello"}},
			wantRooms: []roomSend{{room: "ops@conference.example.com", nick: "pager", body: "Alert\nhello"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			session := &recordingSession{}
			service := newTestService(t, tt.rawURL, session)

			require.NoError(t, service.Send("hello", tt.params))

			assert.Equal(t, tt.wantChats, session.chats)
			assert.Equal(t, tt.wantRooms, session.rooms)
			assert.True(t, session.closed)
		})
	}
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	session := &recordingSession{}
	service := newTestService(t, chatURL, session)
	want := service.Config.Clone()

	require.NoError(t, service.Send("hello", &types.Params{"to": "dave@example.com", "title": "Alert"}))

	assert.Equal(t, want, *service.Config)
}

// TestSendDialsConfiguredAddress verifies that a send dials TCP to the
// configured host and port.
func TestSendDialsConfiguredAddress(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		network string
		addr    string
	)

	service := newTestService(t, "xmpp://alice:secret@xmpp.example.com:5225/?to=bob@example.com", &recordingSession{})
	service.SetDialContext(func(ctx context.Context, gotNetwork, gotAddr string) (net.Conn, error) {
		mu.Lock()
		network, addr = gotNetwork, gotAddr
		mu.Unlock()

		return pipeDialer()(ctx, gotNetwork, gotAddr)
	})

	require.NoError(t, service.Send("hello", nil))

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, "tcp", network)
	assert.Equal(t, "xmpp.example.com:5225", addr)
}

// TestSendContextErrors verifies that a send reports a dial or session error,
// closes the connection when the session cannot be set up, keeps sending
// after a failed target, and rejects invalid params before dialing.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("dial error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, chatURL, &recordingSession{})
		service.SetDialContext(func(ctx context.Context, _, _ string) (net.Conn, error) {
			return nil, ctx.Err()
		})

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := service.SendContext(ctx, "hello", nil)

		require.ErrorIs(t, err, context.Canceled)
		assert.Contains(t, err.Error(), "dialing xmpp.example.com:5222")
	})

	t.Run("default dialer", func(t *testing.T) {
		t.Parallel()

		// A refused loopback port fails the dial without a DNS lookup.
		service := &Service{}
		require.NoError(t, service.Initialize(parseTestURL(t, "xmpp://alice:secret@127.0.0.1:1/?to=bob@example.com"), log.New(t.Output(), "", 0)))

		err := service.SendContext(t.Context(), "hello", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "dialing 127.0.0.1:1")
	})

	t.Run("session setup fails", func(t *testing.T) {
		t.Parallel()

		closed := make(chan struct{})
		service := newTestService(t, chatURL, nil)
		service.SetDialContext(func(context.Context, string, string) (net.Conn, error) {
			client, server := net.Pipe()

			go func() {
				_, _ = io.Copy(io.Discard, server)

				close(closed)
			}()

			return client, nil
		})
		service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
			return nil, errNegotiate
		})

		require.ErrorIs(t, service.SendContext(t.Context(), "hello", nil), errNegotiate)
		<-closed
	})

	t.Run("failed targets", func(t *testing.T) {
		t.Parallel()

		session := &recordingSession{failures: map[string]error{
			"fail@example.com":              errChat,
			"alerts@conference.example.com": errRoom,
		}}
		service := newTestService(t,
			"xmpp://alice:secret@xmpp.example.com/?to=fail@example.com,bob@example.com&rooms=alerts@conference.example.com,ops@conference.example.com",
			session)

		err := service.SendContext(t.Context(), "hello", nil)

		require.ErrorIs(t, err, errChat)
		require.ErrorIs(t, err, errRoom)
		assert.Contains(t, err.Error(), "sending chat to fail@example.com")
		assert.Contains(t, err.Error(), "sending groupchat to alerts@conference.example.com")
		assert.Len(t, session.chats, 2)
		assert.Len(t, session.rooms, 2)
	})

	// rejectedParams lists params that fail the send before any dial.
	rejectedParams := []struct {
		params  *types.Params
		wantErr error
		name    string
	}{
		{name: "unknown param", params: &types.Params{"notakey": "1"}, wantErr: format.ErrInvalidConfigKey},
		{name: "params clear every target", params: &types.Params{"to": ""}, wantErr: ErrMissingTargets},
		{name: "invalid recipient param", params: &types.Params{"to": "not-a-jid"}, wantErr: ErrInvalidRecipientJID},
	}

	for _, tt := range rejectedParams {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, chatURL, &recordingSession{})
			service.SetDialContext(func(context.Context, string, string) (net.Conn, error) {
				t.Error("unexpected dial")

				return nil, errNegotiate
			})

			require.ErrorIs(t, service.SendContext(t.Context(), "hello", tt.params), tt.wantErr)
		})
	}
}

// TestSendContextDeadline verifies what ends a session negotiation that never
// finishes. A send without a deadline is bounded by the 30 second send budget,
// and a caller's longer deadline applies as it is.
func TestSendContextDeadline(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		deadline time.Duration
		want     time.Duration
	}{
		{name: "send budget", want: defaultTimeout},
		{name: "longer caller deadline", deadline: time.Minute, want: time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				service := newTestService(t, chatURL, &recordingSession{})
				service.SetDialContext(pipeDialer())
				service.SetSessionFactory(func(ctx context.Context, _ *Config, _ net.Conn) (Session, error) {
					<-ctx.Done()

					return nil, ctx.Err()
				})

				ctx := t.Context()

				if tt.deadline > 0 {
					var cancel context.CancelFunc

					ctx, cancel = context.WithTimeout(ctx, tt.deadline)
					defer cancel()
				}

				start := time.Now()

				require.ErrorIs(t, service.SendContext(ctx, "hello", nil), context.DeadlineExceeded)
				assert.Equal(t, tt.want, time.Since(start))
			})
		})
	}
}

// TestSendWarnings verifies that a send without TLS, or without certificate
// verification, logs the shared warning once over two sends and again after
// the service is initialized again. A send with verified TLS logs nothing.
func TestSendWarnings(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		params *types.Params
		name   string
		rawURL string
		want   []string
	}{
		{name: "verified TLS", rawURL: chatURL},
		{name: "skiptlsverify in the URL", rawURL: chatURL + "&skiptlsverify=yes", want: []string{"WARN skipping TLS certificate verification"}},
		{
			name:   "skiptlsverify param",
			rawURL: chatURL,
			params: &types.Params{"skiptlsverify": "yes"},
			want:   []string{"WARN skipping TLS certificate verification"},
		},
		{name: "TLS disabled", rawURL: chatURL + "&disabletls=yes", want: []string{"WARN sending without TLS"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			service := &Service{}
			require.NoError(t, service.Initialize(parseTestURL(t, tt.rawURL), log.New(&logs, "", 0)))
			service.SetDialContext(pipeDialer())
			service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
				return &recordingSession{}, nil
			})

			require.NoError(t, service.Send("hello", tt.params))
			require.NoError(t, service.Send("hello", tt.params))
			assert.Equal(t, strings.Join(tt.want, ""), strings.TrimSuffix(logs.String(), "\n"))

			require.NoError(t, service.Initialize(parseTestURL(t, tt.rawURL), log.New(&logs, "", 0)))
			require.NoError(t, service.Send("hello", tt.params))
			assert.Equal(t, 2*len(tt.want), strings.Count(logs.String(), "WARN "))
		})
	}
}

// TestSendThroughDefaultSession verifies a send end to end with the default
// session factory restored by a nil factory: the service negotiates a session
// with an in-process server and delivers the chat message to it.
func TestSendThroughDefaultSession(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	srv := newFakeServer(t, server)

	rawURL := "xmpp://alice:" + testPassword + "@localhost/?to=bob@localhost&disabletls=yes&title=Alert"
	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(t.Output(), "", 0)))
	service.SetSessionFactory(func(context.Context, *Config, net.Conn) (Session, error) {
		return &recordingSession{}, nil
	})
	service.SetSessionFactory(nil)
	service.SetDialContext(func(context.Context, string, string) (net.Conn, error) {
		return client, nil
	})

	require.NoError(t, service.Send("hello", nil))

	<-srv.done

	messages, _ := srv.received()
	require.Len(t, messages, 1)
	assert.Equal(t, messageBody{XMLName: messages[0].XMLName, To: "bob@localhost", Type: "chat", Body: "Alert\nhello"}, messages[0])
}
