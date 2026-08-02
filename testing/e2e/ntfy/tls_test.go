package e2e_test

import (
	"net/url"
	"os"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/push/ntfy"
)

var _ = ginkgo.Describe("ntfy E2E TLS Tests", func() {
	ginkgo.When("running e2e tests against a real ntfy server", func() {
		ginkgo.BeforeEach(func() {
			if !isNtfyServerAvailable() {
				ginkgo.Skip("ntfy server not available, skipping e2e tests")
			}
		})

		ginkgo.It("should send a message over HTTP when DisableTLS is set", func() {
			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(service.Config.DisableTLS).To(gomega.BeTrue())
			gomega.Expect(service.Config.Scheme).To(gomega.Equal("http"))

			topic := service.Config.Topic
			message := "E2E Test: HTTP message"

			err = service.Send(message, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			verifyMessageReceived(topic, message)
		})

		ginkgo.It("should send a message over HTTPS by default", func() {
			disableTLS := os.Getenv("SHOUTRRR_NTFY_DISABLE_TLS")
			if disableTLS == "true" {
				ginkgo.Skip("HTTPS test skipped because DisableTLS is set")
			}

			serviceURLStr := buildServiceURL()
			serviceURL, err := url.Parse(serviceURLStr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			service := &ntfy.Service{}
			err = service.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Expect(service.Config.DisableTLS).To(gomega.BeFalse())
			gomega.Expect(service.Config.Scheme).To(gomega.Equal("https"))

			topic := service.Config.Topic
			message := "E2E Test: HTTPS message"

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
