package e2e_test

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("ntfy E2E Message Features Tests", func() {
	ginkgo.When("running e2e tests against a real ntfy server", func() {
		ginkgo.BeforeEach(func() {
			if !isNtfyServerAvailable() {
				ginkgo.Skip("ntfy server not available, skipping e2e tests")
			}
		})

		ginkgo.It("should send a message with a click action", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Click action message"

			gomega.Expect(service.Send(message, &types.Params{
				"click": "https://example.com",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with an attachment URL", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Attachment message"

			gomega.Expect(service.Send(message, &types.Params{
				"attach": "https://example.com/image.png",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with actions", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Actions message"

			gomega.Expect(service.Send(message, &types.Params{
				"actions": "action=view,label=Open,url=https://example.com",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with delay", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			message := "E2E Test: Delayed message"

			gomega.Expect(service.Send(message, &types.Params{
				"delay": "10s",
			})).NotTo(gomega.HaveOccurred())

			// Delayed messages may not appear immediately, so we just verify no error
			ginkgo.GinkgoWriter.Write([]byte("Delayed message sent successfully\n"))
		})

		ginkgo.It("should send a message with email notification", func() {
			ginkgo.Skip("email notifications require server-side configuration")

			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Email notification"

			gomega.Expect(service.Send(message, &types.Params{
				"email": "test@example.com",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with icon", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Icon message"

			gomega.Expect(service.Send(message, &types.Params{
				"icon": "https://example.com/icon.png",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with filename", func() {
			ginkgo.Skip("attachments require server-side configuration")

			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Filename message"

			gomega.Expect(service.Send(message, &types.Params{
				"filename": "report.pdf",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with multiple tags", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Multiple tags message"

			gomega.Expect(service.Send(message, &types.Params{
				"tags": "warning,skull,fire",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceivedWithTags(topic, message, []string{"warning", "skull", "fire"})
		})

		ginkgo.It("should send a message with combined params", func() {
			serviceURLStr := buildServiceURL()
			service := initializeService(serviceURLStr)

			topic := service.Config.Topic
			message := "E2E Test: Combined params message"

			gomega.Expect(service.Send(message, &types.Params{
				"title":    "Combined",
				"priority": "5",
				"tags":     "warning,skull",
			})).NotTo(gomega.HaveOccurred())

			verifyMessageReceivedWithTitle(topic, message, "Combined")
		})
	})

	ginkgo.When("no server is configured", func() {
		ginkgo.It("should return the correct service ID", func() {
			service := &ntfy.Service{}
			gomega.Expect(service.GetID()).To(gomega.Equal("ntfy"))
		})
	})
})
