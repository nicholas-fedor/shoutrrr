package discord_test

import (
	"net/http"
	"net/url"

	"github.com/jarcoal/httpmock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var _ = ginkgo.Describe("Errors", func() {
	var testService *discord.Service
	var dummyConfig discord.Config

	ginkgo.BeforeEach(func() {
		httpmock.Activate()
		dummyConfig = CreateDummyConfig()
		testService = CreateTestService(dummyConfig)
	})

	ginkgo.AfterEach(func() {
		httpmock.DeactivateAndReset()
	})

	ginkgo.Context("HTTP error handling", func() {
		ginkgo.It("should handle 400 Bad Request errors", func() {
			SetupMockResponder(&dummyConfig, 400)
			err := testService.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("unexpected response status code"))
		})

		ginkgo.It("should handle 401 Unauthorized errors", func() {
			SetupMockResponder(&dummyConfig, 401)
			err := testService.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle 403 Forbidden errors", func() {
			SetupMockResponder(&dummyConfig, 403)
			err := testService.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle 404 Not Found errors", func() {
			SetupMockResponder(&dummyConfig, 404)
			err := testService.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle 429 Rate Limit errors", func() {
			apiURL := discord.CreateAPIURLFromConfig(&dummyConfig)
			httpmock.RegisterResponder(
				"POST",
				apiURL,
				func(_ *http.Request) (*http.Response, error) {
					resp := httpmock.NewStringResponse(429, "")
					resp.Header.Set("Retry-After", "1") // Short retry time for test

					return resp, nil
				},
			)
			err := testService.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should handle 500 Internal Server Error", func() {
			SetupMockResponder(&dummyConfig, 500)
			err := testService.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("payload validation errors", func() {
		ginkgo.It("should reject empty messages", func() {
			err := testService.Send("", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should reject whitespace-only messages", func() {
			err := testService.Send("   \n\t   ", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should reject nil message items", func() {
			var items []types.MessageItem
			err := testService.SendItems(items, nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("malformed payload handling", func() {
		ginkgo.It("should handle invalid JSON in JSON mode", func() {
			// Create a service with JSON mode enabled
			jsonConfig := CreateDummyConfig()
			jsonConfig.JSON = true
			jsonService := CreateTestService(jsonConfig)

			// This should fail during payload processing since invalid JSON is passed as message
			err := jsonService.Send(`{"invalid": json}`, nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("file attachment errors", func() {
		ginkgo.It("should handle file attachment failures", func() {
			// Create a multipart responder that returns an error
			httpmock.RegisterResponder("POST", discord.CreateAPIURLFromConfig(&dummyConfig),
				func(_ *http.Request) (*http.Response, error) {
					return httpmock.NewStringResponse(400, "File too large"), nil
				})

			testData := []byte("test file content")
			items := CreateMessageItemWithFile("Message with file", "test.txt", testData)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.Context("invalid configuration handling", func() {
			ginkgo.It("should handle empty webhook ID", func() {
				invalidConfig := discord.Config{
					WebhookID: "",
					Token:     "test-token",
				}
				invalidService := &discord.Service{}
				err := invalidService.Initialize(invalidConfig.GetURL(), nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})

			ginkgo.It("should handle empty token", func() {
				invalidConfig := discord.Config{
					WebhookID: "123",
					Token:     "",
				}
				invalidService := &discord.Service{}
				err := invalidService.Initialize(invalidConfig.GetURL(), nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})

			ginkgo.It("should handle malformed URLs", func() {
				invalidService := &discord.Service{}
				invalidURL, _ := url.Parse("not-a-url")
				err := invalidService.Initialize(invalidURL, nil)
				gomega.Expect(err).To(gomega.HaveOccurred())
			})
		})
	})
})
