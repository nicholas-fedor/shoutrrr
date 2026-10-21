package mqtt_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
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
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// pipeBroker is an in-process MQTT v5 broker reached through net.Pipe, so the
// real autopaho client runs without any network connection. It accepts every
// connection, records what each one sends, and acknowledges QoS 1 and QoS 2
// publishes with its configured reason code.
type pipeBroker struct {
	// mu guards sessions and addrs.
	mu sync.Mutex
	// sessions holds one entry per dialed connection, in dial order.
	sessions []*brokerSession
	// addrs holds the address of each dial, in dial order.
	addrs []string
	// ackReason is the reason string sent with a failing acknowledgement.
	ackReason string
	// ackCode is the reason code of each PUBACK and PUBREC.
	ackCode byte
}

// brokerSession records the packets one client connection sent to the broker.
type brokerSession struct {
	// mu guards the fields below.
	mu sync.Mutex
	// connect holds the client's CONNECT packet.
	connect brokerConnect
	// publishes holds each PUBLISH packet in arrival order.
	publishes []brokerPublish
	// disconnected reports whether the client sent DISCONNECT.
	disconnected bool
}

// brokerConnect holds the CONNECT fields a service sets from its URL.
type brokerConnect struct {
	// clientID is the client identifier.
	clientID string
	// username is the user name.
	username string
	// password is the password.
	password string
	// usernameFlag reports whether the packet carries a user name.
	usernameFlag bool
	// passwordFlag reports whether the packet carries a password.
	passwordFlag bool
	// cleanStart reports whether the client asked for a new session.
	cleanStart bool
}

// brokerPublish holds the fields of a PUBLISH packet the broker received.
type brokerPublish struct {
	// topic is the topic name.
	topic string
	// payload is the message payload.
	payload string
	// qos is the QoS level.
	qos byte
	// retain is the retain flag.
	retain bool
}

const (
	// pipeBrokerURL is the service URL for the in-process broker. Plain mqtt
	// keeps the pipe free of TLS, and the missing port selects the default.
	pipeBrokerURL = "mqtt://broker.example.invalid/test/topic"
	// mockBrokerURL is the service URL for tests that inject a connection
	// manager, which never dials it.
	mockBrokerURL = "mqtt://broker.example.com/test/topic"
	// publishTimeout is the time a send allows for the connection wait and the
	// publish together.
	publishTimeout = 10 * time.Second
	// disconnectTimeout is the time Close allows for the disconnect.
	disconnectTimeout = 5 * time.Second
	// idleTimeout is how long a connection the service opened stays open
	// without sends.
	idleTimeout = 60 * time.Second
	// replyBuffer is the number of broker answers that can wait for the
	// client.
	replyBuffer = 16
)

// Errors that the injected connection managers return.
var (
	// errRefused is a connection failure.
	errRefused = errors.New("connection refused")
	// errNetwork is a publish failure.
	errNetwork = errors.New("network error")
	// errDisconnect is a disconnect failure.
	errDisconnect = errors.New("disconnect error")
)

