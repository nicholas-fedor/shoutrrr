package e2e_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

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
