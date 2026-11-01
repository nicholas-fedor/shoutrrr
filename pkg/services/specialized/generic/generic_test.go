package generic

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// header is the request header.
	header http.Header
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// body is the request body.
	body string
}

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// captureRequest reads the method, URL, header, and body of req.
//
// Parameters:
//   - t: the test.
//   - req: the request to read.
//
// Returns:
//   - capturedRequest: the recorded request.
func captureRequest(t *testing.T, req *http.Request) capturedRequest {
	t.Helper()

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	return capturedRequest{header: req.Header.Clone(), method: req.Method, url: req.URL.String(), body: string(body)}
}

// newRespondingClient returns a mock HTTP client that expects one request,
// records it, and answers with the given status and an empty body.
//
// Parameters:
//   - t: the test that owns the mock.
//   - status: the response status line, such as "200 OK".
//   - statusCode: the response status code.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the request, filled in once the client receives it.
func newRespondingClient(t *testing.T, status string, statusCode int) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		*captured = captureRequest(t, req)

		return &http.Response{Status: status, StatusCode: statusCode, Body: io.NopCloser(strings.NewReader(""))}, nil
	}).Once()

	return client, captured
}

// newTestService returns a service initialized from rawURL that logs to the
// test output.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, rawURL string) *Service {
	t.Helper()

	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(t.Output(), "", 0)))

	return service
}

// TestGetID verifies that the service identifies itself by its scheme.
func TestGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "generic", (&Service{}).GetID())
}

// TestServiceTimeout verifies that the service reports the router's default
// send budget.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	assert.Equal(t, types.DefaultSendTimeout, (&Service{}).ServiceTimeout(nil))
}

// TestGetServiceURLFromCustom verifies that a generic+ custom URL becomes a
// service URL for the same webhook, and that an invalid prop is rejected.
func TestGetServiceURLFromCustom(t *testing.T) {
	t.Parallel()

	// tests lists custom URLs with the webhook URL the service should post to.
	tests := []struct {
		name        string
		customURL   string
		wantWebhook string
	}{
		{name: "https", customURL: "generic+https://test.tld", wantWebhook: "https://test.tld"},
		{name: "http", customURL: "generic+http://test.tld/hook?template=json", wantWebhook: "http://test.tld/hook"},
		{name: "query kept", customURL: "generic+https://test.tld/hook?token=abc", wantWebhook: "https://test.tld/hook?token=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			serviceURL, err := (&Service{}).GetServiceURLFromCustom(parseTestURL(t, tt.customURL))
			require.NoError(t, err)

			service := newTestService(t, serviceURL.String())
			assert.Equal(t, tt.wantWebhook, service.Config.WebhookURL().String())
		})
	}

	t.Run("invalid prop", func(t *testing.T) {
		t.Parallel()

		serviceURL, err := (&Service{}).GetServiceURLFromCustom(parseTestURL(t, "generic+https://test.tld?disabletls=maybe"))

		require.Error(t, err)
		assert.Nil(t, serviceURL)
	})
}

// TestGetPayload verifies the payload for each template choice: the message
// alone without a template, the params and extra data as JSON for the JSON
// template, and a named template rendered with the params.
func TestGetPayload(t *testing.T) {
	t.Parallel()

	// tests lists service URLs, templates, and params with the payload each
	// should produce. wantJSON compares the payload as JSON.
	tests := []struct {
		params   types.Params
		name     string
		rawURL   string
		template string
		want     string
		wantJSON bool
	}{
		{
			name:   "no template",
			rawURL: "generic://host.tld/webhook",
			params: types.Params{"message": "test message", "title": "ignored"},
			want:   "test message",
		},
		{
			name:     "JSON template",
			rawURL:   "generic://host.tld/webhook?template=JSON&$context=inside+joke",
			params:   types.Params{"title": "test title", "message": "test message"},
			want:     `{"title":"test title","message":"test message","context":"inside joke"}`,
			wantJSON: true,
		},
		{
			name:     "lowercase json template with custom keys",
			rawURL:   "generic://host.tld/webhook?template=json&messagekey=body&titlekey=header",
			params:   types.Params{"header": "test title", "body": "test message"},
			want:     `{"header":"test title","body":"test message"}`,
			wantJSON: true,
		},
		{
			name:     "named template",
			rawURL:   "generic://host.tld/webhook?template=news",
			template: `{{.title}} ==> {{.message}}`,
			params:   types.Params{"title": "BREAKING NEWS", "message": "it's today!"},
			want:     "BREAKING NEWS ==> it's today!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			if tt.template != "" {
				require.NoError(t, service.SetTemplateString(service.Config.Template, tt.template))
			}

			payload, err := service.GetPayload(service.Config, tt.params)
			require.NoError(t, err)

			contents, err := io.ReadAll(payload)
			require.NoError(t, err)

			if tt.wantJSON {
				assert.JSONEq(t, tt.want, string(contents))
			} else {
				assert.Equal(t, tt.want, string(contents))
			}
		})
	}
}