// TestIntegrationPublish verifies that a send through an injected connection
// manager publishes the message to the URL's topic with the QoS and retain
// flag from the URL, overridden by the send's params.
func TestIntegrationPublish(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the publish the manager should
	// receive.
	tests := []struct {
		params *types.Params
		name   string
		rawURL string
		want   paho.Publish
	}{
		{
			name:   "defaults",
			rawURL: mockBrokerURL,
			want:   paho.Publish{Topic: "test/topic", QoS: 0, Retain: false},
		},
		{
			name:   "QoS 1",
			rawURL: mockBrokerURL + "?qos=1",
			want:   paho.Publish{Topic: "test/topic", QoS: 1},
		},
		{
			name:   "QoS 2",
			rawURL: mockBrokerURL + "?qos=2",
			want:   paho.Publish{Topic: "test/topic", QoS: 2},
		},
		{
			name:   "retained yes",
			rawURL: mockBrokerURL + "?retained=yes",
			want:   paho.Publish{Topic: "test/topic", Retain: true},
		},
		{
			name:   "retained true",
			rawURL: mockBrokerURL + "?retained=true",
			want:   paho.Publish{Topic: "test/topic", Retain: true},
		},
		{
			name:   "all options combined",
			rawURL: mockBrokerURL + "?qos=1&retained=yes&clientid=myclient&cleansession=no",
			want:   paho.Publish{Topic: "test/topic", QoS: 1, Retain: true},
		},
		{
			name:   "params override the URL",
			rawURL: mockBrokerURL + "?qos=0",
			params: &types.Params{"qos": "2", "retained": "yes"},
			want:   paho.Publish{Topic: "test/topic", QoS: 2, Retain: true},
		},
		{
			name:   "empty params",
			rawURL: mockBrokerURL,
			params: &types.Params{},
			want:   paho.Publish{Topic: "test/topic"},
		},
		{
			name:   "nested topic",
			rawURL: "mqtt://broker.example.com/home/sensors/temperature",
			want:   paho.Publish{Topic: "home/sensors/temperature"},
		},
		{
			name:   "underscores, hyphens, and numbers",
			rawURL: "mqtt://broker.example.com/test_topic/test-topic/test123",
			want:   paho.Publish{Topic: "test_topic/test-topic/test123"},
		},
		{
			name:   "leading slash topic",
			rawURL: "mqtt://broker.example.com//home/alerts",
			want:   paho.Publish{Topic: "/home/alerts"},
		},
		{
			name:   "deep topic",
			rawURL: "mqtt://broker.example.com/level1/level2/level3/level4/level5/level6/level7/level8/level9/level10",
			want:   paho.Publish{Topic: "level1/level2/level3/level4/level5/level6/level7/level8/level9/level10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got *paho.Publish

			manager := mocks.NewMockConnectionManager(t)
			manager.EXPECT().AwaitConnection(mock.Anything).Return(nil).Once()
			manager.EXPECT().Publish(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, publish *paho.Publish) (*paho.PublishResponse, error) {
					got = publish

					return &paho.PublishResponse{}, nil
				}).Once()

			service := newMockedService(t, tt.rawURL, manager)

			require.NoError(t, service.Send("Hello, MQTT!", tt.params))

			want := tt.want
			want.Payload = []byte("Hello, MQTT!")
			assert.Equal(t, &want, got)
		})
	}
}

// TestIntegrationPublishReasonCodes verifies that a publish response with a
// failure reason code fails the send with a PublishError that carries the code
// and reason string, including when the client also returns an error, as paho
// does for a rejected QoS 1 PUBACK, and that a success or warning code does
// not. A QoS 2 PUBCOMP with Packet Identifier not found completes a publish
// whose PUBREL was resent after a reconnect, which MQTT v5 does not treat as an
// error during recovery.
func TestIntegrationPublishReasonCodes(t *testing.T) {
	t.Parallel()

	// tests lists reason codes, reason strings, and client errors from the
	// publish, with whether the send should fail.
	tests := []struct {
		publishErr   error
		name         string
		reason       string
		code         byte
		wantFail     bool
		noProperties bool
	}{
		{name: "success", code: 0x00},
		{name: "granted QoS 1", code: 0x01},
		{name: "no matching subscribers", code: 0x10},
		{name: "unspecified error without reason string", code: 0x80, wantFail: true},
		{name: "unspecified error", code: 0x80, reason: "Unspecified error", wantFail: true},
		{name: "malformed packet", code: 0x81, reason: "Malformed Packet", wantFail: true},
		{name: "protocol error", code: 0x82, reason: "Protocol Error", wantFail: true},
		{name: "implementation specific error", code: 0x83, reason: "Implementation specific error", wantFail: true},
		{name: "unsupported protocol version", code: 0x84, reason: "Unsupported Protocol Version", wantFail: true},
		{name: "client identifier not valid", code: 0x85, reason: "Client Identifier not valid", wantFail: true},
		{name: "bad user name or password", code: 0x86, reason: "Bad User Name or Password", wantFail: true},
		{name: "not authorized", code: 0x87, reason: "Not authorized", wantFail: true},
		{
			name:       "not authorized with the client's error",
			code:       0x87,
			reason:     "Not authorized",
			publishErr: errors.New("error publishing: The PUBLISH is not authorized."),
			wantFail:   true,
		},
		{name: "failure without properties", code: 0x80, noProperties: true, wantFail: true},
		{name: "packet identifier not found in a recovered QoS 2 PUBCOMP", code: 0x92},
		{
			name:       "packet identifier not found with the client's error",
			code:       0x92,
			publishErr: errors.New("error publishing: Packet Identifier not found"),
			wantFail:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			manager := mocks.NewMockConnectionManager(t)
			manager.EXPECT().AwaitConnection(mock.Anything).Return(nil).Once()

			response := &paho.PublishResponse{
				ReasonCode: tt.code,
				Properties: &paho.PublishResponseProperties{ReasonString: tt.reason},
			}
			if tt.noProperties {
				response.Properties = nil
			}

			manager.EXPECT().Publish(mock.Anything, mock.Anything).Return(response, tt.publishErr).Once()

			service := newMockedService(t, mockBrokerURL, manager)

			err := service.Send("Test message", nil)
			if !tt.wantFail {
				require.NoError(t, err)

				return
			}

			publishErr, ok := errors.AsType[mqtt.PublishError](err)
			require.True(t, ok, "error %v is not a PublishError", err)
			assert.Equal(t, mqtt.PublishError{ReasonCode: tt.code, ReasonString: tt.reason}, publishErr)
		})
	}
}

