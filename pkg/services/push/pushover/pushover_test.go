package pushover

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
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

// TestInitialize verifies that Initialize parses the service URL without
// filling the deprecated client field, keeps a client set before it, and
// rejects an invalid encryption key.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushover://:apptoken@usertoken?devices=phone")

		assert.Equal(t, &Config{User: "usertoken", Token: "apptoken", Devices: []string{"phone"}}, service.Config)
		assert.Nil(t, service.Client, "Initialize leaves the deprecated client field unset")
	})

	t.Run("keeps a client set before it runs", func(t *testing.T) {
		t.Parallel()

		client := &http.Client{}
		service := &Service{Client: client}
		require.NoError(t, service.Initialize(parseTestURL(t, "pushover://:apptoken@usertoken"), log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.client.Prefer(service.Client))
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
		"debug":                       {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Debug"}, wantPriority: "-1"},
		"info":                        {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Info"}, wantPriority: "0"},
		"warning":                     {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Warning"}, wantPriority: "0"},
		"error":                       {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Error"}, wantPriority: "1"},
		"priority in the URL":         {rawURL: "pushover://:apptoken@usertoken?priority=-2", params: &types.Params{"level": "Error"}, wantPriority: "-2"},
		"default priority in the URL": {rawURL: "pushover://:apptoken@usertoken?priority=0", params: &types.Params{"level": "Error"}, wantPriority: "0"},
		"priority param":              {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "Error", "priority": "0"}, wantPriority: "0"},
		"unknown level":               {rawURL: "pushover://:apptoken@usertoken", params: &types.Params{"level": "critical"}, wantPriority: "0"},
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

// TestSendChecksConfig verifies that a send checks a config built in code as
// reading a URL does. A config without a user or a token fails before any
// request. So do an invalid encryption key and a negative timestamp.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "zero config", change: func(config *Config) { *config = Config{} }, wantErr: ErrUserMissing},
		{name: "no token", change: func(config *Config) { config.Token = "" }, wantErr: ErrTokenMissing},
		{name: "invalid encryption key", change: func(config *Config) { config.EncryptionKey = "short" }, wantErr: ErrInvalidEncryptionKey},
		{name: "negative timestamp", change: func(config *Config) { config.Timestamp = -1 }, wantErr: ErrInvalidTimestamp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, "pushover://:apptoken@usertoken")
			// The mock has no expectations, so a request fails the test.
			service.SetHTTPClient(mocks.NewMockHTTPClient(t))
			tt.change(service.Config)

			require.ErrorIs(t, service.Send("Message", nil), tt.wantErr)
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
		assert.Contains(t, err.Error(), "user identifier is invalid")
	})

	t.Run("error status without a JSON body", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushover://:apptoken@usertoken")
		client, _ := newRespondingClient(t, "502 Bad Gateway", http.StatusBadGateway, "<html>Bad Gateway</html>")
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		assert.True(t, strings.HasSuffix(err.Error(), `response status "502 Bad Gateway"`), err.Error())
	})

	t.Run("success status with a rejected request", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushover://:apptoken@usertoken")
		client, _ := newRespondingClient(t, "200 OK", http.StatusOK, invalidUserBody)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		assert.Contains(t, err.Error(), "user identifier is invalid")
	})

	t.Run("success status without a JSON body", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushover://:apptoken@usertoken")
		client, _ := newRespondingClient(t, "200 OK", http.StatusOK, "OK")
		service.SetHTTPClient(client)

		require.NoError(t, service.SendContext(t.Context(), "Message", nil))
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

// TestSendClientField verifies that a send goes through the deprecated
// Client field when the caller set it, and through the default client after
// SetHTTPClient(nil) clears the field.
func TestSendClientField(t *testing.T) {
	t.Parallel()

	t.Run("field set", func(t *testing.T) {
		t.Parallel()

		var captured capturedRequest

		service := newTestService(t, "pushover://:apptoken@usertoken")
		service.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			captured = captureRequest(t, req)

			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(successBody))}, nil
		})}

		require.NoError(t, service.Send("Message", nil))

		assert.Equal(t, hookURL, captured.url)
		assert.Equal(t, "Message", captured.form.Get("message"))
	})

	t.Run("field cleared by a nil client", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "pushover://:apptoken@usertoken")
		service.Client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("the cleared client was used")

			return nil, errTransport
		})}
		service.SetHTTPClient((*http.Client)(nil))

		// The context ends before the send, so the default client returns
		// before it dials.
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, service.SendContext(ctx, "Message", nil), context.Canceled)
		assert.Nil(t, service.Client, "SetHTTPClient clears the deprecated client field")
	})
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client and
// restores the default client for a nil *http.Client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)

		assert.Same(t, client, service.client.Prefer(service.Client))
	})

	t.Run("nil *http.Client", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))
		service.SetHTTPClient((*http.Client)(nil))

		assert.IsType(t, &http.Client{}, service.client.Prefer(service.Client))
	})
}

