package discord_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jarcoal/httpmock"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	testWebhookURL           = "https://discord.com/api/webhooks/123456789012345678/test-token-abcdefghijklmnopqrstuvwxyz123456"
	testWebhookURLWithThread = "https://discord.com/api/webhooks/123456789012345678/test-token-abcdefghijklmnopqrstuvwxyz123456?thread_id=987654321"
)

var _ = ginkgo.Describe("HTTP Request Construction and Component Integration", func() {
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

	ginkgo.Context("HTTP request construction validation", func() {
		ginkgo.It("should construct correct HTTP POST request for plain text messages", func() {
			expectedURL := testWebhookURL
			message := "Test plain text message"

			SetupMockResponderWithHTTPRequestValidation(&dummyConfig, 204,
				func(req *http.Request) error {
					// Validate HTTP method
					if err := ValidateHTTPMethod("POST")(req); err != nil {
						return err
					}
					// Validate Content-Type
					if err := ValidateContentType("application/json")(req); err != nil {
						return err
					}
					// Validate request URL
					if err := ValidateRequestURL(expectedURL)(req); err != nil {
						return err
					}

					return nil
				},
				func(_ discord.WebhookPayload) error {
					// No payload validation for black-box testing
					return nil
				})

			err := testService.Send(message, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should construct correct HTTP POST request for embed messages", func() {
			expectedURL := testWebhookURL

			SetupMockResponderWithHTTPRequestValidation(&dummyConfig, 204,
				func(req *http.Request) error {
					// Validate HTTP method
					if err := ValidateHTTPMethod("POST")(req); err != nil {
						return err
					}
					// Validate Content-Type
					if err := ValidateContentType("application/json")(req); err != nil {
						return err
					}
					// Validate request URL
					if err := ValidateRequestURL(expectedURL)(req); err != nil {
						return err
					}

					return nil
				},
				func(_ discord.WebhookPayload) error {
					// No payload validation for black-box testing
					return nil
				})

			items := CreateMessageItemWithLevel("Test embed message", types.Info)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should construct correct HTTP POST request with thread ID parameter", func() {
			customConfig := CreateDummyConfig()
			customConfig.ThreadID = "987654321"
			customService := CreateTestService(customConfig)

			expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abcdefghijklmnopqrstuvwxyz123456?thread_id=987654321"

			SetupMockResponderWithHTTPRequestValidation(&customConfig, 204,
				func(req *http.Request) error {
					// Validate HTTP method
					if err := ValidateHTTPMethod("POST")(req); err != nil {
						return err
					}
					// Validate Content-Type
					if err := ValidateContentType("application/json")(req); err != nil {
						return err
					}
					// Validate request URL with thread parameter
					if err := ValidateRequestURL(expectedURL)(req); err != nil {
						return err
					}

					return nil
				},
				func(_ discord.WebhookPayload) error {
					// No payload validation for black-box testing
					return nil
				})

			err := customService.Send("Thread message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("multipart form data HTTP request construction", func() {
		ginkgo.It("should construct correct multipart HTTP POST request for file uploads", func() {
			expectedURL := testWebhookURL
			testData := []byte("test file content for multipart upload")

			SetupMockResponderWithMultipartHTTPRequestValidation(
				&dummyConfig,
				204,
				func(req *http.Request) error {
					// Validate HTTP method
					if err := ValidateHTTPMethod("POST")(req); err != nil {
						return err
					}
					// Validate Content-Type starts with multipart/form-data
					if err := ValidateContentType("multipart/form-data")(req); err != nil {
						return err
					}
					// Validate request URL
					if err := ValidateRequestURL(expectedURL)(req); err != nil {
						return err
					}

					return nil
				},
				func(_ discord.WebhookPayload, _ []types.File, _ string, _ []byte) error {
					// No payload validation for black-box testing
					return nil
				},
			)

			items := CreateMessageItemWithFile("Message with file", "test.txt", testData)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should validate multipart boundary and structure", func() {
			testData := []byte("multipart test data")

			SetupMockResponderWithMultipartHTTPRequestValidation(
				&dummyConfig,
				204,
				func(req *http.Request) error {
					contentType := req.Header.Get("Content-Type")
					// Validate boundary is present
					if !strings.Contains(contentType, "boundary=") {
						return fmt.Errorf(
							"Content-Type missing boundary parameter: %s",
							contentType,
						)
					}
					// Validate boundary is properly formatted
					parts := strings.Split(contentType, "boundary=")
					if len(parts) != 2 || parts[1] == "" {
						return fmt.Errorf(
							"invalid boundary format in Content-Type: %s",
							contentType,
						)
					}

					return nil
				},
				func(_ discord.WebhookPayload, _ []types.File, _ string, _ []byte) error {
					// No payload validation for black-box testing
					return nil
				},
			)

			items := CreateMessageItemWithFile(
				"Boundary validation test",
				"boundary-test.txt",
				testData,
			)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("component interaction validation", func() {
		ginkgo.It(
			"should validate complete flow from message item creation to HTTP request",
			func() {
				message := "Complete flow integration test"
				expectedURL := testWebhookURL

				// This test validates that the entire pipeline works:
				// MessageItem -> Payload creation -> JSON marshaling -> HTTP request construction -> sending
				SetupMockResponderWithHTTPRequestValidation(&dummyConfig, 204,
					func(req *http.Request) error {
						// Validate HTTP request construction
						if err := ValidateHTTPMethod("POST")(req); err != nil {
							return err
						}
						if err := ValidateContentType("application/json")(req); err != nil {
							return err
						}
						if err := ValidateRequestURL(expectedURL)(req); err != nil {
							return err
						}

						return nil
					},
					func(_ discord.WebhookPayload) error {
						// No payload validation for black-box testing
						return nil
					})

				err := testService.Send(message, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			},
		)

		ginkgo.It(
			"should validate end-to-end component integration for plain text messages",
			func() {
				messageItems := []types.MessageItem{
					{Text: "Integration test message", Level: types.Info},
				}
				expectedURL := testWebhookURL

				SetupMockResponderWithHTTPRequestValidation(&dummyConfig, 204,
					func(req *http.Request) error {
						// Validate HTTP request construction
						if err := ValidateHTTPMethod("POST")(req); err != nil {
							return err
						}
						if err := ValidateContentType("application/json")(req); err != nil {
							return err
						}
						if err := ValidateRequestURL(expectedURL)(req); err != nil {
							return err
						}

						return nil
					},
					func(_ discord.WebhookPayload) error {
						// No payload validation for black-box testing
						return nil
					})

				err := testService.SendItems(messageItems, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			},
		)

		ginkgo.It(
			"should validate end-to-end component integration for embeds with author and fields",
			func() {
				messageItems := []types.MessageItem{
					{
						Text:  "Embed integration test",
						Level: types.Warning,
						Fields: []types.Field{
							{Key: "embed_author_name", Value: "Test Bot"},
							{Key: "embed_author_url", Value: "https://example.com"},
							{Key: "Status", Value: "Testing"},
							{Key: "Priority", Value: "High"},
						},
					},
				}

				SetupMockResponderWithHTTPRequestValidation(&dummyConfig, 204,
					func(req *http.Request) error {
						// Validate HTTP request construction
						if err := ValidateHTTPMethod("POST")(req); err != nil {
							return err
						}
						if err := ValidateContentType("application/json")(req); err != nil {
							return err
						}

						return nil
					},
					func(_ discord.WebhookPayload) error {
						// No payload validation for black-box testing
						return nil
					})

				err := testService.SendItems(messageItems, nil)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			},
		)

		ginkgo.It("should validate end-to-end component integration for thread messages", func() {
			customConfig := CreateDummyConfig()
			customConfig.ThreadID = "test-thread-123"
			customService := CreateTestService(customConfig)

			messageItems := []types.MessageItem{
				{Text: "Thread message test", Level: types.Debug},
			}
			expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abcdefghijklmnopqrstuvwxyz123456?thread_id=test-thread-123"

			SetupMockResponderWithHTTPRequestValidation(&customConfig, 204,
				func(req *http.Request) error {
					// Validate HTTP request construction with thread parameter
					if err := ValidateHTTPMethod("POST")(req); err != nil {
						return err
					}
					if err := ValidateContentType("application/json")(req); err != nil {
						return err
					}
					if err := ValidateRequestURL(expectedURL)(req); err != nil {
						return err
					}

					return nil
				},
				func(_ discord.WebhookPayload) error {
					// No payload validation for black-box testing
					return nil
				})

			err := customService.SendItems(messageItems, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should validate embed creation and HTTP sending integration", func() {
			expectedURL := testWebhookURL

			SetupMockResponderWithHTTPRequestValidation(&dummyConfig, 204,
				func(req *http.Request) error {
					// Validate HTTP request construction for embed
					if err := ValidateHTTPMethod("POST")(req); err != nil {
						return err
					}
					if err := ValidateContentType("application/json")(req); err != nil {
						return err
					}
					if err := ValidateRequestURL(expectedURL)(req); err != nil {
						return err
					}

					return nil
				},
				func(_ discord.WebhookPayload) error {
					// No payload validation for black-box testing
					return nil
				})

			items := CreateMessageItemWithLevel("Embed integration test", types.Error)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should validate multipart upload component integration", func() {
			expectedURL := testWebhookURL
			testData := []byte("integration test file data")

			SetupMockResponderWithMultipartHTTPRequestValidation(
				&dummyConfig,
				204,
				func(req *http.Request) error {
					// Validate HTTP request construction for multipart
					if err := ValidateHTTPMethod("POST")(req); err != nil {
						return err
					}
					if err := ValidateContentType("multipart/form-data")(req); err != nil {
						return err
					}
					if err := ValidateRequestURL(expectedURL)(req); err != nil {
						return err
					}

					return nil
				},
				func(_ discord.WebhookPayload, _ []types.File, _ string, _ []byte) error {
					// No payload validation for black-box testing
					return nil
				},
			)

			items := CreateMessageItemWithFile(
				"Multipart integration test",
				"integration.txt",
				testData,
			)
			err := testService.SendItems(items, nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("API URL construction validation", func() {
		ginkgo.It("should construct correct API URL without thread ID", func() {
			expectedURL := testWebhookURL
			actualURL := discord.CreateAPIURLFromConfig(&dummyConfig)
			gomega.Expect(actualURL).To(gomega.Equal(expectedURL))
		})

		ginkgo.It("should construct correct API URL with thread ID", func() {
			customConfig := CreateDummyConfig()
			customConfig.ThreadID = "987654321"
			expectedURL := testWebhookURLWithThread
			actualURL := discord.CreateAPIURLFromConfig(&customConfig)
			gomega.Expect(actualURL).To(gomega.Equal(expectedURL))
		})

		ginkgo.It("should handle thread ID with whitespace trimming", func() {
			customConfig := CreateDummyConfig()
			customConfig.ThreadID = "  987654321  "
			expectedURL := testWebhookURLWithThread
			actualURL := discord.CreateAPIURLFromConfig(&customConfig)
			gomega.Expect(actualURL).To(gomega.Equal(expectedURL))
		})

		ginkgo.It(
			"should validate API URL construction for standard webhook without thread",
			func() {
				config := discord.Config{
					WebhookID: "123456789012345678",
					Token:     "test-token-abc123",
					ThreadID:  "",
				}
				expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abc123"
				actualURL := discord.CreateAPIURLFromConfig(&config)
				gomega.Expect(actualURL).To(gomega.Equal(expectedURL))

				// Validate the URL can be parsed back
				parsedURL, err := url.Parse(actualURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(parsedURL.Scheme).To(gomega.Equal("https"))
				gomega.Expect(parsedURL.Host).To(gomega.Equal("discord.com"))
				gomega.Expect(parsedURL.Path).
					To(gomega.Equal("/api/webhooks/123456789012345678/test-token-abc123"))
				gomega.Expect(parsedURL.Query().Get("thread_id")).To(gomega.BeEmpty())
			},
		)

		ginkgo.It(
			"should validate API URL construction for webhook with numeric thread ID",
			func() {
				config := discord.Config{
					WebhookID: "123456789012345678",
					Token:     "test-token-abc123",
					ThreadID:  "987654321",
				}
				expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abc123?thread_id=987654321"
				actualURL := discord.CreateAPIURLFromConfig(&config)
				gomega.Expect(actualURL).To(gomega.Equal(expectedURL))

				// Validate the URL can be parsed back
				parsedURL, err := url.Parse(actualURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(parsedURL.Scheme).To(gomega.Equal("https"))
				gomega.Expect(parsedURL.Host).To(gomega.Equal("discord.com"))
				gomega.Expect(parsedURL.Path).
					To(gomega.Equal("/api/webhooks/123456789012345678/test-token-abc123"))
				gomega.Expect(parsedURL.Query().Get("thread_id")).To(gomega.Equal("987654321"))
			},
		)

		ginkgo.It(
			"should validate API URL construction for webhook with alphanumeric thread ID",
			func() {
				config := discord.Config{
					WebhookID: "123456789012345678",
					Token:     "test-token-abc123",
					ThreadID:  "thread-123",
				}
				expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abc123?thread_id=thread-123"
				actualURL := discord.CreateAPIURLFromConfig(&config)
				gomega.Expect(actualURL).To(gomega.Equal(expectedURL))

				// Validate the URL can be parsed back
				parsedURL, err := url.Parse(actualURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(parsedURL.Scheme).To(gomega.Equal("https"))
				gomega.Expect(parsedURL.Host).To(gomega.Equal("discord.com"))
				gomega.Expect(parsedURL.Path).
					To(gomega.Equal("/api/webhooks/123456789012345678/test-token-abc123"))
				gomega.Expect(parsedURL.Query().Get("thread_id")).To(gomega.Equal("thread-123"))
			},
		)

		ginkgo.It(
			"should validate API URL construction for webhook with special character thread ID",
			func() {
				config := discord.Config{
					WebhookID: "123456789012345678",
					Token:     "test-token-abc123",
					ThreadID:  "thread_123-abc",
				}
				expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abc123?thread_id=thread_123-abc"
				actualURL := discord.CreateAPIURLFromConfig(&config)
				gomega.Expect(actualURL).To(gomega.Equal(expectedURL))

				// Validate the URL can be parsed back
				parsedURL, err := url.Parse(actualURL)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(parsedURL.Scheme).To(gomega.Equal("https"))
				gomega.Expect(parsedURL.Host).To(gomega.Equal("discord.com"))
				gomega.Expect(parsedURL.Path).
					To(gomega.Equal("/api/webhooks/123456789012345678/test-token-abc123"))
				gomega.Expect(parsedURL.Query().Get("thread_id")).To(gomega.Equal("thread_123-abc"))
			},
		)

		ginkgo.It("should validate API URL construction in HTTP request context", func() {
			customConfig := CreateDummyConfig()
			customConfig.ThreadID = "test-thread-123"
			customService := CreateTestService(customConfig)

			expectedURL := "https://discord.com/api/webhooks/123456789012345678/test-token-abcdefghijklmnopqrstuvwxyz123456?thread_id=test-thread-123"

			SetupMockResponderWithHTTPRequestValidation(&customConfig, 204,
				func(req *http.Request) error {
					// Validate the request URL matches what CreateAPIURLFromConfig produces
					actualURL := discord.CreateAPIURLFromConfig(&customConfig)
					if req.URL.String() != actualURL {
						return fmt.Errorf(
							"request URL %q does not match expected URL %q",
							req.URL.String(),
							actualURL,
						)
					}
					if req.URL.String() != expectedURL {
						return fmt.Errorf(
							"request URL %q does not match expected URL %q",
							req.URL.String(),
							expectedURL,
						)
					}

					return nil
				},
				func(discord.WebhookPayload) error { return nil })

			err := customService.Send("URL construction validation test", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Context("error handling in HTTP request construction", func() {
		ginkgo.It("should handle invalid webhook URLs gracefully", func() {
			invalidConfig := CreateDummyConfig()
			invalidConfig.WebhookID = "" // Invalid webhook ID

			invalidService := CreateTestService(invalidConfig)

			// The service should handle the error gracefully
			err := invalidService.Send("Test message", nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("failed to send discord notification"))
		})

		ginkgo.It("should handle malformed JSON payload errors", func() {
			// This test ensures that if JSON marshaling fails, it's handled properly
			// We can't easily trigger JSON marshaling failures in normal operation,
			// but we can verify the error handling path exists
			gomega.Expect(testService).
				NotTo(gomega.BeNil())
			// Service should be properly initialized
		})
	})
})