// TestIntegrationSendErrors verifies that a send reports a failed connection
// wait or publish from the connection manager with its cause, as a caller sees
// it.
func TestIntegrationSendErrors(t *testing.T) {
	t.Parallel()

	// tests lists connection-wait and publish failures with the error prefix
	// the send should report.
	tests := []struct {
		awaitErr   error
		publishErr error
		wantErr    error
		name       string
		wantPrefix string
	}{
		{name: "connection refused", awaitErr: errRefused, wantErr: errRefused, wantPrefix: "connecting to MQTT broker"},
		{name: "connection wait canceled", awaitErr: context.Canceled, wantErr: context.Canceled, wantPrefix: "connecting to MQTT broker"},
		{
			name:       "connection wait deadline",
			awaitErr:   context.DeadlineExceeded,
			wantErr:    context.DeadlineExceeded,
			wantPrefix: "connecting to MQTT broker",
		},
		{name: "publish network error", publishErr: errNetwork, wantErr: errNetwork, wantPrefix: `publishing to MQTT topic "test/topic"`},
		{name: "publish canceled", publishErr: context.Canceled, wantErr: context.Canceled, wantPrefix: `publishing to MQTT topic "test/topic"`},
		{
			name:       "publish deadline",
			publishErr: context.DeadlineExceeded,
			wantErr:    context.DeadlineExceeded,
			wantPrefix: `publishing to MQTT topic "test/topic"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			manager := mocks.NewMockConnectionManager(t)
			manager.EXPECT().AwaitConnection(mock.Anything).Return(tt.awaitErr).Once()

			if tt.awaitErr == nil {
				manager.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil, tt.publishErr).Once()
			}

			service := newMockedService(t, mockBrokerURL, manager)

			err := service.Send("Test message", nil)

			require.ErrorIs(t, err, tt.wantErr)
			assert.True(t, strings.HasPrefix(err.Error(), tt.wantPrefix), "error %q does not start with %q", err, tt.wantPrefix)
		})
	}
}

// TestIntegrationSendInvalidQoS verifies that a send with a QoS outside 0 to 2
// fails with ErrInvalidQoS before it touches the connection.
func TestIntegrationSendInvalidQoS(t *testing.T) {
	t.Parallel()

	// invalidLevels lists QoS values above and below the valid range.
	invalidLevels := []mqtt.QoS{3, -1}

	for _, qos := range invalidLevels {
		t.Run(qos.String(), func(t *testing.T) {
			t.Parallel()

			// The mock has no expectations, so any use of the connection fails
			// the test.
			service := newMockedService(t, mockBrokerURL, mocks.NewMockConnectionManager(t))
			service.Config.QoS = qos

			require.ErrorIs(t, service.Send("Test message", nil), mqtt.ErrInvalidQoS)
		})
	}
}

// TestIntegrationInjectedManagerSends verifies that sends reuse an injected
// connection manager, one after another or concurrently, and that a failure
// after a successful send reaches the caller.
func TestIntegrationInjectedManagerSends(t *testing.T) {
	t.Parallel()

	t.Run("sequential", func(t *testing.T) {
		t.Parallel()

		var payloads []string

		manager := mocks.NewMockConnectionManager(t)
		manager.EXPECT().AwaitConnection(mock.Anything).Return(nil).Times(3)
		manager.EXPECT().Publish(mock.Anything, mock.Anything).
			RunAndReturn(func(_ context.Context, publish *paho.Publish) (*paho.PublishResponse, error) {
				payloads = append(payloads, string(publish.Payload))

				return &paho.PublishResponse{}, nil
			}).Times(3)

		service := newMockedService(t, mockBrokerURL, manager)

		// messages lists the messages to send, in order.
		messages := []string{"First message", "Second message", "Third message"}
		for _, message := range messages {
			require.NoError(t, service.Send(message, nil))
		}

		assert.Equal(t, messages, payloads)
	})

	t.Run("concurrent", func(t *testing.T) {
		t.Parallel()

		// sends is the number of concurrent sends.
		const sends = 5

		manager := mocks.NewMockConnectionManager(t)
		manager.EXPECT().AwaitConnection(mock.Anything).Return(nil).Times(sends)
		manager.EXPECT().Publish(mock.Anything, mock.Anything).Return(&paho.PublishResponse{}, nil).Times(sends)

		service := newMockedService(t, mockBrokerURL, manager)

		errs := make(chan error, sends)
		for range sends {
			go func() { errs <- service.Send("Message", nil) }()
		}

		for range sends {
			require.NoError(t, <-errs)
		}
	})

	t.Run("failure after success", func(t *testing.T) {
		t.Parallel()

		manager := mocks.NewMockConnectionManager(t)
		manager.EXPECT().AwaitConnection(mock.Anything).Return(nil).Once()
		manager.EXPECT().Publish(mock.Anything, mock.Anything).Return(&paho.PublishResponse{}, nil).Once()
		manager.EXPECT().AwaitConnection(mock.Anything).Return(context.DeadlineExceeded).Once()

		service := newMockedService(t, mockBrokerURL, manager)

		require.NoError(t, service.Send("First message", nil))
		require.ErrorIs(t, service.Send("Second message", nil), context.DeadlineExceeded)
	})
}

// TestIntegrationClose verifies that Close disconnects an injected connection
// manager once, even when called again, and reports a disconnect failure with
// its cause.
func TestIntegrationClose(t *testing.T) {
	t.Parallel()

	// tests lists disconnect results with the error Close should match.
	tests := []struct {
		disconnectErr error
		name          string
	}{
		{name: "success"},
		{name: "disconnect error", disconnectErr: errDisconnect},
		{name: "disconnect canceled", disconnectErr: context.Canceled},
		{name: "disconnect deadline", disconnectErr: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			manager := mocks.NewMockConnectionManager(t)
			manager.EXPECT().AwaitConnection(mock.Anything).Return(nil).Once()
			manager.EXPECT().Publish(mock.Anything, mock.Anything).Return(&paho.PublishResponse{}, nil).Once()
			manager.EXPECT().Disconnect(mock.Anything).Return(tt.disconnectErr).Once()

			service := newMockedService(t, mockBrokerURL, manager)

			require.NoError(t, service.Send("Test message", nil))

			err := service.Close()
			if tt.disconnectErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.disconnectErr)
				assert.True(t, strings.HasPrefix(err.Error(), "disconnecting from MQTT broker"), "error %q", err)
			}

			require.NoError(t, service.Close(), "a second Close must not disconnect again")
		})
	}
}

// TestIntegrationTimeouts verifies that a send gives the connection wait and
// the publish 10 seconds together, and that Close gives the disconnect 5
// seconds, against a connection manager that never answers.
func TestIntegrationTimeouts(t *testing.T) {
	t.Parallel()

	// blockUntilDone returns the context's error once it ends.
	blockUntilDone := func(ctx context.Context) error {
		<-ctx.Done()

		return ctx.Err()
	}

	// tests lists connection manager behaviors with the operation to run, the
	// time it should take, and the error prefix it should report.
	tests := []struct {
		expect      func(manager *mocks.MockConnectionManager)
		run         func(service *mqtt.Service) error
		name        string
		wantPrefix  string
		wantElapsed time.Duration
	}{
		{
			name: "connection wait",
			expect: func(manager *mocks.MockConnectionManager) {
				manager.EXPECT().AwaitConnection(mock.Anything).RunAndReturn(blockUntilDone).Once()
			},
			run:         func(service *mqtt.Service) error { return service.Send("Test message", nil) },
			wantPrefix:  "connecting to MQTT broker",
			wantElapsed: publishTimeout,
		},
		{
			name: "publish after a slow connection wait",
			expect: func(manager *mocks.MockConnectionManager) {
				manager.EXPECT().AwaitConnection(mock.Anything).RunAndReturn(func(context.Context) error {
					time.Sleep(4 * time.Second)

					return nil
				}).Once()
				manager.EXPECT().Publish(mock.Anything, mock.Anything).
					RunAndReturn(func(ctx context.Context, _ *paho.Publish) (*paho.PublishResponse, error) {
						return nil, blockUntilDone(ctx)
					}).Once()
			},
			run:         func(service *mqtt.Service) error { return service.Send("Test message", nil) },
			wantPrefix:  "publishing to MQTT topic",
			wantElapsed: publishTimeout,
		},
		{
			name: "disconnect",
			expect: func(manager *mocks.MockConnectionManager) {
				manager.EXPECT().Disconnect(mock.Anything).RunAndReturn(blockUntilDone).Once()
			},
			run:         func(service *mqtt.Service) error { return service.Close() },
			wantPrefix:  "disconnecting from MQTT broker",
			wantElapsed: disconnectTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				manager := mocks.NewMockConnectionManager(t)
				tt.expect(manager)

				service := newMockedService(t, mockBrokerURL, manager)

				start := time.Now()
				err := tt.run(service)

				assert.Equal(t, tt.wantElapsed, time.Since(start))
				require.ErrorIs(t, err, context.DeadlineExceeded)
				assert.True(t, strings.HasPrefix(err.Error(), tt.wantPrefix), "error %q does not start with %q", err, tt.wantPrefix)
			})
		})
	}
}

// TestIntegrationInitializeErrors verifies that Initialize rejects a URL
// without a topic or with a QoS outside the enum.
func TestIntegrationInitializeErrors(t *testing.T) {
	t.Parallel()

	// tests lists invalid URLs with the error or message Initialize should
	// report.
	tests := []struct {
		wantErr error
		name    string
		rawURL  string
		wantMsg string
	}{
		{name: "missing topic", rawURL: "mqtt://broker.example.com", wantErr: mqtt.ErrTopicRequired},
		{name: "trailing slash only", rawURL: "mqtt://broker.example.com/", wantErr: mqtt.ErrTopicRequired},
		{name: "QoS out of range", rawURL: mockBrokerURL + "?qos=3", wantMsg: `"qos"`},
		{name: "QoS not a level", rawURL: mockBrokerURL + "?qos=invalid", wantMsg: `"qos"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, err := url.Parse(tt.rawURL)
			require.NoError(t, err)

			err = (&mqtt.Service{}).Initialize(serviceURL, log.New(t.Output(), "", 0))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.ErrorContains(t, err, tt.wantMsg)
			}
		})
	}
}

