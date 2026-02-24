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

var _ = ginkgo.Describe("MQTT E2E Authentication Test", func() {
	ginkgo.When("testing authentication", func() {
		ginkgo.It("should send message without authentication", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping auth test")

				return
			}

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: No auth message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should send message with username and password", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping auth test")

				return
			}

			username := os.Getenv("SHOUTRRR_MQTT_USERNAME")
			password := os.Getenv("SHOUTRRR_MQTT_PASSWORD")

			// Skip if credentials are not provided
			if username == "" || password == "" {
				ginkgo.Skip(
					"SHOUTRRR_MQTT_USERNAME and/or SHOUTRRR_MQTT_PASSWORD not set, skipping authenticated test",
				)

				return
			}

			// Add credentials to URL
			// Parse the URL and add username/password to the host
			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Reconstruct URL with credentials
			host := serviceURL.Host
			if strings.Contains(host, ":") {
				// Has port, need to handle differently
				host = username + ":" + password + "@" + host
			} else {
				host = username + ":" + password + "@" + host
			}

			// Build new URL with credentials
			newURL := serviceURL.Scheme + "://" + host + serviceURL.Path
			if serviceURL.RawQuery != "" {
				newURL += "?" + serviceURL.RawQuery
			}

			credURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(credURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Send("E2E Test: Authenticated message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = service.Close()
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should fail with wrong credentials", func() {
			envURL := os.Getenv("SHOUTRRR_MQTT_URL")
			if envURL == "" {
				ginkgo.Skip("SHOUTRRR_MQTT_URL not set, skipping auth test")

				return
			}

			// Skip this test if anonymous access is allowed
			// When SHOUTRRR_MQTT_AUTH_REQUIRED is not "true", the broker allows anonymous access
			// and wrong credentials will still work
			if os.Getenv("SHOUTRRR_MQTT_AUTH_REQUIRED") != "true" {
				ginkgo.Skip("Skipping auth failure test when anonymous access is allowed")

				return
			}

			username := os.Getenv("SHOUTRRR_MQTT_USERNAME")
			password := os.Getenv("SHOUTRRR_MQTT_PASSWORD")

			// Skip if credentials are not provided
			if username == "" || password == "" {
				ginkgo.Skip(
					"SHOUTRRR_MQTT_USERNAME and/or SHOUTRRR_MQTT_PASSWORD not set, skipping auth failure test",
				)

				return
			}

			// Use wrong credentials
			wrongPassword := "wrong_password_" + password

			serviceURL, err := url.Parse(envURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Reconstruct URL with wrong credentials
			host := serviceURL.Host
			host = username + ":" + wrongPassword + "@" + host

			newURL := serviceURL.Scheme + "://" + host + serviceURL.Path
			if serviceURL.RawQuery != "" {
				newURL += "?" + serviceURL.RawQuery
			}

			credURL, err := url.Parse(newURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &mqtt.Service{}
			err = service.Initialize(credURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// This should fail with authentication error
			err = service.Send("E2E Test: Should fail", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())

			_ = service.Close()
		})
	})
})
