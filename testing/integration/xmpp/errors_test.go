package xmpp_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/xmpp"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendWithDialError(t *testing.T) {
	t.Parallel()

	service := createTestService(t, chatURL, &mockSession{})
	service.SetDialContext(func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("connection refused")
	})

	err := service.Send("hello", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dialing")
}

func TestSendWithSessionError(t *testing.T) {
	t.Parallel()

	service := createTestService(t, chatURL, &mockSession{})
	service.SetSessionFactory(func(context.Context, *xmpp.Config, net.Conn) (xmpp.Session, error) {
		return nil, errors.New("negotiate failed")
	})

	err := service.Send("hello", nil)
	require.EqualError(t, err, "negotiate failed")
}

func TestSendWithChatError(t *testing.T) {
	t.Parallel()

	session := &mockSession{chatErr: errors.New("chat failed")}
	service := createTestService(t, chatURL, session)

	require.EqualError(t, service.Send("hello", nil), "chat failed")
}

func TestSendWithMUCError(t *testing.T) {
	t.Parallel()

	session := &mockSession{roomErr: errors.New("join failed")}
	service := createTestService(t, mucURL, session)

	require.EqualError(t, service.Send("hello", nil), "join failed")
}

func TestSendWithInvalidParams(t *testing.T) {
	t.Parallel()

	service := createTestService(t, chatURL, &mockSession{})
	err := service.Send("hello", &types.Params{"notakey": "1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "updating config from params")
}

func TestSendWithParamsClearingTargets(t *testing.T) {
	t.Parallel()

	service := createTestService(t, chatURL, &mockSession{})
	err := service.Send("hello", &types.Params{"to": ""})
	require.ErrorIs(t, err, xmpp.ErrMissingTargets)
}
