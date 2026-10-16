package xmpp

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/muc"
	"mellium.im/xmpp/stanza"

	mxmpp "mellium.im/xmpp"
)

// mucJoin is the XEP-0045 element a presence carries to join a room.
type mucJoin struct {
	// History limits the discussion history sent on join.
	History *struct {
		// MaxStanzas is the number of history messages requested.
		MaxStanzas string `xml:"maxstanzas,attr"`
	} `xml:"history"`
	// Password is the room password.
	Password string `xml:"password"`
}

// receivedPresence records a presence stanza and the MUC join element in it.
type receivedPresence struct {
	// MUC holds the join element, when the presence has one.
	MUC *mucJoin
	// Presence is the presence stanza.
	Presence stanza.Presence
}

// fakeServer is an in-process XMPP server on one end of a [net.Pipe]. It
// negotiates a plaintext stream with SASL PLAIN and resource binding, records
// the message stanzas it receives, and answers each presence with the
// self-presence a MUC service sends for a join or leave.
type fakeServer struct {
	// mu guards messages and presences.
	mu sync.Mutex
	// messages holds the received message stanzas in order.
	messages []messageBody
	// presences holds the received presence stanzas in order.
	presences []receivedPresence
	// done is closed when the server stops serving.
	done chan struct{}
}

// Values shared by the client tests.
const (
	// testPassword is the password the fake server accepts.
	testPassword = "secret"
	// testRoom is a MUC room JID.
	testRoom = "alerts@conference.localhost"
)

// newFakeServer runs a fake XMPP server on conn. It negotiates the stream,
// then serves stanzas until the client closes the stream.
//
// Parameters:
//   - t: the test that owns the server.
//   - conn: the server end of the connection.
//
// Returns:
//   - *fakeServer: the server.
func newFakeServer(t *testing.T, conn net.Conn) *fakeServer {
	t.Helper()

	srv := &fakeServer{done: make(chan struct{})}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	go func() {
		defer close(srv.done)
		defer func() { _ = conn.Close() }()

		session, err := mxmpp.ReceiveSession(ctx, conn, 0, mxmpp.NewNegotiator(
			func(*mxmpp.Session, *mxmpp.StreamConfig) mxmpp.StreamConfig {
				return mxmpp.StreamConfig{Features: srv.features()}
			},
		))
		if err != nil {
			return
		}

		_ = session.Serve(mxmpp.HandlerFunc(srv.handle))
	}()

	return srv
}

// startFakeServer runs a fake XMPP server and returns it with a client
// session negotiated against it over a [net.Pipe].
//
// Parameters:
//   - t: the test that owns the server and session.
//   - cfg: the client config, which must disable TLS.
//
// Returns:
//   - *fakeServer: the server.
//   - Session: the client session.
func startFakeServer(t *testing.T, cfg *Config) (*fakeServer, Session) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	srv := newFakeServer(t, serverConn)

	session, err := newMelliumSession(t.Context(), cfg, clientConn)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = session.Close()

		<-srv.done
	})

	return srv, session
}

// features returns the stream features the fake server offers: SASL PLAIN
// that accepts only testPassword, without requiring TLS, and resource binding.
//
// Returns:
//   - []mxmpp.StreamFeature: the features.
func (*fakeServer) features() []mxmpp.StreamFeature {
	saslFeature := mxmpp.SASLServer(func(n *sasl.Negotiator) bool {
		_, password, _ := n.Credentials()

		return string(password) == testPassword
	}, sasl.Plain)
	saslFeature.Necessary = 0

	bind := mxmpp.BindCustom(func(j jid.JID, resource string) (jid.JID, error) {
		return j.WithResource(resource)
	})

	return []mxmpp.StreamFeature{saslFeature, bind}
}

