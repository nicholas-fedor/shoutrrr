package e2e_test

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/muc"
	"mellium.im/xmpp/stanza"

	mxmpp "mellium.im/xmpp"
	xmux "mellium.im/xmpp/mux"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/xmpp"
)

type inbox struct {
	bodies  chan string
	session *mxmpp.Session
	conn    net.Conn
	cancel  context.CancelFunc
}

const (
	probeUser     = "recipient"
	probePassword = "recipientpass"
	probeNick     = "probe"
)

func (in *inbox) close() {
	if in.cancel != nil {
		in.cancel()
	}

	if in.session != nil {
		_ = in.session.Close()
	}

	if in.conn != nil {
		_ = in.conn.Close()
	}
}

func startInbox(serviceURL *url.URL) (*inbox, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	cfg := probeConfig(serviceURL)

	addr := net.JoinHostPort(cfg.Host, strconv.FormatUint(uint64(cfg.Port), 10))

	var d net.Dialer

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		cancel()

		return nil, fmt.Errorf("dialing probe: %w", err)
	}

	session, mucClient, err := probeSession(ctx, cfg, conn)
	if err != nil {
		cancel()

		_ = conn.Close()

		return nil, err
	}

	got := make(chan string, 8)
	handler := xmux.New(
		stanza.NSClient,
		muc.HandleClient(mucClient),
		xmux.MessageFunc(stanza.ChatMessage, xml.Name{Local: "body"}, captureBody(got)),
		xmux.MessageFunc(stanza.GroupChatMessage, xml.Name{Local: "body"}, captureBody(got)),
	)

	go func() {
		_ = session.Serve(handler)
	}()

	if err := session.Encode(ctx, struct {
		XMLName xml.Name `xml:"presence"`
	}{XMLName: xml.Name{Local: "presence"}}); err != nil {
		cancel()

		_ = session.Close()
		_ = conn.Close()

		return nil, fmt.Errorf("sending probe presence: %w", err)
	}

	if rooms := serviceURL.Query()["rooms"]; len(rooms) > 0 {
		if err := joinProbeRoom(ctx, mucClient, session, rooms[0], serviceURL.Query().Get("roompassword")); err != nil {
			cancel()

			_ = session.Close()
			_ = conn.Close()

			return nil, err
		}
	}

	return &inbox{bodies: got, session: session, conn: conn, cancel: cancel}, nil
}

func probeConfig(serviceURL *url.URL) *xmpp.Config {
	cloned := *serviceURL
	cloned.User = url.UserPassword(probeUser, probePassword)

	query := cloned.Query()
	if query.Get("to") == "" && query.Get("rooms") == "" {
		query.Set("to", probeUser+"@"+cloned.Hostname())
		cloned.RawQuery = query.Encode()
	}

	cfg := &xmpp.Config{}
	if err := cfg.SetURL(&cloned); err != nil {
		return &xmpp.Config{
			Host:          serviceURL.Hostname(),
			User:          probeUser,
			Password:      probePassword,
			SkipTLSVerify: serviceURL.Query().Get("skiptlsverify") != "",
			DisableTLS:    serviceURL.Query().Get("disabletls") != "",
		}
	}

	return cfg
}

func probeSession(
	ctx context.Context,
	cfg *xmpp.Config,
	conn net.Conn,
) (*mxmpp.Session, *muc.Client, error) {
	origin, err := jid.New(probeUser, cfg.Host, xmpp.Resource)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing probe JID: %w", err)
	}

	var stream io.ReadWriter = conn

	features := []mxmpp.StreamFeature{}
	tlsCfg := &tls.Config{
		ServerName:         cfg.Host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.SkipTLSVerify,
	}

	if serviceUsesImplicitTLS(cfg) {
		tlsConn := tls.Client(conn, tlsCfg)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil, nil, fmt.Errorf("probe TLS handshake: %w", err)
		}

		stream = tlsConn
	} else if !cfg.DisableTLS {
		features = append(features, mxmpp.StartTLS(tlsCfg))
	}

	saslFeature := mxmpp.SASL("", cfg.Password, sasl.ScramSha256, sasl.ScramSha1, sasl.Plain)
	if cfg.DisableTLS {
		saslFeature.Necessary = 0
	}

	features = append(features, saslFeature, mxmpp.BindResource())

	session, err := mxmpp.NewClientSession(ctx, origin, stream, features...)
	if err != nil {
		return nil, nil, fmt.Errorf("probe session: %w", err)
	}

	return session, &muc.Client{}, nil
}

func serviceUsesImplicitTLS(cfg *xmpp.Config) bool {
	return cfg.GetURL().Scheme == xmpp.SchemeTLS
}

func joinProbeRoom(
	ctx context.Context,
	mucClient *muc.Client,
	session *mxmpp.Session,
	room, password string,
) error {
	roomJID, err := jid.Parse(room)
	if err != nil {
		return fmt.Errorf("parsing probe room: %w", err)
	}

	occupancy, err := roomJID.WithResource(probeNick)
	if err != nil {
		return fmt.Errorf("probe occupancy JID: %w", err)
	}

	opts := []muc.Option{muc.MaxHistory(0)}
	if password != "" {
		opts = append(opts, muc.Password(password))
	}

	if _, err := mucClient.Join(ctx, occupancy, session, opts...); err != nil {
		return fmt.Errorf("probe joining MUC: %w", err)
	}

	return nil
}

func captureBody(got chan string) xmux.MessageHandlerFunc {
	return func(_ stanza.Message, t xmlstream.TokenReadEncoder) error {
		decoder := xml.NewTokenDecoder(t)

		var text string

		var textSb218 strings.Builder

		for {
			tok, err := decoder.Token()
			if err != nil {
				break
			}

			if cd, ok := tok.(xml.CharData); ok {
				textSb218.WriteString(string(cd))
			}
		}

		text += textSb218.String()

		if text == "" {
			return nil
		}

		select {
		case got <- text:
		default:
		}

		return nil
	}
}
