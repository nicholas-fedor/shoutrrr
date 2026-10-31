package discord

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// sentRequest records a request the mock HTTP client received.
type sentRequest struct {
	// deadline is the deadline of the request's context.
	deadline time.Time
	// contentType is the request's Content-Type header.
	contentType string
	// body is the request body.
	body string
}

// requestRecorder collects the requests a mock HTTP client receives.
type requestRecorder struct {
	// mu guards requests.
	mu sync.Mutex
	// requests holds the requests in the order received.
	requests []sentRequest
}

// testWebhookURLAlt is a valid webhook API URL for the doSend tests.
const testWebhookURLAlt = testHooksBaseURL + "/123/abc"

// sent returns the requests received so far.
//
// Returns:
//   - []sentRequest: the requests, in order.
func (r *requestRecorder) sent() []sentRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]sentRequest(nil), r.requests...)
}

// newAcceptingClient returns a mock HTTP client that expects exactly calls
// requests, answers each with 204 No Content, and records them.
//
// Parameters:
//   - t: the test that owns the mock.
//   - calls: the number of requests to expect.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *requestRecorder: the recorded requests.
func newAcceptingClient(t *testing.T, calls int) (*mocks.MockHTTPClient, *requestRecorder) {
	t.Helper()

	recorder := &requestRecorder{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		var body []byte
		if req.Body != nil {
			body, _ = io.ReadAll(req.Body)
		}

		recorder.mu.Lock()
		deadline, _ := req.Context().Deadline()
		recorder.requests = append(recorder.requests, sentRequest{
			deadline:    deadline,
			contentType: req.Header.Get("Content-Type"),
			body:        string(body),
		})
		recorder.mu.Unlock()

		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	}).Times(calls)

	return client, recorder
}

// newFailingClient returns a mock HTTP client whose single expected request
// fails with err.
//
// Parameters:
//   - t: the test that owns the mock.
//   - err: the transport error.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
func newFailingClient(t *testing.T, err error) *mocks.MockHTTPClient {
	t.Helper()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).Return(nil, err).Once()

	return client
}

// newTestService returns a service for webhook 123456789 that sends through
// client and logs to the test output.
//
// Parameters:
//   - t: the test.
//   - client: the HTTP client, or nil for a service that must not send.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, client types.HTTPClient) *Service {
	t.Helper()

	config := &Config{WebhookID: "123456789", Token: "test-token"}
	service := &Service{Config: config, HTTPClient: client, pkr: format.NewPropKeyResolver(config)}
	service.SetLogger(log.New(t.Output(), "", 0))

	return service
}

// TestSendBudget verifies that every send entry bounds a send without a
// caller deadline by [maxRetryTimeout], the budget that ServiceTimeout
// reports, and keeps a caller's deadline as it is.
func TestSendBudget(t *testing.T) {
	t.Parallel()

	items := []types.MessageItem{{Text: "Hello"}}

	// sends lists the send entries, with whether they take the caller's
	// context.
	sends := []struct {
		name     string
		send     func(ctx context.Context, service *Service) error
		takesCtx bool
	}{
		{name: "Send", send: func(_ context.Context, service *Service) error {
			return service.Send("Hello", nil) //nolint:contextcheck // The test covers the entry point that takes no context.
		}},
		{name: "SendContext", takesCtx: true, send: func(ctx context.Context, service *Service) error {
			return service.SendContext(ctx, "Hello", nil)
		}},
		{name: "SendItems", send: func(_ context.Context, service *Service) error {
			return service.SendItems(items, nil) //nolint:contextcheck // The test covers the entry point that takes no context.
		}},
		{name: "SendItemsContext", takesCtx: true, send: func(ctx context.Context, service *Service) error {
			return service.SendItemsContext(ctx, items, nil)
		}},
	}

	for _, tt := range sends {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var deadline time.Time

				client := mocks.NewMockHTTPClient(t)
				client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
					deadline, _ = req.Context().Deadline()

					return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Request: req}, nil
				})

				start := time.Now()

				require.NoError(t, tt.send(t.Context(), newTestService(t, client)))
				assert.Equal(t, start.Add(maxRetryTimeout), deadline, "a send without a deadline gets the budget")

				if !tt.takesCtx {
					return
				}

				ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
				defer cancel()

				require.NoError(t, tt.send(ctx, newTestService(t, client)))
				assert.Equal(t, start.Add(time.Hour), deadline, "the caller's deadline applies")
			})
		})
	}
}

