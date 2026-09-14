package xmpp_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendEmptyMessage(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, chatURL, session)

	require.NoError(t, service.Send("", nil))
	require.Equal(t, [][2]string{{"bob@example.com", ""}}, session.chats)
}

func TestSendTitleOnly(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, chatURL, session)

	require.NoError(t, service.Send("", &types.Params{"title": "Alert"}))
	require.Equal(t, [][2]string{{"bob@example.com", "Alert"}}, session.chats)
}

func TestSendUnicodeMessage(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, chatURL, session)
	body := "アラート 🚨 Привет"

	require.NoError(t, service.Send(body, nil))
	require.Equal(t, [][2]string{{"bob@example.com", body}}, session.chats)
}

func TestSendLongMessage(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, chatURL, session)
	body := strings.Repeat("x", 64*1024)

	require.NoError(t, service.Send(body, nil))
	require.Equal(t, [][2]string{{"bob@example.com", body}}, session.chats)
}

func TestSendNewlinesAndJSON(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	service := createTestService(t, chatURL, session)
	body := "line1\nline2\t{\"ok\":true}"

	require.NoError(t, service.Send(body, nil))
	require.Equal(t, [][2]string{{"bob@example.com", body}}, session.chats)
}