// TestIntegrationBrokerConnect verifies that the first send dials the URL's
// broker address and connects with the client identifier, session setting, and
// percent-decoded credentials from the URL, including a password without a
// username, which MQTT v5 allows.
func TestIntegrationBrokerConnect(t *testing.T) {
	t.Parallel()

	// tests lists service URLs with the address the service should dial and
	// the CONNECT packet the broker should receive.
	tests := []struct {
		name     string
		rawURL   string
		wantAddr string
		want     brokerConnect
	}{
		{
			name:     "defaults",
			rawURL:   pipeBrokerURL,
			wantAddr: "broker.example.invalid:1883",
			want:     brokerConnect{clientID: "shoutrrr", cleanStart: true},
		},
		{
			name:     "username and password",
			rawURL:   "mqtt://user:pass@broker.example.invalid/test/topic",
			wantAddr: "broker.example.invalid:1883",
			want: brokerConnect{
				clientID: "shoutrrr", cleanStart: true,
				username: "user", usernameFlag: true, password: "pass", passwordFlag: true,
			},
		},
		{
			name:     "percent-encoded credentials",
			rawURL:   "mqtt://user%40domain:pass%40word@broker.example.invalid/test/topic",
			wantAddr: "broker.example.invalid:1883",
			want: brokerConnect{
				clientID: "shoutrrr", cleanStart: true,
				username: "user@domain", usernameFlag: true, password: "pass@word", passwordFlag: true,
			},
		},
		{
			name:     "password without username",
			rawURL:   "mqtt://:token@broker.example.invalid/test/topic",
			wantAddr: "broker.example.invalid:1883",
			want:     brokerConnect{clientID: "shoutrrr", cleanStart: true, password: "token", passwordFlag: true},
		},
		{
			name:     "username only",
			rawURL:   "mqtt://user@broker.example.invalid/test/topic",
			wantAddr: "broker.example.invalid:1883",
			want:     brokerConnect{clientID: "shoutrrr", cleanStart: true, username: "user", usernameFlag: true},
		},
		{
			name:     "IPv6 broker",
			rawURL:   "mqtt://[2001:db8::1]/test/topic",
			wantAddr: "[2001:db8::1]:1883",
			want:     brokerConnect{clientID: "shoutrrr", cleanStart: true},
		},
		{
			name:     "client identifier, persistent session, and custom port",
			rawURL:   "mqtt://broker.example.invalid:1884/test/topic?clientid=custom-client&cleansession=no",
			wantAddr: "broker.example.invalid:1884",
			want:     brokerConnect{clientID: "custom-client", cleanStart: false},
		},
		{
			name:     "mqtts with TLS disabled",
			rawURL:   "mqtts://broker.example.invalid/test/topic?disabletls=yes",
			wantAddr: "broker.example.invalid:8883",
			want:     brokerConnect{clientID: "shoutrrr", cleanStart: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				broker := &pipeBroker{}
				service := newBrokerService(t, tt.rawURL, broker)

				require.NoError(t, service.Send("message", nil))
				synctest.Wait()

				assert.Equal(t, []string{tt.wantAddr}, broker.dialedAddrs())

				sessions := broker.dialed()
				require.Len(t, sessions, 1)
				assert.Equal(t, tt.want, sessions[0].connected())
				assert.Len(t, sessions[0].received(), 1)
			})
		})
	}
}