// TestSendRequestCount verifies how many requests a send makes for plain,
// JSON, chunked, and line-split messages.
func TestSendRequestCount(t *testing.T) {
	t.Parallel()

	// tests lists messages and config changes with the number of requests the
	// send should make.
	tests := []struct {
		name      string
		message   string
		configure func(*Config)
		wantCalls int
	}{
		{name: "plain text", message: "Hello, Discord!", wantCalls: 1},
		{name: "JSON mode", message: `{"content":"JSON message"}`, configure: func(c *Config) { c.JSON = true }, wantCalls: 1},
		{name: "longer than one batch", message: strings.Repeat("a", TotalChunkSize+100), wantCalls: 2},
		{name: "split lines", message: "Line 1\nLine 2\nLine 3", configure: func(c *Config) { c.SplitLines = true }, wantCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, _ := newAcceptingClient(t, tt.wantCalls)
			service := newTestService(t, client)

			if tt.configure != nil {
				tt.configure(service.Config)
			}

			require.NoError(t, service.Send(tt.message, &types.Params{}))
		})
	}
}

// TestSendLevelColor verifies that the level param colors the embed of a plain
// message with the color configured for that level, and that a plain color
// from the service URL or the send params, or an unknown level, keeps the
// plain color. Each send has a title, so it goes out as an embed. A short
// message without a title goes out as plain content and has no embed to color.
func TestSendLevelColor(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		query     string
		params    types.Params
		wantColor uint
	}{
		"debug":                    {params: types.Params{"level": "Debug"}, wantColor: 0x7b00ab},
		"info":                     {params: types.Params{"level": "Info"}, wantColor: 0x2488ff},
		"warning":                  {params: types.Params{"level": "Warning"}, wantColor: 0xffc441},
		"error":                    {params: types.Params{"level": "Error"}, wantColor: 0xd60510},
		"configured level color":   {query: "?colorError=0x00ff00", params: types.Params{"level": "Error"}, wantColor: 0x00ff00},
		"color in the URL":         {query: "?color=0x123456", params: types.Params{"level": "Error"}, wantColor: 0x123456},
		"default color in the URL": {query: "?color=0x50d9ff", params: types.Params{"level": "Error"}, wantColor: 0x50d9ff},
		"color param":              {params: types.Params{"level": "Error", "color": "0x50d9ff"}, wantColor: 0x50d9ff},
		"unknown level":            {params: types.Params{"level": "critical"}, wantColor: 0x50d9ff},
		"no level":                 {params: types.Params{}, wantColor: 0x50d9ff},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			params := types.Params{"title": "Alert"}
			maps.Copy(params, tc.params)

			payload := sendPlainForTest(t, tc.query, params)

			require.Len(t, payload.Embeds, 1)
			assert.Equal(t, tc.wantColor, payload.Embeds[0].Color)
		})
	}

	t.Run("plain content keeps its form", func(t *testing.T) {
		t.Parallel()

		payload := sendPlainForTest(t, "", types.Params{"level": "Error"})

		assert.Equal(t, "Message", payload.Content)
		assert.Empty(t, payload.Embeds)
	})
}

