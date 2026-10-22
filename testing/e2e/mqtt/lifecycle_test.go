package e2e_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
)

// Timing for the connection lifecycle e2e test.
const (
	// sendInterval separates the two sends by more than the 10 second publish timeout.
	sendInterval = 11 * time.Second
	// deliveryTimeout bounds how long the subscriber waits for each message.
	deliveryTimeout = 5 * time.Second
)

// errNoAPIToken reports that the EMQX management API returned no login token.
var errNoAPIToken = errors.New("no management API token")

// TestMQTTConnectionLifecycle verifies that one service delivers messages sent
// more than 10 seconds apart, and that a one-shot root Send delivers its
// message and then disconnects its client, as a subscriber and the EMQX
// management API observe.
//
//nolint:paralleltest // Publishes to a live broker one message at a time.
func TestMQTTConnectionLifecycle(t *testing.T) {
	t.Run("messages sent more than 10 seconds apart through one service", func(t *testing.T) {
		serviceURL, err := url.Parse(addTLSParam(addCredentialsToURL(brokerURL(t))))
		require.NoError(t, err)

		serviceURL.Path = fmt.Sprintf("/shoutrrr/e2e/lifecycle/%d", time.Now().UnixNano())

		received := subscribe(t, serviceURL)

		service := &mqtt.Service{}
		require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "[Test] ", 0)))

		t.Cleanup(func() { _ = service.Close() })

		require.NoError(t, service.Send("E2E Test: first lifecycle message", nil))
		waitForPayload(t, received, "E2E Test: first lifecycle message")

		time.Sleep(sendInterval)

		require.NoError(t, service.Send("E2E Test: second lifecycle message", nil))
		waitForPayload(t, received, "E2E Test: second lifecycle message")
	})

	t.Run("root Send delivers and disconnects afterwards", func(t *testing.T) {
		serviceURL, err := url.Parse(addTLSParam(addCredentialsToURL(brokerURL(t))))
		require.NoError(t, err)

		run := time.Now().UnixNano()
		clientID := fmt.Sprintf("shoutrrr-e2e-oneshot-%d", run)
		serviceURL.Path = fmt.Sprintf("/shoutrrr/e2e/oneshot/%d", run)

		query := serviceURL.Query()
		query.Set("clientid", clientID)
		serviceURL.RawQuery = query.Encode()

		// Check the management API before sending, so the test skips without
		// delivering anything when the API is not reachable.
		if _, err := clientConnected(t.Context(), serviceURL.Hostname(), clientID); err != nil {
			t.Skip("EMQX management API not available: " + err.Error())
		}

		received := subscribe(t, serviceURL)

		require.NoError(t, shoutrrr.Send(serviceURL.String(), "E2E Test: one-shot message"))
		waitForPayload(t, received, "E2E Test: one-shot message")

		// The broker records the disconnect asynchronously, so poll for it. A lookup
		// failure here fails the assertion, since the API was reachable before the send.
		require.Eventually(t, func() bool {
			connected, err := clientConnected(t.Context(), serviceURL.Hostname(), clientID)

			return err == nil && !connected
		}, deliveryTimeout, 100*time.Millisecond, "Send must disconnect its one-shot client")
	})
}

// waitForPayload requires a payload equal to want to arrive within
// [deliveryTimeout], skipping any other payloads that arrive first.
//
// Parameters:
//   - t: the test.
//   - received: the payloads the subscriber receives.
//   - want: the expected payload.
func waitForPayload(t *testing.T, received <-chan string, want string) {
	t.Helper()

	timeout := time.After(deliveryTimeout)

	for {
		select {
		case payload := <-received:
			if payload == want {
				return
			}
		case <-timeout:
			require.Failf(t, "message not received", "no payload %q within %s", want, deliveryTimeout)

			return
		}
	}
}

