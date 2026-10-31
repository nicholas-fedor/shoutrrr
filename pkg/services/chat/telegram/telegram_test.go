package telegram

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	typesmocks "github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// roundTripFunc adapts a function to [http.RoundTripper], so a test can answer
// requests without a network.
type roundTripFunc func(*http.Request) (*http.Response, error)

// Test fixtures shared by the service tests.
const (
	// sendSuccessBody is the sendMessage response the Telegram API returns on success.
	sendSuccessBody = `{"ok":true,"result":{"message_id":1}}`
	// testServiceURL configures one chat with a token whose secret part is mock-token.
	testServiceURL = "telegram://12345:mock-token@telegram/?chats=channel-1"
	// sendMessagePath is the request path of sendMessage for the test token.
	sendMessagePath = "/bot12345:mock-token/sendMessage"
)

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the outgoing request.
//
// Returns:
//   - *http.Response: the response f returns.
//   - error: the transport error f returns.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// respondWith returns a transport that answers every request with code and body.
//
// Parameters:
//   - code: the HTTP status code.
//   - body: the response body.
//
// Returns:
//   - roundTripFunc: the transport.
func respondWith(code int, body string) roundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		return newResponse(req, code, body), nil
	}
}

// newResponse builds a response to req with code and body.
//
// Parameters:
//   - req: the request being answered.
//   - code: the HTTP status code.
//   - body: the response body.
//
// Returns:
//   - *http.Response: the response, with Status set as a real client would.
func newResponse(req *http.Request, code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Status:     fmt.Sprintf("%d %s", code, http.StatusText(code)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

// parseURL parses rawURL and fails the test if it is invalid.
//
// Parameters:
//   - t: the test.
//   - rawURL: the URL to parse.
//
// Returns:
//   - *url.URL: the parsed URL.
func parseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	return serviceURL
}

// newTestService initializes a service from rawURL that logs to the test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *Service: the initialized service.
func newTestService(t *testing.T, rawURL string) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(parseURL(t, rawURL), log.New(t.Output(), "", 0)))

	return service
}

// newSendingService initializes a service from testServiceURL that sends its
// requests through transport.
//
// Parameters:
//   - t: the test.
//   - transport: the transport that answers the service's requests.
//
// Returns:
//   - *Service: the initialized service.
func newSendingService(t *testing.T, transport http.RoundTripper) *Service {
	t.Helper()

	service := newTestService(t, testServiceURL)
	service.SetHTTPClient(&http.Client{Transport: transport})

	return service
}

// TestInitializeRejectsInvalidURLs verifies that Initialize fails for URLs
// without a valid token or chats, and leaves the config empty.
func TestInitializeRejectsInvalidURLs(t *testing.T) {
	t.Parallel()

	// tests lists URLs that Initialize must reject.
	tests := []struct {
		name   string
		rawURL string
	}{
		{name: "no arguments", rawURL: "telegram://"},
		{name: "invalid token format", rawURL: "telegram://invalid-token"},
		{name: "token without chats", rawURL: "telegram://12345:mock-token@telegram"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &Service{}
			require.Error(t, service.Initialize(parseURL(t, tt.rawURL), log.New(t.Output(), "", 0)))

			config := service.GetConfig()
			assert.Empty(t, config.Token)
			assert.Empty(t, config.Chats)
		})
	}
}

// TestInitializeHidesInvalidToken verifies that an invalid token is rejected
// without echoing it in the error.
func TestInitializeHidesInvalidToken(t *testing.T) {
	t.Parallel()

	err := (&Service{}).Initialize(parseURL(t, "telegram://bot:SECRETtoken@telegram/?chats=1"), log.New(t.Output(), "", 0))

	require.ErrorIs(t, err, ErrInvalidToken)
	assert.NotContains(t, err.Error(), "SECRETtoken")
}

// TestInitializeReadsConfig verifies that a valid URL sets the token and every
// chat listed in the chats query.
func TestInitializeReadsConfig(t *testing.T) {
	t.Parallel()

	config := newTestService(t, "telegram://12345:mock-token@telegram/?chats=channel-1,channel-2,channel-3").GetConfig()

	require.NotNil(t, config)
	assert.Equal(t, "12345:mock-token", config.Token)
	assert.Equal(t, []string{"channel-1", "channel-2", "channel-3"}, config.Chats)
}

// TestGetID verifies the service identifier.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "telegram", (&Service{}).GetID())
}

