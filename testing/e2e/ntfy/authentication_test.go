package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
)

var _ = ginkgo.Describe("ntfy E2E Authentication Tests", func() {
	ginkgo.When("running e2e tests against a real ntfy server", func() {
		ginkgo.BeforeEach(func() {
			if !isNtfyServerAvailable() {
				ginkgo.Skip("ntfy server not available, skipping e2e tests")
			}
		})

		ginkgo.It("should send a message with basic authentication", func() {
			username := os.Getenv("SHOUTRRR_NTFY_USERNAME")
			password := os.Getenv("SHOUTRRR_NTFY_PASSWORD")

			if username == "" || password == "" {
				ginkgo.Skip("ntfy credentials not configured, skipping auth test")
			}

			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Authenticated message"

			err = service.Send(message, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with username-only authentication", func() {
			username := os.Getenv("SHOUTRRR_NTFY_USERNAME")
			password := os.Getenv("SHOUTRRR_NTFY_PASSWORD")

			if username == "" || password != "" {
				ginkgo.Skip("username-only auth not configured, skipping test")
			}

			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Username-only auth message"

			err = service.Send(message, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})
	})

	ginkgo.When("no server is configured", func() {
		ginkgo.It("should return the correct service ID", func() {
			service := &ntfy.Service{}
			gomega.Expect(service.GetID()).To(gomega.Equal("ntfy"))
		})
	})
})