// sendPlainForTest sends the plain message "Message" through a service built
// from a webhook URL with the given query, and returns the payload it sent.
//
// Parameters:
//   - t: the test.
//   - query: the service URL query, such as "?color=0x123456", or empty.
//   - params: the send params.
//
// Returns:
//   - WebhookPayload: the decoded payload of the single request.
func sendPlainForTest(t *testing.T, query string, params types.Params) WebhookPayload {
	t.Helper()

	client, recorder := newAcceptingClient(t, 1)

	serviceURL, err := url.Parse("discord://test-token@123456789" + query)
	require.NoError(t, err)

	service := &Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", &params))

	var payload WebhookPayload
	require.NoError(t, json.Unmarshal([]byte(recorder.requests[0].body), &payload))

	return payload
}

// TestSendFailures verifies that an empty message is rejected without a
// request and that a failed first batch fails the send.
func TestSendFailures(t *testing.T) {
	t.Parallel()

	t.Run("empty message", func(t *testing.T) {
		t.Parallel()

		require.ErrorIs(t, newTestService(t, mocks.NewMockHTTPClient(t)).Send("", &types.Params{}), ErrEmptyMessage)
	})

	t.Run("first batch fails", func(t *testing.T) {
		t.Parallel()

		err := newTestService(t, newFailingClient(t, ErrMaxRetries)).Send("Test message", &types.Params{})

		require.ErrorIs(t, err, ErrMaxRetries)
		assert.Contains(t, err.Error(), "failed to send discord notification")
	})
}

// TestServiceTimeout verifies that the send budget is the sender's retry limit.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, maxRetryTimeout, (&Service{}).ServiceTimeout(nil))
}

// TestSendItems verifies that the deprecated pointer-params SendItems sends the
// items.
func TestSendItems(t *testing.T) {
	t.Parallel()

	client, _ := newAcceptingClient(t, 1)

	require.NoError(t, newTestService(t, client).SendItems([]types.MessageItem{{Text: "Test message"}}, &types.Params{}))
}

// TestSendItemsContext verifies that items are sent as JSON, or as multipart
// with a file, and that params set the username for the send.
func TestSendItemsContext(t *testing.T) {
	t.Parallel()

	t.Run("plain item", func(t *testing.T) {
		t.Parallel()

		client, recorder := newAcceptingClient(t, 1)
		require.NoError(t, newTestService(t, client).sendItems(t.Context(), []types.MessageItem{{Text: "Simple message"}}, &types.Params{}))

		assert.Equal(t, "application/json", recorder.sent()[0].contentType)
	})

	t.Run("item with a file", func(t *testing.T) {
		t.Parallel()

		client, recorder := newAcceptingClient(t, 1)
		items := []types.MessageItem{{Text: "Message with file", File: &types.File{Name: "test.txt", Data: []byte("file content")}}}

		require.NoError(t, newTestService(t, client).sendItems(t.Context(), items, &types.Params{}))

		sent := recorder.sent()[0]
		assert.True(t, strings.HasPrefix(sent.contentType, "multipart/form-data"), "content type %q", sent.contentType)
		assert.Contains(t, sent.body, "file content")
	})

	t.Run("username from params", func(t *testing.T) {
		t.Parallel()

		client, recorder := newAcceptingClient(t, 1)
		require.NoError(t, newTestService(t, client).sendItems(t.Context(), []types.MessageItem{{Text: "Test message"}}, &types.Params{"username": "CustomBot"}))

		assert.Contains(t, recorder.sent()[0].body, `"username":"CustomBot"`)
	})
}

