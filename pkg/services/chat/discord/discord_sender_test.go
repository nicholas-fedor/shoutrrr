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
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

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

			req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
			resp, err := client.Do(req)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(resp.StatusCode).To(gomega.Equal(200))
			body, _ := io.ReadAll(resp.Body)
			gomega.Expect(string(body)).To(gomega.Equal("success"))
		})

		ginkgo.It("should handle network errors", func() {
			httpmock.RegisterResponder("GET", "http://example.com",
				httpmock.NewErrorResponder(errors.New("network error")))

			req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
			_, err := client.Do(req)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("performing HTTP request"))
		})
	})

	ginkgo.Describe("JSONRequestPreparer.PrepareRequest", func() {
		var preparer *JSONRequestPreparer
		var payload []byte

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
			payload := WebhookPayload{
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
		var mockClient *mockHTTPClient
		var preparer *JSONRequestPreparer
		var sleeper *mockSleeper

		ginkgo.BeforeEach(func() {
			mockClient = &mockHTTPClient{}
			payload := []byte(`{"content":"test"}`)
			preparer = &JSONRequestPreparer{payload: payload}
			sleeper = newMockSleeper()
		})

		ginkgo.It("should succeed on first attempt", func() {
			mockClient.response = &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
			gomega.Expect(sleeper.slept).To(gomega.BeEmpty())
		})

		ginkgo.It("should succeed on first attempt with StatusOK", func() {
			mockClient.response = &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(mockClient.callCount).To(gomega.Equal(1))
			gomega.Expect(sleeper.slept).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle unexpected status codes", func() {
			mockClient.response = &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(strings.NewReader("")),
			}

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).
				To(gomega.ContainSubstring("unexpected response status code"))
			gomega.Expect(sleeper.slept).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle nil response", func() {
			mockClient.response = nil

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).To(gomega.MatchError(ErrUnknownAPIError))
			gomega.Expect(sleeper.slept).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle HTTP client errors", func() {
			mockClient.err = http.ErrHandlerTimeout

			ctx := context.Background()
			err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err.Error()).To(gomega.ContainSubstring("making HTTP POST request"))
			gomega.Expect(sleeper.slept).To(gomega.BeEmpty())
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
			sleeper := &mockSleeper{}
			err := handleRateLimitResponse(context.Background(), resp, 0, startTime, sleeper)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(sleeper.slept).To(gomega.Equal([]time.Duration{2 * time.Second}))
		})

		ginkgo.It("should handle invalid retry-after header", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			resp.Header.Set("Retry-After", "invalid")

			startTime := time.Now()
			sleeper := &mockSleeper{}
			err := handleRateLimitResponse(context.Background(), resp, 0, startTime, sleeper)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			// Should fall back to exponential backoff
			gomega.Expect(sleeper.slept).To(gomega.Equal([]time.Duration{time.Second}))
		})

		ginkgo.It("should handle retry-after exceeding max timeout", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			resp.Header.Set("Retry-After", "400") // Exceeds 5 minutes

			startTime := time.Now()
			sleeper := &mockSleeper{}
			err := handleRateLimitResponse(context.Background(), resp, 0, startTime, sleeper)

			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).To(gomega.MatchError(ErrRateLimited))
			gomega.Expect(sleeper.slept).To(gomega.BeEmpty())
		})

		ginkgo.It("should handle exponential backoff", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			// No retry-after header

			startTime := time.Now()
			sleeper := &mockSleeper{}
			err := handleRateLimitResponse(
				context.Background(),
				resp,
				2,
				startTime,
				sleeper,
			)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(sleeper.slept).To(gomega.Equal([]time.Duration{4 * time.Second}))
		})

		ginkgo.It("should cap exponential backoff", func() {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}

			startTime := time.Now()
			sleeper := &mockSleeper{}
			err := handleRateLimitResponse(
				context.Background(),
				resp,
				6,
				startTime,
				sleeper,
			)

			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(sleeper.slept).To(gomega.Equal([]time.Duration{64 * time.Second}))
		})
	})
})