// TestSendSucceeds verifies that a send to every chat succeeds when Telegram
// accepts the payload, and that it posts to the token's sendMessage endpoint.
func TestSendSucceeds(t *testing.T) {
	t.Parallel()

	var paths []string

	service := newTestService(t, "telegram://12345:mock-token@telegram/?chats=channel-1,channel-2,channel-3")
	service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)

		return newResponse(req, http.StatusOK, sendSuccessBody), nil
	})})

	require.NoError(t, service.Send("Message", nil))
	assert.Equal(t, []string{sendMessagePath, sendMessagePath, sendMessagePath}, paths)
}

// TestSendReportsFailures verifies that a send reports transport errors, error
// statuses, and Telegram API failures, including those returned with HTTP 200.
func TestSendReportsFailures(t *testing.T) {
	t.Parallel()

	errTransport := errors.New("connection reset")

	// tests lists Telegram responses with the error each should produce, and
	// the API error code and description when Telegram sends them.
	tests := []struct {
		name            string
		transport       roundTripFunc
		wantErr         error
		wantText        string
		wantCode        int
		wantDescription string
	}{
		{
			name: "transport error",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errTransport
			},
			wantErr: errTransport,
		},
		{
			name:      "error status without a Telegram error",
			transport: respondWith(http.StatusBadGateway, "<html>Bad Gateway</html>"),
			wantErr:   jsonclient.ErrUnexpectedStatus,
			wantText:  "502",
		},
		{
			name:            "Telegram error with an error status",
			transport:       respondWith(http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`),
			wantErr:         jsonclient.ErrUnexpectedStatus,
			wantText:        "400",
			wantCode:        400,
			wantDescription: "Bad Request: chat not found",
		},
		{
			name:      "failure without a description",
			transport: respondWith(http.StatusOK, `{"ok":false}`),
			wantErr:   ErrUnexpectedResponse,
		},
		{
			name:            "Telegram error with HTTP 200",
			transport:       respondWith(http.StatusOK, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`),
			wantCode:        403,
			wantDescription: "Forbidden: bot was blocked by the user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newSendingService(t, tt.transport).Send("Message", nil)

			require.Error(t, err)
			assert.NotContains(t, err.Error(), "mock-token", "the error must not reveal the token")

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}

			if tt.wantText != "" {
				assert.Contains(t, err.Error(), tt.wantText)
			}

			if tt.wantDescription != "" {
				apiErr, ok := errors.AsType[*responseError](err)
				require.True(t, ok, "the error should carry the Telegram API error")
				assert.Equal(t, tt.wantCode, apiErr.ErrorCode)
				assert.Equal(t, tt.wantDescription, apiErr.Description)
			}
		})
	}
}

// TestSendUsesChatsFromParams verifies that the chats send param replaces the
// configured chats for that send.
func TestSendUsesChatsFromParams(t *testing.T) {
	t.Parallel()

	var chats []string

	service := newSendingService(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload struct {
			ChatID string `json:"chat_id"`
		}

		if err := json.UnmarshalRead(req.Body, &payload); err != nil {
			return nil, err
		}

		chats = append(chats, payload.ChatID)

		return newResponse(req, http.StatusOK, sendSuccessBody), nil
	}))

	require.NoError(t, service.Send("Message", &types.Params{"chats": "channel-9"}))
	assert.Equal(t, []string{"channel-9"}, chats)
}

// TestSendMessageLength verifies that the length limit counts UTF-16 code
// units, so a message of multibyte characters within the limit is sent, and a
// message over it fails before any request is made. A character outside the
// Basic Multilingual Plane, such as an emoji, counts as two units.
func TestSendMessageLength(t *testing.T) {
	t.Parallel()

	// tests lists messages with whether they fit the limit.
	tests := []struct {
		name    string
		message string
		fits    bool
	}{
		{name: "ASCII at the limit", message: strings.Repeat("a", maxlength), fits: true},
		{name: "ASCII over the limit", message: strings.Repeat("a", maxlength+1)},
		{name: "Cyrillic within the limit", message: strings.Repeat("ж", 3000), fits: true},
		{name: "emoji at the limit", message: strings.Repeat("😀", maxlength/2), fits: true},
		{name: "emoji over the limit", message: strings.Repeat("😀", maxlength/2+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !tt.fits {
				service := newTestService(t, testServiceURL)
				// The mock has no expectations, so any request fails the test.
				service.SetHTTPClient(typesmocks.NewMockHTTPClient(t))

				require.ErrorIs(t, service.Send(tt.message, nil), ErrMessageTooLong)

				return
			}

			require.NoError(t, newSendingService(t, respondWith(http.StatusOK, sendSuccessBody)).Send(tt.message, nil))
		})
	}
}