// TestSendItemsContextFailures verifies that empty items and unknown params are
// rejected without a request.
func TestSendItemsContextFailures(t *testing.T) {
	t.Parallel()

	t.Run("no items", func(t *testing.T) {
		t.Parallel()

		err := newTestService(t, mocks.NewMockHTTPClient(t)).sendItems(t.Context(), []types.MessageItem{}, &types.Params{})
		require.ErrorIs(t, err, ErrEmptyMessage)
	})

	t.Run("unknown param", func(t *testing.T) {
		t.Parallel()

		err := newTestService(t, mocks.NewMockHTTPClient(t)).sendItems(t.Context(), []types.MessageItem{{Text: "Test message"}}, &types.Params{"invalid_key": "value"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "updating config from params")
	})
}

// TestSendItemBatchesShareDeadline verifies that the batches of one message
// share the send's deadline. A batch may wait out a rate limit, so giving each
// batch only a share of the time would drop parts from the middle of the
// message.
func TestSendItemBatchesShareDeadline(t *testing.T) {
	t.Parallel()

	client, recorder := newAcceptingClient(t, 2)

	items := make([]types.MessageItem, MaxEmbeds+1)
	for i := range items {
		items[i] = types.MessageItem{Text: "Item " + strconv.Itoa(i)}
	}

	service := newTestService(t, client)
	require.NoError(t, service.sendItemBatches(t.Context(), items, &types.Params{}))

	sent := recorder.sent()
	require.Len(t, sent, 2)
	assert.Equal(t, sent[0].deadline, sent[1].deadline)
}

// TestSendItemBatchesFailures verifies that when several batches fail, every
// batch is still sent and every failure is returned, with nothing logged.
func TestSendItemBatchesFailures(t *testing.T) {
	t.Parallel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
		return statusResponse(http.StatusBadRequest, nil), nil
	}).Times(2)

	var logs bytes.Buffer

	service := newTestService(t, client)
	service.SetLogger(log.New(&logs, "", 0))

	items := make([]types.MessageItem, MaxEmbeds+1)
	for i := range items {
		items[i] = types.MessageItem{Text: "Item " + strconv.Itoa(i)}
	}

	err := service.sendItemBatches(t.Context(), items, &types.Params{})

	require.ErrorIs(t, err, ErrUnexpectedStatus)

	var joined interface{ Unwrap() []error }

	require.ErrorAs(t, err, &joined)
	assert.Len(t, joined.Unwrap(), 2)
	assert.Empty(t, logs.String())
}

// TestSendItemBatchesCanceled verifies that a canceled context stops the
// batches that have not been sent yet and is returned with any batch failure.
// The mock client fails the test on any request past those expected.
func TestSendItemBatchesCanceled(t *testing.T) {
	t.Parallel()

	items := make([]types.MessageItem, 2*MaxEmbeds+1)
	for i := range items {
		items[i] = types.MessageItem{Text: "Item " + strconv.Itoa(i)}
	}

	t.Run("canceled before the first batch", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := newTestService(t, mocks.NewMockHTTPClient(t)).sendItemBatches(ctx, items, &types.Params{})

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("canceled after a sent batch", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
			cancel()

			return statusResponse(http.StatusNoContent, nil), nil
		}).Once()

		err := newTestService(t, client).sendItemBatches(ctx, items, &types.Params{})

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("canceled after a failed batch", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
			cancel()

			return statusResponse(http.StatusBadRequest, nil), nil
		}).Once()

		err := newTestService(t, client).sendItemBatches(ctx, items, &types.Params{})

		require.ErrorIs(t, err, ErrUnexpectedStatus)
		require.ErrorIs(t, err, context.Canceled)
	})
}

