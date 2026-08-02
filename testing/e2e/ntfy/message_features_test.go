package e2e_test

import (
	"net/url"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
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
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Click action message"

			err = service.Send(message, &types.Params{
				"click": "https://example.com",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with an attachment URL", func() {
			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Attachment message"

			err = service.Send(message, &types.Params{
				"attach": "https://example.com/image.png",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with actions", func() {
			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Actions message"

			err = service.Send(message, &types.Params{
				"actions": "action=view,label=Open,url=https://example.com",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with delay", func() {
			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			message := "E2E Test: Delayed message"

		err = service.Send(message, &types.Params{
			"delay": "10s",
		})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// Delayed messages may not appear immediately, so we just verify no error
			ginkgo.GinkgoWriter.Write([]byte("Delayed message sent successfully\n"))
		})

		ginkgo.It("should send a message with email notification", func() {
			ginkgo.Skip("email notifications require server-side configuration")

			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Email notification"

			err = service.Send(message, &types.Params{
				"email": "test@example.com",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with icon", func() {
			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Icon message"

			err = service.Send(message, &types.Params{
				"icon": "https://example.com/icon.png",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with filename", func() {
			ginkgo.Skip("attachments require server-side configuration")

			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Filename message"

			err = service.Send(message, &types.Params{
				"filename": "report.pdf",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message with multiple tags", func() {
			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Multiple tags message"

			err = service.Send(message, &types.Params{
				"tags": "warning,skull,fire",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceivedWithTags(topic, message, []string{"warning", "skull", "fire"})
		})

		ginkgo.It("should send a message with combined params", func() {
			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			topic := service.Config.Topic
			message := "E2E Test: Combined params message"

			err = service.Send(message, &types.Params{
				"title":    "Combined",
				"priority": "5",
				"tags":     "warning,skull",
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

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
