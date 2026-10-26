package xmpp

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/muc"
	"mellium.im/xmpp/stanza"

	mxmpp "mellium.im/xmpp"
	xmux "mellium.im/xmpp/mux"

	"github.com/nicholas-fedor/shoutrrr/internal/transport"
)

// Session is a connected XMPP session used to send notifications.
type Session interface {
	// Close ends the session and the underlying connection.
	Close() error
	// SendChat sends a type=chat message to to.
	SendChat(ctx context.Context, to, body string) error
	// SendToRoom joins room, sends a groupchat message, and leaves.
	SendToRoom(ctx context.Context, room, nick, password, body string) error
}

// SessionFactory constructs an XMPP session over an existing TCP connection.
type SessionFactory func(ctx context.Context, cfg *Config, conn net.Conn) (Session, error)

// messageBody is the XML payload encoded for chat and groupchat stanzas.
type messageBody struct {
	// XMLName names the message element.
	XMLName xml.Name `xml:"message"`
	// To is the recipient or room JID in the to attribute.
	To string `xml:"to,attr"`
	// Type is the message type attribute, chat or groupchat.
	Type string `xml:"type,attr"`
	// Body is the message text.
	Body string `xml:"body"`
}

// melliumSession implements [Session] with mellium over an existing connection.
type melliumSession struct {
	// session is the negotiated mellium client session.
	session *mxmpp.Session
	// joins hands each room's answer to a join to the call waiting for it.
	joins roomJoins
}

// roomJoins hands a room's answer to a join to the room send waiting for it.
// An answer that arrives before the send starts waiting is kept.
type roomJoins struct {
	// waiting holds the channel of each pending join, by bare room JID.
	waiting map[string]chan joinAnswer
	// mu guards waiting.
	mu sync.Mutex
}

// joinAnswer is a room's answer to a join.
type joinAnswer struct {
	// err is the room's refusal, or nil when the room accepted the join.
	err error
	// occupant is the occupant JID the room assigned, set when it accepted.
	occupant jid.JID
}

// mucUserPresence is the part of a room's presence that marks it as the
// joining user's own presence.
type mucUserPresence struct {
	// X is the XEP-0045 user element.
	X struct {
		// Statuses holds the status codes of the presence.
		Statuses []struct {
			// Code is the status code, 110 for the user's own presence.
			Code string `xml:"code,attr"`
		} `xml:"status"`
	} `xml:"http://jabber.org/protocol/muc#user x"`
}

// selfPresenceStatus is the XEP-0045 status code a room includes in the
// presence that tells the joining user it has entered.
const selfPresenceStatus = "110"

var _ io.Closer = (*melliumSession)(nil)

// newMelliumSession negotiates an XMPP client session over conn.
//
// Implicit TLS is applied for xmpps://. STARTTLS is negotiated for xmpp://
// unless disabletls is set. A Serve goroutine is started so MUC join can
// receive presence.
//
// Parameters:
//   - ctx: Parent context for handshake and stream negotiation.
//   - cfg: Session TLS, SASL, and JID settings.
//   - conn: The TCP connection to wrap.
//
// Returns:
//   - Session: a [Session] ready to send chat and MUC messages.
//   - error: an error if TLS, SASL, or stream negotiation fails.
func newMelliumSession(ctx context.Context, cfg *Config, conn net.Conn) (Session, error) {
	local, domain, err := splitJID(cfg.authJIDString())
	if err != nil {
		return nil, err
	}

	origin, err := jid.New(local, domain, Resource)
	if err != nil {
		return nil, fmt.Errorf("parsing auth JID: %w", err)
	}

	var stream io.ReadWriter = conn

	features := []mxmpp.StreamFeature{}

	if cfg.implicitTLS() {
		tlsConn := tls.Client(conn, newTLSConfig(cfg))
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil, fmt.Errorf("TLS handshake: %w", err)
		}

		stream = tlsConn
	} else if cfg.useTLS() {
		features = append(features, mxmpp.StartTLS(newTLSConfig(cfg)))
	}

	saslFeature := mxmpp.SASL("", cfg.Password,
		sasl.ScramSha256Plus,
		sasl.ScramSha256,
		sasl.ScramSha1Plus,
		sasl.ScramSha1,
		sasl.Plain,
	)
	if cfg.DisableTLS {
		// Mellium's SASL feature requires a secured stream by default. Plaintext
		// is an explicit opt-in, so allow SASL before TLS.
		saslFeature.Necessary = 0
	}

	features = append(features, saslFeature, mxmpp.BindResource())

	session, err := mxmpp.NewClientSession(ctx, origin, stream, features...)
	if err != nil {
		return nil, wrapNegotiateError(err)
	}

	clientSession := &melliumSession{
		session: session,
		joins:   roomJoins{waiting: map[string]chan joinAnswer{}, mu: sync.Mutex{}},
	}
	handler := xmux.New(stanza.NSClient,
		xmux.PresenceFunc(
			stanza.AvailablePresence,
			xml.Name{Space: muc.NSUser, Local: "x"},
			clientSession.joins.handleSelfPresence,
		),
		// An empty payload name also matches an error presence without child
		// elements, so a malformed refusal still ends the join.
		xmux.PresenceFunc(
			stanza.ErrorPresence,
			xml.Name{Space: "", Local: ""},
			clientSession.joins.handleErrorPresence,
		),
	)

	go func() {
		if serveErr := session.Serve(handler); serveErr != nil {
			return
		}
	}()

	return clientSession, nil
}