// TestCreateItemsFromPlain verifies that plain text is split into chunks or
// lines, batched, and kept whole and in order.
func TestCreateItemsFromPlain(t *testing.T) {
	t.Parallel()

	t.Run("short message", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, [][]types.MessageItem{{{Text: "Short message"}}}, CreateItemsFromPlain("Short message", false))
	})

	t.Run("split lines", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t,
			[][]types.MessageItem{{{Text: "Line 1"}, {Text: "Line 2"}, {Text: "Line 3"}}},
			CreateItemsFromPlain("Line 1\nLine 2\nLine 3", true),
		)
	})

	t.Run("longer than one chunk", func(t *testing.T) {
		t.Parallel()

		message := strings.Repeat("a", ChunkSize+100)
		result := CreateItemsFromPlain(message, false)

		require.Len(t, result, 1)
		require.Len(t, result[0], 2)
		assert.Equal(t, message, result[0][0].Text+result[0][1].Text)
	})

	t.Run("exactly one chunk", func(t *testing.T) {
		t.Parallel()

		message := strings.Repeat("a", ChunkSize)

		assert.Equal(t, [][]types.MessageItem{{{Text: message}}}, CreateItemsFromPlain(message, false))
	})

	t.Run("empty message", func(t *testing.T) {
		t.Parallel()

		result := CreateItemsFromPlain("", false)

		require.Len(t, result, 1)
		assert.Empty(t, result[0])
	})

	t.Run("long multi-byte message", func(t *testing.T) {
		t.Parallel()

		runes := make([]rune, TotalChunkSize+1500)
		for i := range runes {
			runes[i] = rune(0x4E00 + i%1000)
		}

		plain := string(runes)

		var joined strings.Builder

		for _, batch := range CreateItemsFromPlain(plain, false) {
			for _, item := range batch {
				joined.WriteString(item.Text)
			}
		}

		assert.Equal(t, plain, joined.String())
	})

	t.Run("untitled batches match CreateItemsFromPlain", func(t *testing.T) {
		t.Parallel()

		plain := strings.Repeat("a", TotalChunkSize+1000)

		assert.Equal(t, CreateItemsFromPlain(plain, false), createItemsFromPlain(plain, false, ""))
	})
}

// TestCreateItemsFromPlainTitleLimit verifies that each batch and its title fit
// within the embed text limit.
func TestCreateItemsFromPlainTitleLimit(t *testing.T) {
	t.Parallel()

	title := strings.Repeat("t", 100)

	// tests lists messages and split modes whose batches must leave room for the
	// title.
	tests := []struct {
		name       string
		plain      string
		splitLines bool
	}{
		{name: "split lines", plain: strings.TrimSuffix(strings.Repeat(strings.Repeat("a", 1000)+"\n", 12), "\n"), splitLines: true},
		{name: "chunked", plain: strings.Repeat("a", TotalChunkSize+1000), splitLines: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			batches := createItemsFromPlain(tt.plain, tt.splitLines, title)
			require.NotEmpty(t, batches)

			for _, batch := range batches {
				total := utf8.RuneCountInString(title)
				for _, item := range batch {
					total += utf8.RuneCountInString(item.Text)
				}

				assert.LessOrEqual(t, total, TotalChunkSize)
			}
		})
	}
}

// TestInitialize verifies that Initialize reads the URL, applies the defaults,
// and sets the default sleeper without filling the deprecated client field,
// and that it rejects an incomplete URL.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		require.NoError(t, service.Initialize(parseTestURL(t, "discord://token@webhook?username=TestBot"), log.New(t.Output(), "", 0)))

		require.NotNil(t, service.Config)
		assert.Equal(t, "webhook", service.Config.WebhookID)
		assert.Equal(t, "token", service.Config.Token)
		assert.Equal(t, "TestBot", service.Config.Username)
		assert.Equal(t, uint(0x50d9ff), service.Config.Color, "the default color applies")
		assert.True(t, service.Config.SplitLines, "lines split by default")
		assert.Equal(t, RealSleeper{}, service.Sleeper)
		assert.Nil(t, service.HTTPClient, "Initialize leaves the deprecated client field unset")
	})

	t.Run("keeps a sleeper", func(t *testing.T) {
		t.Parallel()

		sleeper := mocks.NewMockSleeper(t)
		service := &Service{Sleeper: sleeper}
		require.NoError(t, service.Initialize(parseTestURL(t, "discord://token@webhook"), log.New(t.Output(), "", 0)))

		assert.Same(t, sleeper, service.Sleeper)
	})

	t.Run("incomplete URL", func(t *testing.T) {
		t.Parallel()

		require.Error(t, (&Service{}).Initialize(parseTestURL(t, "discord://@"), log.New(t.Output(), "", 0)))
	})
}

