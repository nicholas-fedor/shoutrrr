package zulip

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	ginkgo "github.com/onsi/ginkgo/v2"

	"github.com/nicholas-fedor/shoutrrr/internal/testutils"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/zulip/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// mockLogger is a test helper that implements StdLogger interface.
type mockLogger struct{}

var _ = ginkgo.Describe("Service Unit Tests", func() {
	ginkgo.Describe("NewDefaultHTTPClient", func() {
		ginkgo.It("should create a non-nil client", func() {
			client := NewDefaultHTTPClient()

			gomega.Expect(client).NotTo(gomega.BeNil())
		})

		ginkgo.It("should create a client with the expected timeout", func() {
			client := NewDefaultHTTPClient()

			gomega.Expect(client.client.Timeout).To(gomega.Equal(defaultHTTPTimeout))
		})
	})

	ginkgo.Describe("DefaultHTTPClient.Do", func() {
		var client *DefaultHTTPClient

		ginkgo.BeforeEach(func() {
			client = NewDefaultHTTPClient()
			httpmock.ActivateNonDefault(client.client)
		})

		ginkgo.AfterEach(func() {
			httpmock.DeactivateAndReset()
		})

		ginkgo.It("should successfully make an HTTP request", func() {
			httpmock.RegisterResponder(http.MethodPost, "https://zulip.example.com/api/v1/messages",
				httpmock.NewStringResponder(http.StatusOK, `{"result": "success"}`))

			req, err := http.NewRequest(http.MethodPost, "https://zulip.example.com/api/v1/messages", http.NoBody)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resp, err := client.Do(req)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(resp.StatusCode).To(gomega.Equal(http.StatusOK))
		})

		ginkgo.It("should return an error when the request fails", func() {
			httpmock.RegisterResponder(http.MethodPost, "https://zulip.example.com/api/v1/messages",
				httpmock.NewErrorResponder(errors.New("network error")))

			req, err := http.NewRequest(http.MethodPost, "https://zulip.example.com/api/v1/messages", http.NoBody)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			resp, err := client.Do(req)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(resp).To(gomega.BeNil())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("performing HTTP request"))
		})
	})

	ginkgo.Describe("GetID", func() {
		ginkgo.It("should return the zulip scheme identifier", func() {
			svc := &Service{}

			gomega.Expect(svc.GetID()).To(gomega.Equal(Scheme))
		})

		ginkgo.It("should always return 'zulip' regardless of service state", func() {
			svc := &Service{
				Config: &Config{
					Host: "zulip.example.com",
				},
			}

			gomega.Expect(svc.GetID()).To(gomega.Equal("zulip"))
		})
	})

	ginkgo.Describe("Initialize", func() {
		ginkgo.It("should initialize from a valid URL", func() {
			svc := &Service{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?stream=general&topic=announcements")

			err := svc.Initialize(serviceURL, testutils.TestLogger())

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(svc.Config).NotTo(gomega.BeNil())
			gomega.Expect(svc.Config.BotMail).To(gomega.Equal("bot@example.com"))
			gomega.Expect(svc.Config.BotKey).To(gomega.Equal("secret-key"))
			gomega.Expect(svc.Config.Host).To(gomega.Equal("zulip.example.com"))
			gomega.Expect(svc.Config.Stream).To(gomega.Equal("general"))
			gomega.Expect(svc.Config.Topic).To(gomega.Equal("announcements"))
			gomega.Expect(svc.HTTPClient).NotTo(gomega.BeNil())
			gomega.Expect(svc.contentMaxSize).To(gomega.Equal(contentMaxSize))
			gomega.Expect(svc.topicMaxLength).To(gomega.Equal(topicMaxLength))
		})

		ginkgo.It("should set the logger", func() {
			svc := &Service{}
			logger := testutils.TestLogger()
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com")

			err := svc.Initialize(serviceURL, logger)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(svc.Logger).NotTo(gomega.BeNil())
		})

		ginkgo.It("should return an error for URL with missing bot mail", func() {
			svc := &Service{}
			serviceURL := testutils.URLMust("zulip://:secret-key@zulip.example.com")

			err := svc.Initialize(serviceURL, testutils.TestLogger())

			gomega.Expect(err).To(gomega.MatchError(ErrMissingBotMail))
		})

		ginkgo.It("should return an error for URL with missing API key", func() {
			svc := &Service{}
			serviceURL := testutils.URLMust("zulip://bot@example.com@zulip.example.com")

			err := svc.Initialize(serviceURL, testutils.TestLogger())

			gomega.Expect(err).To(gomega.MatchError(ErrMissingAPIKey))
		})

		ginkgo.It("should return an error for URL with missing host", func() {
			svc := &Service{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@")

			err := svc.Initialize(serviceURL, testutils.TestLogger())

			gomega.Expect(err).To(gomega.MatchError(ErrMissingHost))
		})
	})

	ginkgo.Describe("Send", func() {
		ginkgo.It("should send a message successfully", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.Send("Test message", nil)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return an error when HTTP client fails", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("network error")).Once()

			service := newTestService(mockClient)

			err := service.Send("Test message", nil)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("making HTTP POST request"))
		})

		ginkgo.It("should return an error for non-200 response status", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.Send("Test message", nil)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("401")))
		})

		ginkgo.It("should return an error when topic exceeds max length", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := newTestService(mockClient)
			longTopic := strings.Repeat("a", topicMaxLength+1)

			err := service.Send("Test message", &types.Params{"topic": longTopic})

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrTopicTooLong))
		})

		ginkgo.It("should accept topic at exactly max length", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)
			exactTopic := strings.Repeat("a", topicMaxLength)

			err := service.Send("Test message", &types.Params{"topic": exactTopic})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should count topic length in runes, not bytes", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)
			// 60 multi-byte characters (3 bytes each = 180 bytes, but only 60 runes)
			unicodeTopic := strings.Repeat("日", topicMaxLength) //nolint:gosmopolitan // unicode topic test

			err := service.Send("Test message", &types.Params{"topic": unicodeTopic})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should reject topic exceeding max length in runes", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := newTestService(mockClient)
			// 61 multi-byte characters = 61 runes > 60 max
			unicodeTopic := strings.Repeat("日", topicMaxLength+1) //nolint:gosmopolitan // unicode topic test

			err := service.Send("Test message", &types.Params{"topic": unicodeTopic})

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrTopicTooLong))
		})

		ginkgo.It("should use title as topic when topic is not set", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := &Service{
				Config: &Config{
					BotMail: "bot@example.com",
					BotKey:  "secret-key",
					Host:    "zulip.example.com",
					Stream:  "general",
				},
				HTTPClient: mockClient,
			}
			service.SetLogger(&mockLogger{})
			service.pkr = format.NewPropKeyResolver(service.Config)
			service.contentMaxSize = contentMaxSize
			service.topicMaxLength = topicMaxLength
			service.mu.Do(func() {})

			err := service.Send("Test message", &types.Params{"title": "My Topic"})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			body, _ := io.ReadAll(capturedReq.Body)
			gomega.Expect(string(body)).To(gomega.ContainSubstring("topic=My+Topic"))

			mockClient.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should not override topic with title when both are set", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.Send("Test message", &types.Params{"title": "Notification Title"})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			body, _ := io.ReadAll(capturedReq.Body)
			gomega.Expect(string(body)).To(gomega.ContainSubstring("topic=announcements"))
			gomega.Expect(string(body)).To(gomega.ContainSubstring("content=Notification+Title"))

			mockClient.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should prepend title to message when both topic and title are set", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.Send("body text", &types.Params{"title": "My Title"})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			body, _ := io.ReadAll(capturedReq.Body)
			gomega.Expect(string(body)).To(gomega.ContainSubstring("topic=announcements"))
			gomega.Expect(string(body)).To(gomega.ContainSubstring("content=My+Title%0A%0Abody+text"))

			mockClient.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should return error when title used as topic fallback exceeds max length", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := &Service{
				Config: &Config{
					BotMail: "bot@example.com",
					BotKey:  "secret-key",
					Host:    "zulip.example.com",
					Stream:  "general",
				},
				HTTPClient: mockClient,
			}
			service.SetLogger(&mockLogger{})
			service.pkr = format.NewPropKeyResolver(service.Config)
			service.contentMaxSize = contentMaxSize
			service.topicMaxLength = topicMaxLength
			service.mu.Do(func() {})

			longTitle := strings.Repeat("a", topicMaxLength+1)
			err := service.Send("Test message", &types.Params{"title": longTitle})

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrTopicTooLong))
		})

		ginkgo.It("should handle empty title param gracefully", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.Send("Test message", &types.Params{"title": ""})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			body, _ := io.ReadAll(capturedReq.Body)
			gomega.Expect(string(body)).To(gomega.ContainSubstring("topic=announcements"))

			mockClient.AssertExpectations(ginkgo.GinkgoT())
		})

		ginkgo.It("should return an error when message exceeds max size", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := newTestService(mockClient)
			longMessage := strings.Repeat("a", contentMaxSize+1)

			err := service.Send(longMessage, nil)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrMessageTooLong))
		})

		ginkgo.It("should accept message at exactly max size", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)
			exactMessage := strings.Repeat("a", contentMaxSize)

			err := service.Send(exactMessage, nil)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should error for invalid message type", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := newTestService(mockClient)

			err := service.Send("Test message", &types.Params{"type": "invalid"})

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("invalid message type"))
		})

		ginkgo.It("should error when no stream is set for channel message", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := &Service{
				Config: &Config{
					BotMail: "bot@example.com",
					BotKey:  "secret-key",
					Host:    "zulip.example.com",
				},
				HTTPClient: mockClient,
			}
			service.SetLogger(&mockLogger{})
			service.pkr = format.NewPropKeyResolver(service.Config)
			service.contentMaxSize = contentMaxSize
			service.topicMaxLength = topicMaxLength
			service.mu.Do(func() {})

			err := service.Send("Test message", nil)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrMissingRecipient))
		})
	})

	ginkgo.Describe("SendWithContext", func() {
		ginkgo.It("should send a message with background context", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.SendWithContext(context.Background(), "Test message", nil)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should pass context to the HTTP request", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.SendWithContext(context.Background(), "Test message", nil)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(capturedReq).NotTo(gomega.BeNil())
		})

		ginkgo.It("should return an error when HTTP client returns error", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("context canceled")).Once()

			service := newTestService(mockClient)

			err := service.SendWithContext(context.Background(), "Test message", nil)

			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("should not modify the original config", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Maybe()

			service := newTestService(mockClient)
			originalTopic := service.Config.Topic

			err := service.SendWithContext(context.Background(), "Test message", &types.Params{"topic": "different-topic"})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Topic).To(gomega.Equal(originalTopic))
		})

		ginkgo.It("should return an error for invalid host", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := newTestService(mockClient)
			service.Config.Host = "invalid host with spaces"

			err := service.SendWithContext(context.Background(), "Test message", nil)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidHost))
		})

		ginkgo.It("should accept host with valid port", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)
			service.Config.Host = "zulip.example.com:8443"

			err := service.SendWithContext(context.Background(), "Test message", nil)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should reject host with invalid port format", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := newTestService(mockClient)
			service.Config.Host = "zulip.example.com:abc"

			err := service.SendWithContext(context.Background(), "Test message", nil)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrInvalidHost))
		})

		ginkgo.It("should send direct message with type=direct and to param as JSON", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)
			service.Config.Stream = "" // no stream, use to

			err := service.SendWithContext(context.Background(), "Test direct", &types.Params{
				"type": "direct",
				"to":   "user1@example.com,user2@example.com",
			})

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			body, _ := io.ReadAll(capturedReq.Body)
			gomega.Expect(string(body)).To(gomega.ContainSubstring("type=direct"))
			gomega.Expect(string(body)).To(gomega.ContainSubstring(`to=%5B%22user1%40example.com%22%2C%22user2%40example.com%22%5D`))
		})

		ginkgo.It("should error when no recipient for direct message", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			service := newTestService(mockClient)
			service.Config.Stream = ""

			err := service.SendWithContext(context.Background(), "Test message", &types.Params{"type": "direct"})

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrMissingRecipient))
		})
	})

	ginkgo.Describe("doSend", func() {
		ginkgo.It("should send to the correct API URL path", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.doSend(context.Background(), service.Config, "Test message")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(capturedReq).NotTo(gomega.BeNil())
			gomega.Expect(capturedReq.URL.Path).To(gomega.Equal("/api/v1/messages"))
		})

		ginkgo.It("should set content type header", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.doSend(context.Background(), service.Config, "Test message")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(capturedReq.Header.Get("Content-Type")).To(
				gomega.Equal("application/x-www-form-urlencoded"),
			)
		})

		ginkgo.It("should send form-encoded payload", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())

			var capturedReq *http.Request

			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Run(func(args mock.Arguments) {
				capturedReq = args.Get(0).(*http.Request)
			}).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.doSend(context.Background(), service.Config, "Hello World")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			body, readErr := io.ReadAll(capturedReq.Body)
			gomega.Expect(readErr).NotTo(gomega.HaveOccurred())
			gomega.Expect(string(body)).To(gomega.ContainSubstring("type=channel"))
			gomega.Expect(string(body)).To(gomega.ContainSubstring("to=general"))
			gomega.Expect(string(body)).To(gomega.ContainSubstring("content=Hello+World"))
			gomega.Expect(string(body)).To(gomega.ContainSubstring("topic=announcements"))
		})

		ginkgo.It("should proceed with host validation already done by caller", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Once()

			service := newTestService(mockClient)
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
				Stream:  "general",
			}

			err := service.doSend(context.Background(), cfg, "Test message")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("should return an error for non-200 response", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil).Once()

			service := newTestService(mockClient)

			err := service.doSend(context.Background(), service.Config, "Test message")

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrResponseStatusFailure))
		})

		ginkgo.It("should return an error when HTTP client fails", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(nil, errors.New("connection refused")).Once()

			service := newTestService(mockClient)

			err := service.doSend(context.Background(), service.Config, "Test message")

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("making HTTP POST request"))
		})
	})

	ginkgo.Describe("getAPIURL", func() {
		ginkgo.It("should construct the correct API URL", func() {
			svc := &Service{}
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			apiURL := svc.getAPIURL(cfg)

			parsed, err := url.Parse(apiURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(parsed.Scheme).To(gomega.Equal("https"))
			gomega.Expect(parsed.Host).To(gomega.Equal("zulip.example.com"))
			gomega.Expect(parsed.Path).To(gomega.Equal("/api/v1/messages"))
			gomega.Expect(parsed.User.Username()).To(gomega.Equal("bot@example.com"))
			password, isSet := parsed.User.Password()
			gomega.Expect(isSet).To(gomega.BeTrue())
			gomega.Expect(password).To(gomega.Equal("secret-key"))
		})

		ginkgo.It("should use https scheme", func() {
			svc := &Service{}
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			apiURL := svc.getAPIURL(cfg)

			gomega.Expect(apiURL).To(gomega.HavePrefix("https://"))
		})

		ginkgo.It("should include credentials in the URL", func() {
			svc := &Service{}
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			apiURL := svc.getAPIURL(cfg)

			parsed, err := url.Parse(apiURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(parsed.User.Username()).To(gomega.Equal("bot@example.com"))
			password, isSet := parsed.User.Password()
			gomega.Expect(isSet).To(gomega.BeTrue())
			gomega.Expect(password).To(gomega.Equal("secret-key"))
		})

		ginkgo.It("should always use /api/v1/messages path", func() {
			svc := &Service{}
			cfg := &Config{
				BotMail: "bot@example.com",
				BotKey:  "secret-key",
				Host:    "zulip.example.com",
			}

			apiURL := svc.getAPIURL(cfg)

			parsed, err := url.Parse(apiURL)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(parsed.Path).To(gomega.Equal("/api/v1/messages"))
		})

		ginkgo.It("should use server-side size limits from register endpoint", func() {
			httpmock.Activate()

			defer httpmock.DeactivateAndReset()

			registerURL := (&url.URL{
				User:   url.UserPassword("bot@example.com", "secret-key"),
				Host:   "zulip.example.com",
				Path:   "/api/v1/register",
				Scheme: "https",
			}).String()
			httpmock.RegisterResponder(
				"POST",
				registerURL,
				httpmock.NewStringResponder(http.StatusOK, `{"max_message_length":5000,"max_topic_length":30}`),
			)

			apiURL := (&url.URL{
				Host:   "zulip.example.com",
				Path:   "/api/v1/messages",
				Scheme: "https",
				User:   url.UserPassword("bot@example.com", "secret-key"),
			}).String()
			httpmock.RegisterResponder(
				"POST",
				apiURL,
				httpmock.NewStringResponder(http.StatusOK, ""),
			)

			svc := &Service{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?stream=general")
			err := svc.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			longMessage := strings.Repeat("a", 5001)
			err = svc.Send(longMessage, nil)
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.Equal("message exceeds max size: 5000 bytes, got 5001 bytes"))
		})

		ginkgo.It("should fall back to defaults when register fails", func() {
			httpmock.Activate()

			defer httpmock.DeactivateAndReset()

			registerURL := (&url.URL{
				User:   url.UserPassword("bot@example.com", "secret-key"),
				Host:   "zulip.example.com",
				Path:   "/api/v1/register",
				Scheme: "https",
			}).String()
			httpmock.RegisterResponder(
				"POST",
				registerURL,
				httpmock.NewStringResponder(http.StatusServiceUnavailable, ""),
			)

			apiURL := (&url.URL{
				Host:   "zulip.example.com",
				Path:   "/api/v1/messages",
				Scheme: "https",
				User:   url.UserPassword("bot@example.com", "secret-key"),
			}).String()
			httpmock.RegisterResponder(
				"POST",
				apiURL,
				httpmock.NewStringResponder(http.StatusOK, ""),
			)

			svc := &Service{}
			serviceURL := testutils.URLMust("zulip://bot@example.com:secret-key@zulip.example.com?stream=general")
			err := svc.Initialize(serviceURL, testutils.TestLogger())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			err = svc.Send("test message", nil)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})
	})

	ginkgo.Describe("Params update", func() {
		ginkgo.It("should update config from params without mutating original", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			mockClient.On("Do", mock.AnythingOfType("*http.Request")).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"result": "success"}`))),
			}, nil).Maybe()

			service := newTestService(mockClient)
			originalTopic := service.Config.Topic

			params := types.Params{"topic": "new-topic"}

			err := service.Send("Test message", &params)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(service.Config.Topic).To(gomega.Equal(originalTopic))
		})
	})
})

func (m *mockLogger) Print(_ ...any)            {}
func (m *mockLogger) Printf(_ string, _ ...any) {}
func (m *mockLogger) Println(_ ...any)          {}

func newTestService(mockClient *mocks.MockHTTPClient) *Service {
	service := &Service{
		Config: &Config{
			BotMail: "bot@example.com",
			BotKey:  "secret-key",
			Host:    "zulip.example.com",
			Stream:  "general",
			Topic:   "announcements",
		},
		HTTPClient: mockClient,
	}
	service.SetLogger(&mockLogger{})
	service.pkr = format.NewPropKeyResolver(service.Config)
	service.contentMaxSize = contentMaxSize
	service.topicMaxLength = topicMaxLength
	// Prime the Once so fetchLimits (which would call HTTPClient) is skipped in mock-based unit tests.
	service.mu.Do(func() {})

	return service
}