// TestSendBudget verifies that a send without a caller deadline is bounded by
// the budget that ServiceTimeout reports, and that a caller's deadline is
// kept as it is.
func TestSendBudget(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var deadline time.Time

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			deadline, _ = req.Context().Deadline()

			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(successBody))}, nil
		})

		service := newTestService(t, "pushover://:apptoken@usertoken")
		service.SetHTTPClient(client)

		start := time.Now()

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, start.Add(service.ServiceTimeout(nil)), deadline, "a send without a deadline gets the budget")

		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		defer cancel()

		require.NoError(t, service.SendContext(ctx, "Message", nil))
		assert.Equal(t, start.Add(time.Hour), deadline, "the caller's deadline applies")
	})
}

// TestSendTimestamp verifies that a timestamp from the service URL or the send
// params is sent, with the params taking precedence, that 0 leaves it out, and
// that end-to-end encryption sends it as is, since Pushover does not encrypt it.
func TestSendTimestamp(t *testing.T) {
	t.Parallel()

	const rawURL = "pushover://:apptoken@usertoken"

	tests := map[string]struct {
		rawURL        string
		params        *types.Params
		wantTimestamp string
		wantEncrypted string
	}{
		"not set":            {rawURL: rawURL, wantTimestamp: ""},
		"zero":               {rawURL: rawURL + "?timestamp=0", wantTimestamp: ""},
		"in the URL":         {rawURL: rawURL + "?timestamp=1331249662", wantTimestamp: "1331249662"},
		"param":              {rawURL: rawURL, params: &types.Params{"timestamp": "1700000000"}, wantTimestamp: "1700000000"},
		"param over the URL": {rawURL: rawURL + "?timestamp=1331249662", params: &types.Params{"timestamp": "1700000000"}, wantTimestamp: "1700000000"},
		"with encryption":    {rawURL: rawURL + "?timestamp=1331249662&encryptionkey=" + testEncryptionKey, wantTimestamp: "1331249662", wantEncrypted: "1"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, tc.rawURL)
			client, captured := newRespondingClient(t, "200 OK", http.StatusOK, successBody)
			service.SetHTTPClient(client)

			require.NoError(t, service.Send("Message", tc.params))

			if tc.wantTimestamp == "" {
				assert.NotContains(t, captured.form, "timestamp", "the timestamp should be left out")
			} else {
				assert.Equal(t, tc.wantTimestamp, captured.form.Get("timestamp"))
			}

			assert.Equal(t, tc.wantEncrypted, captured.form.Get("encrypted"))
		})
	}
}

// TestTimestampValidation verifies that a negative timestamp is rejected in the
// service URL and in the send params, the latter before any request, and that
// a timestamp that is not a number fails to parse.
func TestTimestampValidation(t *testing.T) {
	t.Parallel()

	err := (&Service{}).Initialize(parseTestURL(t, "pushover://:apptoken@usertoken?timestamp=-1"), log.New(t.Output(), "", 0))
	require.ErrorIs(t, err, ErrInvalidTimestamp)

	service := newTestService(t, "pushover://:apptoken@usertoken")
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	require.ErrorIs(t, service.Send("Message", &types.Params{"timestamp": "-5"}), ErrInvalidTimestamp)
	require.ErrorIs(t, service.Send("Message", &types.Params{"timestamp": "soon"}), strconv.ErrSyntax)
}
