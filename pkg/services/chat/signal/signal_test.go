package signal

import (
	"log"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSignal(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Signal Suite")
}

var (
	logger *log.Logger

	_ = ginkgo.BeforeSuite(func() {
		logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)
	})
)

var _ = ginkgo.Describe("the signal service", func() {
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
			})
		})
	})

	ginkgo.Describe("sending the payload", func() {
		var err error

		ginkgo.BeforeEach(func() {
			httpmock.Activate()
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("should not report an error if the server accepts the payload", func() {
			serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
			err = signal.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupResponder(200, `{"timestamp": 1234567890}`)

			err = signal.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should report an error if the server returns an error", func() {
			serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
			err = signal.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupResponder(400, `{"error": "Bad Request"}`)

			err = signal.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("server returned status 400"))
		})

		ginkgo.It("should handle attachments in parameters", func() {
			serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
			err = signal.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupResponder(200, `{"timestamp": 1234567890}`)

			params := types.Params{
				"attachments": "base64data1,base64data2",
			}

			err = signal.Send("Test message", &params)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should handle different response formats", func() {
			serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
			err = signal.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupResponder(201, `{"timestamp": "1234567890"}`) // String timestamp

			err = signal.Send("Test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should handle server errors gracefully", func() {
			serviceURL, _ := url.Parse("signal://localhost:8080/+1234567890/+0987654321")
			err = signal.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			setupResponder(500, `{"error": "Internal Server Error"}`)

			err = signal.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("server returned status 500"))
		})

		ginkgo.It("should return error when no recipients configured", func() {
			// Create a config with no recipients
			signal.Config = &Config{
				Host:       "localhost",
				Port:       8080,
				Source:     "+1234567890",
				Recipients: []string{}, // Empty recipients
			}

			err = signal.Send("Test message", nil)
			gomega.Expect(err).To(gomega.MatchError(ErrNoRecipients))
		})
	})

	ginkgo.Describe("parsing recipients", func() {
		ginkgo.It("should parse valid phone numbers", func() {
			recipients, err := parseRecipients([]string{"+1234567890", "+0987654321"})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(recipients).To(gomega.Equal([]string{"+1234567890", "+0987654321"}))
		})

		ginkgo.It("should parse valid group IDs", func() {
			recipients, err := parseRecipients([]string{"group.testgroup", "group.abcdef123"})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(recipients).
				To(gomega.Equal([]string{"group.testgroup", "group.abcdef123"}))
		})

		ginkgo.It("should parse group IDs with base64 characters", func() {
			recipients, err := parseRecipients([]string{"group.ABCD/EFGH=", "group.xyz+abc"})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(recipients).
				To(gomega.Equal([]string{"group.ABCD/EFGH=", "group.xyz+abc"}))
		})

		ginkgo.It("should parse mixed phone numbers and group IDs", func() {
			recipients, err := parseRecipients(
				[]string{"+1234567890", "group.testgroup", "+0987654321"},
			)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(recipients).
				To(gomega.Equal([]string{"+1234567890", "group.testgroup", "+0987654321"}))
		})

		ginkgo.It("should return error for invalid recipients", func() {
			_, err := parseRecipients([]string{"invalid-recipient"})
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid recipient"))
		})

		ginkgo.It("should return error for mixed valid and invalid recipients", func() {
			_, err := parseRecipients([]string{"+1234567890", "invalid-recipient"})
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid recipient"))
		})

		ginkgo.It("should return error for empty recipient list", func() {
			_, err := parseRecipients([]string{})
			gomega.Expect(err).To(gomega.MatchError(ErrNoRecipients))
		})

		ginkgo.It("should handle group IDs split across path segments", func() {
			recipients, err := parseRecipients([]string{"group.ABCD", "EFGH="})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(recipients).To(gomega.Equal([]string{"group.ABCD/EFGH="}))
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
		testutils.TestConfigGetEnumsCount(config, 0)
		testutils.TestConfigGetFieldsCount(config, 10)
	})

	ginkgo.It("should return the correct service ID", func() {
		service := &Service{}
		gomega.Expect(service.GetID()).To(gomega.Equal("signal"))
	})
})

func setupResponder(code int, body string) {
	targetURL := "https://localhost:8080/v2/send"
	httpmock.RegisterResponder("POST", targetURL, httpmock.NewStringResponder(code, body))
}
