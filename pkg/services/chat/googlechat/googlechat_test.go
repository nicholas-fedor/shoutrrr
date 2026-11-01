package googlechat

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/types/mocks"
)

// roundTripFunc adapts a function to [http.RoundTripper].
type roundTripFunc func(*http.Request) (*http.Response, error)

// capturedRequest records the parts of a request that the send tests check.
type capturedRequest struct {
	// method is the request method.
	method string
	// url is the full request URL.
	url string
	// contentType is the request's Content-Type header.
	contentType string
	// body is the request body.
	body string
}

// testServiceURL is a valid Google Chat service URL.
const testServiceURL = "googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz"

// testAPIURL is the webhook URL that the service builds from testServiceURL.
const testAPIURL = "https://chat.googleapis.com/v1/spaces/FOO/messages?key=bar&token=baz"

// errNetwork is the transport error returned by the failing round tripper.
var errNetwork = errors.New("network failure")

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the outgoing request.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// newResponse returns a response with the given status code and an empty body.
//
// Parameters:
//   - status: the HTTP status code.
//
// Returns:
//   - *http.Response: the response.
func newResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}
}

// newTestService returns a service initialized from testServiceURL that sends
// through client and logs to the test output.
//
// Parameters:
//   - t: the test.
//   - client: the HTTP client the service sends through.
//
// Returns:
//   - *Service: the service.
func newTestService(t *testing.T, client types.HTTPClient) *Service {
	t.Helper()

	serviceURL, err := url.Parse(testServiceURL)
	require.NoError(t, err)

	service := &Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetHTTPClient(client)

	return service
}

// newCapturingClient returns a mock HTTP client that expects one request,
// records it, and answers with 200 OK.
//
// Parameters:
//   - t: the test that owns the mock.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *capturedRequest: the recorded request, filled in when the client is called.
func newCapturingClient(t *testing.T) (*mocks.MockHTTPClient, *capturedRequest) {
	t.Helper()

	captured := &capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		captured.method = req.Method
		captured.url = req.URL.String()
		captured.contentType = req.Header.Get("Content-Type")
		captured.body = string(body)

		return newResponse(http.StatusOK), nil
	}).Once()

	return client, captured
}

// TestServiceGetID verifies that the service reports the googlechat scheme as
// its identifier.
func TestServiceGetID(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "googlechat", (&Service{}).GetID())
}

// TestServiceInitialize verifies that Initialize parses the config, installs a
// default HTTP client when none is set, and keeps a client injected beforehand.
func TestServiceInitialize(t *testing.T) {
	t.Parallel()

	serviceURL, err := url.Parse(testServiceURL)
	require.NoError(t, err)

	t.Run("default client", func(t *testing.T) {
		t.Parallel()

		service := &Service{}
		require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

		assert.Equal(t, &Config{Host: "chat.googleapis.com", Path: "/v1/spaces/FOO/messages", Key: "bar", Token: "baz"}, service.Config)
		assert.Equal(t, &http.Client{}, service.client.Client(), "the default client has no request timeout")
	})

	t.Run("injected client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)
		require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.client.Client())
	})

	t.Run("invalid config", func(t *testing.T) {
		t.Parallel()

		badURL, err := url.Parse("googlechat://chat.googleapis.com/v1/spaces/FOO/messages?key=&token=baz")
		require.NoError(t, err)

		require.ErrorIs(t, (&Service{}).Initialize(badURL, log.New(t.Output(), "", 0)), ErrMissingKey)
	})
}

// TestServiceSendRequest verifies that Send posts the message as a JSON payload
// to the webhook URL built from the config.
func TestServiceSendRequest(t *testing.T) {
	t.Parallel()

	client, captured := newCapturingClient(t)

	require.NoError(t, newTestService(t, client).Send("Test Message", nil))

	assert.Equal(t, http.MethodPost, captured.method)
	assert.Equal(t, testAPIURL, captured.url)
	assert.Equal(t, "application/json", captured.contentType)
	assert.JSONEq(t, `{"text":"Test Message"}`, captured.body)
}

// TestServiceSendChecksConfig verifies that a send checks a config built in
// code as reading a URL does. A config without a key, token, or webhook path
// fails before any request. A config without a host posts to [DefaultHost].
func TestServiceSendChecksConfig(t *testing.T) {
	t.Parallel()

	// tests lists config changes with the error the send must return.
	tests := []struct {
		change  func(config *Config)
		wantErr error
		name    string
	}{
		{name: "zero config", change: func(config *Config) { *config = Config{} }, wantErr: ErrMissingKey},
		{name: "no token", change: func(config *Config) { config.Token = "" }, wantErr: ErrMissingToken},
		{name: "no path", change: func(config *Config) { config.Path = "" }, wantErr: ErrMissingPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// The mock has no expectations, so a request fails the test.
			service := newTestService(t, mocks.NewMockHTTPClient(t))
			tt.change(service.Config)

			require.ErrorIs(t, service.Send("Message", nil), tt.wantErr)
		})
	}

	t.Run("no host", func(t *testing.T) {
		t.Parallel()

		client, captured := newCapturingClient(t)
		service := newTestService(t, client)
		service.Config.Host = ""

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, testAPIURL, captured.url)
	})
}

