package pushover

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// capturedRequest records the parts of a request that an HTTP client received.
type capturedRequest struct {
	// form is the parsed form body.
	form url.Values
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
}

// roundTripFunc adapts a function to an [http.RoundTripper].
type roundTripFunc func(req *http.Request) (*http.Response, error)

// Response bodies in the format the Pushover API returns.
const (
	// successBody is the body of an accepted message.
	successBody = `{"status":1,"request":"647d2300-702c-4b38-8b2f-d56326ae460b"}`
	// invalidUserBody is the body of a message to an invalid user key.
	invalidUserBody = `{"user":"invalid","errors":["user identifier is invalid"],"status":0,"request":"5042853c-402d-4a18-abcb-168734a801de"}`
)

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// captureRequest reads the method, URL, Content-Type header, and form body of
// req.
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

	form, err := url.ParseQuery(string(body))
	require.NoError(t, err)

	return capturedRequest{
		form:        form,
		method:      req.Method,
		url:         req.URL.String(),
		contentType: req.Header.Get("Content-Type"),
	}
}

// newRespondingClient returns a mock HTTP client that expects one request,
// records it, and answers with the given status and body.
//
// Parameters:
//   - t: the test that owns the mock.
//   - status: the response status line, such as "200 OK".
//   - statusCode: the response status code.
//   - body: the response body.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the request, filled in once the client receives it.
func newRespondingClient(t *testing.T, status string, statusCode int, body string) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		*captured = captureRequest(t, req)

		return &http.Response{
			Status:     status,
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
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

	assert.Equal(t, "pushover", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the service URL, builds the
// default client unless one is set, and rejects an invalid encryption key.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushover://:apptoken@usertoken?devices=phone")

		assert.Equal(t, &Config{User: "usertoken", Token: "apptoken", Devices: []string{"phone"}}, service.Config)
		require.NotNil(t, service.Client)
		assert.Equal(t, defaultHTTPTimeout, service.Client.Timeout)
	})

	t.Run("keeps a client set before it runs", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		service := &Service{Client: client}
		require.NoError(t, service.Initialize(parseTestURL(t, "pushover://:apptoken@usertoken"), log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.Client)
	})

	t.Run("invalid encryption key", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "pushover://:apptoken@usertoken?encryptionkey=short"), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrInvalidEncryptionKey)
	})
}

// TestSend verifies the request a send makes: a form POST to the messages
// endpoint with the devices, user, token, message, and priority, and the title
// only when set.
func TestSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and params with the form the send should
	// produce.
	tests := []struct {
		params   *types.Params
		name     string
		rawURL   string
		wantForm url.Values
	}{
		{
			name:   "all devices",
			rawURL: "pushover://:apptoken@usertoken",
			wantForm: url.Values{
				"device": {""}, "user": {"usertoken"}, "token": {"apptoken"}, "message": {"Message"}, "priority": {"0"},
			},
		},
		{
			name:   "devices, priority, and title",
			rawURL: "pushover://:apptoken@usertoken?devices=phone,tablet&priority=1&title=Plain+Title",
			wantForm: url.Values{
				"device": {"phone,tablet"}, "user": {"usertoken"}, "token": {"apptoken"}, "message": {"Message"},
				"priority": {"1"}, "title": {"Plain Title"},
			},
		},
		{
			name:   "lowest priority",
			rawURL: "pushover://:apptoken@usertoken?priority=-2",
			wantForm: url.Values{
				"device": {""}, "user": {"usertoken"}, "token": {"apptoken"}, "message": {"Message"}, "priority": {"-2"},
			},
		},
		{
			// Emergency priority needs the retry and expire parameters, which
			// the service does not send, so the priority is left out.
			name:   "emergency priority is left out",
			rawURL: "pushover://:apptoken@usertoken?priority=2",
			wantForm: url.Values{
				"device": {""}, "user": {"usertoken"}, "token": {"apptoken"}, "message": {"Message"},
			},
		},
		{
			name:   "params",
			rawURL: "pushover://:apptoken@usertoken?devices=phone&title=Configured",
			params: &types.Params{"devices": "tablet", "priority": "-1", "title": "Override"},
			wantForm: url.Values{
				"device": {"tablet"}, "user": {"usertoken"}, "token": {"apptoken"}, "message": {"Message"},
				"priority": {"-1"}, "title": {"Override"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Message", tt.params))

			assert.Equal(t, http.MethodPost, captured.method)
			assert.Equal(t, hookURL, captured.url)
			assert.Equal(t, contentType, captured.contentType)
			assert.Equal(t, tt.wantForm, captured.form)
		})
	}
}

