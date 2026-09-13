package xmpp_test

import (
	"context"
	"io"
	"net"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/xmpp"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

type mockLogger struct{}

type mockSession struct {
	chats   [][2]string
	rooms   [][4]string
	err     error
	chatErr error
	roomErr error
}

const (
	chatURL = "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com"
	mucURL  = "xmpp://alice:secret@xmpp.example.com/?rooms=alerts@conference.example.com"
	bothURL = "xmpp://alice:secret@xmpp.example.com/?to=bob@example.com&rooms=alerts@conference.example.com&nick=bot&roompassword=pw"
)

func (*mockLogger) Print(...any)          {}
func (*mockLogger) Printf(string, ...any) {}
func (*mockLogger) Println(...any)        {}

func (*mockSession) Close() error {
	return nil
}

func (m *mockSession) SendChat(_ context.Context, to, body string) error {
	m.chats = append(m.chats, [2]string{to, body})
	if m.chatErr != nil {
		return m.chatErr
	}

	return m.err
}

func (m *mockSession) SendToRoom(_ context.Context, room, nick, password, body string) error {
	m.rooms = append(m.rooms, [4]string{room, nick, password, body})
	if m.roomErr != nil {
		return m.roomErr
	}

	return m.err
}

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

func createTestService(t *testing.T, rawURL string, session xmpp.Session) *xmpp.Service {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &xmpp.Service{}
	require.NoError(t, service.Initialize(parsed, &mockLogger{}))
	service.SetDialContext(pipeDialer())
	service.SetSessionFactory(func(context.Context, *xmpp.Config, net.Conn) (xmpp.Session, error) {
		return session, nil
	})

	return service
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(raw)
	require.NoError(t, err)

	return parsed
}
