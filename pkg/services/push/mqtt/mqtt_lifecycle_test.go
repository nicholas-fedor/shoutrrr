package mqtt

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"log"
	"log/slog"
	"net/url"
	"slices"
	"strings"
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
	// gate, when set, holds each publish until it is closed.
	gate chan struct{}
}

// fakeConnector creates fake connections and records them in order.
type fakeConnector struct {
	// mu guards connections.
	mu sync.Mutex
	// connections holds every connection created so far.
	connections []*fakeConnection
	// gateFirst, when set, becomes the publish gate of the first connection.
	gateFirst chan struct{}
	// tlsConfigs holds the TLS settings of each connection, nil for one
	// without TLS.
	tlsConfigs []*tls.Config
}

// warningContexts records the test value of the context each warning was
// logged with, as a slog handler. It is safe for concurrent use.
type warningContexts struct {
	// values holds the test value of each warning's context.
	values []any
	// mu guards values.
	mu sync.Mutex
}

// sendKey identifies the test value in a send's context.
type sendKey struct{}

// lifecycleURL is a broker URL for the lifecycle tests.
const lifecycleURL = "mqtt://broker.example.invalid/test/topic"

// errConnectionEnded is returned by a fake connection used after it ended.
var errConnectionEnded = errors.New("connection ended")

// TestConnectionOutlivesSendBudget verifies that a connection the service
// creates is not bounded by a send's budget, so a send more than 10 seconds
// after the first one reuses the same live connection.
func TestConnectionOutlivesSendBudget(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{}
		service := newLifecycleService(t, connector)

		require.NoError(t, service.Send("first", nil))

		time.Sleep(types.DefaultSendTimeout + time.Second)

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

// TestCloseWaitsForInFlightSend verifies that Close lets a send already using the
// connection finish before disconnecting it, and that a send started meanwhile
// opens a new connection.
func TestCloseWaitsForInFlightSend(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{gateFirst: make(chan struct{})}
		service := newLifecycleService(t, connector)

		sendErr := make(chan error, 1)

		go func() { sendErr <- service.Send("in flight", nil) }()

		synctest.Wait()

		closeErr := make(chan error, 1)

		go func() { closeErr <- service.Close() }()

		synctest.Wait()

		first := connector.created()[0]
		assert.Zero(t, first.disconnects.Load(), "Close must wait for the in-flight send")

		require.NoError(t, service.Send("during close", nil))

		connections := connector.created()
		require.Len(t, connections, 2)
		assert.Equal(t, int32(1), connections[1].publishes.Load())

		close(connector.gateFirst)
		require.NoError(t, <-sendErr)
		require.NoError(t, <-closeErr)

		assert.Equal(t, int32(1), first.publishes.Load())
		assert.Equal(t, int32(1), first.disconnects.Load())
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

// TestSendContextCancel verifies that canceling the caller's context ends a
// publish the broker does not answer.
func TestSendContextCancel(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{gateFirst: make(chan struct{})}
		service := newLifecycleService(t, connector)

		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(time.Second, cancel)

		start := time.Now()
		err := service.SendContext(ctx, "held", nil)

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, time.Second, time.Since(start))
	})
}

// TestSendBudget verifies that a send without a deadline gives up on a publish
// the broker does not answer after MQTT's send budget.
func TestSendBudget(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		connector := &fakeConnector{gateFirst: make(chan struct{})}
		service := newLifecycleService(t, connector)

		start := time.Now()
		err := service.Send("held", nil)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, service.ServiceTimeout(nil), time.Since(start))
	})
}

// TestSendWarnings verifies that each warning about a connection's settings
// is logged once per service instance, even when an idle teardown makes a
// later send open the connection again.
func TestSendWarnings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup  func(service *Service)
		params *types.Params
		name   string
		rawURL string
		want   []string
	}{
		{
			name:   "plain mqtt",
			rawURL: "mqtt://broker.example.invalid/t",
			want:   []string{"WARN sending without TLS"},
		},
		{
			name:   "TLS disabled",
			rawURL: "mqtts://broker.example.invalid/t?disabletls=yes",
			want:   []string{"WARN sending without TLS"},
		},
		{name: "TLS", rawURL: "mqtts://broker.example.invalid/t"},
		{
			name:   "verification skipped",
			rawURL: "mqtts://broker.example.invalid/t?skiptlsverify=yes",
			want:   []string{"WARN skipping TLS certificate verification"},
		},
		{
			name:   "deprecated verification key in params",
			rawURL: "mqtts://broker.example.invalid/t",
			params: &types.Params{"disabletlsverification": "yes"},
			want: []string{
				`WARN config key is deprecated key=disabletlsverification replacement=skiptlsverify`,
				"WARN skipping TLS certificate verification",
			},
		},
		{
			name:   "deprecated verification field",
			rawURL: "mqtts://broker.example.invalid/t",
			setup:  func(service *Service) { service.Config.DisableTLSVerification = true },
			want: []string{
				"WARN config field is deprecated field=DisableTLSVerification replacement=SkipTLSVerify",
				"WARN skipping TLS certificate verification",
			},
		},
		{
			name:   "mqtt scheme on the TLS port",
			rawURL: "mqtt://broker.example.invalid:8883/t",
			want: []string{
				"WARN mqtt scheme on the TLS port, not using TLS port=8883",
				"WARN sending without TLS",
			},
		},
		{
			name:   "mqtts scheme on the plain port",
			rawURL: "mqtts://broker.example.invalid:1883/t",
			want:   []string{"WARN mqtts scheme on the plain MQTT port, attempting TLS port=1883"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var logs bytes.Buffer

				serviceURL, err := url.Parse(tt.rawURL)
				require.NoError(t, err)

				service := &Service{}
				require.NoError(t, service.Initialize(serviceURL, log.New(&logs, "", 0)))

				connector := &fakeConnector{}
				service.connect = connector.connect

				t.Cleanup(func() { _ = service.Close() })

				if tt.setup != nil {
					tt.setup(service)
				}

				require.NoError(t, service.Send("first", tt.params))
				require.NoError(t, service.Send("second", tt.params))

				time.Sleep(idleTimeout + time.Second)

				require.NoError(t, service.Send("after reconnect", tt.params))
				require.Len(t, connector.created(), 2)
				assert.Equal(t, tt.want, warningLines(logs.String()))
			})
		})
	}
}

