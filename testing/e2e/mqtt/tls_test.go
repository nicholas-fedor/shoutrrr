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

// envValueTrue is the string value for boolean true in environment variables.
const envValueTrue = "true"

var _ = ginkgo.Describe("MQTT E2E TLS Test", func() {
	ginkgo.When("testing TLS connections", func() {
		// Helper function to add TLS skip verify parameter to URL if enabled
		addTLSParam := func(rawURL string) string {
			tlsSkipVerify := os.Getenv("SHOUTRRR_MQTT_TLS_SKIP_VERIFY")
			if tlsSkipVerify == envValueTrue {
				if strings.Contains(rawURL, "?") {
					return rawURL + "&disabletlsverification=yes"
				}

				return rawURL + "?disabletlsverification=yes"
			}

			return rawURL
		}

		ginkgo.It("should connect using mqtts:// scheme", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_TLS_URL")
			if envURL == "" {
				// Try to construct from the base URL by changing scheme
				envURL = os.Getenv("SHOUTRRR_MQTT_URL")
				if envURL == "" {
					ginkgo.Skip(
						"SHOUTRRR_MQTT_URL or SHOUTRRR_MQTT_TLS_URL not set, skipping TLS test",
					)

					return
				}

				// Replace mqtt:// with mqtts://
				envURL = strings.Replace(envURL, "mqtt://", "mqtts://", 1)
			}

			// Add TLS skip verify param if enabled (for self-signed certificates)
			envURL = addTLSParam(envURL)

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: TLS message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should connect using mqtts:// with custom port", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping TLS test")

				return
			}

			// Parse the URL and change the port to 8883
			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Change scheme to mqtts and port to 8883
			scheme := "mqtts"
			port := "8883"

			// Build new URL
			newURL := scheme + "://" + serviceURL.Hostname() + ":" + port + serviceURL.Path

			// Add TLS skip verify param if enabled
			newURL = addTLSParam(newURL)

			if serviceURL.RawQuery != "" {
				newURL += "?" + serviceURL.RawQuery
			}

			tlsURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(tlsURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: TLS on port 8883", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should fail when connecting with TLS to non-TLS port", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping TLS test")

				return
			}

			// Skip this test if TLS verification is disabled
			// When TLS verification is skipped, the connection may succeed even on non-TLS port
			if os.Getenv("SHOUTRRR_MQTT_TLS_SKIP_VERIFY") == envValueTrue {
				ginkgo.Skip("Skipping TLS port mismatch test when TLS verification is disabled")

				return
			}

			// Parse the URL and change the scheme to mqtts
			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Change scheme to mqtts but keep the standard MQTT port (1883)
			// This should fail since port 1883 typically doesn't support TLS
			scheme := "mqtts"
			port := "1883"

			newURL := scheme + "://" + serviceURL.Hostname() + ":" + port + serviceURL.Path

			// Add TLS skip verify param if enabled
			newURL = addTLSParam(newURL)

			if serviceURL.RawQuery != "" {
				newURL += "?" + serviceURL.RawQuery
			}

			tlsURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(tlsURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// This should fail because we're trying TLS on a non-TLS port
			err = service.Send("E2E Test: Should fail", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())

			_ = service.Close()
		})
	})
})
