package xmpp_test

import (
	"context"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/xmpp"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// roomMessage records one message a session sent to a room.
type roomMessage struct {
	// room is the room JID.
	room string
	// nick is the nickname used to join.
	nick string
	// password is the room password.
	password string
	// body is the message body.
	body string
}

// recordingSession is an [xmpp.Session] over a connection to the fake server.
// It records the messages it is asked to send and closes the connection when
// the session closes.
type recordingSession struct {
	// conn is the connection the session runs over.
	conn net.Conn
	// chats holds the recipient and body of each chat message.
	chats [][2]string
	// rooms holds each room message.
	rooms []roomMessage
	// closed reports whether Close was called.
	closed bool
}

// connection records one connection a send opened through the fake server.
type connection struct {
	// session is the session the factory returned.
	session *recordingSession
	// network is the dialed network.
	network string
	// addr is the dialed address.
	addr string
	// config is the config the session factory received.
	config xmpp.Config
	// hasDeadline reports whether the session factory's context had a
	// deadline.
	hasDeadline bool
}

// fakeServer stands in for an XMPP server. Its dial function connects to a
// [net.Pipe] whose server end discards what it receives, and its session
// factory returns a recordingSession over that connection. It records each
// connection a send opens.
type fakeServer struct {
	// dials maps each dialed connection to its network and address.
	dials map[net.Conn][2]string
	// mu guards dials and connections.
	mu sync.Mutex
	// connections holds each negotiated connection, in order.
	connections []connection
}

// Service URLs and limits shared by the tests.
const (
	// chatURL is a service URL with one chat recipient.
	chatURL = "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"
	// bothURL is a service URL with a chat recipient and a protected room
	// joined with a nick.
	bothURL = "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&rooms=alerts@conference.example.com&nick=bot&roompassword=pw"
	// testTimeout bounds every send, so a send that blocks fails the test
	// instead of hanging it.
	testTimeout = 10 * time.Second
)

// Close records that the session was closed and closes its connection.
//
// Returns:
//   - error: the error closing the connection.
func (s *recordingSession) Close() error {
	s.closed = true

	return s.conn.Close()
}

// SendChat records the chat message.
//
// Parameters:
//   - ctx: unused.
//   - to: the recipient JID.
//   - body: the message body.
//
// Returns:
//   - error: nil.
func (s *recordingSession) SendChat(_ context.Context, to, body string) error {
	s.chats = append(s.chats, [2]string{to, body})

	return nil
}

// SendToRoom records the room message.
//
// Parameters:
//   - ctx: unused.
//   - room: the room JID.
//   - nick: the nickname used to join.
//   - password: the room password.
//   - body: the message body.
//
// Returns:
//   - error: nil.
func (s *recordingSession) SendToRoom(_ context.Context, room, nick, password, body string) error {
	s.rooms = append(s.rooms, roomMessage{room: room, nick: nick, password: password, body: body})

	return nil
}

// Dial records the dial and connects to a [net.Pipe] whose server end
// discards everything it receives until the client end closes.
//
// Parameters:
//   - ctx: unused.
//   - network: the network to dial.
//   - addr: the address to dial.
//
// Returns:
//   - net.Conn: the client end of the pipe.
//   - error: nil.
func (f *fakeServer) Dial(_ context.Context, network, addr string) (net.Conn, error) {
	client, server := net.Pipe()

	f.mu.Lock()
	f.dials[client] = [2]string{network, addr}
	f.mu.Unlock()

	go func() {
		_, _ = io.Copy(io.Discard, server)
		_ = server.Close()
	}()

	return client, nil
}

// NewSession records the connection and returns a recordingSession over it.
//
// Parameters:
//   - ctx: the send's context, checked for a deadline.
//   - cfg: the config for this send.
//   - conn: the dialed connection.
//
// Returns:
//   - xmpp.Session: the session.
//   - error: nil.
func (f *fakeServer) NewSession(ctx context.Context, cfg *xmpp.Config, conn net.Conn) (xmpp.Session, error) {
	session := &recordingSession{conn: conn}
	_, hasDeadline := ctx.Deadline()

	f.mu.Lock()
	defer f.mu.Unlock()

	dial := f.dials[conn]
	f.connections = append(f.connections, connection{
		session:     session,
		network:     dial[0],
		addr:        dial[1],
		config:      cfg.Clone(),
		hasDeadline: hasDeadline,
	})

	return session, nil
}

// opened returns the connections opened so far.
//
// Returns:
//   - []connection: the connections, in order.
func (f *fakeServer) opened() []connection {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]connection(nil), f.connections...)
}

// newTestService returns a service initialized from rawURL that logs to the
// test output and connects to a new fake server.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *xmpp.Service: the service.
//   - *fakeServer: the fake server.
func newTestService(t *testing.T, rawURL string) (*xmpp.Service, *fakeServer) {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	server := &fakeServer{dials: map[net.Conn][2]string{}}
	service := &xmpp.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetDialContext(server.Dial)
	service.SetSessionFactory(server.NewSession)

	return service, server
}

// sendContext returns a context for a send that ends after testTimeout or
// when the test ends.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - context.Context: the context.
func sendContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)

	return ctx
}