// mockHTTPClient is a test helper that implements HTTPClient interface.
type mockHTTPClient struct {
	response  *http.Response
	err       error
	callCount int
	doFunc    func(*http.Request) (*http.Response, error)
}

// mockSleeper is a test helper that implements Sleeper interface and records sleep durations.
type mockSleeper struct {
	slept []time.Duration
}

func newMockSleeper() *mockSleeper {
	return &mockSleeper{slept: make([]time.Duration, 0)}
}

func (m *mockSleeper) Sleep(d time.Duration) {
	m.slept = append(m.slept, d)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.callCount++
	if m.doFunc != nil {
		return m.doFunc(req)
	}

	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

func TestSendWithRetryRateLimitRetryAfter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

		mockClient := &mockHTTPClient{}
		payload := []byte(`{"content":"test"}`)
		preparer := &JSONRequestPreparer{payload: payload}
		sleeper := &mockSleeper{}

		// First call returns 429 with retry-after
		callCount := 0
		mockClient.doFunc = func(_ *http.Request) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				resp := &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("")),
				}
				resp.Header.Set("Retry-After", "1")

				return resp, nil
			}

			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}

		ctx := context.Background()
		err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		gomega.Expect(callCount).To(gomega.Equal(2))
		gomega.Expect(sleeper.slept).To(gomega.Equal([]time.Duration{time.Second}))
	})
}

func TestSendWithRetryMaxRetriesExceeded(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

		mockClient := &mockHTTPClient{}
		payload := []byte(`{"content":"test"}`)
		preparer := &JSONRequestPreparer{payload: payload}
		sleeper := &mockSleeper{}

		mockClient.response = &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "500 Internal Server Error",
			Body:       io.NopCloser(strings.NewReader("")),
		}

		ctx := context.Background()
		err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err).To(gomega.MatchError(ErrMaxRetries))
		gomega.Expect(mockClient.callCount).To(gomega.Equal(6)) // MaxRetries + 1 = 6
		gomega.Expect(sleeper.slept).To(gomega.Equal([]time.Duration{
			time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
			16 * time.Second,
		}))
	})
}

func TestSendWithRetryMaxRetryTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

		mockClient := &mockHTTPClient{}
		payload := []byte(`{"content":"test"}`)
		preparer := &JSONRequestPreparer{payload: payload}
		sleeper := &mockSleeper{}

		// Mock a very long retry-after that exceeds MaxRetryTimeout
		mockClient.doFunc = func(_ *http.Request) (*http.Response, error) {
			resp := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}
			resp.Header.Set("Retry-After", "400") // Exceeds 5 minutes

			return resp, nil
		}

		ctx := context.Background()
		err := sendWithRetry(ctx, preparer, "http://example.com", mockClient, sleeper)

		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("rate limited by Discord"))
		gomega.Expect(sleeper.slept).To(gomega.BeEmpty())
	})
}