// clientConnected asks the EMQX management API whether clientID is connected.
//
// Parameters:
//   - ctx: cancellation for the API requests.
//   - host: the broker host, which also serves the management API on port 18083.
//   - clientID: the MQTT client ID to look up.
//
// Returns:
//   - bool: true when the broker reports the client as connected.
//   - error: the failure to reach or authenticate with the management API.
func clientConnected(ctx context.Context, host, clientID string) (bool, error) {
	apiURL := "http://" + net.JoinHostPort(host, "18083") + "/api/v5"
	client := &http.Client{Timeout: deliveryTimeout}

	credentials, err := json.Marshal(map[string]string{"username": "admin", "password": "public"})
	if err != nil {
		return false, fmt.Errorf("encoding credentials: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()

	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/login", bytes.NewReader(credentials))
	if err != nil {
		return false, fmt.Errorf("creating login request: %w", err)
	}

	loginReq.Header.Set("Content-Type", "application/json")

	loginRes, err := client.Do(loginReq)
	if err != nil {
		return false, fmt.Errorf("logging in: %w", err)
	}
	defer func() { _ = loginRes.Body.Close() }()

	var login struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(loginRes.Body).Decode(&login); err != nil || login.Token == "" {
		return false, fmt.Errorf("%w: login status %s", errNoAPIToken, loginRes.Status)
	}

	clientReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/clients/"+url.PathEscape(clientID), http.NoBody)
	if err != nil {
		return false, fmt.Errorf("creating client request: %w", err)
	}

	clientReq.Header.Set("Authorization", "Bearer "+login.Token)

	clientRes, err := client.Do(clientReq)
	if err != nil {
		return false, fmt.Errorf("looking up client: %w", err)
	}
	defer func() { _ = clientRes.Body.Close() }()

	if clientRes.StatusCode == http.StatusNotFound {
		return false, nil
	}

	var info struct {
		Connected bool `json:"connected"`
	}

	if err := json.NewDecoder(clientRes.Body).Decode(&info); err != nil {
		return false, fmt.Errorf("decoding client: %w", err)
	}

	return info.Connected, nil
}

// subscribe connects a separate client to the broker in serviceURL, subscribes to
// its topic, and returns a channel that receives each payload published there.
// The client disconnects when the test ends.
//
// Parameters:
//   - t: the test.
//   - serviceURL: the service URL whose broker, credentials, and topic are used.
//
// Returns:
//   - <-chan string: the payloads received on the topic.
func subscribe(t *testing.T, serviceURL *url.URL) <-chan string {
	t.Helper()

	topic := serviceURL.Path[1:]
	received := make(chan string, 4)
	subscribed := make(chan error, 1)

	// The test's context ends before its cleanups run, so the client keeps a
	// context of its own until the cleanup below has disconnected it.
	ctx, cancel := context.WithCancel(context.WithoutCancel(t.Context()))
	t.Cleanup(cancel)

	brokerURL := &url.URL{Scheme: serviceURL.Scheme, Host: serviceURL.Host}
	password, _ := serviceURL.User.Password()

	//nolint:exhaustruct // autopaho.ClientConfig has many optional fields with library defaults
	config := autopaho.ClientConfig{
		ServerUrls:      []*url.URL{brokerURL},
		KeepAlive:       20,
		ConnectUsername: serviceURL.User.Username(),
		ConnectPassword: []byte(password),
		OnConnectionUp: func(manager *autopaho.ConnectionManager, _ *paho.Connack) {
			//nolint:exhaustruct // Optional subscription properties use broker defaults
			_, err := manager.Subscribe(ctx, &paho.Subscribe{
				Subscriptions: []paho.SubscribeOptions{{Topic: topic, QoS: 1}},
			})
			subscribed <- err
		},
		ClientID: fmt.Sprintf("shoutrrr-e2e-subscriber-%d", time.Now().UnixNano()),
		OnPublishReceived: []func(paho.PublishReceived) (bool, error){
			func(publish paho.PublishReceived) (bool, error) {
				received <- string(publish.Packet.Payload)

				return true, nil
			},
		},
	}

	if serviceURL.Scheme == mqtt.SchemeTLS {
		config.TlsCfg = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: os.Getenv("SHOUTRRR_MQTT_TLS_SKIP_VERIFY") == envValueTrue,
		}
	}

	manager, err := autopaho.NewConnection(ctx, config)
	require.NoError(t, err)

	t.Cleanup(func() {
		disconnectCtx, stop := context.WithTimeout(context.WithoutCancel(t.Context()), deliveryTimeout)
		defer stop()

		_ = manager.Disconnect(disconnectCtx)
	})

	select {
	case err := <-subscribed:
		require.NoError(t, err)
	case <-time.After(deliveryTimeout):
		require.Fail(t, "subscription not confirmed", "no SUBACK within %s", deliveryTimeout)
	}

	return received
}
