package xmpp

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
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
	// silentLeave makes the server record an unavailable presence without
	// answering it, as when the room's confirmation of a leave is lost.
	silentLeave atomic.Bool
	// refuseJoin makes the server answer a join with a not-authorized error
	// presence, as a room does for a missing or wrong password.
	refuseJoin atomic.Bool
	// silentJoin makes the server record a join without answering it.
	silentJoin atomic.Bool
	// bareRefusal makes the server refuse a join with an error presence that
	// has no child elements.
	bareRefusal atomic.Bool
	// omitSelfStatus makes the server answer a join without status 110, as
	// the presence of another occupant would.
	omitSelfStatus atomic.Bool
	// assignedNick, when set, is the nickname the server assigns on join
	// instead of the requested one.
	assignedNick atomic.Pointer[string]
}

// Values shared by the client tests.
const (
	// testPassword is the password the fake server accepts.
	testPassword = "secret"
	// testRoom is a MUC room JID.
	testRoom = "alerts@conference.localhost"
	// testTimeout bounds each session call in the tests, so a reply the client
	// waits for and never gets fails the test instead of hanging it.
	testTimeout = 5 * time.Second
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

// answerJoin answers a join presence, already addressed back to the client, as
// the server's modes select: a refusal, or the self-presence with status 110
// from the requested or the assigned occupant JID.
//
// Parameters:
//   - t: the stream to write the answer to.
//   - presence: the join presence with its to and from swapped.
//
// Returns:
//   - error: an encoding error.
func (s *fakeServer) answerJoin(t xmlstream.TokenReadEncoder, presence *stanza.Presence) error {
	switch {
	case s.bareRefusal.Load():
		presence.Type = stanza.ErrorPresence
		_, err := xmlstream.Copy(t, presence.Wrap(nil))

		return err
	case s.refuseJoin.Load():
		presence.Type = stanza.ErrorPresence
		refusal := stanza.Error{Type: stanza.Auth, Condition: stanza.NotAuthorized}
		_, err := xmlstream.Copy(t, presence.Wrap(refusal.TokenReader()))

		return err
	default:
	}

	if nick := s.assignedNick.Load(); nick != nil {
		assigned, err := presence.From.WithResource(*nick)
		if err != nil {
			return err
		}

		presence.From = assigned
	}

	var status xml.TokenReader
	if !s.omitSelfStatus.Load() {
		status = xmlstream.Wrap(nil, xml.StartElement{
			Name: xml.Name{Local: "status"},
			Attr: []xml.Attr{{Name: xml.Name{Local: "code"}, Value: "110"}},
		})
	}

	_, err := xmlstream.Copy(t, presence.Wrap(xmlstream.Wrap(
		status,
		xml.StartElement{Name: xml.Name{Space: muc.NSUser, Local: "x"}},
	)))

	return err
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

		if presence.Type == stanza.UnavailablePresence && s.silentLeave.Load() ||
			presence.Type == stanza.AvailablePresence && s.silentJoin.Load() {
			return nil
		}

		presence.To, presence.From = presence.From, presence.To

		if presence.Type == stanza.AvailablePresence {
			return s.answerJoin(t, &presence)
		}

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
	}, testTimeout, 5*time.Millisecond)
}

// waitForPresences waits until the server has received n presences.
//
// Parameters:
//   - t: the test.
//   - n: the number of presences to wait for.
func (s *fakeServer) waitForPresences(t *testing.T, n int) {
	t.Helper()

	require.Eventually(t, func() bool {
		_, presences := s.received()

		return len(presences) >= n
	}, testTimeout, 5*time.Millisecond)
}

// testContext returns a context for one session call that ends after
// testTimeout.
//
// Parameters:
//   - t: the test.
//
// Returns:
//   - context.Context: the context.
func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	t.Cleanup(cancel)

	return ctx
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

	require.NoError(t, session.SendChat(testContext(t), "bob@localhost", "Hello, Bob"))

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

			require.NoError(t, session.SendToRoom(testContext(t), testRoom, "bot", tt.password, "Deploy finished"))

			// The leave is sent without waiting for the room to answer it.
			srv.waitForPresences(t, 2)
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

