package smtp

import (
	"context"
	"net"
	"strconv"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("TLS and live SMTP dialog", func() {
	ginkgo.BeforeEach(func() {
		service = &Service{}
	})

	ginkgo.It("should complete implicit TLS when certificate verification is skipped", func() {
		mock := newMockSMTP()
		mock.implicitTLS = true

		address, stop := startMockSMTP(mock)
		defer stop()

		serviceURL := testutils.URLMust(mockSMTPURL(address,
			"encryption=ImplicitTLS&usestarttls=no&skiptlsverify=yes"))
		gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
		gomega.Expect(service.Send("hello", nil)).To(gomega.Succeed())
		gomega.Eventually(mock.firstByte).Should(gomega.Receive(gomega.Equal(byte(0x16))))
		gomega.Expect(mock.dataString()).To(gomega.ContainSubstring("hello"))
	})

	ginkgo.It("should reject implicit TLS with an untrusted certificate", func() {
		mock := newMockSMTP()
		mock.implicitTLS = true

		address, stop := startMockSMTP(mock)
		defer stop()

		serviceURL := testutils.URLMust(mockSMTPURL(address,
			"encryption=ImplicitTLS&usestarttls=no&skiptlsverify=no"))
		gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
		gomega.Expect(service.Send("hello", nil)).To(matchFailure(FailGetSMTPClient))
	})

	ginkgo.It("should apply skiptlsverify as a send param on implicit TLS", func() {
		mock := newMockSMTP()
		mock.implicitTLS = true

		address, stop := startMockSMTP(mock)
		defer stop()

		serviceURL := testutils.URLMust(mockSMTPURL(address,
			"encryption=ImplicitTLS&usestarttls=no&skiptlsverify=no"))
		gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
		gomega.Expect(service.Send("hello", &types.Params{"skiptlsverify": "yes"})).To(gomega.Succeed())
	})

	ginkgo.It("should upgrade with STARTTLS when certificate verification is skipped", func() {
		mock := newMockSMTP()
		mock.advertiseStartTLS = true
		mock.acceptStartTLS = true

		address, stop := startMockSMTP(mock)
		defer stop()

		serviceURL := testutils.URLMust(mockSMTPURL(address,
			"encryption=ExplicitTLS&usestarttls=yes&skiptlsverify=yes"))
		gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
		gomega.Expect(service.Send("hello", nil)).To(gomega.Succeed())
		gomega.Eventually(mock.firstByte).Should(gomega.Receive(gomega.Equal(byte('E'))))
		gomega.Expect(mock.commandsCopy()).To(gomega.ContainElement("STARTTLS"))
	})

	ginkgo.It("should fail STARTTLS when the certificate is untrusted", func() {
		mock := newMockSMTP()
		mock.advertiseStartTLS = true
		mock.acceptStartTLS = true

		address, stop := startMockSMTP(mock)
		defer stop()

		serviceURL := testutils.URLMust(mockSMTPURL(address,
			"encryption=ExplicitTLS&usestarttls=yes&skiptlsverify=no"))
		gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
		gomega.Expect(service.Send("hello", nil)).To(matchFailure(FailEnableStartTLS))
	})

	ginkgo.It("should authenticate after a successful STARTTLS upgrade", func() {
		mock := newMockSMTP()
		mock.advertiseStartTLS = true
		mock.acceptStartTLS = true
		mock.authMechs = "LOGIN PLAIN"

		address, stop := startMockSMTP(mock)
		defer stop()

		serviceURL := testutils.URLMust(
			"smtp://user:password@" + address +
				"/?auth=Login&encryption=ExplicitTLS&usestarttls=yes&skiptlsverify=yes" +
				"&fromAddress=sender@example.com&toAddresses=rec1@example.com&timeout=5s",
		)
		gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
		gomega.Expect(service.Send("hello", nil)).To(gomega.Succeed())

		commands := mock.commandsCopy()
		gomega.Expect(commands).To(gomega.ContainElement("STARTTLS"))
		gomega.Expect(commands).To(gomega.ContainElement(gomega.HavePrefix("AUTH ")))
	})

	ginkgo.It("should allow the mock SMTP stop function to be called twice", func() {
		mock := newMockSMTP()
		_, stop := startMockSMTP(mock)
		stop()
		stop()
	})

	ginkgo.It("should fail when the server does not send a 220 greeting", func() {
		mock := newMockSMTP()
		mock.failGreeting = true

		address, stop := startMockSMTP(mock)
		defer stop()

		host, portStr, err := net.SplitHostPort(address)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		port, err := strconv.ParseUint(portStr, 10, 16)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		_, err = dialClient(context.Background(), &Config{
			Host: host,
			Port: uint16(port),
		})
		gomega.Expect(err).To(matchFailure(FailCreateSMTPClient))
	})

	ginkgo.It("should fail when the DATA stream is closed after 354", func() {
		mock := newMockSMTP()
		mock.closeAfter354 = true

		address, stop := startMockSMTP(mock)
		defer stop()

		serviceURL := testutils.URLMust(mockSMTPURL(address, "encryption=None&usestarttls=no"))
		gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
		err := service.Send("hello", nil)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err).To(matchFailure(FailSendRecipient))
	})

	ginkgo.It("should time out when the server hangs after AUTH", func() {
		mock := newMockSMTP()
		mock.authMechs = "PLAIN"
		mock.hangAfter = "auth"

		address, stop := startMockSMTP(mock)
		defer stop()

		sendTimeout := 200 * time.Millisecond
		serviceURL := testutils.URLMust(
			"smtp://user:password@" + address +
				"/?auth=Plain&encryption=None&usestarttls=no" +
				"&fromAddress=sender@example.com&toAddresses=rec1@example.com&timeout=" +
				sendTimeout.String(),
		)
		gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

		start := time.Now()
		err := service.Send("hello", nil)

		gomega.Expect(time.Since(start)).To(gomega.BeNumerically("<", sendTimeout+2*time.Second))
		gomega.Expect(err).To(gomega.HaveOccurred())
	})
})