// TestGetID verifies the service identifier.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "discord", (&Service{}).GetID())
}

// TestDoSend verifies that doSend posts a valid webhook URL, rejects other URLs
// without a request, and reports a failed request.
func TestDoSend(t *testing.T) {
	t.Parallel()

	payload := []byte(`{"content":"test"}`)

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		client, recorder := newAcceptingClient(t, 1)
		require.NoError(t, (&Service{HTTPClient: client}).doSend(t.Context(), payload, testWebhookURLAlt))

		assert.Equal(t, string(payload), recorder.sent()[0].body)
	})

	t.Run("request fails", func(t *testing.T) {
		t.Parallel()

		client := newFailingClient(t, errors.New("network error"))
		require.Error(t, (&Service{HTTPClient: client}).doSend(t.Context(), payload, testWebhookURLAlt))
	})

	// invalid lists URLs doSend must reject with the error each should produce.
	invalid := []struct {
		name    string
		postURL string
		wantErr error
	}{
		{name: "empty URL", postURL: "", wantErr: ErrEmptyURL},
		{name: "http scheme", postURL: "http://discord.com/api/v10/webhooks/123/abc", wantErr: ErrInvalidScheme},
		{name: "other host", postURL: "https://example.com/api/v10/webhooks/123/abc", wantErr: ErrInvalidHost},
		{name: "other path", postURL: "https://discord.com/api/invalid/123/abc", wantErr: ErrInvalidURLPrefix},
		{name: "unversioned path", postURL: "https://discord.com/api/webhooks/123/abc", wantErr: ErrInvalidURLPrefix},
		{name: "no webhook ID or token", postURL: testHooksBaseURL + "//", wantErr: ErrMalformedURL},
	}

	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{HTTPClient: mocks.NewMockHTTPClient(t)}
			require.ErrorIs(t, service.doSend(t.Context(), payload, tt.postURL), tt.wantErr)
		})
	}
}

// TestDoSendMultipart verifies that doSendMultipart posts multipart form data,
// with or without files, and reports a failed request.
func TestDoSendMultipart(t *testing.T) {
	t.Parallel()

	// tests lists files to attach with whether the body should name a file.
	tests := []struct {
		name      string
		files     []types.File
		wantFiles bool
	}{
		{name: "with a file", files: []types.File{{Name: "test.txt", Data: []byte("file content")}}, wantFiles: true},
		{name: "without files", files: []types.File{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, recorder := newAcceptingClient(t, 1)
			payload := WebhookPayload{Content: "Message"}

			require.NoError(t, (&Service{HTTPClient: client}).doSendMultipart(t.Context(), &payload, tt.files, testWebhookURLAlt))

			sent := recorder.sent()[0]
			assert.True(t, strings.HasPrefix(sent.contentType, "multipart/form-data"), "content type %q", sent.contentType)
			assert.Contains(t, sent.body, "payload_json")
			assert.Equal(t, tt.wantFiles, strings.Contains(sent.body, "files[0]"))
		})
	}

	t.Run("request fails", func(t *testing.T) {
		t.Parallel()

		client := newFailingClient(t, errors.New("network error"))
		payload := WebhookPayload{Content: "Test message"}

		require.Error(t, (&Service{HTTPClient: client}).doSendMultipart(t.Context(), &payload, []types.File{}, testWebhookURLAlt))
	})
}

// TestSendRedactsUnparsableWebhookURL verifies that a token that makes the
// webhook API URL unparsable fails the send without the token in the error.
func TestSendRedactsUnparsableWebhookURL(t *testing.T) {
	t.Parallel()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, "discord://SECRETtoken%25zz@123456789012345678"), log.New(t.Output(), "", 0)))
	// The mock has no expectations, so a request fails the test.
	service.HTTPClient = mocks.NewMockHTTPClient(t)

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETtoken")
}