// newTLSConfig builds a TLS config from cfg with the shared helpers.
//
// TLS 1.2 is the minimum version, and the certificate is verified against
// Host unless [Config.SkipTLSVerify] is set, such as for a self-signed or
// internal server.
//
// Parameters:
//   - cfg: The service configuration supplying Host and SkipTLSVerify.
//
// Returns:
//   - *tls.Config: a TLS config with TLS 1.2 as the minimum version.
func newTLSConfig(cfg *Config) *tls.Config {
	if cfg.SkipTLSVerify {
		return transport.InsecureTLSConfig(cfg.Host)
	}

	return transport.TLSConfig(cfg.Host)
}

// wrapNegotiateError wraps a session negotiation error, marking an
// authentication failure with [ErrAuthenticationFailed].
//
// Parameters:
//   - err: the error from session negotiation.
//
// Returns:
//   - error: err wrapped with [ErrAuthenticationFailed] when it is an
//     authentication failure, otherwise err wrapped as a negotiation error.
func wrapNegotiateError(err error) error {
	if isAuthFailure(err) {
		return fmt.Errorf("%w: %w", ErrAuthenticationFailed, err)
	}

	return fmt.Errorf("negotiating XMPP session: %w", err)
}

// isAuthFailure reports whether the error text names an authentication
// failure, such as the SASL condition not-authorized or an invalid username.
// The match ignores case.
//
// Parameters:
//   - err: the error from session negotiation.
//
// Returns:
//   - bool: true when the text contains a known authentication failure.
func isAuthFailure(err error) bool {
	text := strings.ToLower(err.Error())

	return strings.Contains(text, "not-authorized") ||
		strings.Contains(text, "invalid username") ||
		strings.Contains(text, "invalid-authzid") ||
		strings.Contains(text, "credentials-expired") ||
		strings.Contains(text, "account-disabled") ||
		strings.Contains(text, "mechanism-too-weak") ||
		strings.Contains(text, "temporary-auth-failure")
}

// Close ends the XMPP session and the underlying connection.
//
// Returns:
//   - error: combined close errors, or nil when the session is already unset.
func (s *melliumSession) Close() error {
	if s.session == nil {
		return nil
	}

	err := s.session.Close()
	if conn := s.session.Conn(); conn != nil {
		err = errors.Join(err, conn.Close())
	}

	return err
}

// SendChat sends a type=chat message to to.
//
// Parameters:
//   - ctx: Parent context for encoding the stanza.
//   - to: Recipient JID.
//   - body: Message body.
//
// Returns:
//   - error: an error if the JID is invalid or encoding fails.
func (s *melliumSession) SendChat(ctx context.Context, to, body string) error {
	toJID, err := jid.Parse(to)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidRecipientJID, to)
	}

	msg := messageBody{
		XMLName: xml.Name{Space: "", Local: "message"},
		To:      toJID.String(),
		Type:    string(stanza.ChatMessage),
		Body:    body,
	}
	if err := s.session.Encode(ctx, msg); err != nil {
		return fmt.Errorf("sending chat message: %w", err)
	}

	return nil
}

