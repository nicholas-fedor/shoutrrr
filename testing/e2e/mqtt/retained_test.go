package e2e_test

import (
	"net/url"
	"os"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/mqtt"
)

var _ = ginkgo.Describe("MQTT E2E Retained Message Test", func() {
	ginkgo.When("testing retained message functionality", func() {
		ginkgo.It("should send non-retained message by default", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping retained message test")

				return
			}

			// Add credentials from environment variables if set
			envURL = addCredentialsToURL(envURL)

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Send a non-retained message
			err = service.Send("E2E Test: Non-retained message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send retained message when configured", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping retained message test")

				return
			}

			// Add credentials from environment variables if set
			envURL = addCredentialsToURL(envURL)

			// Add retained parameter to URL
			if strings.Contains(envURL, "?") {
				envURL += "&retained=true"
			} else {
				envURL += "?retained=true"
			}

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Send a retained message
			err = service.Send("E2E Test: Retained message content", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send multiple retained messages to same topic", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping retained message test")

				return
			}

			// Add credentials from environment variables if set
			envURL = addCredentialsToURL(envURL)

			// Add retained parameter to URL
			if strings.Contains(envURL, "?") {
				envURL += "&retained=true"
			} else {
				envURL += "?retained=true"
			}

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Send first retained message
			err = service.Send("E2E Test: First retained message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Send second retained message (should replace the first)
			err = service.Send("E2E Test: Second retained message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})
})
