package discord

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

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
			sleeper = &mockSleeper{}
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

func TestHandleRateLimitResponseRetryAfterHeader(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

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
}

func TestHandleRateLimitResponseInvalidRetryAfterHeader(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

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
}

func TestHandleRateLimitResponseRetryAfterExceedingMaxTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

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
}

func TestHandleRateLimitResponseExponentialBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

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
}

func TestHandleRateLimitResponseCapExponentialBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		gomega.RegisterTestingT(t)

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
}