// handle records a message stanza, or records a presence and echoes it back
// with a MUC user element, as a room does to confirm a join or leave.
//
// Parameters:
//   - t: the stream to read the stanza from and write replies to.
//   - start: the start element of the stanza.
//
// Returns:
//   - error: a decoding or encoding error.
func (s *fakeServer) handle(t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
	switch start.Name.Local {
	case "message":
		// Serve reads the closing tag itself, so only the inner tokens are
		// decoded, wrapped in the start element.
		var msg messageBody
		if err := xml.NewTokenDecoder(xmlstream.Wrap(xmlstream.Inner(t), *start)).Decode(&msg); err != nil {
			return err
		}

		s.mu.Lock()
		s.messages = append(s.messages, msg)
		s.mu.Unlock()

		return nil
	case "presence":
		presence, err := stanza.NewPresence(*start)
		if err != nil {
			return err
		}

		var children struct {
			MUC *mucJoin `xml:"http://jabber.org/protocol/muc x"`
		}
		if err := xml.NewTokenDecoder(xmlstream.Wrap(xmlstream.Inner(t), *start)).Decode(&children); err != nil {
			return err
		}

		s.mu.Lock()
		s.presences = append(s.presences, receivedPresence{MUC: children.MUC, Presence: presence})
		s.mu.Unlock()

		presence.To, presence.From = presence.From, presence.To
		_, err = xmlstream.Copy(t, presence.Wrap(xmlstream.Wrap(
			nil,
			xml.StartElement{Name: xml.Name{Space: muc.NSUser, Local: "x"}},
		)))

		return err
	default:
		return nil
	}
}

// received returns the message and presence stanzas received so far.
//
// Returns:
//   - []messageBody: the messages, in order.
//   - []receivedPresence: the presences, in order.
func (s *fakeServer) received() ([]messageBody, []receivedPresence) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]messageBody(nil), s.messages...), append([]receivedPresence(nil), s.presences...)
}

// waitForMessages waits until the server has received n messages.
//
// Parameters:
//   - t: the test.
//   - n: the number of messages to wait for.
func (s *fakeServer) waitForMessages(t *testing.T, n int) {
	t.Helper()

	require.Eventually(t, func() bool {
		messages, _ := s.received()

		return len(messages) >= n
	}, 5*time.Second, 5*time.Millisecond)
}

// newPlaintextConfig returns a config for a plaintext session as alice.
//
// Returns:
//   - *Config: the config.
func newPlaintextConfig() *Config {
	return &Config{User: "alice", Host: "localhost", Password: testPassword, scheme: Scheme, DisableTLS: true}
}

// TestMelliumSessionSendChat verifies that a chat message is sent as a
// type=chat message stanza to the recipient's JID with the body.
func TestMelliumSessionSendChat(t *testing.T) {
	t.Parallel()

	srv, session := startFakeServer(t, newPlaintextConfig())

	require.NoError(t, session.SendChat(t.Context(), "bob@localhost", "Hello, Bob"))

	srv.waitForMessages(t, 1)
	messages, _ := srv.received()
	assert.Equal(t, "bob@localhost", messages[0].To)
	assert.Equal(t, "chat", messages[0].Type)
	assert.Equal(t, "Hello, Bob", messages[0].Body)
}

// TestMelliumSessionSendToRoom verifies the XEP-0045 exchange for a room
// message: an available presence to the occupant JID room/nick that requests
// no history and carries the room password when one is set, a type=groupchat
// message to the bare room JID, and an unavailable presence to the occupant
// JID to leave.
func TestMelliumSessionSendToRoom(t *testing.T) {
	t.Parallel()

	// tests lists room passwords and whether the join should carry one.
	tests := []struct {
		name     string
		password string
	}{
		{name: "open room"},
		{name: "protected room", password: "roompass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv, session := startFakeServer(t, newPlaintextConfig())

			require.NoError(t, session.SendToRoom(t.Context(), testRoom, "bot", tt.password, "Deploy finished"))

			messages, presences := srv.received()
			require.Len(t, messages, 1)
			assert.Equal(t, testRoom, messages[0].To)
			assert.Equal(t, "groupchat", messages[0].Type)
			assert.Equal(t, "Deploy finished", messages[0].Body)

			require.Len(t, presences, 2)

			join := presences[0]
			assert.Equal(t, testRoom+"/bot", join.Presence.To.String())
			assert.Equal(t, stanza.AvailablePresence, join.Presence.Type)
			require.NotNil(t, join.MUC)
			require.NotNil(t, join.MUC.History)
			assert.Equal(t, "0", join.MUC.History.MaxStanzas)
			assert.Equal(t, tt.password, join.MUC.Password)

			leave := presences[1]
			assert.Equal(t, testRoom+"/bot", leave.Presence.To.String())
			assert.Equal(t, stanza.UnavailablePresence, leave.Presence.Type)
		})
	}
}