// TestIsTransientError tests the isTransientError function with table-driven tests.
func TestIsTransientError(t *testing.T) {
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
			result := isTransientError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockTemporaryError implements net.Error for testing.
type mockTemporaryError struct{}

func (m *mockTemporaryError) Error() string   { return "temporary error" }
func (m *mockTemporaryError) Timeout() bool   { return false }
func (m *mockTemporaryError) Temporary() bool { return true }

// mockTimeoutError implements net.Error for testing timeout.
type mockTimeoutError struct{}

func (m *mockTimeoutError) Error() string   { return "timeout error" }
func (m *mockTimeoutError) Timeout() bool   { return true }
func (m *mockTimeoutError) Temporary() bool { return false }

// TestExecuteWithTransportRetry tests the executeWithTransportRetry function.
func TestExecuteWithTransportRetry(t *testing.T) {
	tests := []struct {
		name           string
		setupClient    func() *mockHTTPClient
		expectedError  bool
		expectedCalls  int
		expectedSleeps []time.Duration
	}{
		{
			name: "successful request on first attempt",
			setupClient: func() *mockHTTPClient {
				return &mockHTTPClient{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("")),
					},
				}
			},
			expectedError:  false,
			expectedCalls:  1,
			expectedSleeps: nil,
		},
		{
			name: "transient error with retry success",
			setupClient: func() *mockHTTPClient {
				callCount := 0

				return &mockHTTPClient{
					doFunc: func(_ *http.Request) (*http.Response, error) {
						callCount++
						if callCount == 1 {
							return nil, context.DeadlineExceeded
						}

						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader("")),
						}, nil
					},
				}
			},
			expectedError:  false,
			expectedCalls:  2,
			expectedSleeps: []time.Duration{time.Second},
		},
		{
			name: "persistent error after max retries",
			setupClient: func() *mockHTTPClient {
				return &mockHTTPClient{
					err: context.DeadlineExceeded,
				}
			},
			expectedError:  true,
			expectedCalls:  4, // maxTransportRetries + 1 = 4
			expectedSleeps: []time.Duration{time.Second, 2 * time.Second, 4 * time.Second},
		},
		{
			name: "context canceled during retry",
			setupClient: func() *mockHTTPClient {
				return &mockHTTPClient{
					err: context.DeadlineExceeded,
				}
			},
			expectedError:  true,
			expectedCalls:  1,
			expectedSleeps: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			sleeper := &mockSleeper{}
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

			assert.Equal(t, tt.expectedCalls, client.callCount)

			if tt.expectedSleeps == nil {
				assert.Nil(t, sleeper.slept)
			} else {
				assert.Equal(t, tt.expectedSleeps, sleeper.slept)
			}
		})
	}
}

// TestHandleServerError tests the handleServerError function.
func TestHandleServerError(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		attempt        int
		expectedError  bool
		expectedSleeps []time.Duration
	}{
		{
			name:           "status code below 500 should not wait",
			statusCode:     404,
			attempt:        0,
			expectedError:  false,
			expectedSleeps: nil,
		},
		{
			name:           "500 status code should wait",
			statusCode:     500,
			attempt:        0,
			expectedError:  false,
			expectedSleeps: []time.Duration{time.Second},
		},
		{
			name:           "max retries exceeded should return error",
			statusCode:     500,
			attempt:        maxRetries,
			expectedError:  true,
			expectedSleeps: nil,
		},
		{
			name:           "exponential backoff calculation",
			statusCode:     502,
			attempt:        2,
			expectedError:  false,
			expectedSleeps: []time.Duration{4 * time.Second},
		},
		{
			name:           "exponential backoff calculation for higher attempt",
			statusCode:     503,
			attempt:        4,
			expectedError:  false,
			expectedSleeps: []time.Duration{16 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader("")),
			}
			sleeper := &mockSleeper{}
			startTime := time.Now()

			err := handleServerError(context.Background(), resp, tt.attempt, startTime, sleeper)

			if tt.expectedError {
				require.Error(t, err)
				assert.Equal(t, ErrMaxRetries, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedSleeps, sleeper.slept)
		})
	}
}

// TestWaitWithTimeout tests the waitWithTimeout function.
func TestWaitWithTimeout(t *testing.T) {
	tests := []struct {
		name          string
		wait          time.Duration
		startTime     time.Time
		cancelContext bool
		expectedError bool
		expectedSleep bool
	}{
		{
			name:          "normal wait should succeed",
			wait:          time.Second,
			startTime:     time.Now(),
			cancelContext: false,
			expectedError: false,
			expectedSleep: true,
		},
		{
			name:          "wait exceeding timeout should fail",
			wait:          6 * time.Minute, // Exceeds maxRetryTimeout
			startTime:     time.Now(),
			cancelContext: false,
			expectedError: true,
			expectedSleep: false,
		},
		{
			name:          "context canceled should fail",
			wait:          time.Second,
			startTime:     time.Now(),
			cancelContext: true,
			expectedError: true,
			expectedSleep: false,
		},
		{
			name:          "zero wait should succeed",
			wait:          0,
			startTime:     time.Now(),
			cancelContext: false,
			expectedError: false,
			expectedSleep: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sleeper := &mockSleeper{}
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

			if tt.expectedSleep {
				assert.NotEmpty(t, sleeper.slept)
			} else {
				assert.Empty(t, sleeper.slept)
			}
		})
	}
}