// TestIntegrationBrokerPublish verifies that a published message reaches the
// broker byte for byte with the URL's topic, QoS, and retain flag, and that
// the send completes the QoS 1 and QoS 2 acknowledgement flows.
func TestIntegrationBrokerPublish(t *testing.T) {
	t.Parallel()

	// binaryLike holds every byte value once.
	var binaryLike strings.Builder
	for i := range 256 {
		binaryLike.WriteByte(byte(i))
	}

	// tests lists URL queries and messages with the publish the broker should
	// receive. The topic is always test/topic.
	tests := []struct {
		name    string
		query   string
		message string
		qos     byte
		retain  bool
	}{
		{name: "basic message", message: "Hello, MQTT!"},
		{name: "empty message", message: ""},
		{name: "1 MiB message", message: strings.Repeat("x", 1024*1024)},
		{name: "unicode", message: "Hello 👋 世界 こんにちは مرحبا بالعالم Привет мир Test™ © • ★ ♠"},
		{name: "newlines and tabs", message: "Line1\nLine2\r\nLine3\n\nColumn1\tColumn2"},
		{name: "whitespace only", message: " \t\n\r "},
		{name: "JSON", message: `{"key": "value", "nested": {"array": [1, 2, 3]}}`},
		{name: "XML", message: `<message><to>user</to><from>sender</from><body>Hello</body></message>`},
		{name: "HTML entities", message: `<tag> & "quoted" 'apostrophe'`},
		{name: "every byte value", message: binaryLike.String()},
		{name: "retained", query: "?retained=yes", message: "retained", retain: true},
		{name: "QoS 1", query: "?qos=1", message: "at least once", qos: 1},
		{name: "QoS 2", query: "?qos=2", message: "exactly once", qos: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				broker := &pipeBroker{}
				service := newBrokerService(t, pipeBrokerURL+tt.query, broker)

				require.NoError(t, service.Send(tt.message, nil))
				synctest.Wait()

				sessions := broker.dialed()
				require.Len(t, sessions, 1)
				assert.Equal(t, []brokerPublish{{topic: "test/topic", payload: tt.message, qos: tt.qos, retain: tt.retain}}, sessions[0].received())
			})
		})
	}
}