// TestSendLevelPriority verifies that the level param sets the priority, debug
// to low (-1), info and warning to normal (0), and error to high (1), and that
// a priority from the service URL or the send params, or an unknown level,
// leaves the priority as it is.
func TestSendLevelPriority(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		rawURL       string
		params       *types.Params
		wantPriority string
	}{
		"debug":               {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Debug"}, wantPriority: "-1"},
		"info":                {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Info"}, wantPriority: "0"},
		"warning":             {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Warning"}, wantPriority: "0"},
		"error":               {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Error"}, wantPriority: "1"},
		"priority in the URL": {rawURL: "pushover://:apptoken@usertoken?priority=-2", params: &types.Params{"level": "Error"}, wantPriority: "-2"},
		"priority param":      {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Error", "priority": "0"}, wantPriority: "0"},
		"unknown level":       {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "critical"}, wantPriority: "0"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tc.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Message", tc.params))

			assert.Equal(t, tc.wantPriority, captured.form.Get("priority"))
		})
	}
}

// TestSendEncrypted verifies that with an encryption key the message and
// title are sent encrypted with the documented scheme and marked with
// encrypted=1, and that an encryption key param applies to one send.
func TestSendEncrypted(t *testing.T) {
	t.Parallel()

	// tests lists how each case supplies the encryption key.
	tests := []struct {
		params *types.Params
		name   string
		rawURL string
	}{
		{name: "configured key", rawURL: "pushover://:apptoken@usertoken?encryptionkey=" + testEncryptionKey + "&title=Secret+Title"},
		{
			name:   "key param",
			rawURL: "pushover://:apptoken@usertoken?title=Secret+Title",
			params: &types.Params{"key": testEncryptionKey},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tt.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Secret message", tt.params))

			assert.Equal(t, "1", captured.form.Get("encrypted"))

			message, err := decryptField(t, captured.form.Get("message"), testKey(t))
			require.NoError(t, err)
			assert.Equal(t, "Secret message", message)

			title, err := decryptField(t, captured.form.Get("title"), testKey(t))
			require.NoError(t, err)
			assert.Equal(t, "Secret Title", title)
		})
	}
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the stored config and the next send keep the
// configured values.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "pushover://:apptoken@usertoken?devices=phone&title=Configured")
	want := *service.Config

	client, _ := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
	service.SetHTTPClient(client)

	params := &types.Params{"devices": "tablet", "title": "Override", "encryptionkey": testEncryptionKey}
	require.NoError(t, service.Send("Message", params))
	assert.Equal(t, want, *service.Config)

	client, captured := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", nil))
	assert.Equal(t, url.Values{
		"device": {"phone"}, "user": {"usertoken"}, "token": {"apptoken"}, "message": {"Message"},
		"priority": {"0"}, "title": {"Configured"},
	}, captured.form)
}

// TestSendContextErrors verifies that a send fails with ErrSendFailed for an
// error status, wraps a transport error, rejects an invalid param or
// encryption key before any request, and stops when the caller's context is
// canceled.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("error status", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushover://:apptoken@usertoken?devices=phone")
		client, _ := newRespondingClient(t, "400 Bad Request", http.StatusBadRequest, invalidUserBody)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		assert.Contains(t, err.Error(), `"phone"`)
		assert.Contains(t, err.Error(), "400 Bad Request")
	})

	t.Run("transport error", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushover://:apptoken@usertoken")
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).Return(nil, errTransport).Once()
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(t.Context(), "Message", nil), errTransport)
	})

	// rejectedParams lists params that fail the send before any request.
	rejectedParams := []struct {
		params  *types.Params
		wantErr error
		name    string
	}{
		{name: "invalid encryption key", params: &types.Params{"encryptionkey": "short"}, wantErr: ErrInvalidEncryptionKey},
		{name: "unknown param", params: &types.Params{"foo": "bar"}, wantErr: format.ErrInvalidConfigKey},
	}

	for _, tt := range rejectedParams {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "pushover://:apptoken@usertoken")
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))

			require.ErrorIs(t, service.SendContext(t.Context(), "Message", tt.params), tt.wantErr)
		})
	}

	t.Run("canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, "pushover://:token@user/?devices=device")
		service.SetHTTPClient(client)

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})
}

// TestSendDefaultClient verifies that a send without a custom client, or with
// a nil *http.Client, goes through the service's Client.
func TestSendDefaultClient(t *testing.T) {
	t.Parallel()

	// tests lists how each case sets up the service's custom client.
	tests := []struct {
		configure func(*Service)
		name      string
	}{
		{name: "no custom client", configure: func(*Service) {}},
		{name: "nil *http.Client", configure: func(s *Service) { s.SetHTTPClient((*http.Client)(nil)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured capturedRequest

			service := newTestService(t, "pushover://:apptoken@usertoken")
			service.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				captured = captureRequest(t, req)

				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(successBody))}, nil
			})}
			tt.configure(service)

			require.NoError(t, service.Send("Message", nil))

			assert.Equal(t, hookURL, captured.url)
			assert.Equal(t, "Message", captured.form.Get("message"))
		})
	}
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client and
// clears it for a nil *http.Client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)

		assert.Same(t, client, service.httpClient)
	})

	t.Run("nil *http.Client", func(t *testing.T) {
		t.Parallel()

		service := &Service{httpClient: mocks.NewMockHTTPClient(t)}
		service.SetHTTPClient((*http.Client)(nil))

		assert.Nil(t, service.httpClient)
	})
}
