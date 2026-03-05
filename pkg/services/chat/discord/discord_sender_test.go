package discord

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// mockTemporaryError implements net.Error for testing.
type mockTemporaryError struct{}

// mockTimeoutError implements net.Error for testing timeout.
type mockTimeoutError struct{}

var _ = ginkgo.Describe("Discord Sender", func() {
	ginkgo.Describe("NewDefaultHTTPClient", func() {
		ginkgo.It("should create a client with proper timeout", func() {
			client := NewDefaultHTTPClient()
			gomega.Expect(client).NotTo(gomega.BeNil())
			gomega.Expect(client.client).NotTo(gomega.BeNil())
			gomega.Expect(client.client.Timeout).To(gomega.Equal(30 * time.Second))
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

		ginkgo.It("should successfully make HTTP request", func() {
			httpmock.RegisterResponder("GET", "http://example.com",
				httpmock.NewStringResponder(200, "success"))

			req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
			resp, err := client.Do(req)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(resp.StatusCode).To(gomega.Equal(200))
			body, _ := io.ReadAll(resp.Body)
			gomega.Expect(string(body)).To(gomega.Equal("success"))
		})

		ginkgo.It("should handle network errors", func() {
			httpmock.RegisterResponder("GET", "http://example.com",
				httpmock.NewErrorResponder(errors.New("network error")))

			req, _ := http.NewRequest(http.MethodGet, "http://example.com", http.NoBody)
			_, err := client.Do(req)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("performing HTTP request"))
		})
	})

	ginkgo.Describe("JSONRequestPreparer.PrepareRequest", func() {
		var (
			preparer *JSONRequestPreparer
			payload  []byte
		)

		ginkgo.BeforeEach(func() {
			payload = []byte(`{"content":"test message"}`)
			preparer = &JSONRequestPreparer{payload: payload}
		})

		ginkgo.It("should create a valid JSON POST request", func() {
			ctx := context.Background()
			url := testWebhookURLAlt

			req, err := preparer.PrepareRequest(ctx, url)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(req.Method).To(gomega.Equal("POST"))
			gomega.Expect(req.URL.String()).To(gomega.Equal(url))
			gomega.Expect(req.Header.Get("Content-Type")).To(gomega.Equal("application/json"))

			body, _ := io.ReadAll(req.Body)
			gomega.Expect(body).To(gomega.Equal(payload))
		})

		ginkgo.It("should handle context properly", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			url := testWebhookURLAlt
			_, err := preparer.PrepareRequest(ctx, url)

			gomega.Expect(err).
				ToNot(gomega.HaveOccurred())
			// http.NewRequestWithContext doesn't fail on canceled context
		})
	})

	ginkgo.Describe("MultipartRequestPreparer.PrepareRequest", func() {
		var preparer *MultipartRequestPreparer

		ginkgo.BeforeEach(func() {
			payload := &WebhookPayload{
				Content: "test message",
			}
			files := []types.File{
				{Name: "test.txt", Data: []byte("file content")},
			}
			preparer = &MultipartRequestPreparer{
				payload: payload,
				files:   files,
			}
		})

		ginkgo.It("should create a valid multipart POST request", func() {
			ctx := context.Background()
			url := testWebhookURLAlt

			req, err := preparer.PrepareRequest(ctx, url)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(req.Method).To(gomega.Equal("POST"))
			gomega.Expect(req.URL.String()).To(gomega.Equal(url))
			gomega.Expect(req.Header.Get("Content-Type")).
				To(gomega.ContainSubstring("multipart/form-data"))

			body, _ := io.ReadAll(req.Body)
			gomega.Expect(body).ToNot(gomega.BeEmpty())

			// Check if payload_json is in the body
			bodyStr := string(body)
			gomega.Expect(bodyStr).To(gomega.ContainSubstring("payload_json"))
			gomega.Expect(bodyStr).To(gomega.ContainSubstring("test message"))
			gomega.Expect(bodyStr).To(gomega.ContainSubstring("files[0]"))
			gomega.Expect(bodyStr).To(gomega.ContainSubstring("test.txt"))
		})

		ginkgo.It("should handle empty files", func() {
			preparer.files = []types.File{}
			ctx := context.Background()
			url := testWebhookURLAlt

			req, err := preparer.PrepareRequest(ctx, url)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())

			body, _ := io.ReadAll(req.Body)
			bodyStr := string(body)
			gomega.Expect(bodyStr).To(gomega.ContainSubstring("payload_json"))
			gomega.Expect(bodyStr).NotTo(gomega.ContainSubstring("files["))
		})
	})

	ginkgo.Describe("sendWithRetry", func() {
		var preparer *JSONRequestPreparer

		ginkgo.BeforeEach(func() {
			payload := []byte(`{"content":"test"}`)
			preparer = &JSONRequestPreparer{payload: payload}
		})

		ginkgo.It("should succeed on first attempt", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())

			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil).Once()

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})

		ginkgo.It("should succeed on first attempt with StatusOK", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())

			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil).Once()

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})

		ginkgo.It("should handle unexpected status codes", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())

			mockClient.On("Do", mock.Anything).Return(&http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil).Once()

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("unexpected response status code"))
		})

		ginkgo.It("should handle nil response", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())

			mockClient.On("Do", mock.Anything).Return((*http.Response)(nil), nil).Once()

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).To(gomega.MatchError(ErrUnknownAPIError))
		})

		ginkgo.It("should handle HTTP client errors", func() {
			mockClient := mocks.NewMockHTTPClient(ginkgo.GinkgoT())
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())

			mockClient.On("Do", mock.Anything).Return(nil, http.ErrHandlerTimeout).Once()

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("making HTTP POST request"))
		})
	})

	ginkgo.Describe("handleRateLimitResponse", func() {
		ginkgo.It("should handle retry-after header", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			resp.Header.Set("Retry-After", "2")

			startTime := time.Now()
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())
			sleeper.On("Sleep", 2*time.Second).Return().Once()

			err := handleRateLimitResponse(context.Background(), resp, 0, startTime, sleeper)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})

		ginkgo.It("should handle invalid retry-after header", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			resp.Header.Set("Retry-After", "invalid")

			startTime := time.Now()
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())
			sleeper.On("Sleep", time.Second).Return().Once()

			err := handleRateLimitResponse(context.Background(), resp, 0, startTime, sleeper)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// Should fall back to exponential backoff
		})

		ginkgo.It("should handle retry-after exceeding max timeout", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			resp.Header.Set("Retry-After", "400") // Exceeds 5 minutes

			startTime := time.Now()
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())

			err := handleRateLimitResponse(context.Background(), resp, 0, startTime, sleeper)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrRateLimited))
		})

		ginkgo.It("should handle exponential backoff", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			// No retry-after header

			startTime := time.Now()
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())
			sleeper.On("Sleep", 4*time.Second).Return().Once()

			err := handleRateLimitResponse(
				context.Background(),
				resp,
				2,
				startTime,
				sleeper,
			)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})

		ginkgo.It("should cap exponential backoff", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}

			startTime := time.Now()
			sleeper := mocks.NewMockSleeper(ginkgo.GinkgoT())
			sleeper.On("Sleep", 64*time.Second).Return().Once()

			err := handleRateLimitResponse(
				context.Background(),
				resp,
				6,
				startTime,
				sleeper,
			)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})
	})
})

