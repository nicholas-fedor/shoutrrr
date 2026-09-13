package signal

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
)

var _ = ginkgo.Describe("config", func() {
	var signal *Service

	ginkgo.BeforeEach(func() {
		signal = &Service{}
	})

	ginkgo.Describe("creating configurations", func() {
		ginkgo.When("given a url", func() {
			ginkgo.It("should return an error if no source phone number is supplied", func() {
				serviceURL, _ := url.Parse("signal://localhost:8080")
				err := signal.Initialize(serviceURL, logger)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err).To(gomega.MatchError(ErrNoRecipients))
			})

			ginkgo.It("should return an error if no recipients are supplied", func() {
				serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890")
				err := signal.Initialize(serviceURL, logger)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err).To(gomega.MatchError(ErrNoRecipients))
			})

			ginkgo.It("should return an error for invalid phone number format", func() {
				serviceURL, _ := url.Parse("signal://localhost:8080/invalid-phone/+1234567890")
				err := signal.Initialize(serviceURL, logger)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).
					To(gomega.ContainSubstring("invalid phone number format"))
			})

			ginkgo.It("should return an error for invalid group ID format", func() {
				serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/invalid.group!")
				err := signal.Initialize(serviceURL, logger)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid recipient"))
			})

			ginkgo.It("should accept group IDs with base64 characters / and =", func() {
				serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/group.ABCD/EFGH=")
				err := signal.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("should accept group IDs with base64 character +", func() {
				serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/group.ABCD+EFGH")
				err := signal.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})

			ginkgo.It("should accept u: username recipients", func() {
				serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/u:someuser.123")
				err := signal.Initialize(serviceURL, logger)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(signal.Config.Recipients).To(gomega.Equal([]string{"u:someuser.123"}))
			})

			ginkgo.It("should reject an empty u: username", func() {
				serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/u:")
				err := signal.Initialize(serviceURL, logger)
				gomega.Expect(err).To(gomega.HaveOccurred())
				gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid recipient"))
			})

			ginkgo.When("parsing authentication", func() {
				ginkgo.It("should parse user without password", func() {
					serviceURL, _ := url.Parse(
						"signal://user@localhost:8080/+1234567890/+0987654321",
					)
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.User).To(gomega.Equal("user"))
					gomega.Expect(signal.Config.Password).To(gomega.BeEmpty())
				})

				ginkgo.It("should parse user with password", func() {
					serviceURL, _ := url.Parse(
						"signal://user:pass@localhost:8080/+1234567890/+0987654321",
					)
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.User).To(gomega.Equal("user"))
					gomega.Expect(signal.Config.Password).To(gomega.Equal("pass"))
				})
			})

			ginkgo.When("parsing host and port", func() {
				ginkgo.It("should parse custom host and port", func() {
					serviceURL, _ := url.Parse("signal://myserver:9999/+1234567890/+0987654321")
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.Host).To(gomega.Equal("myserver"))
					gomega.Expect(signal.Config.Port).To(gomega.Equal(9999))
				})

				ginkgo.It("should use default port when not specified", func() {
					serviceURL, _ := url.Parse("signal://myserver/+1234567890/+0987654321")
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.Host).To(gomega.Equal("myserver"))
					gomega.Expect(signal.Config.Port).To(gomega.Equal(8080))
				})

				ginkgo.It("should store an IPv6 host without brackets", func() {
					serviceURL, _ := url.Parse("signal://[::1]:9999/+1234567890/+0987654321")
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.Host).To(gomega.Equal("::1"))
					gomega.Expect(signal.Config.Port).To(gomega.Equal(9999))
					gomega.Expect(signal.Config.GetURL().Host).To(gomega.Equal("[::1]:9999"))
				})
			})

			ginkgo.When("parsing TLS settings", func() {
				ginkgo.It("should enable TLS by default", func() {
					serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.DisableTLS).To(gomega.BeFalse())
				})

				ginkgo.It("should disable TLS when disabletls=yes", func() {
					serviceURL, _ := url.Parse(
						"signal://localhost:8080/+1234567890/+0987654321?disabletls=yes",
					)
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.DisableTLS).To(gomega.BeTrue())
				})

				ginkgo.It("should skip TLS verification when skiptlsverify=yes", func() {
					serviceURL, _ := url.Parse(
						"signal://localhost:8080/+1234567890/+0987654321?skiptlsverify=yes",
					)
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.SkipTLSVerify).To(gomega.BeTrue())

					httpClient, ok := signal.httpClient.(*http.Client)
					gomega.Expect(ok).To(gomega.BeTrue())
					transport, ok := httpClient.Transport.(*http.Transport)
					gomega.Expect(ok).To(gomega.BeTrue())
					gomega.Expect(transport.TLSClientConfig).NotTo(gomega.BeNil())
					gomega.Expect(transport.TLSClientConfig.InsecureSkipVerify).To(gomega.BeTrue())
					gomega.Expect(transport.TLSClientConfig.MinVersion).
						To(gomega.Equal(uint16(tls.VersionTLS12)))
				})
			})

			ginkgo.When("parsing text mode", func() {
				ginkgo.It("should default to None", func() {
					serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.TextMode).To(gomega.Equal(TextModeNone))
				})

				ginkgo.It("should parse textmode=styled", func() {
					serviceURL, _ := url.Parse(
						"signal://localhost:8080/+1234567890/+0987654321?textmode=styled",
					)
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.TextMode).To(gomega.Equal(TextModeStyled))
				})

				ginkgo.It("should parse text_mode=styled", func() {
					serviceURL, _ := url.Parse(
						"signal://localhost:8080/+1234567890/+0987654321?text_mode=styled",
					)
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(signal.Config.TextMode).To(gomega.Equal(TextModeStyled))
				})

				ginkgo.It("should reject an invalid text mode", func() {
					serviceURL, _ := url.Parse(
						"signal://localhost:8080/+1234567890/+0987654321?textmode=markdown",
					)
					err := signal.Initialize(serviceURL, logger)
					gomega.Expect(err).To(gomega.HaveOccurred())
				})
			})

			ginkgo.When("the url is valid", func() {
				var (
					config *Config
					err    error
				)

				ginkgo.BeforeEach(func() {
					serviceURL, _ := url.Parse(
						"signal://localhost:8080/+1234567890/+0987654321/group.testgroup",
					)
					err = signal.Initialize(serviceURL, logger)
					config = signal.Config
				})

				ginkgo.It("should create a config object", func() {
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
					gomega.Expect(config).ToNot(gomega.BeNil())
				})

				ginkgo.It("should parse the source phone number", func() {
					gomega.Expect(config.Source).To(gomega.Equal("+1234567890"))
				})

				ginkgo.It("should parse the recipients", func() {
					gomega.Expect(config.Recipients).
						To(gomega.Equal([]string{"+0987654321", "group.testgroup"}))
				})

				ginkgo.It("should set default host and port", func() {
					gomega.Expect(config.Host).To(gomega.Equal("localhost"))
					gomega.Expect(config.Port).To(gomega.Equal(8080))
				})

				ginkgo.It("should default notify_self to yes", func() {
					gomega.Expect(config.NotifySelf).To(gomega.BeTrue())
				})
			})
		})
	})

	ginkgo.It("should implement basic service API methods correctly", func() {
		serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
		err := signal.Initialize(serviceURL, logger)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())

		config := signal.Config
		testutils.TestConfigGetInvalidQueryValue(config)
		testutils.TestConfigSetInvalidQueryValue(
			config,
			"signal://localhost:8080/+1234567890/+0987654321?foo=bar",
		)
		testutils.TestConfigGetEnumsCount(config, 1)
		testutils.TestConfigGetFieldsCount(config, 16)
	})
})
