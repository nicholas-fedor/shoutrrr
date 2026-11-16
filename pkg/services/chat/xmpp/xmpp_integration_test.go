package xmpp_test

import (
	"context"
	"encoding/binary"
	"errors"
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
	"golang.org/x/net/dns/dnsmessage"

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

// dialAttempt records one dial other than a DNS query.
type dialAttempt struct {
	// deadline is the dial context's deadline, or the zero time for a context
	// without one.
	deadline time.Time
	// addr is the dialed address.
	addr string
}

// fakeServer stands in for an XMPP server and its DNS server. Its dial
// function connects to a [net.Pipe]. A dial to port 53 reaches a DNS server
// that answers the SRV records in srv and NXDOMAIN for any other name. Any
// other dial reaches a server end that discards what it receives, unless its
// address is in refuse. The session factory returns a recordingSession over
// the connection. It records each address a send dials, the DNS queries, and
// each connection a send opens.
type fakeServer struct {
	// dials maps each dialed connection to its network and address.
	dials map[net.Conn][2]string
	// srv holds the SRV records the DNS server answers, by query name, such
	// as "_xmpp-client._tcp.example.com.".
	srv map[string][]dnsmessage.SRVResource
	// refuse holds the addresses whose dial fails.
	refuse map[string]bool
	// queries holds the name of each DNS query, in order.
	queries []string
	// attempts holds each dial other than a DNS query, in order.
	attempts []dialAttempt
	// connections holds each negotiated connection, in order.
	connections []connection
	// mu guards every other field.
	mu sync.Mutex
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
	// dnsPort is the port of every DNS query the resolver dials.
	dnsPort = "53"
	// domainURL is a service URL whose host is the auth JID's domain, with no
	// port, so a send looks the server up through SRV records.
	domainURL = "xmpp://alice:secret@example.com/?to=bob@example.com"
	// clientSRV is the name of the STARTTLS SRV records of example.com.
	clientSRV = "_xmpp-client._tcp.example.com."
)

// errRefused is the error a dial to a refused address returns.
var errRefused = errors.New("connection refused")

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

// Dial records the dial and connects to a [net.Pipe]. A dial to port 53
// reaches the DNS server, a refused address fails, and any other dial reaches
// a server end that discards everything it receives until the client end
// closes.
//
// Parameters:
//   - ctx: the dial's context, whose deadline is recorded.
//   - network: the network to dial.
//   - addr: the address to dial.
//
// Returns:
//   - net.Conn: the client end of the pipe.
//   - error: nil.
func (f *fakeServer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	client, server := net.Pipe()

	if _, port, _ := net.SplitHostPort(addr); port == dnsPort {
		go f.serveDNS(server)

		return client, nil
	}

	deadline, _ := ctx.Deadline()

	f.mu.Lock()
	f.attempts = append(f.attempts, dialAttempt{deadline: deadline, addr: addr})
	refused := f.refuse[addr]

	if !refused {
		f.dials[client] = [2]string{network, addr}
	}
	f.mu.Unlock()

	if refused {
		_ = client.Close()
		_ = server.Close()

		return nil, errRefused
	}

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

// addresses returns the addresses of the dials so far, other than DNS
// queries, and whether any DNS query was made.
//
// Returns:
//   - []string: the dialed addresses, in order.
//   - bool: true when the resolver queried the DNS server.
func (f *fakeServer) addresses() ([]string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	addrs := make([]string, 0, len(f.attempts))
	for _, attempt := range f.attempts {
		addrs = append(addrs, attempt.addr)
	}

	return addrs, len(f.queries) > 0
}

// answerDNS builds the answer to one DNS query: the SRV records of the queried
// name, or NXDOMAIN when there are none.
//
// Parameters:
//   - query: the query message.
//
// Returns:
//   - []byte: the answer message.
//   - error: the error from parsing the query or building the answer.
func (f *fakeServer) answerDNS(query []byte) ([]byte, error) {
	var parser dnsmessage.Parser

	header, err := parser.Start(query)
	if err != nil {
		return nil, err
	}

	question, err := parser.Question()
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.queries = append(f.queries, question.Name.String())
	records := f.srv[question.Name.String()]
	f.mu.Unlock()

	rcode := dnsmessage.RCodeSuccess
	if len(records) == 0 {
		rcode = dnsmessage.RCodeNameError
	}

	builder := dnsmessage.NewBuilder(nil, dnsmessage.Header{
		ID:            header.ID,
		Response:      true,
		Authoritative: true,
		RCode:         rcode,
	})
	builder.EnableCompression()

	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}

	if err := builder.Question(question); err != nil {
		return nil, err
	}

	if err := builder.StartAnswers(); err != nil {
		return nil, err
	}

	for i := range records {
		resource := dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeSRV, Class: dnsmessage.ClassINET, TTL: 60}
		if err := builder.SRVResource(resource, records[i]); err != nil {
			return nil, err
		}
	}

	return builder.Finish()
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

// serveDNS answers the DNS queries on conn until the resolver closes it. The
// resolver frames each message with a two-byte length, as over TCP, because
// conn is not a [net.PacketConn].
//
// Parameters:
//   - conn: the server end of the connection.
func (f *fakeServer) serveDNS(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	for {
		var size uint16
		if err := binary.Read(conn, binary.BigEndian, &size); err != nil {
			return
		}

		query := make([]byte, size)
		if _, err := io.ReadFull(conn, query); err != nil {
			return
		}

		answer, err := f.answerDNS(query)
		if err != nil {
			return
		}

		if err := binary.Write(conn, binary.BigEndian, uint16(len(answer))); err != nil {
			return
		}

		if _, err := conn.Write(answer); err != nil {
			return
		}
	}
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

	server := &fakeServer{dials: map[net.Conn][2]string{}, srv: map[string][]dnsmessage.SRVResource{}, refuse: map[string]bool{}}
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

// srvRecord returns an SRV record for target at port and priority.
//
// Parameters:
//   - target: the target host, rooted with a trailing dot, or "." for none.
//   - port: the target port.
//   - priority: the record's priority. Lower comes first.
//
// Returns:
//   - dnsmessage.SRVResource: the record.
func srvRecord(target string, port, priority uint16) dnsmessage.SRVResource {
	return dnsmessage.SRVResource{Priority: priority, Weight: 1, Port: port, Target: dnsmessage.MustNewName(target)}
}

// TestIntegrationSendSRV verifies how a send finds its server. Without a
// port, a host that is the auth JID's domain is looked up through the SRV
// records of the URL's scheme, and the targets are dialed in order until one
// connects, each at its rooted name. Without records, the host is dialed at
// the scheme's default port. A configured port skips the lookup. The DNS
// queries go through the injected dial function.
func TestIntegrationSendSRV(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and SRV records with the addresses the send
	// should dial and whether it should look anything up.
	tests := []struct {
		srv          map[string][]dnsmessage.SRVResource
		name         string
		rawURL       string
		wantAttempts []string
		wantLookup   bool
	}{
		{
			name:         "STARTTLS record",
			rawURL:       domainURL,
			srv:          map[string][]dnsmessage.SRVResource{clientSRV: {srvRecord("xmpp.example.net.", 5300, 10)}},
			wantAttempts: []string{"xmpp.example.net.:5300"},
			wantLookup:   true,
		},
		{
			name:         "direct TLS record",
			rawURL:       "xmpps://alice:secret@example.com/?to=bob@example.com",
			srv:          map[string][]dnsmessage.SRVResource{"_xmpps-client._tcp.example.com.": {srvRecord("xmpp.example.net.", 443, 10)}},
			wantAttempts: []string{"xmpp.example.net.:443"},
			wantLookup:   true,
		},
		{
			name:         "direct TLS without its own record",
			rawURL:       "xmpps://alice:secret@example.com/?to=bob@example.com",
			srv:          map[string][]dnsmessage.SRVResource{clientSRV: {srvRecord("xmpp.example.net.", 5222, 10)}},
			wantAttempts: []string{"example.com:5223"},
			wantLookup:   true,
		},
		{
			name:         "root target beside another",
			rawURL:       domainURL,
			srv:          map[string][]dnsmessage.SRVResource{clientSRV: {srvRecord(".", 0, 10), srvRecord("xmpp.example.net.", 5222, 20)}},
			wantAttempts: []string{"xmpp.example.net.:5222"},
			wantLookup:   true,
		},
		{
			name:   "first target that connects",
			rawURL: domainURL,
			srv: map[string][]dnsmessage.SRVResource{clientSRV: {
				srvRecord("backup.example.net.", 5222, 20),
				srvRecord("primary.example.net.", 5222, 10),
			}},
			wantAttempts: []string{"primary.example.net.:5222"},
			wantLookup:   true,
		},
		{
			name:         "internationalized domain",
			rawURL:       "xmpp://alice:secret@bücher.example/?to=bob@example.com",
			srv:          map[string][]dnsmessage.SRVResource{"_xmpp-client._tcp.xn--bcher-kva.example.": {srvRecord("xmpp.example.net.", 5222, 10)}},
			wantAttempts: []string{"xmpp.example.net.:5222"},
			wantLookup:   true,
		},
		{
			name:         "no records",
			rawURL:       domainURL,
			wantAttempts: []string{"example.com:5222"},
			wantLookup:   true,
		},
		{
			name:         "configured port",
			rawURL:       "xmpp://alice:secret@example.com:5222/?to=bob@example.com",
			srv:          map[string][]dnsmessage.SRVResource{clientSRV: {srvRecord("xmpp.example.net.", 5300, 10)}},
			wantAttempts: []string{"example.com:5222"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service, server := newTestService(t, tt.rawURL)
			server.srv = tt.srv

			require.NoError(t, service.SendContext(sendContext(t), "hello", nil))

			attempts, lookedUp := server.addresses()
			assert.Equal(t, tt.wantAttempts, attempts)
			assert.Equal(t, tt.wantLookup, lookedUp)

			connections := server.opened()
			require.Len(t, connections, 1)
			assert.Equal(t, tt.wantAttempts[len(tt.wantAttempts)-1], connections[0].addr)
		})
	}
}

// TestIntegrationSendSRVNextTarget verifies that a refused target gives way to
// the next. Every target but the last gets at most 10 seconds of a caller's
// longer deadline, and the last gets the deadline as it is.
func TestIntegrationSendSRVNextTarget(t *testing.T) {
	t.Parallel()

	service, server := newTestService(t, domainURL)
	server.srv = map[string][]dnsmessage.SRVResource{clientSRV: {
		srvRecord("primary.example.net.", 5222, 10),
		srvRecord("backup.example.net.", 5222, 20),
	}}
	server.refuse["primary.example.net.:5222"] = true

	ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
	defer cancel()

	want, _ := ctx.Deadline()

	require.NoError(t, service.SendContext(ctx, "hello", nil))

	server.mu.Lock()
	defer server.mu.Unlock()

	require.Len(t, server.attempts, 2)
	assert.Equal(t, "primary.example.net.:5222", server.attempts[0].addr)
	assert.WithinDuration(t, time.Now().Add(10*time.Second), server.attempts[0].deadline, 5*time.Second)
	assert.Equal(t, dialAttempt{deadline: want, addr: "backup.example.net.:5222"}, server.attempts[1])
}

// TestIntegrationSendSRVErrors verifies the sends that SRV records fail. When
// every target is ".", the domain offers no XMPP service, so nothing is
// dialed. When every target is refused, the error names each, and the host
// itself is not tried, as RFC 6120 section 3.2.1 asks.
func TestIntegrationSendSRVErrors(t *testing.T) {
	t.Parallel()

	// rootTargets lists records whose targets are all ".".
	rootTargets := map[string][]dnsmessage.SRVResource{
		"one record":  {srvRecord(".", 0, 10)},
		"two records": {srvRecord(".", 0, 10), srvRecord(".", 0, 20)},
	}

	for name, records := range rootTargets {
		t.Run("no XMPP service with "+name, func(t *testing.T) {
			t.Parallel()

			service, server := newTestService(t, domainURL)
			server.srv = map[string][]dnsmessage.SRVResource{clientSRV: records}

			require.ErrorIs(t, service.SendContext(sendContext(t), "hello", nil), xmpp.ErrServiceNotOffered)

			attempts, _ := server.addresses()
			assert.Empty(t, attempts)
		})
	}

	t.Run("every target refused", func(t *testing.T) {
		t.Parallel()

		service, server := newTestService(t, domainURL)
		server.srv = map[string][]dnsmessage.SRVResource{clientSRV: {
			srvRecord("primary.example.net.", 5222, 10),
			srvRecord("backup.example.net.", 5222, 20),
		}}
		server.refuse["primary.example.net.:5222"] = true
		server.refuse["backup.example.net.:5222"] = true

		err := service.SendContext(sendContext(t), "hello", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "dialing primary.example.net.:5222")
		assert.Contains(t, err.Error(), "dialing backup.example.net.:5222")

		attempts, _ := server.addresses()
		assert.Equal(t, []string{"primary.example.net.:5222", "backup.example.net.:5222"}, attempts)
	})
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