// TestGetPayloadErrors verifies that a template that is not loaded or fails
// to execute is an error.
func TestGetPayloadErrors(t *testing.T) {
	t.Parallel()

	t.Run("template not loaded", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "generic://host.tld/webhook?template=missing")

		_, err := service.GetPayload(service.Config, nil)

		require.ErrorIs(t, err, ErrTemplateNotLoaded)
	})

	t.Run("template execution fails", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "generic://host.tld/webhook?template=broken")
		require.NoError(t, service.SetTemplateString("broken", `{{template "undefined"}}`))

		_, err := service.GetPayload(service.Config, types.Params{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), `executing template "broken"`)
	})
}

// TestSend verifies the request a send makes: the configured method to the
// webhook URL, the payload for the template with its content type, the
// custom headers, and the title param under the title key.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the request each send should
	// make. wantJSON compares the body as JSON.
	tests := []struct {
		params      *types.Params
		wantHeaders map[string]string
		name        string
		rawURL      string
		wantMethod  string
		wantURL     string
		wantBody    string
		wantJSON    bool
	}{
		{
			name:        "plain text",
			rawURL:      "generic://host.tld/webhook",
			wantMethod:  http.MethodPost,
			wantURL:     "https://host.tld/webhook",
			wantBody:    "Message",
			wantHeaders: map[string]string{"Content-Type": "text/plain", "Accept": "text/plain"},
		},
		{
			name:        "JSON with a title param and custom keys",
			rawURL:      "generic://host.tld/webhook?template=json&titlekey=subject&messagekey=text",
			params:      &types.Params{"title": "TITLE", "priority": "high"},
			wantMethod:  http.MethodPost,
			wantURL:     "https://host.tld/webhook",
			wantBody:    `{"subject":"TITLE","text":"Message","priority":"high"}`,
			wantJSON:    true,
			wantHeaders: map[string]string{"Content-Type": "application/json", "Accept": "application/json"},
		},
		{
			name:        "method, content type, headers, and HTTP",
			rawURL:      "generic://host.tld/webhook?method=PUT&disabletls=yes&template=json&contenttype=application/vnd.api%2Bjson&@authorization=frend",
			wantMethod:  http.MethodPut,
			wantURL:     "http://host.tld/webhook",
			wantBody:    `{"message":"Message"}`,
			wantJSON:    true,
			wantHeaders: map[string]string{"Content-Type": "application/vnd.api+json", "Authorization": "frend"},
		},
		{
			name:       "custom header keys with acronyms, digits, and camelCase",
			rawURL:     "generic://host.tld/webhook?@X-API-Key=key&@x-1st=first&@acceptLanguage=tlh-Piqd",
			wantMethod: http.MethodPost,
			wantURL:    "https://host.tld/webhook",
			wantBody:   "Message",
			wantHeaders: map[string]string{
				"X-Api-Key": "key", "X-1st": "first", "Accept-Language": "tlh-Piqd",
			},
		},
		{
			name:       "config props from params",
			rawURL:     "generic://host.tld/webhook",
			params:     &types.Params{"method": "GET"},
			wantMethod: http.MethodGet,
			wantURL:    "https://host.tld/webhook",
			wantBody:   "Message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Message", tt.params))

			assert.Equal(t, tt.wantMethod, captured.method)
			assert.Equal(t, tt.wantURL, captured.url)

			if tt.wantJSON {
				assert.JSONEq(t, tt.wantBody, captured.body)
			} else {
				assert.Equal(t, tt.wantBody, captured.body)
			}

			for key, value := range tt.wantHeaders {
				assert.Equal(t, value, captured.header.Get(key), key)
			}
		})
	}
}

// TestSendChecksConfig verifies that a send with a config built in code
// without a webhook fails before any request, as reading a URL without a host
// does.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "generic://example.com/webhook")
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))
	service.Config = &Config{}

	require.ErrorIs(t, service.Send("Message", nil), ErrHostMissing)
}

// TestSendLeavesParamsAndConfigUnchanged verifies that a send neither adds the
// message to the caller's params nor keeps param overrides in the config.
func TestSendLeavesParamsAndConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "generic://host.tld/webhook?template=json")
	want := *service.Config
	client, _ := newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)

	params := types.Params{"title": "TITLE", "method": "PUT"}
	require.NoError(t, service.Send("Message", &params))

	assert.Equal(t, types.Params{"title": "TITLE", "method": "PUT"}, params)
	assert.Equal(t, want, *service.Config)
}