// TestMelliumSessionSendToRoomWithoutLeaveReply verifies that a room send
// returns once the leave is sent, without waiting for the room to confirm it,
// so a lost confirmation cannot hold the send until its deadline.
func TestMelliumSessionSendToRoomWithoutLeaveReply(t *testing.T) {
	t.Parallel()

	srv, session := startFakeServer(t, newPlaintextConfig())
	srv.silentLeave.Store(true)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	require.NoError(t, session.SendToRoom(ctx, testRoom, "bot", "", "Deploy finished"))

	srv.waitForPresences(t, 2)
	messages, presences := srv.received()
	require.Len(t, messages, 1)
	assert.Equal(t, stanza.UnavailablePresence, presences[1].Presence.Type)
}

// TestMelliumSessionSendToRoomAssignedNick verifies that when the room assigns
// another nickname on join, the send succeeds and the leave goes to the
// assigned occupant JID.
func TestMelliumSessionSendToRoomAssignedNick(t *testing.T) {
	t.Parallel()

	srv, session := startFakeServer(t, newPlaintextConfig())
	assigned := "bot2"
	srv.assignedNick.Store(&assigned)

	require.NoError(t, session.SendToRoom(testContext(t), testRoom, "bot", "", "Deploy finished"))

	srv.waitForPresences(t, 2)
	messages, presences := srv.received()
	require.Len(t, messages, 1)
	assert.Equal(t, testRoom+"/bot", presences[0].Presence.To.String())
	assert.Equal(t, testRoom+"/bot2", presences[1].Presence.To.String())
	assert.Equal(t, stanza.UnavailablePresence, presences[1].Presence.Type)
}

// TestMelliumSessionSendToRoomRefused verifies that a room that refuses the
// join fails the send with the room's stanza error, before any message is sent.
func TestMelliumSessionSendToRoomRefused(t *testing.T) {
	t.Parallel()

	srv, session := startFakeServer(t, newPlaintextConfig())
	srv.refuseJoin.Store(true)

	err := session.SendToRoom(testContext(t), testRoom, "bot", "", "Deploy finished")

	var stanzaErr stanza.Error
	require.ErrorAs(t, err, &stanzaErr)
	assert.Equal(t, stanza.NotAuthorized, stanzaErr.Condition)

	messages, _ := srv.received()
	assert.Empty(t, messages)
}

// TestMelliumSessionSendToRoomJoinErrors verifies the joins that fail before
// any message is sent. A join the room never answers, or answers without
// status 110, ends with the caller's context. A refusal without child elements
// fails with a read error. A join to a room the session already joins, or on a
// closed session, fails without waiting for the room.
func TestMelliumSessionSendToRoomJoinErrors(t *testing.T) {
	t.Parallel()

	t.Run("unanswered join", func(t *testing.T) {
		t.Parallel()

		srv, session := startFakeServer(t, newPlaintextConfig())
		srv.silentJoin.Store(true)

		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()

		err := session.SendToRoom(ctx, testRoom, "bot", "", "Deploy finished")
		require.ErrorIs(t, err, context.DeadlineExceeded)

		messages, _ := srv.received()
		assert.Empty(t, messages)
	})

	t.Run("answer without status 110", func(t *testing.T) {
		t.Parallel()

		srv, session := startFakeServer(t, newPlaintextConfig())
		srv.omitSelfStatus.Store(true)

		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()

		err := session.SendToRoom(ctx, testRoom, "bot", "", "Deploy finished")
		require.ErrorIs(t, err, context.DeadlineExceeded)

		messages, _ := srv.received()
		assert.Empty(t, messages)
	})

	t.Run("refusal without child elements", func(t *testing.T) {
		t.Parallel()

		srv, session := startFakeServer(t, newPlaintextConfig())
		srv.bareRefusal.Store(true)

		err := session.SendToRoom(testContext(t), testRoom, "bot", "", "Deploy finished")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading the room's error")

		messages, _ := srv.received()
		assert.Empty(t, messages)
	})

	t.Run("join already in progress", func(t *testing.T) {
		t.Parallel()

		srv, session := startFakeServer(t, newPlaintextConfig())

		client, ok := session.(*melliumSession)
		require.True(t, ok, "the session must be a *melliumSession")

		_, done, err := client.joins.register(jid.MustParse(testRoom + "/other"))
		require.NoError(t, err)

		defer done()

		err = session.SendToRoom(testContext(t), testRoom, "bot", "", "Deploy finished")
		require.ErrorIs(t, err, ErrJoinInProgress)

		_, presences := srv.received()
		assert.Empty(t, presences)
	})

	t.Run("closed session", func(t *testing.T) {
		t.Parallel()

		srv, session := startFakeServer(t, newPlaintextConfig())
		require.NoError(t, session.Close())

		err := session.SendToRoom(testContext(t), testRoom, "bot", "", "Deploy finished")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "sending join presence")

		messages, _ := srv.received()
		assert.Empty(t, messages)
	})
}