// TestMelliumSessionClose verifies that closing a session ends the stream,
// and that a session without a stream closes without error.
func TestMelliumSessionClose(t *testing.T) {
	t.Parallel()

	t.Run("negotiated session", func(t *testing.T) {
		t.Parallel()

		srv, session := startFakeServer(t, newPlaintextConfig())

		require.NoError(t, session.Close())

		select {
		case <-srv.done:
		case <-time.After(5 * time.Second):
			t.Fatal("the server kept serving after the client closed the session")
		}
	})

	t.Run("no session", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, (&melliumSession{}).Close())
	})
}

// TestMelliumSessionInvalidJIDs verifies that an invalid recipient or room JID
// fails before anything is sent.
func TestMelliumSessionInvalidJIDs(t *testing.T) {
	t.Parallel()

	session := &melliumSession{}

	require.ErrorIs(t, session.SendChat(t.Context(), "alice@", "hello"), ErrInvalidRecipientJID)
	require.ErrorIs(t, session.SendToRoom(t.Context(), "alice@", "nick", "", "hello"), ErrInvalidRoomJID)
}

// TestNewMelliumSessionErrors verifies the negotiation failures: an invalid
// auth JID, a wrong password reported as ErrAuthenticationFailed, and a
// connection that closes during the TLS handshake, STARTTLS, or a plaintext
// stream.
func TestNewMelliumSessionErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid auth JID", func(t *testing.T) {
		t.Parallel()

		_, err := newMelliumSession(t.Context(), &Config{User: "alice@", Host: "xmpp.example.com", Password: testPassword}, nil)

		require.Error(t, err)
	})

	t.Run("wrong password", func(t *testing.T) {
		t.Parallel()

		clientConn, serverConn := net.Pipe()
		srv := newFakeServer(t, serverConn)

		cfg := newPlaintextConfig()
		cfg.Password = "wrong"

		_, err := newMelliumSession(t.Context(), cfg, clientConn)

		require.ErrorIs(t, err, ErrAuthenticationFailed)

		_ = clientConn.Close()

		<-srv.done
	})

	// closedTests lists configs whose negotiation fails on a closed
	// connection, with the text the error should contain.
	closedTests := []struct {
		name string
		want string
		cfg  Config
	}{
		{
			name: "implicit TLS",
			cfg:  Config{User: "alice", Host: "localhost", Password: testPassword, scheme: SchemeTLS, SkipTLSVerify: true},
			want: "TLS handshake",
		},
		{
			name: "STARTTLS",
			cfg:  Config{User: "alice", Host: "localhost", Password: testPassword, scheme: Scheme},
			want: "negotiating XMPP session",
		},
		{
			name: "plaintext",
			cfg:  Config{User: "alice", Host: "localhost", Password: testPassword, scheme: Scheme, DisableTLS: true},
			want: "negotiating XMPP session",
		},
	}

	for _, tt := range closedTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, server := net.Pipe()
			require.NoError(t, server.Close())

			_, err := newMelliumSession(t.Context(), &tt.cfg, client)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// TestWrapNegotiateError verifies that SASL failures become
// ErrAuthenticationFailed and other negotiation errors do not.
func TestWrapNegotiateError(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"not-authorized",
		"Invalid username or password",
		"invalid-authzid",
		"credentials-expired",
		"account-disabled",
		"mechanism-too-weak",
		"temporary-auth-failure",
	} {
		require.ErrorIs(t, wrapNegotiateError(errors.New(text)), ErrAuthenticationFailed, text)
	}

	err := wrapNegotiateError(errors.New("features advertised out of order"))

	require.NotErrorIs(t, err, ErrAuthenticationFailed)
	assert.Contains(t, err.Error(), "negotiating XMPP session")
}

// TestNewTLSConfig verifies that the TLS config names the host, requires TLS
// 1.2, and skips verification only when asked.
func TestNewTLSConfig(t *testing.T) {
	t.Parallel()

	verified := newTLSConfig(&Config{Host: "xmpp.example.com"})
	assert.Equal(t, "xmpp.example.com", verified.ServerName)
	assert.Equal(t, uint16(tls.VersionTLS12), verified.MinVersion)
	assert.False(t, verified.InsecureSkipVerify)

	assert.True(t, newTLSConfig(&Config{Host: "xmpp.example.com", SkipTLSVerify: true}).InsecureSkipVerify)
}
