package smtp

import (
	"bytes"
	"log"
	"net/smtp"
	"net/url"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

var _ = ginkgo.Describe("Service", func() {
	ginkgo.BeforeEach(func() {
		service = &Service{}
	})

	ginkgo.Describe("GetID", func() {
		ginkgo.It("should return smtp", func() {
			gomega.Expect(service.GetID()).To(gomega.Equal(Scheme))
		})
	})

	ginkgo.Describe("Initialize", func() {
		ginkgo.It("should apply the default SMTP port and timeout when omitted", func() {
			mockLogger := typesmocks.NewMockStdLogger(ginkgo.GinkgoT())
			serviceURL := testutils.URLMust(
				"smtp://example.com/?fromAddress=sender@example.com&toAddresses=rec1@example.com",
			)

			gomega.Expect(service.Initialize(serviceURL, mockLogger)).To(gomega.Succeed())
			gomega.Expect(service.Config.Port).To(gomega.Equal(uint16(DefaultSMTPPort)))
			gomega.Expect(service.Config.Timeout).To(gomega.Equal(defaultTimeout))
		})

		ginkgo.It("should infer AuthPlain when a username is present", func() {
			serviceURL := testutils.URLMust(
				"smtp://user:pass@example.com/?fromAddress=sender@example.com&toAddresses=rec1@example.com",
			)

			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Config.Auth).To(gomega.Equal(AuthTypes.Plain))
		})

		ginkgo.It("should infer AuthNone when no username is present", func() {
			serviceURL := testutils.URLMust(
				"smtp://example.com/?fromAddress=sender@example.com&toAddresses=rec1@example.com",
			)

			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Config.Auth).To(gomega.Equal(AuthTypes.None))
		})

		ginkgo.It("should keep an explicit auth method", func() {
			serviceURL := testutils.URLMust(
				"smtp://user:pass@example.com/?auth=Login&fromAddress=sender@example.com&toAddresses=rec1@example.com",
			)

			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Config.Auth).To(gomega.Equal(AuthTypes.Login))
		})

		ginkgo.It("should use the specified timeout", func() {
			serviceURL := testutils.URLMust(modifyURL(baseNoAuthURL, map[string]string{"timeout": "5s"}))

			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Config.Timeout).To(gomega.Equal(5 * time.Second))
		})

		ginkgo.It("should fail when the configuration URL is invalid", func() {
			serviceURL := testutils.URLMust("smtp://example.com/")

			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.MatchError(ErrFromAddressMissing))
		})

		ginkgo.It("should keep the default port when the URL port is out of range", func() {
			serviceURL := testutils.URLMust(
				"smtp://example.com:99999/?fromAddress=sender@example.com&toAddresses=rec1@example.com",
			)

			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Config.Port).To(gomega.Equal(uint16(DefaultSMTPPort)))
		})
	})

	ginkgo.Describe("Send", func() {
		ginkgo.It("should use the default timeout when Timeout is non-positive", func() {
			gomega.Expect(effectiveTimeout(0)).To(gomega.Equal(defaultTimeout))
			gomega.Expect(effectiveTimeout(-time.Second)).To(gomega.Equal(defaultTimeout))
			gomega.Expect(effectiveTimeout(5 * time.Second)).To(gomega.Equal(5 * time.Second))
		})

		ginkgo.It("should fail when the service is not configured with a reachable host", func() {
			svc := Service{Config: &Config{Host: "127.0.0.1", Port: 1, Timeout: time.Millisecond}}
			gomega.Expect(svc.Send("test message", nil)).To(matchFailure(FailGetSMTPClient))

			svc.Config.Encryption = EncMethods.ImplicitTLS
			gomega.Expect(svc.Send("test message", nil)).To(matchFailure(FailGetSMTPClient))
		})

		ginkgo.It("should fail when an invalid param is passed", func() {
			svc := Service{Config: &Config{}}
			gomega.Expect(svc.Send("test message", &types.Params{"invalid": "value"})).
				To(matchFailure(FailApplySendParams))
		})

		ginkgo.It("should log a warning when SkipTLSVerify is enabled", func() {
			var buf bytes.Buffer

			testLogger := log.New(&buf, "", 0)
			svc := &Service{}
			gomega.Expect(svc.Initialize(testutils.URLMust(baseNoAuthURL), testLogger)).To(gomega.Succeed())
			svc.Config.SkipTLSVerify = true
			svc.Config.Host = "127.0.0.1"
			svc.Config.Port = 1
			svc.Config.Timeout = time.Millisecond

			gomega.Expect(svc.Send("test message", nil)).To(matchFailure(FailGetSMTPClient))
			gomega.Expect(buf.String()).
				To(gomega.ContainSubstring("Warning: TLS verification is disabled, making connections insecure"))
		})

		ginkgo.It("should apply send params before dialing", func() {
			address, firstByte, stop := startGreetingServer()
			defer stop()

			serviceURL := testutils.URLMust(
				"smtp://" + address + "/?encryption=ImplicitTLS&auth=none&usestarttls=no" +
					"&fromAddress=sender@example.com&toAddresses=rec1@example.com&timeout=5s",
			)
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

			gomega.Expect(service.Send("test message", &types.Params{"encryption": "None"})).
				To(matchFailure(FailHandshake))
			gomega.Eventually(firstByte).Should(gomega.Receive(gomega.Equal(byte('E'))))
		})

		ginkgo.It("should time out the handshake using the configured deadline", func() {
			address, stop := startHangingGreetingServer()
			defer stop()

			sendTimeout := 200 * time.Millisecond
			serviceURL := testutils.URLMust(
				"smtp://" + address + "/?auth=none&usestarttls=no" +
					"&fromAddress=sender@example.com&toAddresses=rec1@example.com&timeout=" +
					sendTimeout.String(),
			)
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())

			start := time.Now()
			err := service.Send("test message", nil)

			gomega.Expect(time.Since(start)).To(gomega.BeNumerically("<", sendTimeout+2*time.Second))
			gomega.Expect(err).To(matchFailure(FailHandshake))
		})
	})

	ginkgo.When("SkipTLSVerify is enabled on the parsed URL", func() {
		ginkgo.It("should log a security warning when Send is called", func() {
			localService := &Service{}
			testURL := modifyURL(baseNoAuthURL, map[string]string{"skiptlsverify": "yes"})

			var buf bytes.Buffer

			testLogger := log.New(&buf, "", 0)
			gomega.Expect(localService.Initialize(testutils.URLMust(testURL), testLogger)).To(gomega.Succeed())
			localService.Config.Host = "127.0.0.1"
			localService.Config.Port = 1
			localService.Config.Timeout = time.Millisecond

			gomega.Expect(localService.Send("test message", nil)).To(matchFailure(FailGetSMTPClient))
			gomega.Expect(buf.String()).
				To(gomega.ContainSubstring("Warning: TLS verification is disabled, making connections insecure"))
		})
	})

	ginkgo.When("send params change the message content", func() {
		ginkgo.It("should build the headers from the params", func() {
			gomega.Expect(service.Initialize(testutils.URLMust(baseNoAuthURL), logger)).To(gomega.Succeed())

			config := service.Config.Clone()
			gomega.Expect(service.propKeyResolver.UpdateConfigFromParams(&config, &types.Params{
				"fromaddress": "override@example.com",
				"fromname":    "Override",
				"subject":     "Overridden",
				"usehtml":     "yes",
			})).To(gomega.Succeed())

			textCon, tcfaker := testutils.CreateTextConFaker([]string{
				"250-mx.google.com at your service",
				"250 8BITMIME",
				"250 Sender OK",
				"250 Receiver OK",
				"354 Go ahead",
				"250 Data OK",
				"221 OK",
			}, "\r\n")
			client := &smtp.Client{Text: textCon}
			fakeTLSEnabled(client, "example.com")

			gomega.Expect(runSMTPSession(service, client, &config)).To(gomega.Succeed())

			received := tcfaker.GetClientSentences()
			gomega.Expect(received).To(gomega.ContainElement("MAIL FROM:<override@example.com> BODY=8BITMIME"))
			gomega.Expect(received).To(gomega.ContainElement(`From: "Override" <override@example.com>`))
			gomega.Expect(received).To(gomega.ContainElement("Subject: Overridden"))
			gomega.Expect(received).
				To(gomega.ContainElement(gomega.HavePrefix("Content-Type: multipart/alternative; boundary=")))
		})
	})

	ginkgo.When("running E2E tests", func() {
		ginkgo.It("should work without errors", func() {
			if envSMTPURL == "" {
				ginkgo.Skip("environment not set up for E2E testing")
			}

			serviceURL, err := url.Parse(envSMTPURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Initialize(serviceURL, logger)).To(gomega.Succeed())
			gomega.Expect(service.Send("this is an integration test", nil)).To(gomega.Succeed())
		})
	})

	ginkgo.When("configuring timeout via params", func() {
		ginkgo.It("should use the specified timeout", func() {
			config := &Config{}
			resolver := format.NewPropKeyResolver(config)
			params := types.Params{"timeout": "1h2m3s"}

			gomega.Expect(resolver.UpdateConfigFromParams(config, &params)).To(gomega.Succeed())
			gomega.Expect(config.Timeout).To(gomega.Equal(1*time.Hour + 2*time.Minute + 3*time.Second))
		})

		ginkgo.It("should reject a bare non-zero timeout value", func() {
			config := &Config{Timeout: 5 * time.Second}
			resolver := format.NewPropKeyResolver(config)
			params := types.Params{"timeout": "10"}

			gomega.Expect(resolver.UpdateConfigFromParams(config, &params)).
				To(gomega.MatchError(format.ErrParseDurationFailed))
			gomega.Expect(config.Timeout).To(gomega.Equal(5 * time.Second))
		})
	})
})