// TestSendAppliesOnlyConfigKeyParams verifies that params that are not config
// keys fill the payload without a logged error, while config keys in any case
// still update the config of the send.
func TestSendAppliesOnlyConfigKeyParams(t *testing.T) {
	t.Parallel()

	logs := &bytes.Buffer{}
	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, "generic://host.tld/webhook?template=json"), log.New(logs, "", 0)))

	client, captured := newRespondingClient(t, "200 OK", http.StatusOK)
	service.SetHTTPClient(client)

	params := types.Params{"priority": "high", "Method": "PUT"}
	require.NoError(t, service.Send("Message", &params))

	assert.Equal(t, http.MethodPut, captured.method)
	assert.JSONEq(t, `{"message":"Message","priority":"high","Method":"PUT"}`, captured.body)
	assert.NotContains(t, logs.String(), format.ErrInvalidConfigKey.Error())
}

// TestSendLogsStatusWithoutBody verifies that a send logs the response status
// but not a body that echoes the request URL.
func TestSendLogsStatusWithoutBody(t *testing.T) {
	t.Parallel()

	logs := &bytes.Buffer{}
	service := &Service{}
	require.NoError(t, service.Initialize(parseTestURL(t, "generic://host.tld/webhook?token=SECRETquery"), log.New(logs, "", 0)))

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(req.URL.String() + " BODYmarker")),
		}, nil
	}).Once()
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", nil))

	assert.Contains(t, logs.String(), "200 OK")
	assert.NotContains(t, logs.String(), "SECRETquery")
	assert.NotContains(t, logs.String(), "BODYmarker")
}

// TestSendBudget verifies that Send and SendContext without a caller deadline
// bound the request by the budget that ServiceTimeout reports, and that
// SendContext keeps a caller's deadline as it is.
func TestSendBudget(t *testing.T) {
	t.Parallel()

	// sends lists the send entries with the caller deadline they pass, or zero
	// for none.
	sends := []struct {
		name           string
		callerDeadline time.Duration
		viaContext     bool
	}{
		{name: "Send"},
		{name: "SendContext without a deadline", viaContext: true},
		{name: "SendContext with a deadline", viaContext: true, callerDeadline: time.Hour},
	}

	for _, tt := range sends {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var deadline time.Time

				service := newTestService(t, "generic://hooks.example.invalid/webhook")
				client := mocks.NewMockHTTPClient(t)
				client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
					deadline, _ = req.Context().Deadline()

					return &http.Response{Status: "200 OK", StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
				}).Once()
				service.SetHTTPClient(client)

				want := time.Now().Add(service.ServiceTimeout(nil))

				switch {
				case !tt.viaContext:
					require.NoError(t, service.Send("message", nil))
				case tt.callerDeadline > 0:
					ctx, cancel := context.WithTimeout(t.Context(), tt.callerDeadline)
					defer cancel()

					want = time.Now().Add(tt.callerDeadline)

					require.NoError(t, service.SendContext(ctx, "message", nil))
				default:
					require.NoError(t, service.SendContext(t.Context(), "message", nil))
				}

				assert.Equal(t, want, deadline)
			})
		})
	}
}

// TestSendWarnsPlaintext verifies that sends without TLS log the plaintext
// warning once, and again after the service is initialized again.
func TestSendWarnsPlaintext(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	rawURL := "generic://hooks.example.invalid/webhook?disabletls=yes"

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(*http.Request) (*http.Response, error) {
		return &http.Response{Status: "200 OK", StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
	}).Times(3)

	service := &Service{}
	service.SetHTTPClient(client)
	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))

	require.NoError(t, service.Send("first", nil))
	require.NoError(t, service.Send("second", nil))
	assert.Equal(t, 1, strings.Count(logs.String(), "WARN sending without TLS"))

	require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))
	require.NoError(t, service.Send("third", nil))
	assert.Equal(t, 2, strings.Count(logs.String(), "WARN sending without TLS"))
}

// TestSendSkipTLSVerify verifies sends to a server whose certificate cannot be
// verified. Without skiptlsverify the send fails. With it in the URL or the
// params, the send goes through the insecure default client and warns once
// over two sends, and the key is not forwarded in the webhook query. An injected
// client is used unchanged and never warned about.
func TestSendSkipTLSVerify(t *testing.T) {
	t.Parallel()

	const insecureWarning = "WARN skipping TLS certificate verification"

	// tests lists the URL query, the send params, and whether a client is
	// injected, with the expected outcome.
	tests := []struct {
		params       *types.Params
		name         string
		query        string
		injected     bool
		wantErr      bool
		wantWarnings int
	}{
		{name: "verified by default", wantErr: true},
		{name: "URL", query: "?skiptlsverify=yes", wantWarnings: 1},
		{name: "params", params: &types.Params{"skiptlsverify": "yes"}, wantWarnings: 1},
		{name: "injected client", query: "?skiptlsverify=yes", injected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				assert.Empty(t, req.URL.RawQuery, "the webhook received a query")
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)

			var logs strings.Builder

			service := &Service{}
			rawURL := "generic://" + server.Listener.Addr().String() + "/hook" + tt.query
			require.NoError(t, service.Initialize(parseTestURL(t, rawURL), log.New(&logs, "", 0)))

			if tt.injected {
				service.SetHTTPClient(server.Client())
			}

			for range 2 {
				err := service.Send("Message", tt.params)
				require.Equal(t, tt.wantErr, err != nil, "send error: %v", err)
			}

			assert.Equal(t, tt.wantWarnings, strings.Count(logs.String(), insecureWarning))
		})
	}
}

