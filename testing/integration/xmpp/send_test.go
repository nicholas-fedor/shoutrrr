package xmpp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendChat(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, chatURL, session)

	require.NoError(t, service.Send("hello", nil))
	require.Equal(t, [][2]string{{"bob@example.com", "hello"}}, session.chats)
	require.Empty(t, session.rooms)
}

func TestSendMUC(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, mucURL, session)

	require.NoError(t, service.Send("hello", nil))
	require.Empty(t, session.chats)
	require.Equal(t, [][4]string{{
		"alerts@conference.example.com",
		"alice",
		"",
		"hello",
	}}, session.rooms)
}

func TestSendChatAndMUC(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, bothURL, session)

	require.NoError(t, service.Send("hello", &types.Params{"title": "Alert"}))
	require.Equal(t, [][2]string{{"bob@example.com", "Alert\nhello"}}, session.chats)
	require.Equal(t, [][4]string{{
		"alerts@conference.example.com",
		"bot",
		"pw",
		"Alert\nhello",
	}}, session.rooms)
}

func TestSendMultipleChatRecipients(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(
		t,
		"xmpp://alice:secret@xmpp.example.com/?to=bob@example.com,carol@example.com",
		session,
	)

	require.NoError(t, service.Send("hello", nil))
	require.Equal(t, [][2]string{
		{"bob@example.com", "hello"},
		{"carol@example.com", "hello"},
	}, session.chats)
}

func TestSendParamsOverrideNickAndTitle(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, mucURL, session)

	require.NoError(t, service.Send("hello", &types.Params{
		"nick":  "pager",
		"title": "Down",
	}))
	require.Equal(t, [][4]string{{
		"alerts@conference.example.com",
		"pager",
		"",
		"Down\nhello",
	}}, session.rooms)
}
