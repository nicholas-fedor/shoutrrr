package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
)

var _ = ginkgo.Describe("MQTT E2E Topic Test", func() {
	ginkgo.When("testing different topic formats", func() {
		ginkgo.It("should send to single-level topic", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping topic test")

				return
			}

			// Parse the base URL
			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Change topic to single level
			newURL := serviceURL.Scheme + "://" + serviceURL.Host + "/test/single"

			topicURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(topicURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Single-level topic", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send to nested topic", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping topic test")

				return
			}

			// Parse the base URL
			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Change topic to nested levels
			newURL := serviceURL.Scheme + "://" + serviceURL.Host + "/home/alerts/notifications"

			topicURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(topicURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Nested topic message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send to wildcard topic base", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping topic test")

				return
			}

			// Parse the base URL
			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Change topic to multi-level wildcard base
			newURL := serviceURL.Scheme + "://" + serviceURL.Host + "/sensors/+/temperature"

			topicURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(topicURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Wildcard topic base", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send to multi-level wildcard topic", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping topic test")

				return
			}

			// Parse the base URL
			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Change topic to multi-level wildcard
			newURL := serviceURL.Scheme + "://" + serviceURL.Host + "/home/#"

			topicURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(topicURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Multi-level wildcard topic", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send to topic with special characters", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping topic test")

				return
			}

			// Parse the base URL
			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Change topic with special characters (encoded)
			newURL := serviceURL.Scheme + "://" + serviceURL.Host + "/sensor%2Fdevice%231/temperature"

			topicURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(topicURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Topic with special characters", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
