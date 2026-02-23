package discord

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const testWebhookURLAlt = "https://discord.com/api/webhooks/123/abc"

var _ = ginkgo.Describe("Discord Service Unit Tests", func() {
	ginkgo.Describe("Service.Send method", func() {
		var (
			service    *Service
			mockClient *mockServiceHTTPClient
		)

		ginkgo.BeforeEach(func() {
			service = &Service{
				Config: &Config{
					WebhookID: "123456789",
					Token:     "test-token",
				},
				HTTPClient: &mockServiceHTTPClient{},
			}
			service.SetLogger(&mockLogger{})
			mockClient = service.HTTPClient.(*mockServiceHTTPClient)
			mockClient.response = &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}
		})

		ginkgo.It("should send plain text message successfully", func() {
			message := "Hello, Discord!"
			params := &types.Params{}

			err := service.Send(message, params)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
		})

		ginkgo.It("should send JSON message in JSON mode", func() {
			service.Config.JSON = true
			jsonMessage := `{"content":"JSON message"}`
			params := &types.Params{}

			err := service.Send(jsonMessage, params)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
		})

		ginkgo.It("should handle empty message", func() {
			message := ""
			params := &types.Params{}

			err := service.Send(message, params)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrEmptyMessage))
		})

		ginkgo.It("should handle large message with chunking", func() {
			// Create a message larger than TotalChunkSize to force multiple batches
			largeMessage := strings.Repeat("a", TotalChunkSize+100)
			params := &types.Params{}

			err := service.Send(largeMessage, params)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// Should make multiple calls for chunks
			gomega.Expect(mockClient.callCount).To(gomega.BeNumerically(">=", 2))
		})

		ginkgo.It("should handle split lines option", func() {
			service.Config.SplitLines = true
			message := "Line 1\nLine 2\nLine 3"
			params := &types.Params{}

			err := service.Send(message, params)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).
				To(gomega.Equal(1))
			// One call for the batch of lines
		})

		ginkgo.It("should return error when first batch fails", func() {
			mockClient.err = ErrMaxRetries
			message := "Test message"
			params := &types.Params{}

			err := service.Send(message, params)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("failed to send discord notification"))
		})
	})

	ginkgo.Describe("Service.SendItems method", func() {
		var service *Service

		ginkgo.BeforeEach(func() {
			service = &Service{
				Config: &Config{
					WebhookID: "123456789",
					Token:     "test-token",
				},
				HTTPClient: &mockServiceHTTPClient{
					response: &http.Response{
						StatusCode: http.StatusNoContent,
						Body:       io.NopCloser(strings.NewReader("")),
					},
				},
			}
		})

		ginkgo.It("should delegate to sendItems method", func() {
			items := []types.MessageItem{
				{Text: "Test message"},
			}
			params := &types.Params{}

			err := service.SendItems(items, params)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Service.sendItems method", func() {
		var (
			service    *Service
			mockClient *mockServiceHTTPClient
		)

		ginkgo.BeforeEach(func() {
			service = &Service{
				Config: &Config{
					WebhookID: "123456789",
					Token:     "test-token",
				},
				pkr:        format.NewPropKeyResolver(&Config{}),
				HTTPClient: &mockServiceHTTPClient{},
			}
			mockClient = service.HTTPClient.(*mockServiceHTTPClient)
			mockClient.response = &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}
		})

		ginkgo.It("should send simple message item successfully", func() {
			items := []types.MessageItem{
				{Text: "Simple message"},
			}
			params := &types.Params{}

			err := service.sendItems(items, params)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
		})

		ginkgo.It("should handle message with file attachment", func() {
			items := []types.MessageItem{
				{
					Text: "Message with file",
					File: &types.File{
						Name: "test.txt",
						Data: []byte("file content"),
					},
				},
			}
			params := &types.Params{}

			err := service.sendItems(items, params)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
		})

		ginkgo.It("should update config from params", func() {
			service.pkr = format.NewPropKeyResolver(service.Config)
			items := []types.MessageItem{
				{Text: "Test message"},
			}
			params := &types.Params{
				"username": "CustomBot",
			}

			err := service.sendItems(items, params)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// The payload should include the updated username
		})

		ginkgo.It("should handle empty items", func() {
			items := []types.MessageItem{}
			params := &types.Params{}

			err := service.sendItems(items, params)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrEmptyMessage))
		})

		ginkgo.It("should handle config update error", func() {
			// Use invalid params to trigger an error
			service.pkr = format.NewPropKeyResolver(service.Config)
			items := []types.MessageItem{
				{Text: "Test message"},
			}
			params := &types.Params{
				"invalid_key": "value",
			}

			err := service.sendItems(items, params)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("updating config from params"))
		})
	})

	ginkgo.Describe("CreateItemsFromPlain function", func() {
		ginkgo.It("should create single batch for small message without split lines", func() {
			plain := "Short message"
			splitLines := false

			result := CreateItemsFromPlain(plain, splitLines)

			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0]).To(gomega.HaveLen(1))
			gomega.Expect(result[0][0].Text).To(gomega.Equal(plain))
		})

		ginkgo.It("should split lines when splitLines is true", func() {
			plain := "Line 1\nLine 2\nLine 3"
			splitLines := true

			result := CreateItemsFromPlain(plain, splitLines)

			gomega.Expect(result).To(gomega.HaveLen(1)) // All lines fit in one batch
			gomega.Expect(result[0]).To(gomega.HaveLen(3))
			gomega.Expect(result[0][0].Text).To(gomega.Equal("Line 1"))
			gomega.Expect(result[0][1].Text).To(gomega.Equal("Line 2"))
			gomega.Expect(result[0][2].Text).To(gomega.Equal("Line 3"))
		})

		ginkgo.It("should chunk large message without split lines", func() {
			largeMessage := strings.Repeat("a", ChunkSize+100)
			splitLines := false

			result := CreateItemsFromPlain(largeMessage, splitLines)

			gomega.Expect(result).To(gomega.HaveLen(1))    // All chunks fit in one batch
			gomega.Expect(result[0]).To(gomega.HaveLen(2)) // 2 chunks: 2000 and 100

			totalLength := 0
			for _, item := range result[0] {
				totalLength += len(item.Text)
			}

			gomega.Expect(totalLength).To(gomega.Equal(len(largeMessage)))
		})

		ginkgo.It("should handle empty string", func() {
			plain := ""
			splitLines := false

			result := CreateItemsFromPlain(plain, splitLines)

			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0]).To(gomega.BeEmpty()) // Empty input returns empty items
		})

		ginkgo.It("should handle message exactly at chunk size", func() {
			exactMessage := strings.Repeat("a", ChunkSize)
			splitLines := false

			result := CreateItemsFromPlain(exactMessage, splitLines)

			gomega.Expect(result).To(gomega.HaveLen(1))
			gomega.Expect(result[0][0].Text).To(gomega.Equal(exactMessage))
		})
	})

	ginkgo.Describe("Service.Initialize method", func() {
		var service *Service

		ginkgo.BeforeEach(func() {
			service = &Service{}
		})

		ginkgo.It("should initialize service successfully with valid URL", func() {
			configURL, _ := url.Parse("discord://token@webhook?username=TestBot")
			logger := &mockLogger{}

			err := service.Initialize(configURL, logger)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(service.Config).NotTo(gomega.BeNil())
			gomega.Expect(service.Config.WebhookID).To(gomega.Equal("webhook"))
			gomega.Expect(service.Config.Token).To(gomega.Equal("token"))
			gomega.Expect(service.Config.Username).To(gomega.Equal("TestBot"))
			gomega.Expect(service.HTTPClient).NotTo(gomega.BeNil())
		})

		ginkgo.It("should handle URL parsing error", func() {
			// Simulate config.SetURL error by using invalid URL
			invalidURL, _ := url.Parse("discord://@") // Missing token
			logger := &mockLogger{}

			err := service.Initialize(invalidURL, logger)

			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should set default properties", func() {
			configURL, _ := url.Parse("discord://token@webhook")
			logger := &mockLogger{}

			err := service.Initialize(configURL, logger)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// Verify default properties are set
			gomega.Expect(service.pkr).NotTo(gomega.BeNil())
		})
	})

	ginkgo.Describe("Service.GetID method", func() {
		var service *Service

		ginkgo.BeforeEach(func() {
			service = &Service{}
		})

		ginkgo.It("should return discord scheme", func() {
			id := service.GetID()

			gomega.Expect(id).To(gomega.Equal(Scheme))
			gomega.Expect(id).To(gomega.Equal("discord"))
		})
	})

	ginkgo.Describe("Service.doSend method", func() {
		var (
			service    *Service
			mockClient *mockServiceHTTPClient
		)

		ginkgo.BeforeEach(func() {
			service = &Service{
				HTTPClient: &mockServiceHTTPClient{},
			}
			mockClient = service.HTTPClient.(*mockServiceHTTPClient)
			mockClient.response = &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}
		})

		ginkgo.It("should send payload successfully", func() {
			payload := []byte(`{"content":"test"}`)
			postURL := testWebhookURLAlt

			err := service.doSend(payload, postURL)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
		})

		ginkgo.It("should handle empty URL", func() {
			payload := []byte(`{"content":"test"}`)
			postURL := ""

			err := service.doSend(payload, postURL)

			gomega.Expect(err).To(gomega.MatchError(ErrEmptyURL))
		})

		ginkgo.It("should validate URL scheme", func() {
			payload := []byte(`{"content":"test"}`)
			postURL := "http://" + testWebhookURLAlt[8:] // Change https to http

			err := service.doSend(payload, postURL)

			gomega.Expect(err).To(gomega.MatchError(ErrInvalidScheme))
		})

		ginkgo.It("should validate host", func() {
			payload := []byte(`{"content":"test"}`)
			postURL := "https://example.com" + testWebhookURLAlt[20:] // Replace discord.com with example.com

			err := service.doSend(payload, postURL)

			gomega.Expect(err).To(gomega.MatchError(ErrInvalidHost))
		})

		ginkgo.It("should validate URL prefix", func() {
			payload := []byte(`{"content":"test"}`)
			postURL := "https://discord.com/api/invalid" + testWebhookURLAlt[28:] // Replace /webhooks with /invalid

			err := service.doSend(payload, postURL)

			gomega.Expect(err).To(gomega.MatchError(ErrInvalidURLPrefix))
		})

		ginkgo.It("should validate webhook ID and token", func() {
			payload := []byte(`{"content":"test"}`)
			postURL := "https://discord.com/api/webhooks//"

			err := service.doSend(payload, postURL)

			gomega.Expect(err).To(gomega.MatchError(ErrMalformedURL))
		})

		ginkgo.It("should handle HTTP client error", func() {
			mockClient.err = errors.New("network error")
			payload := []byte(`{"content":"test"}`)
			postURL := testWebhookURLAlt

			err := service.doSend(payload, postURL)

			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Service.doSendMultipart method", func() {
		var (
			service    *Service
			mockClient *mockServiceHTTPClient
		)

		ginkgo.BeforeEach(func() {
			service = &Service{
				HTTPClient: &mockServiceHTTPClient{},
			}
			mockClient = service.HTTPClient.(*mockServiceHTTPClient)
			mockClient.response = &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}
		})

		ginkgo.It("should send multipart payload with files successfully", func() {
			payload := WebhookPayload{
				Content: "Message with files",
			}
			files := []types.File{
				{Name: "test.txt", Data: []byte("file content")},
			}
			postURL := testWebhookURLAlt

			err := service.doSendMultipart(payload, files, postURL)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
		})

		ginkgo.It("should handle empty files array", func() {
			payload := WebhookPayload{
				Content: "Message without files",
			}
			files := []types.File{}
			postURL := testWebhookURLAlt

			err := service.doSendMultipart(payload, files, postURL)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
		})

		ginkgo.It("should handle HTTP client error", func() {
			mockClient.err = errors.New("network error")
			payload := WebhookPayload{
				Content: "Test message",
			}
			files := []types.File{}
			postURL := testWebhookURLAlt

			err := service.doSendMultipart(payload, files, postURL)

			gomega.Expect(err).To(gomega.HaveOccurred())
		})
	})
})

// mockServiceHTTPClient is a test helper that implements HTTPClient interface.
type mockServiceHTTPClient struct {
	response  *http.Response
	err       error
	callCount int
}

func (m *mockServiceHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

// mockLogger is a test helper that implements StdLogger interface.
type mockLogger struct{}

func (m *mockLogger) Print(_ ...any)            {}
func (m *mockLogger) Printf(_ string, _ ...any) {}
func (m *mockLogger) Println(_ ...any)          {}
