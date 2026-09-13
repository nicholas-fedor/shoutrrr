package e2e_test

import (
	"net/url"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/xmpp"
)

var _ = ginkgo.Describe("XMPP E2E", func() {
	sendURL := func(rawURL, message string) {
		ginkgo.GinkgoHelper()

		serviceURL, err := url.Parse(rawURL)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		service := &xmpp.Service{}
		err = service.Initialize(serviceURL, testutils.TestLogger())
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(service.Send(message, nil)).To(gomega.Succeed())
	}

	ginkgo.It("should send a 1:1 chat over STARTTLS", func() {
		sendURL(envOrSkip("SHOUTRRR_XMPP_URL"), "E2E Test: STARTTLS chat")
	})

	ginkgo.It("should send a 1:1 chat over implicit TLS", func() {
		sendURL(envOrSkip("SHOUTRRR_XMPP_TLS_URL"), "E2E Test: implicit TLS chat")
	})

	ginkgo.It("should send a 1:1 chat over plaintext when disabletls is set", func() {
		sendURL(envOrSkip("SHOUTRRR_XMPP_PLAIN_URL"), "E2E Test: plaintext chat")
	})

	ginkgo.It("should send to an open MUC", func() {
		sendURL(envOrSkip("SHOUTRRR_XMPP_MUC_URL"), "E2E Test: open MUC")
	})

	ginkgo.It("should send to a password-protected MUC", func() {
		sendURL(envOrSkip("SHOUTRRR_XMPP_MUC_PASSWORD_URL"), "E2E Test: protected MUC")
	})

	ginkgo.It("should fail with bad credentials", func() {
		rawURL := envOrSkip("SHOUTRRR_XMPP_URL")
		serviceURL, err := url.Parse(rawURL)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		serviceURL.User = url.UserPassword("sender", "wrong-password")

		service := &xmpp.Service{}
		err = service.Initialize(serviceURL, testutils.TestLogger())
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(service.Send("should fail", nil)).NotTo(gomega.Succeed())
	})
})