func TestSendWithRetryRateLimitRetryAfter(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

		mockClient := mocks.NewMockHTTPClient(t)
		payload := []byte(`{"content":"test"}`)
		preparer := &JSONRequestPreparer{payload: payload}
		sleeper := mocks.NewMockSleeper(t)

		// First call returns 429 with retry-after
		resp1 := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
		}
		resp1.Header.Set("Retry-After", "1")

		mockClient.On("Do", mock.Anything).Return(resp1, nil).Once()
		mockClient.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil).Once()
		sleeper.On("Sleep", time.Second).Return().Once()

		ctx := context.Background()
		err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	})
}

func TestSendWithRetryMaxRetriesExceeded(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

		mockClient := mocks.NewMockHTTPClient(t)
		payload := []byte(`{"content":"test"}`)
		preparer := &JSONRequestPreparer{payload: payload}
		sleeper := mocks.NewMockSleeper(t)

		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
			Body:       io.NopCloser(strings.NewReader("")),
		}

		// Expect 6 calls (MaxRetries + 1)
		mockClient.On("Do", mock.Anything).Return(resp, nil).Times(6)
		// Expect 5 sleeps with exponential backoff
		sleeper.On("Sleep", time.Second).Return().Once()
		sleeper.On("Sleep", 2*time.Second).Return().Once()
		sleeper.On("Sleep", 4*time.Second).Return().Once()
		sleeper.On("Sleep", 8*time.Second).Return().Once()
		sleeper.On("Sleep", 16*time.Second).Return().Once()

		ctx := context.Background()
		err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err).To(gomega.MatchError(ErrMaxRetries))
	})
}

