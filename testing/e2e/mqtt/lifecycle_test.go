package e2e_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr"
	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
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

var _ = ginkgo.Describe("MQTT E2E Connection Lifecycle Test", func() {
	ginkgo.It("should deliver messages sent more than 10 seconds apart through one service", func() {
		envURL := os.Getenv("SHOUTRRR_MQTT_URL")
		if envURL == "" {
			ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping connection lifecycle test")

			return
		}

		serviceURL, err := url.Parse(addTLSParam(addCredentialsToURL(envURL)))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		serviceURL.Path = fmt.Sprintf("/shoutrrr/e2e/lifecycle/%d", time.Now().UnixNano())

		received := subscribe(serviceURL)

		service := &mqtt.Service{}
		gomega.Expect(service.Initialize(serviceURL, testutils.TestLogger())).To(gomega.Succeed())

		ginkgo.DeferCleanup(service.Close)

		gomega.Expect(service.Send("E2E Test: first lifecycle message", nil)).To(gomega.Succeed())
		gomega.Eventually(received, deliveryTimeout).
			Should(gomega.Receive(gomega.Equal("E2E Test: first lifecycle message")))

		time.Sleep(sendInterval)

		gomega.Expect(service.Send("E2E Test: second lifecycle message", nil)).To(gomega.Succeed())
		gomega.Eventually(received, deliveryTimeout).
			Should(gomega.Receive(gomega.Equal("E2E Test: second lifecycle message")))
	})
})

var _ = ginkgo.Describe("MQTT E2E One-Shot Send Test", func() {
	ginkgo.It("should deliver a root Send and disconnect afterwards", func() {
		envURL := os.Getenv("SHOUTRRR_MQTT_URL")
		if envURL == "" {
			ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping one-shot send test")

			return
		}

		serviceURL, err := url.Parse(addTLSParam(addCredentialsToURL(envURL)))
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		run := time.Now().UnixNano()
		clientID := fmt.Sprintf("shoutrrr-e2e-oneshot-%d", run)
		serviceURL.Path = fmt.Sprintf("/shoutrrr/e2e/oneshot/%d", run)

		query := serviceURL.Query()
		query.Set("clientid", clientID)
		serviceURL.RawQuery = query.Encode()

		received := subscribe(serviceURL)

		gomega.Expect(shoutrrr.Send(serviceURL.String(), "E2E Test: one-shot message")).To(gomega.Succeed())
		gomega.Eventually(received, deliveryTimeout).
			Should(gomega.Receive(gomega.Equal("E2E Test: one-shot message")))

		if _, err := clientConnected(serviceURL.Hostname(), clientID); err != nil {
			ginkgo.Skip("EMQX management API not available: " + err.Error())
		}

		// The broker records the disconnect asynchronously, so poll for it.
		gomega.Eventually(func() (bool, error) {
			return clientConnected(serviceURL.Hostname(), clientID)
		}, deliveryTimeout, 100*time.Millisecond).
			Should(gomega.BeFalse(), "Send must disconnect its one-shot client")
	})
})

// clientConnected asks the EMQX management API whether clientID is connected.
//
// Parameters:
//   - host: the broker host, which also serves the management API on port 18083.
//   - clientID: the MQTT client ID to look up.
//
// Returns:
//   - bool: true when the broker reports the client as connected.
//   - error: the failure to reach or authenticate with the management API.
func clientConnected(host, clientID string) (bool, error) {
	apiURL := "http://" + net.JoinHostPort(host, "18083") + "/api/v5"
	client := &http.Client{Timeout: deliveryTimeout}

	credentials, err := json.Marshal(map[string]string{"username": "admin", "password": "public"})
	if err != nil {
		return false, fmt.Errorf("encoding credentials: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), deliveryTimeout)
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
// The client disconnects when the spec ends.
//
// Parameters:
//   - serviceURL: the service URL whose broker, credentials, and topic are used.
//
// Returns:
//   - <-chan string: the payloads received on the topic.
func subscribe(serviceURL *url.URL) <-chan string {
	ginkgo.GinkgoHelper()

	topic := serviceURL.Path[1:]
	received := make(chan string, 4)
	subscribed := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())
	ginkgo.DeferCleanup(cancel)

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
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	ginkgo.DeferCleanup(func() {
		disconnectCtx, stop := context.WithTimeout(context.Background(), deliveryTimeout)
		defer stop()

		_ = manager.Disconnect(disconnectCtx)
	})

	gomega.Eventually(subscribed, deliveryTimeout).Should(gomega.Receive(gomega.Succeed()))

	return received
}