// SendToRoom joins room, sends a groupchat message, and leaves.
//
// The join waits for the room's self-presence before the message is sent, and
// a room that refuses the join fails the send with its [stanza.Error]. When
// the room assigns another nickname, the leave uses it. password is used only
// for protected rooms. The leave is sent without waiting for the room to
// confirm it, because the session closes after each send.
//
// Parameters:
//   - ctx: Parent context for join, send, and leave.
//   - room: MUC room JID.
//   - nick: Nickname used when joining.
//   - password: Optional room password.
//   - body: Message body.
//
// Returns:
//   - error: an error if join, send, or leave fails, or [ErrJoinInProgress]
//     when another call on the session is joining the same room.
func (s *melliumSession) SendToRoom(ctx context.Context, room, nick, password, body string) error {
	roomJID, err := jid.Parse(room)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidRoomJID, room)
	}

	occupancy, err := roomJID.WithResource(nick)
	if err != nil {
		return fmt.Errorf("joining MUC %s: %w", room, err)
	}

	occupant, err := s.joinRoom(ctx, occupancy, password)
	if err != nil {
		return fmt.Errorf("joining MUC %s: %w", room, err)
	}

	msg := messageBody{
		XMLName: xml.Name{Space: "", Local: "message"},
		To:      roomJID.Bare().String(),
		Type:    string(stanza.GroupChatMessage),
		Body:    body,
	}
	if err := s.session.Encode(ctx, msg); err != nil {
		leaveErr := s.leaveRoom(ctx, occupant)

		return errors.Join(fmt.Errorf("sending groupchat message: %w", err), leaveErr)
	}

	if err := s.leaveRoom(ctx, occupant); err != nil {
		return fmt.Errorf("leaving MUC %s: %w", room, err)
	}

	return nil
}

// joinRoom joins a room as occupancy and waits for the room's self-presence.
// The join requests no discussion history.
//
// Parameters:
//   - ctx: cancellation for writing the join and waiting for the answer.
//   - occupancy: the occupant JID, the room with the nickname as its resource.
//   - password: the room password, or empty for an open room.
//
// Returns:
//   - jid.JID: the occupant JID the room assigned, which may carry another
//     nickname.
//   - error: [ErrJoinInProgress], the error from writing the join, the
//     [stanza.Error] of a room that refuses it, or ctx's error.
func (s *melliumSession) joinRoom(ctx context.Context, occupancy jid.JID, password string) (jid.JID, error) {
	answer, done, err := s.joins.register(occupancy)
	if err != nil {
		return jid.JID{}, err
	}
	defer done()

	// The muc package's Join writes through a context it cancels as soon as the
	// join completes, and the write deadline that cancellation can leave on the
	// connection fails the next write. The join is written with ctx instead.
	//nolint:exhaustruct_v5 // The other presence attributes are optional.
	join := stanza.Presence{To: occupancy}
	if err := s.session.Send(ctx, join.Wrap(mucJoinPayload(password))); err != nil {
		return jid.JID{}, fmt.Errorf("sending join presence: %w", err)
	}

	select {
	case result := <-answer:
		return result.occupant, result.err
	case <-ctx.Done():
		return jid.JID{}, fmt.Errorf("waiting for the room to answer the join: %w", ctx.Err())
	}
}

// leaveRoom sends the unavailable presence that leaves a room, without waiting
// for the room to confirm it.
//
// Parameters:
//   - ctx: cancellation for writing the presence.
//   - occupancy: the occupant JID, the room with the nickname as its resource.
//
// Returns:
//   - error: the error from writing the presence.
func (s *melliumSession) leaveRoom(ctx context.Context, occupancy jid.JID) error {
	// The muc package's Channel.Leave waits for the room's confirmation, but it
	// misses one that arrives before it starts waiting and then blocks until
	// ctx ends, so the presence is sent directly.
	//nolint:exhaustruct_v5 // The other presence attributes are optional.
	presence := stanza.Presence{To: occupancy, Type: stanza.UnavailablePresence}

	if err := s.session.Send(ctx, presence.Wrap(nil)); err != nil {
		return fmt.Errorf("sending unavailable presence: %w", err)
	}

	return nil
}

// answer passes result to the join waiting for the room from belongs to,
// unless that join already has an answer or no join waits for the room.
//
// Parameters:
//   - from: the JID the answer comes from, the room or one of its occupants.
//   - result: the room's answer.
func (j *roomJoins) answer(from jid.JID, result joinAnswer) {
	j.mu.Lock()
	defer j.mu.Unlock()

	answer, ok := j.waiting[from.Bare().String()]
	if !ok {
		return
	}

	select {
	case answer <- result:
	default:
	}
}

