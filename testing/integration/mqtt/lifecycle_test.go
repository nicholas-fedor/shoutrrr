package mqtt_test

import (
	"context"
	"net"
	"net/url"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// pipeBroker is an in-process MQTT broker reached through net.Pipe, so the real
// autopaho client runs without any network connection. It accepts every
// connection and records what each one receives.
type pipeBroker struct {
	// mu guards sessions.
	mu sync.Mutex
	// sessions holds one entry per dialed connection, in dial order.
	sessions []*brokerSession
}

// brokerSession records the packets one client connection sent to the broker.
type brokerSession struct {
	// mu guards the fields below.
	mu sync.Mutex
	// publishes holds the payload of each PUBLISH packet.
	publishes []string
	// disconnected reports whether the client sent DISCONNECT.
	disconnected bool
}

// idleTimeout matches the service's idle teardown of a connection it created.
const idleTimeout = 60 * time.Second

// pipeBrokerURL is the service URL for the in-process broker. Plain mqtt keeps
// the pipe free of TLS.
const pipeBrokerURL = "mqtt://broker.example.invalid:1883/test/topic"

// TestOwnedConnectionOutlivesPublishTimeout verifies that a connection the service
// opens is not bounded by the 10 second publish timeout: a send 11 seconds after
// the first one is delivered over the same connection.
func TestOwnedConnectionOutlivesPublishTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		broker := &pipeBroker{}
		service := createPipeBrokerService(t, broker)

		require.NoError(t, service.Send("first", nil))

		time.Sleep(11 * time.Second)

		require.NoError(t, service.Send("second", nil))
		synctest.Wait()

		sessions := broker.dialed()
		require.Len(t, sessions, 1)
		assert.Equal(t, []string{"first", "second"}, sessions[0].received())
	})
}

// TestIdleOwnedConnectionDisconnectsAndReconnects verifies that a connection the
// service opens is disconnected after the idle timeout, and that the next send
// opens a new connection.
func TestIdleOwnedConnectionDisconnectsAndReconnects(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		broker := &pipeBroker{}
		service := createPipeBrokerService(t, broker)

		require.NoError(t, service.Send("first", nil))

		time.Sleep(idleTimeout + time.Second)
		synctest.Wait()

		sessions := broker.dialed()
		require.Len(t, sessions, 1)
		assert.True(t, sessions[0].sawDisconnect(), "the idle connection must be disconnected")

		require.NoError(t, service.Send("second", nil))
		synctest.Wait()

		sessions = broker.dialed()
		require.Len(t, sessions, 2)
		assert.Equal(t, []string{"second"}, sessions[1].received())
	})
}

// TestCloseDisconnectsOwnedConnection verifies that Close disconnects a connection
// the service opened, that a second Close is a no-op, and that a later send
// reconnects.
func TestCloseDisconnectsOwnedConnection(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		broker := &pipeBroker{}
		service := createPipeBrokerService(t, broker)

		require.NoError(t, service.Send("first", nil))
		require.NoError(t, service.Close())
		require.NoError(t, service.Close())
		synctest.Wait()

		sessions := broker.dialed()
		require.Len(t, sessions, 1)
		assert.True(t, sessions[0].sawDisconnect())

		require.NoError(t, service.Send("second", nil))
		assert.Len(t, broker.dialed(), 2)
	})
}

// TestInjectedConnectionManagerIsNotClosedWhenIdle verifies that a connection
// manager set by the caller stays open past the idle timeout.
func TestInjectedConnectionManagerIsNotClosedWhenIdle(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On(publishMethodName, mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil)

		service := createTestService(t, pipeBrokerURL, mockManager)

		require.NoError(t, service.Send("message", nil))

		time.Sleep(idleTimeout + time.Second)
		synctest.Wait()

		mockManager.AssertNotCalled(t, "Disconnect", mock.Anything)
	})
}