// TestConnectionWarningsContext verifies that the warnings a send logs while it
// opens a connection carry the send's context, though the connection outlives
// the send.
func TestConnectionWarningsContext(t *testing.T) {
	t.Parallel()

	// tests lists URLs whose connection logs one warning.
	tests := map[string]string{
		"plaintext": "mqtt://broker.example.invalid/t",
		"insecure":  "mqtts://broker.example.invalid/t?skiptlsverify=yes",
	}

	for name, rawURL := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				serviceURL, err := url.Parse(rawURL)
				require.NoError(t, err)

				recorder := &warningContexts{}

				service := &Service{}
				require.NoError(t, service.Initialize(serviceURL, types.NewSlogLogger(slog.New(recorder))))

				connector := &fakeConnector{}
				service.connect = connector.connect

				t.Cleanup(func() { _ = service.Close() })

				ctx := context.WithValue(t.Context(), sendKey{}, "send")
				require.NoError(t, service.SendContext(ctx, "message", nil))

				assert.Equal(t, []any{"send"}, recorder.values)
			})
		})
	}
}

// TestConnectErrorWarning verifies that a failed connection attempt, such as a
// broker rejecting the credentials, is logged at Warn with its error. A send
// that cannot connect reports only its context's error, so the warning is the
// only report of why.
func TestConnectErrorWarning(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var (
			logs bytes.Buffer
			cfg  *autopaho.ClientConfig
		)

		serviceURL, err := url.Parse(lifecycleURL)
		require.NoError(t, err)

		service := &Service{}
		require.NoError(t, service.Initialize(serviceURL, log.New(&logs, "", 0)))

		service.connect = func(ctx context.Context, config *autopaho.ClientConfig) (ConnectionManager, error) {
			cfg = config

			return newFakeConnection(ctx), nil
		}

		t.Cleanup(func() { _ = service.Close() })

		require.NoError(t, service.Send("message", nil))
		require.NotNil(t, cfg)

		cfg.OnConnectError(errors.New("bad user name or password"))

		assert.Contains(t, logs.String(), `WARN connection attempt failed error="bad user name or password"`+"\n")
	})
}

// TestInitializeWarnings verifies that Initialize warns about a deprecated
// key in the URL, and that initializing the service again lets each warning
// appear again.
func TestInitializeWarnings(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var logs bytes.Buffer

		serviceURL, err := url.Parse("mqtt://broker.example.invalid/t?disabletlsverification=no")
		require.NoError(t, err)

		service := &Service{}
		connector := &fakeConnector{}

		t.Cleanup(func() { _ = service.Close() })

		for range 2 {
			require.NoError(t, service.Initialize(serviceURL, log.New(&logs, "", 0)))
			service.connect = connector.connect

			require.NoError(t, service.Send("hello", nil))
		}

		warnings := []string{
			`WARN config key is deprecated key=disabletlsverification replacement=skiptlsverify`,
			"WARN sending without TLS",
		}
		assert.Equal(t, slices.Concat(warnings, warnings), warningLines(logs.String()))
	})
}