// TestRoomJoinsAnswer verifies that an answer reaches only the join waiting
// for its room, that a join keeps its first answer, and that an error presence
// without an error element still ends the join with an error.
func TestRoomJoinsAnswer(t *testing.T) {
	t.Parallel()

	occupancy := jid.MustParse(testRoom + "/bot")

	t.Run("answer without a waiting join", func(t *testing.T) {
		t.Parallel()

		joins := newRoomJoins()
		joins.answer(occupancy, joinAnswer{err: nil, occupant: occupancy})

		assert.Empty(t, joins.waiting)
	})

	t.Run("first answer kept", func(t *testing.T) {
		t.Parallel()

		joins := newRoomJoins()

		answer, done, err := joins.register(occupancy)
		require.NoError(t, err)

		defer done()

		refusal := errors.New("refused")
		joins.answer(occupancy, joinAnswer{err: refusal, occupant: jid.JID{}})
		joins.answer(occupancy, joinAnswer{err: nil, occupant: occupancy})

		require.ErrorIs(t, (<-answer).err, refusal)
	})

	t.Run("error presence without an error element", func(t *testing.T) {
		t.Parallel()

		joins := newRoomJoins()

		answer, done, err := joins.register(occupancy)
		require.NoError(t, err)

		defer done()

		tokens := struct {
			xml.TokenReader
			xmlstream.Encoder
		}{
			TokenReader: xml.NewDecoder(strings.NewReader(`<presence type="error"/>`)),
			Encoder:     nil,
		}
		presence := stanza.Presence{From: occupancy, Type: stanza.ErrorPresence}
		require.NoError(t, joins.handleErrorPresence(presence, tokens))

		result := <-answer
		require.Error(t, result.err)
		assert.Contains(t, result.err.Error(), "reading the room's error")
	})
}

// TestRoomJoinsRegister verifies that a second join to a room waiting for an
// answer is rejected, and that a finished join's cleanup leaves a later join's
// registration in place.
func TestRoomJoinsRegister(t *testing.T) {
	t.Parallel()

	occupancy := jid.MustParse(testRoom + "/bot")
	joins := newRoomJoins()

	_, firstDone, err := joins.register(occupancy)
	require.NoError(t, err)

	_, _, err = joins.register(jid.MustParse(testRoom + "/other"))
	require.ErrorIs(t, err, ErrJoinInProgress)

	firstDone()

	_, secondDone, err := joins.register(occupancy)
	require.NoError(t, err)

	defer secondDone()

	firstDone()
	assert.Contains(t, joins.waiting, testRoom)
}

// newRoomJoins returns an empty [roomJoins].
//
// Returns:
//   - *roomJoins: the joins.
func newRoomJoins() *roomJoins {
	return &roomJoins{waiting: map[string]chan joinAnswer{}, mu: sync.Mutex{}}
}

// TestReadPresenceErrorWithoutTokens verifies that an empty token stream is
// reported as an error rather than an empty stanza error.
func TestReadPresenceErrorWithoutTokens(t *testing.T) {
	t.Parallel()

	_, err := readPresenceError(xml.NewDecoder(strings.NewReader("")))
	require.Error(t, err)
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
		case <-time.After(testTimeout):
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

	require.ErrorIs(t, session.SendChat(testContext(t), "alice@", "hello"), ErrInvalidRecipientJID)
	require.ErrorIs(t, session.SendToRoom(testContext(t), "alice@", "nick", "", "hello"), ErrInvalidRoomJID)
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
