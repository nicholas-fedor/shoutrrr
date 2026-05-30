package teams

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	workflowURL = "https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"
)

var serviceURLBase = "teams://?host=" + url.QueryEscape(workflowURL)

var logger = log.New(ginkgo.GinkgoWriter, "Test", log.LstdFlags)

var _ = ginkgo.Describe("the teams service", func() {
	ginkgo.Describe("sending the payload", func() {
		var (
			err     error
			service Service
		)

		ginkgo.BeforeEach(func() {
			httpmock.Activate()
		})
		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})
		ginkgo.It("should not report an error if the server accepts the payload", func() {
			serviceURL, _ := url.Parse(serviceURLBase)
			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				workflowURL,
				httpmock.NewStringResponder(http.StatusOK, ""),
			)

			err = service.Send("Message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
		ginkgo.It("should report an error if the server returns non-200", func() {
			serviceURL, _ := url.Parse(serviceURLBase)
			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				workflowURL,
				httpmock.NewStringResponder(http.StatusInternalServerError, ""),
			)

			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
		ginkgo.It("should not panic if an HTTP error occurs when sending the payload", func() {
			serviceURL, _ := url.Parse(serviceURLBase)
			err = service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			httpmock.RegisterResponder(
				"POST",
				workflowURL,
				httpmock.NewErrorResponder(errors.New("dummy error")),
			)

			err = service.Send("Message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.It("should return the correct service ID", func() {
		service := &Service{}
		gomega.Expect(service.GetID()).To(gomega.Equal("teams"))
	})

	ginkgo.Describe("the teams config", func() {
		ginkgo.Describe("setURL", func() {
			ginkgo.It("should set all fields correctly from URL", func() {
				config := &Config{}
				urlStr := serviceURLBase + "&title=Test&color=red"
				parsedURL, err := url.Parse(urlStr)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				err = config.SetURL(parsedURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				gomega.Expect(config.Host).To(gomega.Equal(workflowURL))
				gomega.Expect(config.Title).To(gomega.Equal("Test"))
				gomega.Expect(config.Color).To(gomega.Equal("red"))
			})

			ginkgo.It("should silently skip unknown query params from workflow URL", func() {
				config := &Config{}
				// Simulates the query params appended by Power Automate workflow URLs
				urlStr := serviceURLBase + "&api-version=2016-06-00&sp=/triggers/manual/run&sv=1.0&sig=XXXXXXXX"
				parsedURL, err := url.Parse(urlStr)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				err = config.SetURL(parsedURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(config.Host).To(gomega.Equal(workflowURL))
			})

			ginkgo.It("should accept valid string values for known keys", func() {
				config := &Config{}
				urlStr := serviceURLBase + "&title=Test"
				parsedURL, err := url.Parse(urlStr)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				err = config.SetURL(parsedURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(config.Title).To(gomega.Equal("Test"))
			})
		})

		ginkgo.Describe("getURL", func() {
			ginkgo.It("should generate URL containing host, title, and color params", func() {
				config := &Config{
					Host:  workflowURL,
					Title: "Test",
					Color: "red",
				}

				urlObj := config.GetURL()
				urlStr := urlObj.String()
				gomega.Expect(urlStr).To(gomega.ContainSubstring("host="))
				gomega.Expect(urlStr).To(gomega.ContainSubstring("color=red"))
				gomega.Expect(urlStr).To(gomega.ContainSubstring("title=Test"))
			})
		})
	})

	ginkgo.Describe("Initialize", func() {
		ginkgo.It("should initialize with a valid workflow URL", func() {
			service := &Service{}
			serviceURL, _ := url.Parse(serviceURLBase)
			err := service.Initialize(serviceURL, logger)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config).NotTo(gomega.BeNil())
			gomega.Expect(service.Config.Host).To(gomega.Equal(workflowURL))
		})
	})

	ginkgo.Describe("ValidateWebhookURL", func() {
		ginkgo.It("should accept valid Power Automate workflow URLs", func() {
			validURLs := []string{
				"https://prod-00.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke",
				"https://prod-00.westus.logic.azure.com/workflows/abc123/triggers/manual/paths/invoke",
				"https://mytenant.logic.azure.com/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-00",
				"https://prod-00.westus.logic.azure.us/workflows/abc123/triggers/manual/paths/invoke",
				"https://prod-00.westus.logic.azure.cn/workflows/abc123/triggers/manual/paths/invoke",
				"https://prod-00.westus.logic.azure.de/workflows/abc123/triggers/manual/paths/invoke",
			}
			for _, u := range validURLs {
				err := ValidateWebhookURL(u)
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "URL: %s", u)
			}
		})

		ginkgo.It("should reject invalid URLs", func() {
			invalidURLs := []string{
				"https://example.com/webhook",
				"",
			}
			for _, u := range invalidURLs {
				err := ValidateWebhookURL(u)
				gomega.Expect(err).To(gomega.HaveOccurred(), "URL: %s", u)
			}
		})
	})

	ginkgo.Describe("doSend", func() {
		ginkgo.It("should return ErrMissingHost when host is empty", func() {
			service := &Service{}
			service.Config = &Config{}
			service.SetLogger(logger)

			err := service.doSend(&Config{}, "test message")
			gomega.Expect(err).To(gomega.Equal(ErrMissingHost))
		})

		ginkgo.It("should return ErrInvalidWebhookURL for invalid host before any HTTP call", func() {
			service := &Service{}
			service.Config = &Config{}
			service.SetLogger(logger)

			// No httpmock activated — this must fail at validation, not at the network layer.
			err := service.doSend(&Config{Host: "https://example.com"}, "test message")
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidWebhookURL))
		})
	})
})

// TestTeams runs the test suite for the Teams package.
func TestTeams(t *testing.T) {
	t.Parallel()
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Shoutrrr Teams Suite")
}