// TestIntegrationBrokerRejects verifies that a publish the broker rejects in
// its PUBACK for QoS 1 or its PUBREC for QoS 2 fails the send with a
// PublishError that carries the broker's reason code and reason string.
func TestIntegrationBrokerRejects(t *testing.T) {
	t.Parallel()

	// tests lists the QoS levels whose acknowledgement the broker rejects.
	tests := []struct {
		name  string
		query string
	}{
		{name: "QoS 1", query: "?qos=1"},
		{name: "QoS 2", query: "?qos=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				broker := &pipeBroker{ackCode: 0x87, ackReason: "Not authorized"}
				service := newBrokerService(t, pipeBrokerURL+tt.query, broker)

				err := service.Send("denied", nil)

				publishErr, ok := errors.AsType[mqtt.PublishError](err)
				require.True(t, ok, "error %v is not a PublishError", err)
				assert.Equal(t, mqtt.PublishError{ReasonCode: 0x87, ReasonString: "Not authorized"}, publishErr)
			})
		})
	}
}

// TestIntegrationOwnedConnection verifies the lifecycle of a connection the
// service opens with the real MQTT client: it outlives the publish timeout,
// is disconnected once idle or closed, and the next send reconnects.
func TestIntegrationOwnedConnection(t *testing.T) {
	t.Parallel()

	t.Run("outlives the publish timeout", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			broker := &pipeBroker{}
			service := newBrokerService(t, pipeBrokerURL, broker)

			require.NoError(t, service.Send("first", nil))

			time.Sleep(publishTimeout + time.Second)

			require.NoError(t, service.Send("second", nil))
			synctest.Wait()

			sessions := broker.dialed()
			require.Len(t, sessions, 1)
			assert.Equal(t, []string{"first", "second"}, payloads(sessions[0].received()))
		})
	})

	t.Run("idle disconnect and reconnect", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			broker := &pipeBroker{}
			service := newBrokerService(t, pipeBrokerURL, broker)

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
			assert.Equal(t, []string{"second"}, payloads(sessions[1].received()))
		})
	})

	t.Run("close and reconnect", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			broker := &pipeBroker{}
			service := newBrokerService(t, pipeBrokerURL, broker)

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
	})
}

