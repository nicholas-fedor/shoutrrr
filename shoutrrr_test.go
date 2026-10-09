package shoutrrr

import (
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// loopbackBroker is an in-process MQTT broker on a loopback listener. It accepts
// every connection and records whether a client sent DISCONNECT.
type loopbackBroker struct {
	// listener accepts client connections.
	listener net.Listener
	// published counts PUBLISH packets.
	published atomic.Int32
	// disconnected counts DISCONNECT packets.
	disconnected atomic.Int32
}

// rootTestSecret is a credential embedded in test URLs that must never appear in errors.
const rootTestSecret = "SECRETrootTOKEN"

// Service URLs that fail without any network traffic.
const (
	// initFailureURL fails to initialize because pushover requires a user.
	initFailureURL = "pushover://:" + rootTestSecret + "@"
	// sendFailureURL fails to send because nothing listens on port 1 of the loopback address.
	sendFailureURL = "smtp://user:" + rootTestSecret + "@127.0.0.1:1/?fromaddress=from@example.invalid&toaddresses=to@example.invalid"
)

// TestSendErrorsOmitURL verifies that Send reports initialization and send failures
// without echoing the service URL, which carries credentials.
func TestSendErrorsOmitURL(t *testing.T) {
	t.Parallel()

	for name, rawURL := range map[string]string{"initialization": initFailureURL, "send": sendFailureURL} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Send(rawURL, "message")

			require.Error(t, err)
			assert.NotContains(t, err.Error(), rootTestSecret)
		})
	}
}

// TestSenderConstructorErrorsOmitURLs verifies that every sender constructor reports
// an invalid URL without echoing any of the configured URLs.
func TestSenderConstructorErrorsOmitURLs(t *testing.T) {
	t.Parallel()

	urls := []string{"logger://", initFailureURL}

	constructors := map[string]func() error{
		"CreateSender": func() error {
			_, err := CreateSender(urls...)

			return err
		},
		"CreateSenderWithOptions": func() error {
			_, err := CreateSenderWithOptions(types.SenderOptions{}, urls...)

			return err
		},
		"NewSender": func() error {
			_, err := NewSender(nil, urls...)

			return err
		},
		"NewSenderWithOptions": func() error {
			_, err := NewSenderWithOptions(nil, types.SenderOptions{}, urls...)

			return err
		},
	}

	for name, construct := range constructors {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := construct()

			require.Error(t, err)
			assert.NotContains(t, err.Error(), rootTestSecret)
			assert.Contains(t, err.Error(), "URL 1", "the error should identify the failing URL by position")
		})
	}
}

// TestSendClosesOneShotService verifies that Send closes the service it created,
// so a one-shot MQTT send disconnects from the broker instead of holding the
// connection until its idle teardown.
func TestSendClosesOneShotService(t *testing.T) {
	t.Parallel()

	broker := newLoopbackBroker(t)

	require.NoError(t, Send("mqtt://"+broker.listener.Addr().String()+"/shoutrrr/test", "message"))

	require.Eventually(t, func() bool { return broker.disconnected.Load() == 1 },
		2*time.Second, 10*time.Millisecond, "Send must disconnect the one-shot MQTT service")

	// The broker reads packets in order, so the publish was recorded before the disconnect.
	assert.Equal(t, int32(1), broker.published.Load())
}

// TestSetLoggerDuringSends verifies that SetLogger can run while sends are in
// progress. Run with -race, it guards against unsynchronized access to the logger.
func TestSetLoggerDuringSends(t *testing.T) {
	t.Parallel()

	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			SetLogger(log.New(io.Discard, "", 0))
		})
		wg.Go(func() {
			assert.NoError(t, Send("logger://", "message"))
		})
	}

	wg.Wait()
}

// newLoopbackBroker starts a broker on a loopback port and stops it when the test
// ends.
//
// Parameters:
//   - t: the test that owns the broker.
//
// Returns:
//   - *loopbackBroker: the running broker.
func newLoopbackBroker(t *testing.T) *loopbackBroker {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	broker := &loopbackBroker{listener: listener}

	t.Cleanup(func() { _ = listener.Close() })

	go broker.accept()

	return broker
}

// accept serves each connection until the listener closes.
func (b *loopbackBroker) accept() {
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}

		go b.serve(conn)
	}
}

// serve answers CONNECT and records PUBLISH and DISCONNECT until the client
// disconnects or the connection closes.
//
// It reads PINGREQ without answering. The client pings as soon as it connects,
// and a one-shot send can close before reading a PINGRESP. On Windows, closing a
// socket with unread data resets the connection, and the reset discards the
// PUBLISH and DISCONNECT the broker has not read yet. The 20 second keepalive
// never expires during the test, so the unanswered ping is harmless.
//
// Parameters:
//   - conn: the client connection.
func (b *loopbackBroker) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	for {
		packet, err := packets.ReadPacket(conn)
		if err != nil {
			return
		}

		switch packet.Content.(type) {
		case *packets.Connect:
			if _, err := packets.NewControlPacket(packets.CONNACK).WriteTo(conn); err != nil {
				return
			}
		case *packets.Pingreq:
		case *packets.Publish:
			b.published.Add(1)
		case *packets.Disconnect:
			b.disconnected.Add(1)

			return
		default:
		}
	}
}
