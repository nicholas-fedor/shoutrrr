package xmpp

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"

	"mellium.im/sasl"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/muc"
	"mellium.im/xmpp/stanza"

	mxmpp "mellium.im/xmpp"
	xmux "mellium.im/xmpp/mux"
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
	XMLName xml.Name `xml:"message"`
	To      string   `xml:"to,attr"`
	Type    string   `xml:"type,attr"`
	Body    string   `xml:"body"`
}

// melliumSession implements [Session] with mellium over an existing connection.
type melliumSession struct {
	session   *mxmpp.Session
	mucClient *muc.Client
}

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
//   - A [Session] ready to send chat and MUC messages.
//   - An error if TLS, SASL, or stream negotiation fails.
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
		return nil, fmt.Errorf("negotiating XMPP session: %w", err)
	}

	mucClient := &muc.Client{}
	handler := xmux.New(stanza.NSClient, muc.HandleClient(mucClient))
	clientSession := &melliumSession{session: session, mucClient: mucClient}

	go func() {
		if serveErr := session.Serve(handler); serveErr != nil {
			return
		}
	}()

	return clientSession, nil
}

// newTLSConfig builds a TLS config from cfg.
//
// TLS 1.2 is the minimum version. [Config.SkipTLSVerify] is copied onto
// InsecureSkipVerify for self-signed or internal servers.
//
// Parameters:
//   - cfg: The service configuration supplying Host and SkipTLSVerify.
//
// Returns:
//   - A TLS config with TLS 1.2 as the minimum version.
func newTLSConfig(cfg *Config) *tls.Config {
	return &tls.Config{
		ServerName:         cfg.Host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.SkipTLSVerify,
	}
}

// Close ends the XMPP session and the underlying connection.
//
// Returns:
//   - Combined close errors, or nil when the session is already unset.
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
//   - An error if the JID is invalid or encoding fails.
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
// Join waits for self-presence before the message is sent. password is used
// only for protected rooms.
//
// Parameters:
//   - ctx: Parent context for join, send, and leave.
//   - room: MUC room JID.
//   - nick: Nickname used when joining.
//   - password: Optional room password.
//   - body: Message body.
//
// Returns:
//   - An error if join, send, or leave fails.
func (s *melliumSession) SendToRoom(ctx context.Context, room, nick, password, body string) error {
	roomJID, err := jid.Parse(room)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidRoomJID, room)
	}

	occupancy, err := roomJID.WithResource(nick)
	if err != nil {
		return fmt.Errorf("joining MUC %s: %w", room, err)
	}

	opts := []muc.Option{muc.MaxHistory(0)}
	if password != "" {
		opts = append(opts, muc.Password(password))
	}

	channel, err := s.mucClient.Join(ctx, occupancy, s.session, opts...)
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
		leaveErr := channel.Leave(ctx, "")

		return errors.Join(fmt.Errorf("sending groupchat message: %w", err), leaveErr)
	}

	if err := channel.Leave(ctx, ""); err != nil {
		return fmt.Errorf("leaving MUC %s: %w", room, err)
	}

	return nil
}