// TestSendMessageLengthWithTitle verifies that the length limit counts the
// title line a send with a title carries, whether the title comes from the
// service URL or the send params. A Markdown message carries no title, so its
// title does not count.
func TestSendMessageLengthWithTitle(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with whether a message of
	// maxlength-4 units fits the limit with the title "Alert" and its newline.
	tests := []struct {
		params *types.Params
		name   string
		rawURL string
		fits   bool
	}{
		{name: "no title", rawURL: testServiceURL, fits: true},
		{name: "title param", rawURL: testServiceURL, params: &types.Params{"title": "Alert"}},
		{name: "title in the URL", rawURL: testServiceURL + "&title=Alert"},
		{name: "title with HTML", rawURL: testServiceURL + "&title=Alert&parsemode=HTML"},
		{name: "title with Markdown", rawURL: testServiceURL + "&title=Alert&parsemode=Markdown", fits: true},
	}

	message := strings.Repeat("a", maxlength-4)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)

			if !tt.fits {
				// The mock has no expectations, so any request fails the test.
				service.SetHTTPClient(typesmocks.NewMockHTTPClient(t))

				require.ErrorIs(t, service.Send(message, tt.params), ErrMessageTooLong)

				return
			}

			service.SetHTTPClient(&http.Client{Transport: respondWith(http.StatusOK, sendSuccessBody)})

			require.NoError(t, service.Send(message, tt.params))
		})
	}
}