// TestSendContextErrors verifies that a send fails with ErrSendFailed for a
// payload error, an error status, or a transport error without the webhook
// query, accepts statuses below 400, and stops when the caller's context is
// canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("error status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "generic://host.tld/webhook")
		client, _ := newRespondingClient(t, "404 Not Found", http.StatusNotFound)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, ErrUnexpectedStatus)
		assert.Contains(t, err.Error(), "404 Not Found")
	})

	t.Run("redirect status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "generic://host.tld/webhook")
		client, _ := newRespondingClient(t, "304 Not Modified", http.StatusNotModified)
		service.SetHTTPClient(client)

		require.NoError(t, service.SendContext(t.Context(), "Message", nil))
	})

	t.Run("payload error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "generic://host.tld/webhook?template=missing")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, ErrTemplateNotLoaded)
	})

	t.Run("invalid config param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "generic://host.tld/webhook")
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		err := service.SendContext(t.Context(), "Message", &types.Params{"disabletls": "maybe"})

		require.ErrorIs(t, err, format.ErrInvalidBoolValue)
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "generic://host.tld/webhook?token=SECRETquery")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errTransport}
		}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), "SECRETquery")
	})

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		service := newTestService(t, "generic://hooks.example.invalid/webhook")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendRedactsUnparsableURL verifies that a host that makes the webhook URL
// unparsable fails the send before any request, without the credentials in
// the error.
func TestSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := &Service{}
	serviceURL := &url.URL{Scheme: "generic", Host: "bad host", Path: "/hook", User: url.UserPassword("user", "SECRETpassword")}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETpassword")
}

// TestSendDefaultClient verifies that a send without a custom client, or with
// a nil *http.Client, reaches a plain HTTP server through the default client.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	// tests lists how each case sets up the service's HTTP client.
	tests := []struct {
		configure func(*Service)
		name      string
	}{
		{name: "no client set"},
		{name: "nil *http.Client", configure: func(s *Service) { s.SetHTTPClient((*http.Client)(nil)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requests := make(chan capturedRequest, 1)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				body, _ := io.ReadAll(req.Body)

				// A non-blocking send keeps an extra request from stalling
				// server.Close, which waits for running handlers.
				select {
				case requests <- capturedRequest{method: req.Method, url: req.URL.String(), body: string(body)}:
				default:
					t.Errorf("unexpected extra request: %s %s", req.Method, req.URL)
				}

				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(server.Close)

			service := newTestService(t, "generic://"+server.Listener.Addr().String()+"/hook?disabletls=yes")

			if tt.configure != nil {
				tt.configure(service)
			}

			require.NoError(t, service.Send("Message", nil))

			received := <-requests
			assert.Equal(t, http.MethodPost, received.method)
			assert.Equal(t, "/hook", received.url)
			assert.Equal(t, "Message", received.body)
		})
	}
}

// TestSetHTTPClient verifies that SetHTTPClient makes sends use a custom
// client, and that a nil client or a nil *http.Client restores the default.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := newTestService(t, "generic://host.tld/webhook")
		service.SetHTTPClient(client)

		assert.Same(t, client, service.client.Client())
	})

	// nilClients lists the nil clients that restore the default client.
	nilClients := []struct {
		name   string
		client types.HTTPClient
	}{
		{name: "nil", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range nilClients {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "generic://host.tld/webhook")
			defaultClient := service.client.Client()

			service.SetHTTPClient(mocks.NewMockHTTPClient(t))
			service.SetHTTPClient(tt.client)

			assert.Same(t, defaultClient, service.client.Client())
		})
	}
}

// TestDefaultHTTPClient verifies the default client's timeout, and that it
// sends through http.DefaultTransport, so it honors the proxy environment
// variables.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "generic://host.tld/webhook")

	httpClient, ok := service.client.Client().(*http.Client)
	require.True(t, ok)
	assert.Zero(t, httpClient.Timeout, "the send context bounds each request")
	assert.Nil(t, httpClient.Transport)
}