// TestSendTLSSettings verifies the TLS settings of the connection a send
// opens. They require TLS 1.2 and skip verification only when the URL, the
// params, or the deprecated field ask for it. A connection without TLS has
// none.
func TestSendTLSSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		setup    func(service *Service)
		params   *types.Params
		name     string
		rawURL   string
		wantTLS  bool
		wantSkip bool
	}{
		{name: "plain", rawURL: "mqtt://broker.example.invalid/t"},
		{name: "TLS disabled", rawURL: "mqtts://broker.example.invalid/t?disabletls=yes"},
		{name: "verified", rawURL: "mqtts://broker.example.invalid/t", wantTLS: true},
		{name: "skipped in the URL", rawURL: "mqtts://broker.example.invalid/t?skiptlsverify=yes", wantTLS: true, wantSkip: true},
		{
			name:     "skipped in params",
			rawURL:   "mqtts://broker.example.invalid/t",
			params:   &types.Params{"skiptlsverify": "yes"},
			wantTLS:  true,
			wantSkip: true,
		},
		{
			name:     "skipped through the deprecated field",
			rawURL:   "mqtts://broker.example.invalid/t",
			setup:    func(service *Service) { service.Config.DisableTLSVerification = true },
			wantTLS:  true,
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				serviceURL, err := url.Parse(tt.rawURL)
				require.NoError(t, err)

				service := &Service{}
				require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

				connector := &fakeConnector{}
				service.connect = connector.connect

				t.Cleanup(func() { _ = service.Close() })

				if tt.setup != nil {
					tt.setup(service)
				}

				require.NoError(t, service.Send("hello", tt.params))
				require.Len(t, connector.tlsConfigs, 1)

				tlsConfig := connector.tlsConfigs[0]
				if !tt.wantTLS {
					assert.Nil(t, tlsConfig)

					return
				}

				require.NotNil(t, tlsConfig)
				assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
				assert.Equal(t, tt.wantSkip, tlsConfig.InsecureSkipVerify)
			})
		})
	}
}

// warningLines returns the logged lines that are warnings.
//
// Parameters:
//   - logs: the log output.
//
// Returns:
//   - []string: the warning lines in order.
func warningLines(logs string) []string {
	var lines []string

	for line := range strings.Lines(logs) {
		if strings.HasPrefix(line, "WARN ") {
			lines = append(lines, strings.TrimSuffix(line, "\n"))
		}
	}

	return lines
}

// Enabled reports whether level is Warn or above.
//
// Parameters:
//   - level: the level of a record.
//
// Returns:
//   - bool: true for warnings.
func (*warningContexts) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

// Handle records the test value of the record's context.
//
// Parameters:
//   - ctx: the context of the call.
//
// Returns:
//   - error: always nil.
//
//nolint:gocritic // slog.Handler defines Handle with a Record value.
func (w *warningContexts) Handle(ctx context.Context, _ slog.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.values = append(w.values, ctx.Value(sendKey{}))

	return nil
}

// WithAttrs returns w. The service adds no attributes to the handler.
//
// Returns:
//   - slog.Handler: w.
func (w *warningContexts) WithAttrs([]slog.Attr) slog.Handler { return w }

// WithGroup returns w. The service adds no groups to the handler.
//
// Returns:
//   - slog.Handler: w.
func (w *warningContexts) WithGroup(string) slog.Handler { return w }

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
//
// Returns:
//   - error: [errConnectionEnded] after the connection ended, otherwise nil.
func (f *fakeConnection) AwaitConnection(context.Context) error {
	if f.ended() {
		return errConnectionEnded
	}

	return nil
}

// Disconnect records the call and ends the connection.
//
// Returns:
//   - error: always nil.
func (f *fakeConnection) Disconnect(context.Context) error {
	f.disconnects.Add(1)
	f.end()

	return nil
}

// Done returns a channel that is closed when the connection ends.
//
// Returns:
//   - <-chan struct{}: the channel closed when the connection ends.
func (f *fakeConnection) Done() <-chan struct{} {
	return f.done
}

// Publish records a publish on a live connection, after the gate opens when one is set.
//
// Parameters:
//   - ctx: cancellation for the wait on the gate.
//   - publish: the message to publish, whose Retain flag is recorded.
//
// Returns:
//   - *paho.PublishResponse: an empty response for a recorded publish.
//   - error: the context error when ctx ends before the gate opens, or
//     [errConnectionEnded] after the connection ended.
func (f *fakeConnection) Publish(ctx context.Context, publish *paho.Publish) (*paho.PublishResponse, error) {
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

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
//
// Returns:
//   - bool: true once done is closed.
func (f *fakeConnection) ended() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// connect creates a fake connection bound to ctx. It records the TLS settings
// and gives the first connection the gateFirst gate.
//
// Parameters:
//   - ctx: the connection's lifetime.
//   - cfg: the client config, whose TLS settings are recorded.
//
// Returns:
//   - ConnectionManager: the new fake connection.
//   - error: always nil.
func (c *fakeConnector) connect(ctx context.Context, cfg *autopaho.ClientConfig) (ConnectionManager, error) {
	connection := newFakeConnection(ctx)

	c.mu.Lock()
	c.tlsConfigs = append(c.tlsConfigs, cfg.TlsCfg)

	if len(c.connections) == 0 {
		connection.gate = c.gateFirst
	}

	c.connections = append(c.connections, connection)
	c.mu.Unlock()

	return connection, nil
}

// created returns the connections created so far.
//
// Returns:
//   - []*fakeConnection: a copy of the connections in creation order.
func (c *fakeConnector) created() []*fakeConnection {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]*fakeConnection(nil), c.connections...)
}