// TestSendContinuesAfterFailedChat verifies that a chat Telegram rejects does
// not stop the send to the chats after it, and that the error names each
// chat that failed.
func TestSendContinuesAfterFailedChat(t *testing.T) {
	t.Parallel()

	var chats []string

	service := newTestService(t, "telegram://12345:mock-token@telegram/?chats=channel-1,channel-2,channel-3")
	service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload struct {
			ChatID string `json:"chat_id"`
		}

		if err := json.UnmarshalRead(req.Body, &payload); err != nil {
			return nil, err
		}

		chats = append(chats, payload.ChatID)

		if payload.ChatID == "channel-2" {
			return newResponse(req, http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`), nil
		}

		return newResponse(req, http.StatusOK, sendSuccessBody), nil
	})})

	err := service.Send("Message", nil)

	require.Error(t, err)
	assert.Equal(t, []string{"channel-1", "channel-2", "channel-3"}, chats)
	assert.Contains(t, err.Error(), `"channel-2"`)
	assert.NotContains(t, err.Error(), `"channel-1"`)

	apiErr, ok := errors.AsType[*responseError](err)
	require.True(t, ok, "the error should carry the Telegram API error")
	assert.Equal(t, "Bad Request: chat not found", apiErr.Description)
}

// TestSendStopsWhenContextEnds verifies that once the caller's context ends,
// the chats not yet sent are skipped and the error matches the context's
// error.
func TestSendStopsWhenContextEnds(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var requests int

	service := newTestService(t, "telegram://12345:mock-token@telegram/?chats=channel-1,channel-2,channel-3")
	service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++

		cancel()

		return newResponse(req, http.StatusOK, sendSuccessBody), nil
	})})

	err := service.SendContext(ctx, "Message", nil)

	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, requests)
}

// TestSendBudget verifies that each request of a send may use an equal share
// of the time left, so a chat that does not answer cannot use up the
// others' time. Without a caller deadline, the time is the budget that
// ServiceTimeout reports for all chats. Chats that params set count too. A
// caller's deadline is shared the same way, and the last request keeps it as
// it is.
func TestSendBudget(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var deadlines []time.Time

		client := typesmocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			deadline, _ := req.Context().Deadline()
			deadlines = append(deadlines, deadline)

			return newResponse(req, http.StatusOK, sendSuccessBody), nil
		})

		service := newTestService(t, "telegram://12345:mock-token@telegram/?chats=channel-1,channel-2")
		service.SetHTTPClient(client)

		start := time.Now()
		budget := service.ServiceTimeout(nil)

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, []time.Time{start.Add(budget / 2), start.Add(budget)}, deadlines,
			"the first chat gets half of the budget and the last gets the rest")

		deadlines = nil

		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		defer cancel()

		require.NoError(t, service.SendContext(ctx, "Message", nil))
		assert.Equal(t, []time.Time{start.Add(30 * time.Minute), start.Add(time.Hour)}, deadlines,
			"the caller's deadline is shared, and the last request keeps it")

		deadlines = nil

		params := types.Params{"chats": "channel-1,channel-2,channel-3"}
		budget = service.ServiceTimeout(&params)

		require.NoError(t, service.Send("Message", &params))
		require.Len(t, deadlines, 3)
		assert.Equal(t, start.Add(budget), deadlines[2], "the chats from params set the budget")
	})
}

// TestSendContextCanceled verifies that a context canceled before the send
// skips every chat and reports the context's error.
func TestSendContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	service := newTestService(t, testServiceURL)
	// The mock has no expectations, so any request fails the test.
	service.SetHTTPClient(typesmocks.NewMockHTTPClient(t))

	require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
}

// TestSetHTTPClient verifies that the service uses an injected client, and
// otherwise a default client without a request timeout, since the send
// budget bounds each send.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		t.Parallel()

		client, ok := newTestService(t, testServiceURL).client.Client().(*http.Client)
		require.True(t, ok, "the default client should be an *http.Client")
		assert.Zero(t, client.Timeout)
	})

	t.Run("injected", func(t *testing.T) {
		t.Parallel()

		injected := typesmocks.NewMockHTTPClient(t)
		service := newTestService(t, testServiceURL)
		service.SetHTTPClient(injected)

		assert.Same(t, injected, service.client.Client())
	})
}

// TestServiceTimeout verifies that the send budget is one client timeout for
// each chat, using the chats from params when they are valid.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	// tests lists send params with the budget they should produce for a service
	// configured with three chats.
	tests := []struct {
		name   string
		params *types.Params
		want   time.Duration
	}{
		{name: "configured chats", params: nil, want: 3 * defaultHTTPTimeout},
		{name: "chats set by params", params: &types.Params{"chats": "a,b"}, want: 2 * defaultHTTPTimeout},
		{name: "configured chats when a param is invalid", params: &types.Params{"preview": "maybe"}, want: 3 * defaultHTTPTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "telegram://12345:mock-token@telegram/?chats=a,b,c")

			assert.Equal(t, tt.want, service.ServiceTimeout(tt.params))
		})
	}
}

// TestSendUsesAPIURL verifies that a send posts to the configured Bot API
// server, such as a self-hosted server on plain HTTP or a proxy under a path,
// and to api.telegram.org when none is set. A trailing slash is ignored.
func TestSendUsesAPIURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		query   string
		params  *types.Params
		wantURL string
	}{
		"default":          {wantURL: "https://api.telegram.org" + sendMessagePath},
		"self-hosted":      {query: "&apiurl=http://localhost:8081", wantURL: "http://localhost:8081" + sendMessagePath},
		"proxy under path": {query: "&apiurl=https://proxy.example.com/telegram/", wantURL: "https://proxy.example.com/telegram" + sendMessagePath},
		"IPv6 host":        {query: "&apiurl=http://[::1]:8081", wantURL: "http://[::1]:8081" + sendMessagePath},
		"API URL as param": {
			params:  &types.Params{"apiurl": "https://tg.example.com"},
			wantURL: "https://tg.example.com" + sendMessagePath,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var urls []string

			service := newTestService(t, testServiceURL+tc.query)
			service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				urls = append(urls, req.URL.String())

				return newResponse(req, http.StatusOK, sendSuccessBody), nil
			})})

			require.NoError(t, service.Send("Message", tc.params))
			assert.Equal(t, []string{tc.wantURL}, urls)
		})
	}
}

// TestSendSkipTLSVerify verifies sends to a Bot API server whose certificate
// cannot be verified, such as a self-hosted server behind a proxy with its own
// certificate. Without skiptlsverify the send fails. With it in the URL or the
// params, every chat's request goes through the insecure default client, and
// the warning is logged once over two sends. An injected client is used
// unchanged and never warned about.
func TestSendSkipTLSVerify(t *testing.T) {
	t.Parallel()

	const insecureWarning = "WARN skipping TLS certificate verification"

	// tests lists the URL query, the send params, and whether a client is
	// injected, with the expected outcome of both sends.
	tests := []struct {
		params       *types.Params
		name         string
		query        string
		injected     bool
		wantErr      bool
		wantWarnings int
	}{
		{name: "verified by default", wantErr: true},
		{name: "URL", query: "&skiptlsverify=yes", wantWarnings: 1},
		{name: "params", params: &types.Params{"skiptlsverify": "yes"}, wantWarnings: 1},
		{name: "injected client", query: "&skiptlsverify=yes", injected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var requests atomic.Int32

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)

				_, _ = io.WriteString(w, sendSuccessBody)
			}))
			t.Cleanup(server.Close)

			var logs strings.Builder

			service := &Service{}
			rawURL := testServiceURL + ",channel-2&apiurl=" + url.QueryEscape(server.URL) + tt.query
			require.NoError(t, service.Initialize(parseURL(t, rawURL), log.New(&logs, "", 0)))

			if tt.injected {
				service.SetHTTPClient(server.Client())
			}

			for range 2 {
				err := service.Send("Message", tt.params)
				require.Equal(t, tt.wantErr, err != nil, "send error: %v", err)
			}

			wantRequests := int32(4)
			if tt.wantErr {
				wantRequests = 0
			}

			assert.Equal(t, wantRequests, requests.Load(), "two chats over two sends")
			assert.Equal(t, tt.wantWarnings, strings.Count(logs.String(), insecureWarning))
		})
	}
}

// TestSendWarnsPlaintext verifies that sends to an http Bot API server, which
// carry the bot token in their path, log the plaintext warning once, and again
// after the service is initialized again. An https or default API URL logs
// nothing.
func TestSendWarnsPlaintext(t *testing.T) {
	t.Parallel()

	const plaintextWarning = "WARN sending without TLS"

	// tests lists the URL query and the send params with the number of
	// warnings over two sends.
	tests := []struct {
		params       *types.Params
		name         string
		query        string
		wantWarnings int
	}{
		{name: "default API URL"},
		{name: "https API URL", query: "&apiurl=https://tg.example.com"},
		{name: "http API URL", query: "&apiurl=http://localhost:8081", wantWarnings: 1},
		{name: "http API URL as param", params: &types.Params{"apiurl": "http://localhost:8081"}, wantWarnings: 1},
		{name: "uppercase http scheme", query: "&apiurl=HTTP://localhost:8081", wantWarnings: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs strings.Builder

			service := &Service{}
			require.NoError(t, service.Initialize(parseURL(t, testServiceURL+tt.query), log.New(&logs, "", 0)))
			service.SetHTTPClient(&http.Client{Transport: respondWith(http.StatusOK, sendSuccessBody)})

			require.NoError(t, service.Send("first", tt.params))
			require.NoError(t, service.Send("second", tt.params))
			assert.Equal(t, tt.wantWarnings, strings.Count(logs.String(), plaintextWarning))
		})
	}

	t.Run("again after Initialize", func(t *testing.T) {
		t.Parallel()

		var logs strings.Builder

		rawURL := testServiceURL + "&apiurl=http://localhost:8081"
		service := &Service{}
		service.SetHTTPClient(&http.Client{Transport: respondWith(http.StatusOK, sendSuccessBody)})

		require.NoError(t, service.Initialize(parseURL(t, rawURL), log.New(&logs, "", 0)))
		require.NoError(t, service.Send("first", nil))
		require.NoError(t, service.Initialize(parseURL(t, rawURL), log.New(&logs, "", 0)))
		require.NoError(t, service.Send("second", nil))

		assert.Equal(t, 2, strings.Count(logs.String(), plaintextWarning))
	})
}

// TestAPIURLValidation verifies that an API URL must be an http or https URL
// with a host and without userinfo, a query, or a fragment, both in the
// service URL and in the send params, and that a rejected URL makes no request.
func TestAPIURLValidation(t *testing.T) {
	t.Parallel()

	invalid := map[string]string{
		"other scheme":   "ftp://tg.example.com",
		"no host":        "https://",
		"port only":      "http://:8081",
		"no scheme":      "tg.example.com",
		"userinfo":       "https://user:pass@tg.example.com",
		"query":          "https://tg.example.com/?a=1",
		"empty query":    "https://tg.example.com/?",
		"fragment":       "https://tg.example.com/#top",
		"empty fragment": "https://tg.example.com/#",
		"does not parse": "https://tg.example.com:port",
	}

	for name, apiURL := range invalid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := (&Service{}).Initialize(
				parseURL(t, testServiceURL+"&apiurl="+url.QueryEscape(apiURL)),
				log.New(t.Output(), "", 0),
			)
			require.ErrorIs(t, err, ErrInvalidAPIURL)

			service := newTestService(t, testServiceURL)
			service.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Error("a send with an invalid API URL made a request")

				return nil, errors.New("unexpected request")
			})})

			require.ErrorIs(t, service.Send("Message", &types.Params{"apiurl": apiURL}), ErrInvalidAPIURL)
		})
	}
}