func TestSendWithRetryMaxRetryTimeout(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

		mockClient := mocks.NewMockHTTPClient(t)
		payload := []byte(`{"content":"test"}`)
		preparer := &JSONRequestPreparer{payload: payload}
		sleeper := mocks.NewMockSleeper(t)

		// Mock a very long retry-after that exceeds MaxRetryTimeout
		resp := &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("")),
		}
		resp.Header.Set("Retry-After", "400") // Exceeds 5 minutes

		mockClient.On("Do", mock.Anything).Return(resp, nil).Once()

		ctx := context.Background()
		err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("rate limited by Discord"))
	})
}

// TestIsTransientError tests the isTransientError function with table-driven tests.
func TestIsTransientError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "context canceled should not be transient",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context deadline exceeded should be transient",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "timeout network error should be transient",
			err:      &mockTimeoutError{},
			expected: true,
		},
		{
			name:     "temporary network error should be transient",
			err:      &mockTemporaryError{},
			expected: true,
		},
		{
			name:     "connection refused should be transient",
			err:      errors.New("dial tcp 127.0.0.1:8080: connect: connection refused"),
			expected: true,
		},
		{
			name:     "connection reset should be transient",
			err:      errors.New("read tcp 127.0.0.1:8080: connection reset by peer"),
			expected: true,
		},
		{
			name: "wrapped transient error should be transient",
			err: &url.Error{
				Op:  "Get",
				URL: "http://example.com",
				Err: context.DeadlineExceeded,
			},
			expected: true,
		},
		{
			name:     "generic error should not be transient",
			err:      errors.New("some other error"),
			expected: false,
		},
		{
			name:     "nil error should not be transient",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := isTransientError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func (m *mockTemporaryError) Error() string   { return "temporary error" }
func (m *mockTemporaryError) Temporary() bool { return true }
func (m *mockTemporaryError) Timeout() bool   { return false }

func (m *mockTimeoutError) Error() string   { return "timeout error" }
func (m *mockTimeoutError) Temporary() bool { return false }
func (m *mockTimeoutError) Timeout() bool   { return true }

// TestExecuteWithTransportRetry tests the executeWithTransportRetry function.
func TestExecuteWithTransportRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupClient    func(t *testing.T) HTTPClient
		setupSleeper   func(t *testing.T) *mocks.MockSleeper
		expectedError  bool
		expectedCalls  int
		expectedSleeps []time.Duration
	}{
		{
			name: "successful request on first attempt",
			setupClient: func(t *testing.T) HTTPClient {
				t.Helper()
				mockClient := mocks.NewMockHTTPClient(t)
				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil).Once()

				return mockClient
			},
			setupSleeper: func(t *testing.T) *mocks.MockSleeper {
				t.Helper()

				return mocks.NewMockSleeper(t)
			},
			expectedError:  false,
			expectedCalls:  1,
			expectedSleeps: nil,
		},
		{
			name: "transient error with retry success",
			setupClient: func(t *testing.T) HTTPClient {
				t.Helper()
				mockClient := mocks.NewMockHTTPClient(t)
				mockClient.On("Do", mock.Anything).Return(nil, context.DeadlineExceeded).Once()
				mockClient.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil).Once()

				return mockClient
			},
			setupSleeper: func(t *testing.T) *mocks.MockSleeper {
				t.Helper()
				sleeper := mocks.NewMockSleeper(t)
				sleeper.On("Sleep", time.Second).Return().Once()

				return sleeper
			},
			expectedError:  false,
			expectedCalls:  2,
			expectedSleeps: []time.Duration{time.Second},
		},
		{
			name: "persistent error after max retries",
			setupClient: func(t *testing.T) HTTPClient {
				t.Helper()
				mockClient := mocks.NewMockHTTPClient(t)
				// Expect 4 calls (maxTransportRetries + 1)
				mockClient.On("Do", mock.Anything).Return(nil, context.DeadlineExceeded).Times(4)

				return mockClient
			},
			setupSleeper: func(t *testing.T) *mocks.MockSleeper {
				t.Helper()
				sleeper := mocks.NewMockSleeper(t)
				sleeper.On("Sleep", time.Second).Return().Once()
				sleeper.On("Sleep", 2*time.Second).Return().Once()
				sleeper.On("Sleep", 4*time.Second).Return().Once()

				return sleeper
			},
			expectedError: true,
			// Expected calls: maxTransportRetries + 1 = 4
			expectedCalls:  4,
			expectedSleeps: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second},
		},
		{
			name: "context canceled during retry",
			setupClient: func(t *testing.T) HTTPClient {
				t.Helper()
				mockClient := mocks.NewMockHTTPClient(t)
				mockClient.On("Do", mock.Anything).Return(nil, context.DeadlineExceeded).Once()

				return mockClient
			},
			setupSleeper: func(t *testing.T) *mocks.MockSleeper {
				t.Helper()

				return mocks.NewMockSleeper(t)
			},
			expectedError:  true,
			expectedCalls:  1,
			expectedSleeps: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := tt.setupClient(t)
			sleeper := tt.setupSleeper(t)
			ctx := context.Background()

			if tt.name == "context canceled during retry" {
				var cancel context.CancelFunc

				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			req, _ := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				"http://example.com",
				strings.NewReader("test"),
			)
			bodyBytes := []byte("test")

			_, err := executeWithTransportRetry(ctx, client, bodyBytes, req, sleeper)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestHandleServerError tests the handleServerError function.
func TestHandleServerError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		statusCode    int
		attempt       int
		expectedError bool
		setupSleeper  func(t *testing.T) Sleeper
	}{
		{
			name:          "status code below 500 should not wait",
			statusCode:    404,
			attempt:       0,
			expectedError: false,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()

				return mocks.NewMockSleeper(t)
			},
		},
		{
			name:          "500 status code should wait",
			statusCode:    500,
			attempt:       0,
			expectedError: false,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()
				sleeper := mocks.NewMockSleeper(t)
				sleeper.On("Sleep", time.Second).Return().Once()

				return sleeper
			},
		},
		{
			name:          "max retries exceeded should return error",
			statusCode:    500,
			attempt:       maxRetries,
			expectedError: true,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()

				return mocks.NewMockSleeper(t)
			},
		},
		{
			name:          "exponential backoff calculation",
			statusCode:    502,
			attempt:       2,
			expectedError: false,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()
				sleeper := mocks.NewMockSleeper(t)
				sleeper.On("Sleep", 4*time.Second).Return().Once()

				return sleeper
			},
		},
		{
			name:          "exponential backoff calculation for higher attempt",
			statusCode:    503,
			attempt:       4,
			expectedError: false,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()
				sleeper := mocks.NewMockSleeper(t)
				sleeper.On("Sleep", 16*time.Second).Return().Once()

				return sleeper
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader("")),
			}
			sleeper := tt.setupSleeper(t)
			startTime := time.Now()

			err := handleServerError(context.Background(), resp, tt.attempt, startTime, sleeper)

			if tt.expectedError {
				require.Error(t, err)
				assert.Equal(t, ErrMaxRetries, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestWaitWithTimeout tests the waitWithTimeout function.
func TestWaitWithTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		wait          time.Duration
		startTime     time.Time
		cancelContext bool
		expectedError bool
		setupSleeper  func(t *testing.T) Sleeper
	}{
		{
			name:          "normal wait should succeed",
			wait:          time.Second,
			startTime:     time.Now(),
			cancelContext: false,
			expectedError: false,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()
				sleeper := mocks.NewMockSleeper(t)
				sleeper.On("Sleep", time.Second).Return().Once()

				return sleeper
			},
		},
		{
			name:          "wait exceeding timeout should fail",
			wait:          6 * time.Minute, // Exceeds maxRetryTimeout
			startTime:     time.Now(),
			cancelContext: false,
			expectedError: true,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()

				return mocks.NewMockSleeper(t)
			},
		},
		{
			name:          "context canceled should fail",
			wait:          time.Second,
			startTime:     time.Now(),
			cancelContext: true,
			expectedError: true,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()

				return mocks.NewMockSleeper(t)
			},
		},
		{
			name:          "zero wait should succeed",
			wait:          0,
			startTime:     time.Now(),
			cancelContext: false,
			expectedError: false,
			setupSleeper: func(t *testing.T) Sleeper {
				t.Helper()
				sleeper := mocks.NewMockSleeper(t)
				sleeper.On("Sleep", time.Duration(0)).Return().Once()

				return sleeper
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sleeper := tt.setupSleeper(t)
			ctx := context.Background()

			if tt.cancelContext {
				var cancel context.CancelFunc

				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err := waitWithTimeout(ctx, tt.wait, tt.startTime, sleeper)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