// TestSendParamsApplyToOneSend verifies that params affect only the send they are
// passed to, not the service configuration used by later sends.
func TestSendParamsApplyToOneSend(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		mockManager := &MockConnectionManager{}
		mockManager.On("AwaitConnection", mock.Anything).Return(nil)
		mockManager.On(publishMethodName, mock.Anything, mock.Anything).
			Return(createMockPublishResponse(0), nil)

		service := createTestService(t, pipeBrokerURL, mockManager)

		require.NoError(t, service.Send("retained", &types.Params{"retained": "yes", "qos": "1"}))
		require.NoError(t, service.Send("plain", nil))

		assert.False(t, service.Config.Retained)

		var publishes []bool

		for i := range mockManager.Calls {
			call := &mockManager.Calls[i]
			if call.Method == publishMethodName {
				publish, ok := call.Arguments[1].(*paho.Publish)
				require.True(t, ok)

				publishes = append(publishes, publish.Retain)
			}
		}

		assert.Equal(t, []bool{true, false}, publishes)
	})
}

// TestGetURLKeepsTLSScheme verifies that a service configured with mqtts reports
// an mqtts URL, so a service re-created from that URL still uses TLS.
func TestGetURLKeepsTLSScheme(t *testing.T) {
	t.Parallel()

	serviceURL, err := url.Parse("mqtts://broker.example.com/test/topic")
	require.NoError(t, err)

	service := &mqtt.Service{}
	require.NoError(t, service.Initialize(serviceURL, &mockLogger{}))

	assert.Equal(t, mqtt.SchemeTLS, service.Config.GetURL().Scheme)
}

// createPipeBrokerService initializes a service whose connections are dialed to
// broker through net.Pipe, and closes it when the test ends.
//
// Parameters:
//   - t: the test.
//   - broker: the in-process broker that serves each dialed connection.
//
// Returns:
//   - *mqtt.Service: the initialized service.
func createPipeBrokerService(t *testing.T, broker *pipeBroker) *mqtt.Service {
	t.Helper()

	service := createTestService(t, pipeBrokerURL)
	service.SetDialContext(broker.dial)

	t.Cleanup(func() { _ = service.Close() })

	return service
}

// dial connects a client to the broker over a new in-memory pipe.
func (b *pipeBroker) dial(context.Context, string, string) (net.Conn, error) {
	client, server := net.Pipe()
	session := &brokerSession{}

	b.mu.Lock()
	b.sessions = append(b.sessions, session)
	b.mu.Unlock()

	go session.serve(server)

	return client, nil
}

// dialed returns the sessions of every connection dialed so far.
func (b *pipeBroker) dialed() []*brokerSession {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]*brokerSession(nil), b.sessions...)
}

// received returns the payloads published on the session.
func (s *brokerSession) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.publishes...)
}

// sawDisconnect reports whether the client sent DISCONNECT.
func (s *brokerSession) sawDisconnect() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.disconnected
}

// serve answers CONNECT and PINGREQ, records PUBLISH and DISCONNECT, and returns
// when the client disconnects or the pipe closes.
//
// Parameters:
//   - conn: the broker's end of the pipe.
func (s *brokerSession) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	for {
		packet, err := packets.ReadPacket(conn)
		if err != nil {
			return
		}

		switch content := packet.Content.(type) {
		case *packets.Connect:
			if _, err := packets.NewControlPacket(packets.CONNACK).WriteTo(conn); err != nil {
				return
			}
		case *packets.Pingreq:
			if _, err := packets.NewControlPacket(packets.PINGRESP).WriteTo(conn); err != nil {
				return
			}
		case *packets.Publish:
			s.mu.Lock()
			s.publishes = append(s.publishes, string(content.Payload))
			s.mu.Unlock()
		case *packets.Disconnect:
			s.mu.Lock()
			s.disconnected = true
			s.mu.Unlock()

			return
		default:
		}
	}
}