// TestServiceSendParams verifies that the reserved title, message, and level
// params send as usual. Any other param fails before a request, since Google
// Chat has no options a send can set.
func TestServiceSendParams(t *testing.T) {
	t.Parallel()

	t.Run("reserved params", func(t *testing.T) {
		t.Parallel()

		client, captured := newCapturingClient(t)

		params := types.Params{}
		params.SetTitle("Title")
		params.SetMessage("Ignored")
		params.SetLevel(types.Warning)

		require.NoError(t, newTestService(t, client).Send("Test Message", &params))
		assert.JSONEq(t, `{"text":"Test Message"}`, captured.body)
	})

	t.Run("unknown param", func(t *testing.T) {
		t.Parallel()

		// The mock has no expectations, so a request fails the test.
		service := newTestService(t, mocks.NewMockHTTPClient(t))

		err := service.Send("Message", &types.Params{"thread": "spaces/AAA/threads/BBB"})
		require.ErrorIs(t, err, format.ErrInvalidConfigKey)
	})
}

// TestServiceSendStatus verifies that a 2xx response is a success and any other
// status yields ErrUnexpectedStatus with the status code.
func TestServiceSendStatus(t *testing.T) {
	t.Parallel()

	// tests lists response statuses and whether Send accepts them.
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "200 OK", status: http.StatusOK},
		{name: "204 No Content", status: http.StatusNoContent},
		{name: "299", status: 299},
		{name: "199", status: 199, wantErr: true},
		{name: "300 Multiple Choices", status: http.StatusMultipleChoices, wantErr: true},
		{name: "400 Bad Request", status: http.StatusBadRequest, wantErr: true},
		{name: "500 Internal Server Error", status: http.StatusInternalServerError, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := mocks.NewMockHTTPClient(t)
			client.EXPECT().Do(mock.Anything).Return(newResponse(tt.status), nil).Once()

			err := newTestService(t, client).Send("Message", nil)

			if !tt.wantErr {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, ErrUnexpectedStatus)
			assert.ErrorContains(t, err, ": "+strconv.Itoa(tt.status))
		})
	}
}

// TestServiceSendTransportError verifies that a transport failure is wrapped,
// stays matchable, and has the webhook credentials redacted from its message.
func TestServiceSendTransportError(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errNetwork
	})}

	err := newTestService(t, client).Send("Message", nil)

	require.ErrorIs(t, err, errNetwork)
	assert.EqualError(t, err, `sending notification to Google Chat: Post "https://chat.googleapis.com": network failure`)
}

// TestServiceSendContextCanceled verifies that SendContext stops the request
// when the caller's context is canceled.
func TestServiceSendContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		return nil, req.Context().Err()
	}).Once()

	require.ErrorIs(t, newTestService(t, client).SendContext(ctx, "message", nil), context.Canceled)
}

// TestServiceSendInvalidURL verifies that SendContext reports a request URL
// that cannot be built and does not call the HTTP client.
func TestServiceSendInvalidURL(t *testing.T) {
	t.Parallel()

	service := &Service{Config: &Config{Host: "bad host", Path: "/v1/spaces/FOO/messages", Key: "bar", Token: "baz"}}
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.SendContext(t.Context(), "Message", nil)

	urlErr, ok := errors.AsType[*url.Error](err)
	require.True(t, ok, "error %v is not a *url.Error", err)
	assert.Equal(t, "parse", urlErr.Op)
	assert.ErrorContains(t, err, "creating HTTP request")
}

// TestSetHTTPClient verifies that SetHTTPClient installs a custom client and
// restores the default client for a nil or typed-nil client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	t.Run("custom client", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)

		assert.Same(t, client, service.client.Client())
	})

	// nilClients lists clients that SetHTTPClient replaces with the default.
	nilClients := []struct {
		name   string
		client types.HTTPClient
	}{
		{name: "nil interface", client: nil},
		{name: "nil *http.Client", client: (*http.Client)(nil)},
	}

	for _, tt := range nilClients {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := newTestService(t, mocks.NewMockHTTPClient(t))
			service.SetHTTPClient(tt.client)

			assert.Equal(t, &http.Client{}, service.client.Client(), "the default client has no request timeout")
		})
	}
}

// TestServiceSendRedactsUnparsableURL verifies that a host that makes the
// webhook URL unparsable fails the send without the key or token in the error.
func TestServiceSendRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	service := &Service{Config: &Config{Host: "bad host", Path: "/v1/spaces/FOO/messages", Key: "SECRETkey", Token: "SECRETtoken"}}
	// The mock has no expectations, so a request fails the test.
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	err := service.Send("message", nil)

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "SECRETkey")
	assert.NotContains(t, err.Error(), "SECRETtoken")
}