// handleErrorPresence hands the stanza error of an error presence to the join
// waiting for the presence's room. Presences no join waits for are ignored.
//
// Parameters:
//   - presence: the presence.
//   - tokens: the presence's tokens.
//
// Returns:
//   - error: always nil, so the session keeps serving.
//
//nolint:gocritic // hugeParam: the mux handler signature passes the presence by value.
func (j *roomJoins) handleErrorPresence(presence stanza.Presence, tokens xmlstream.TokenReadEncoder) error {
	stanzaErr, err := readPresenceError(tokens)
	if err != nil {
		//nolint:exhaustruct_v5 // A refusal assigns no occupant JID.
		j.answer(presence.From, joinAnswer{err: fmt.Errorf("reading the room's error: %w", err)})

		return nil
	}

	//nolint:exhaustruct_v5 // A refusal assigns no occupant JID.
	j.answer(presence.From, joinAnswer{err: stanzaErr})

	return nil
}

// handleSelfPresence completes the join waiting for the presence's room when
// the presence carries status 110, which marks the joining user's own
// presence. Its sender is the occupant JID the room assigned. Presences of
// other occupants, which a room sends first, are ignored.
//
// Parameters:
//   - presence: the presence.
//   - tokens: the presence's tokens.
//
// Returns:
//   - error: always nil, so the session keeps serving.
//
//nolint:gocritic // hugeParam: the mux handler signature passes the presence by value.
func (j *roomJoins) handleSelfPresence(presence stanza.Presence, tokens xmlstream.TokenReadEncoder) error {
	var user mucUserPresence
	if err := xml.NewTokenDecoder(tokens).Decode(&user); err != nil {
		return nil //nolint:nilerr // A presence that does not decode is not a self-presence.
	}

	for _, status := range user.X.Statuses {
		if status.Code == selfPresenceStatus {
			j.answer(presence.From, joinAnswer{err: nil, occupant: presence.From})

			return nil
		}
	}

	return nil
}

// register starts waiting for the room's answer to a join as occupancy.
//
// Parameters:
//   - occupancy: the occupant JID of the join.
//
// Returns:
//   - <-chan joinAnswer: receives the room's answer.
//   - func(): stops waiting, called when the join call returns. It leaves a
//     later join's registration in place.
//   - error: [ErrJoinInProgress] when a join to the same room is waiting.
func (j *roomJoins) register(occupancy jid.JID) (<-chan joinAnswer, func(), error) {
	key := occupancy.Bare().String()
	answer := make(chan joinAnswer, 1)

	j.mu.Lock()
	defer j.mu.Unlock()

	if _, waiting := j.waiting[key]; waiting {
		return nil, nil, fmt.Errorf("%w: %s", ErrJoinInProgress, key)
	}

	j.waiting[key] = answer

	return answer, func() {
		j.mu.Lock()
		defer j.mu.Unlock()

		if j.waiting[key] == answer {
			delete(j.waiting, key)
		}
	}, nil
}

// readPresenceError decodes the stanza error of an error presence.
//
// Parameters:
//   - tokens: the presence's tokens, starting with the presence element.
//
// Returns:
//   - stanza.Error: the error the presence carries.
//   - error: the error from reading the tokens, or stanza's error when the
//     presence carries no error element.
func readPresenceError(tokens xml.TokenReader) (stanza.Error, error) {
	// The error is a child of the presence element, so its start is skipped.
	if _, err := tokens.Token(); err != nil {
		return stanza.Error{}, fmt.Errorf("reading the presence element: %w", err)
	}

	stanzaErr, err := stanza.UnmarshalError(tokens)
	if err != nil {
		return stanza.Error{}, fmt.Errorf("decoding the stanza error: %w", err)
	}

	return stanzaErr, nil
}

// mucJoinPayload builds the XEP-0045 element of a join presence, which
// requests no discussion history and carries the room password when one is
// set.
//
// Parameters:
//   - password: the room password, or empty for an open room.
//
// Returns:
//   - xml.TokenReader: the element.
func mucJoinPayload(password string) xml.TokenReader {
	children := []xml.TokenReader{
		xmlstream.Wrap(nil, xml.StartElement{
			Name: xml.Name{Space: "", Local: "history"},
			Attr: []xml.Attr{{Name: xml.Name{Space: "", Local: "maxstanzas"}, Value: "0"}},
		}),
	}

	if password != "" {
		children = append(children, xmlstream.Wrap(
			xmlstream.Token(xml.CharData(password)),
			xml.StartElement{Name: xml.Name{Space: "", Local: "password"}, Attr: nil},
		))
	}

	return xmlstream.Wrap(
		xmlstream.MultiReader(children...),
		xml.StartElement{Name: xml.Name{Space: muc.NS, Local: "x"}, Attr: nil},
	)
}
