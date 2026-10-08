package mqtt

import (
	"context"
	"errors"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// fakeConnection is a connection manager whose lifetime ends with its context or
// on Disconnect, like autopaho's.
type fakeConnection struct {
	// done is closed when the connection ends.
	done chan struct{}
	// endOnce closes done once.
	endOnce sync.Once
	// publishes counts successful publishes.
	publishes atomic.Int32
	// disconnects counts Disconnect calls.
	disconnects atomic.Int32
	// lastRetain records the Retain flag of the latest publish.
	lastRetain atomic.Bool
}

// fakeConnector creates fake connections and records them in order.
type fakeConnector struct {
	// mu guards connections.
	mu sync.Mutex
	// connections holds every connection created so far.
	connections []*fakeConnection
}

// lifecycleURL is a broker URL for the lifecycle tests.
const lifecycleURL = "mqtt://broker.example.invalid/test/topic"

// errConnectionEnded is returned by a fake connection used after it ended.
var errConnectionEnded = errors.New("connection ended")

// TestConnectionOutlivesPublishTimeout verifies that a connection the service
// creates is not bounded by the publish timeout, so a send more than 10 seconds
// after the first one reuses the same live connection.
func TestConnectionOutlivesPublishTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{}
		service := newLifecycleService(t, connector)

		require.NoError(t, service.Send("first", nil))

		time.Sleep(publishTimeout*time.Second + time.Second)

		require.NoError(t, service.Send("second", nil))

		connections := connector.created()
		require.Len(t, connections, 1)
		assert.False(t, connections[0].ended())
		assert.Equal(t, int32(2), connections[0].publishes.Load())
	})
}

// TestIdleConnectionIsClosed verifies that a connection the service creates is
// disconnected once no send has used it for the idle timeout, and that the next
// send opens a new one.
func TestIdleConnectionIsClosed(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{}
		service := newLifecycleService(t, connector)

		require.NoError(t, service.Send("first", nil))

		time.Sleep(idleTimeout - time.Second)
		synctest.Wait()

		first := connector.created()[0]
		assert.False(t, first.ended(), "the connection must stay open before the idle timeout")

		time.Sleep(2 * time.Second)
		synctest.Wait()

		assert.Equal(t, int32(1), first.disconnects.Load())

		require.NoError(t, service.Send("second", nil))
		assert.Len(t, connector.created(), 2)
	})
}

// TestSendRestartsIdleTimeout verifies that each send restarts the idle period.
func TestSendRestartsIdleTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{}
		service := newLifecycleService(t, connector)

		require.NoError(t, service.Send("first", nil))

		time.Sleep(idleTimeout - time.Second)
		require.NoError(t, service.Send("second", nil))

		time.Sleep(idleTimeout - time.Second)
		synctest.Wait()

		connections := connector.created()
		require.Len(t, connections, 1)
		assert.False(t, connections[0].ended())
	})
}

// TestEndedConnectionIsReplaced verifies that a send replaces a connection the
// service created once that connection has ended.
func TestEndedConnectionIsReplaced(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{}
		service := newLifecycleService(t, connector)

		require.NoError(t, service.Send("first", nil))
		connector.created()[0].end()

		require.NoError(t, service.Send("second", nil))

		connections := connector.created()
		require.Len(t, connections, 2)
		assert.Equal(t, int32(1), connections[1].publishes.Load())
	})
}

// TestInjectedConnectionIsNotClosedWhenIdle verifies that a connection manager set
// by the caller stays open past the idle timeout.
func TestInjectedConnectionIsNotClosedWhenIdle(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{}
		service := newLifecycleService(t, connector)

		injected := newFakeConnection(context.Background())
		service.SetConnectionManager(injected)

		require.NoError(t, service.Send("message", nil))

		time.Sleep(idleTimeout + time.Second)
		synctest.Wait()

		assert.Zero(t, injected.disconnects.Load())
		assert.Empty(t, connector.created())
	})
}

// TestCloseDisconnectsAndAllowsReconnect verifies that Close disconnects the
// connection, is safe to repeat, and that a later send reconnects.
func TestCloseDisconnectsAndAllowsReconnect(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{}
		service := newLifecycleService(t, connector)

		require.NoError(t, service.Send("first", nil))
		require.NoError(t, service.Close())
		require.NoError(t, service.Close())

		first := connector.created()[0]
		assert.Equal(t, int32(1), first.disconnects.Load())
		assert.True(t, first.ended())

		require.NoError(t, service.Send("second", nil))
		assert.Len(t, connector.created(), 2)
	})
}

// TestSendParamsDoNotChangeConfig verifies that params apply to one send without
// changing the service configuration used by later sends.
func TestSendParamsDoNotChangeConfig(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{}
		service := newLifecycleService(t, connector)

		require.NoError(t, service.Send("retained", &types.Params{"retained": "yes"}))

		connection := connector.created()[0]
		assert.True(t, connection.lastRetain.Load())
		assert.False(t, service.Config.Retained)

		require.NoError(t, service.Send("plain", nil))
		assert.False(t, connection.lastRetain.Load())
	})
}

// newLifecycleService initializes a service that creates its connections with
// connector, and closes it when the test ends.
//
// Parameters:
//   - t: the test.
//   - connector: creates the service's connections.
//
// Returns:
//   - *Service: the initialized service.
func newLifecycleService(t *testing.T, connector *fakeConnector) *Service {
	t.Helper()

	serviceURL, err := url.Parse(lifecycleURL)
	require.NoError(t, err)

	service := &Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

	service.connect = connector.connect

	t.Cleanup(func() { _ = service.Close() })

	return service
}

// newFakeConnection returns a fake connection that ends when ctx does.
//
// Parameters:
//   - ctx: the connection's lifetime.
//
// Returns:
//   - *fakeConnection: the connection.
func newFakeConnection(ctx context.Context) *fakeConnection {
	connection := &fakeConnection{done: make(chan struct{})}
	context.AfterFunc(ctx, connection.end)

	return connection
}

// AwaitConnection reports whether the connection is still up.
func (f *fakeConnection) AwaitConnection(context.Context) error {
	if f.ended() {
		return errConnectionEnded
	}

	return nil
}

// Disconnect records the call and ends the connection.
func (f *fakeConnection) Disconnect(context.Context) error {
	f.disconnects.Add(1)
	f.end()

	return nil
}

// Done returns a channel that is closed when the connection ends.
func (f *fakeConnection) Done() <-chan struct{} {
	return f.done
}

// Publish records a publish on a live connection.
func (f *fakeConnection) Publish(_ context.Context, publish *paho.Publish) (*paho.PublishResponse, error) {
	if f.ended() {
		return nil, errConnectionEnded
	}

	f.publishes.Add(1)
	f.lastRetain.Store(publish.Retain)

	return &paho.PublishResponse{}, nil
}

// end ends the connection.
func (f *fakeConnection) end() {
	f.endOnce.Do(func() { close(f.done) })
}

// ended reports whether the connection has ended.
func (f *fakeConnection) ended() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// connect creates a fake connection bound to ctx.
func (c *fakeConnector) connect(ctx context.Context, _ *autopaho.ClientConfig) (ConnectionManager, error) {
	connection := newFakeConnection(ctx)

	c.mu.Lock()
	c.connections = append(c.connections, connection)
	c.mu.Unlock()

	return connection, nil
}

// created returns the connections created so far.
func (c *fakeConnector) created() []*fakeConnection {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]*fakeConnection(nil), c.connections...)
}