// TestIntegrationSendConnection verifies how a send sets up its connection
// from the service URL and params: it dials TCP to the host with the port the
// URL names or the default for its scheme, and hands the session factory the
// TLS and auth settings and targets for this send, with a context the service
// bounds with a deadline even when the caller's context has none. The session
// is closed after the send.
func TestIntegrationSendConnection(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the dialed address and the
	// config settings the session factory should receive.
	tests := []struct {
		params            *types.Params
		name              string
		rawURL            string
		wantAddr          string
		wantUser          string
		wantTo            []string
		wantRooms         []string
		wantDisableTLS    bool
		wantSkipTLSVerify bool
	}{
		{
			name:     "STARTTLS on the default port",
			rawURL:   chatURL,
			wantAddr: "xmpp.example.com:5222",
			wantUser: "alice",
			wantTo:   []string{"bob@example.com"},
		},
		{
			name:     "implicit TLS on the default port",
			rawURL:   "xmpps://alice:secret@xmpp.example.com/?to=bob@example.com",
			wantAddr: "xmpp.example.com:5223",
			wantUser: "alice",
			wantTo:   []string{"bob@example.com"},
		},
		{
			name:              "custom port without TLS verification",
			rawURL:            "xmpps://alice:secret@xmpp.example.com:5224/?to=bob@example.com&skiptlsverify=yes",
			wantAddr:          "xmpp.example.com:5224",
			wantUser:          "alice",
			wantTo:            []string{"bob@example.com"},
			wantSkipTLSVerify: true,
		},
		{
			name:           "plaintext",
			rawURL:         chatURL + "&disabletls=yes",
			wantAddr:       "xmpp.example.com:5222",
			wantUser:       "alice",
			wantTo:         []string{"bob@example.com"},
			wantDisableTLS: true,
		},
		{
			name:      "full auth JID distinct from the dial host",
			rawURL:    "xmpp://alice%40jabber.example.com:secret@xmpp.example.com/?rooms=alerts@conference.example.com",
			wantAddr:  "xmpp.example.com:5222",
			wantUser:  "alice@jabber.example.com",
			wantRooms: []string{"alerts@conference.example.com"},
		},
		{
			name:      "targets from params",
			rawURL:    chatURL,
			params:    &types.Params{"to": "dave@example.com,", "rooms": "ops@conference.example.com"},
			wantAddr:  "xmpp.example.com:5222",
			wantUser:  "alice",
			wantTo:    []string{"dave@example.com"},
			wantRooms: []string{"ops@conference.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service, server := newTestService(t, tt.rawURL)

			require.NoError(t, service.SendContext(t.Context(), "hello", tt.params))

			connections := server.opened()
			require.Len(t, connections, 1)

			conn := connections[0]
			assert.Equal(t, "tcp", conn.network)
			assert.Equal(t, tt.wantAddr, conn.addr)
			assert.Equal(t, "secret", conn.config.Password)
			assert.Equal(t, tt.wantUser, conn.config.User)
			assert.Equal(t, tt.wantTo, conn.config.To)
			assert.Equal(t, tt.wantRooms, conn.config.Rooms)
			assert.Equal(t, tt.wantDisableTLS, conn.config.DisableTLS)
			assert.Equal(t, tt.wantSkipTLSVerify, conn.config.SkipTLSVerify)
			assert.True(t, conn.hasDeadline)
			assert.True(t, conn.session.closed)
		})
	}
}

// TestIntegrationSendMessageContent verifies that the message reaches chat
// recipients and rooms exactly as given, with the title from params on its own
// line, whatever its content or length.
func TestIntegrationSendMessageContent(t *testing.T) {
	t.Parallel()

	// tests lists messages and params with the body every target should
	// receive.
	tests := []struct {
		params   *types.Params
		name     string
		message  string
		wantBody string
	}{
		{name: "empty message"},
		{name: "title only", params: &types.Params{"title": "Alert"}, wantBody: "Alert"},
		{name: "title and message", message: "hello", params: &types.Params{"title": "Alert"}, wantBody: "Alert\nhello"},
		{name: "unicode", message: "Привет 🚨 ✓", wantBody: "Привет 🚨 ✓"},
		{name: "long message", message: strings.Repeat("x", 64*1024), wantBody: strings.Repeat("x", 64*1024)},
		{name: "newlines, tabs, and JSON", message: "line1\nline2\t{\"ok\":true}", wantBody: "line1\nline2\t{\"ok\":true}"},
		{name: "XML special characters", message: `<b>a & b</b> "c" 'd'`, wantBody: `<b>a & b</b> "c" 'd'`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service, server := newTestService(t, bothURL)

			require.NoError(t, service.SendContext(sendContext(t), tt.message, tt.params))

			connections := server.opened()
			require.Len(t, connections, 1)

			session := connections[0].session
			assert.Equal(t, [][2]string{{"bob@example.com", tt.wantBody}}, session.chats)
			assert.Equal(t, []roomMessage{{room: "alerts@conference.example.com", nick: "bot", password: "pw", body: tt.wantBody}}, session.rooms)
		})
	}
}

// TestIntegrationConcurrentSends verifies that concurrent sends through one
// service each open their own connection and deliver their own message, and
// that their params leave the service config unchanged.
func TestIntegrationConcurrentSends(t *testing.T) {
	t.Parallel()

	const sends = 8

	service, server := newTestService(t, chatURL)
	want := service.Config.Clone()

	var wg sync.WaitGroup
	for i := range sends {
		wg.Go(func() {
			title := "Alert " + strconv.Itoa(i)
			assert.NoError(t, service.SendContext(sendContext(t), "hello", &types.Params{"title": title}))
		})
	}

	wg.Wait()

	connections := server.opened()
	require.Len(t, connections, sends)

	bodies := make([]string, 0, sends)

	for _, conn := range connections {
		require.Len(t, conn.session.chats, 1)
		assert.Equal(t, "bob@example.com", conn.session.chats[0][0])
		assert.True(t, conn.session.closed)

		bodies = append(bodies, conn.session.chats[0][1])
	}

	wantBodies := make([]string, 0, sends)
	for i := range sends {
		wantBodies = append(wantBodies, "Alert "+strconv.Itoa(i)+"\nhello")
	}

	assert.ElementsMatch(t, wantBodies, bodies)
	assert.Equal(t, want, *service.Config)
}