// newMockedService returns a service initialized from rawURL that logs to the
// test output and publishes through manager.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - manager: the connection manager to inject.
//
// Returns:
//   - *mqtt.Service: the service.
func newMockedService(t *testing.T, rawURL string, manager mqtt.ConnectionManager) *mqtt.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &mqtt.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetConnectionManager(manager)

	return service
}

// newBrokerService returns a service initialized from rawURL whose
// connections are dialed to broker through net.Pipe, and closes it when the
// test ends.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - broker: the in-process broker that serves each dialed connection.
//
// Returns:
//   - *mqtt.Service: the service.
func newBrokerService(t *testing.T, rawURL string, broker *pipeBroker) *mqtt.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &mqtt.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetDialContext(broker.dial)

	t.Cleanup(func() { _ = service.Close() })

	return service
}

// acknowledgement returns the broker's answer to publish: nothing for QoS 0,
// a PUBACK for QoS 1, and a PUBREC for QoS 2.
//
// Parameters:
//   - publish: the received PUBLISH packet.
//   - code: the reason code of the answer.
//   - reason: the reason string of the answer.
//
// Returns:
//   - io.WriterTo: the answer, or nil for QoS 0.
func acknowledgement(publish *packets.Publish, code byte, reason string) io.WriterTo {
	properties := &packets.Properties{ReasonString: reason}

	switch publish.QoS {
	case 1:
		return &packets.Puback{PacketID: publish.PacketID, ReasonCode: code, Properties: properties}
	case 2:
		return &packets.Pubrec{PacketID: publish.PacketID, ReasonCode: code, Properties: properties}
	default:
		return nil
	}
}

// writeReplies writes each reply to conn until replies is closed. After a
// failed write it discards the remaining replies.
//
// Parameters:
//   - conn: the broker's end of the pipe.
//   - replies: the answers to write, in order.
func writeReplies(conn net.Conn, replies <-chan io.WriterTo) {
	for reply := range replies {
		if _, err := reply.WriteTo(conn); err != nil {
			for range replies {
			}

			return
		}
	}
}

// payloads returns the payload of each publish.
//
// Parameters:
//   - publishes: the publishes.
//
// Returns:
//   - []string: the payloads in order.
func payloads(publishes []brokerPublish) []string {
	result := make([]string, 0, len(publishes))
	for _, publish := range publishes {
		result = append(result, publish.payload)
	}

	return result
}

