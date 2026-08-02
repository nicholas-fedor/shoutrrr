package e2e_test

import (
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

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
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Authenticated message"

			gomega.Expect(service.Send(message, nil)).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with username-only authentication", func() {
			username := os.Getenv("SHOUTRRR_NTFY_USERNAME")
			password := os.Getenv("SHOUTRRR_NTFY_PASSWORD")

			if username == "" || password != "" {
				ginkgo.Skip("username-only auth not configured, skipping test")
			}

			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Username-only auth message"

			gomega.Expect(service.Send(message, nil)).NotTo(gomega.HaveOccurred())

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