// dial records addr and connects a client to the broker over a new in-memory
// pipe.
//
// Parameters:
//   - addr: the address the service dials.
//
// Returns:
//   - net.Conn: the client's end of the pipe.
//   - error: always nil.
func (b *pipeBroker) dial(_ context.Context, _, addr string) (net.Conn, error) {
	client, server := net.Pipe()
	session := &brokerSession{}

	b.mu.Lock()
	b.sessions = append(b.sessions, session)
	b.addrs = append(b.addrs, addr)
	b.mu.Unlock()

	go session.serve(server, b.ackCode, b.ackReason)

	return client, nil
}

// dialed returns the sessions of every connection dialed so far.
//
// Returns:
//   - []*brokerSession: the sessions in dial order.
func (b *pipeBroker) dialed() []*brokerSession {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]*brokerSession(nil), b.sessions...)
}

// dialedAddrs returns the address of every dial so far.
//
// Returns:
//   - []string: the addresses in dial order.
func (b *pipeBroker) dialedAddrs() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return append([]string(nil), b.addrs...)
}

// connected returns the session's CONNECT fields.
//
// Returns:
//   - brokerConnect: the CONNECT fields.
func (s *brokerSession) connected() brokerConnect {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.connect
}

// received returns the publishes the session received.
//
// Returns:
//   - []brokerPublish: the publishes in arrival order.
func (s *brokerSession) received() []brokerPublish {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]brokerPublish(nil), s.publishes...)
}

// recordConnect records the CONNECT fields a service sets from its URL.
//
// Parameters:
//   - connect: the received CONNECT packet.
func (s *brokerSession) recordConnect(connect *packets.Connect) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connect = brokerConnect{
		clientID:     connect.ClientID,
		username:     connect.Username,
		password:     string(connect.Password),
		usernameFlag: connect.UsernameFlag,
		passwordFlag: connect.PasswordFlag,
		cleanStart:   connect.CleanStart,
	}
}

// recordPublish records a PUBLISH packet.
//
// Parameters:
//   - publish: the received PUBLISH packet.
func (s *brokerSession) recordPublish(publish *packets.Publish) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.publishes = append(s.publishes, brokerPublish{
		topic:   publish.Topic,
		payload: string(publish.Payload),
		qos:     publish.QoS,
		retain:  publish.Retain,
	})
}

// sawDisconnect reports whether the client sent DISCONNECT.
//
// Returns:
//   - bool: true once DISCONNECT arrived.
func (s *brokerSession) sawDisconnect() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.disconnected
}

// serve answers CONNECT, PINGREQ, QoS 1 and QoS 2 PUBLISH, and PUBREL, records
// CONNECT, PUBLISH, and DISCONNECT, and returns when the client disconnects or
// the pipe closes.
//
// A separate goroutine writes the answers, so the session keeps reading while
// an answer waits for the client. A net.Pipe has no buffer, and the client
// writes PUBREL from its reading goroutine, so an answer written inline, such
// as the PINGRESP to the client's first PINGREQ, could block both ends.
//
// Parameters:
//   - conn: the broker's end of the pipe.
//   - ackCode: the reason code of each PUBACK and PUBREC.
//   - ackReason: the reason string of each PUBACK and PUBREC.
func (s *brokerSession) serve(conn net.Conn, ackCode byte, ackReason string) {
	replies := make(chan io.WriterTo, replyBuffer)

	defer close(replies)
	defer func() { _ = conn.Close() }()

	go writeReplies(conn, replies)

	for {
		packet, err := packets.ReadPacket(conn)
		if err != nil {
			return
		}

		switch content := packet.Content.(type) {
		case *packets.Connect:
			s.recordConnect(content)

			replies <- packets.NewControlPacket(packets.CONNACK)
		case *packets.Pingreq:
			replies <- packets.NewControlPacket(packets.PINGRESP)
		case *packets.Publish:
			s.recordPublish(content)

			if reply := acknowledgement(content, ackCode, ackReason); reply != nil {
				replies <- reply
			}
		case *packets.Pubrel:
			replies <- &packets.Pubcomp{PacketID: content.PacketID, Properties: &packets.Properties{}}
		case *packets.Disconnect:
			s.mu.Lock()
			s.disconnected = true
			s.mu.Unlock()

			return
		default:
		}
	}
}
