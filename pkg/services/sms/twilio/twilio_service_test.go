package twilio

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
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
	// username is the basic auth user name.
	username string
	// password is the basic auth password.
	password string
}

// cannedResponse is a response the fake Twilio API returns.
type cannedResponse struct {
	// err fails the request instead of answering it when set.
	err error
	// status is the response status line.
	status string
	// body is the response body.
	body string
	// statusCode is the response status code.
	statusCode int
}

// Values shared by the service tests.
const (
	// testServiceURL is a service URL with two recipients.
	testServiceURL = "twilio://" + testAccountSID + ":authToken@" + testFrom + "/" + testTo + "/+15559876544"
	// messageBody is the body of a created message.
	messageBody = `{"sid":"SMb7c0a2ce80504485a6f653a7110836f5","status":"queued","to":"+15559876543","from":"+15551234567","body":"Message"}`
	// invalidToBody is the body of a message to an invalid number.
	invalidToBody = `{"code":21211,"message":"The 'To' number +15559876543 is not a valid phone number.","more_info":"https://www.twilio.com/docs/errors/21211","status":400}`
)

// errTransport is the transport failure returned by the failing HTTP client.
var errTransport = errors.New("transport failure")

// created is the response to an accepted message.
var created = cannedResponse{status: "201 Created", statusCode: http.StatusCreated, body: messageBody}

// newRespondingClient returns a mock HTTP client that expects one request for
// each response, records the requests, and answers them in order.
//
// Parameters:
//   - t: the test that owns the mock.
//   - responses: the responses, one for each expected request.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *[]capturedRequest: the requests, appended as the client receives them.
func newRespondingClient(t *testing.T, responses ...cannedResponse) (*mocks.MockHTTPClient, *[]capturedRequest) {
	t.Helper()

	captured := &[]capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)

		username, password, _ := req.BasicAuth()
		response := responses[len(*captured)]
		*captured = append(*captured, capturedRequest{
			form:        form,
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			username:    username,
			password:    password,
		})

		if response.err != nil {
			return nil, response.err
		}

		return &http.Response{
			Status:     response.status,
			StatusCode: response.statusCode,
			Body:       io.NopCloser(strings.NewReader(response.body)),
		}, nil
	}).Times(len(responses))

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

	assert.Equal(t, "twilio", (&Service{}).GetID())
}

// TestInitialize verifies that Initialize parses the service URL without
// filling the deprecated client field, keeps a client set before it, and
// rejects an invalid URL.
func TestInitialize(t *testing.T) {
	t.Parallel()

	t.Run("valid URL", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)

		assert.Equal(t, &Config{
			AccountSID: testAccountSID,
			AuthToken:  "authToken",
			FromNumber: testFrom,
			ToNumbers:  []string{testTo, "+15559876544"},
		}, service.Config)
		assert.Nil(t, service.HTTPClient, "Initialize leaves the deprecated client field unset")
	})

	t.Run("keeps a client set before it", func(t *testing.T) {
		t.Parallel()

		client := mocks.NewMockHTTPClient(t)
		service := &Service{}
		service.SetHTTPClient(client)
		require.NoError(t, service.Initialize(parseTestURL(t, testServiceURL), log.New(t.Output(), "", 0)))

		assert.Same(t, client, service.client.Prefer(service.HTTPClient))
	})

	t.Run("missing account SID", func(t *testing.T) {
		t.Parallel()

		err := (&Service{}).Initialize(parseTestURL(t, "twilio://:authToken@"+testFrom+"/"+testTo), log.New(t.Output(), "", 0))

		require.ErrorIs(t, err, ErrAccountSIDMissing)
	})
}

// TestSend verifies that a send makes one authenticated form POST to the
// Messages endpoint for each recipient, in order, and that a title param
// applies to every request.
func TestSend(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	client, captured := newRespondingClient(t, created, created)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", &types.Params{"title": "Alert"}))

	wantForms := []url.Values{
		{"To": {testTo}, "Body": {"Alert\nMessage"}, "From": {testFrom}},
		{"To": {"+15559876544"}, "Body": {"Alert\nMessage"}, "From": {testFrom}},
	}

	require.Len(t, *captured, len(wantForms))

	for i, request := range *captured {
		assert.Equal(t, http.MethodPost, request.method, "request %d", i)
		assert.Equal(t, "https://api.twilio.com/2010-04-01/Accounts/"+testAccountSID+"/Messages.json", request.url, "request %d", i)
		assert.Equal(t, contentType, request.contentType, "request %d", i)
		assert.Equal(t, testAccountSID, request.username, "request %d", i)
		assert.Equal(t, "authToken", request.password, "request %d", i)
		assert.Equal(t, wantForms[i], request.form, "request %d", i)
	}
}

// TestSendChecksConfig verifies that a send with a config built in code
// without recipients fails before any request, as reading a URL without them
// does.
func TestSendChecksConfig(t *testing.T) {
	t.Parallel()

	service := newTestService(t, testServiceURL)
	service.SetHTTPClient(mocks.NewMockHTTPClient(t))

	config := *service.Config
	config.ToNumbers = nil
	service.Config = &config

	require.ErrorIs(t, service.Send("Message", nil), ErrToNumbersMissing)
}

// TestSendParamsLeaveConfigUnchanged verifies that params apply to a copy of
// the config for one send, so the next send has no title.
func TestSendParamsLeaveConfigUnchanged(t *testing.T) {
	t.Parallel()

	service := newTestService(t, "twilio://"+testAccountSID+":authToken@"+testFrom+"/"+testTo)
	want := *service.Config

	client, _ := newRespondingClient(t, created)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", &types.Params{"title": "Updated Title"}))
	assert.Equal(t, want, *service.Config)

	client, captured := newRespondingClient(t, created)
	service.SetHTTPClient(client)

	require.NoError(t, service.Send("Message", nil))
	require.Len(t, *captured, 1)
	assert.Equal(t, []string{"Message"}, (*captured)[0].form["Body"])
}

// TestSendContextErrors verifies that a send reports each failed recipient
// while still sending to the others, wraps a transport error, rejects an
// unknown param before any request, and stops when the caller's context ends.
func TestSendContextErrors(t *testing.T) {
	t.Parallel()

	t.Run("one recipient fails", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client, captured := newRespondingClient(t,
			cannedResponse{status: "400 Bad Request", statusCode: http.StatusBadRequest, body: invalidToBody},
			created)
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, ErrSendFailed)
		assert.Contains(t, err.Error(), "sending to "+testTo)
		assert.Contains(t, err.Error(), "code 21211")
		assert.NotContains(t, err.Error(), "+15559876544")
		assert.Len(t, *captured, 2)
	})

	t.Run("every recipient fails", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		client, _ := newRespondingClient(t,
			cannedResponse{err: errTransport},
			cannedResponse{status: "500 Internal Server Error", statusCode: http.StatusInternalServerError, body: "oops"})
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, errTransport)
		require.ErrorIs(t, err, ErrSendFailed)
		assert.Contains(t, err.Error(), "sending to "+testTo)
		assert.Contains(t, err.Error(), "sending to +15559876544")
	})

	t.Run("transport error without the account SID", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, "twilio://"+testAccountSID+":authToken@"+testFrom+"/"+testTo)
		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: req.URL.String(), Err: errTransport}
		}).Once()
		service.SetHTTPClient(client)

		err := service.SendContext(t.Context(), "Message", nil)

		require.ErrorIs(t, err, errTransport)
		assert.NotContains(t, err.Error(), testAccountSID)
	})

	t.Run("unknown param", func(t *testing.T) {
		t.Parallel()

		service := newTestService(t, testServiceURL)
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		require.ErrorIs(t, service.SendContext(t.Context(), "Message", &types.Params{"foo": "bar"}), format.ErrInvalidConfigKey)
	})

	t.Run("context already canceled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		service := newTestService(t, testServiceURL)
		// The mock has no expectations, so a request fails the test.
		service.SetHTTPClient(mocks.NewMockHTTPClient(t))

		require.ErrorIs(t, service.SendContext(ctx, "message", nil), context.Canceled)
	})

	t.Run("context ends during a request", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			cancel()

			return nil, req.Context().Err()
		}).Once()

		service := newTestService(t, testServiceURL)
		service.SetHTTPClient(client)

		err := service.SendContext(ctx, "message", nil)

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, strings.Count(err.Error(), context.Canceled.Error()))
	})
}

// TestServiceTimeout verifies that the router budget is one request timeout
// for each recipient, and at least one timeout.
func TestServiceTimeout(t *testing.T) {
	t.Parallel()

	// tests lists services with the budget each should get.
	tests := []struct {
		service *Service
		name    string
		want    int
	}{
		{name: "uninitialized service", service: &Service{}, want: 1},
		{name: "two recipients", service: newTestService(t, testServiceURL), want: 2},
		{name: "no recipients", service: &Service{Config: &Config{}}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, defaultHTTPTimeout*time.Duration(tt.want), tt.service.ServiceTimeout(nil))
		})
	}
}

// TestSetHTTPClient verifies that SetHTTPClient stores a custom client in
// place of the deprecated client field, and selects the default client for
// nil and a nil *http.Client.
func TestSetHTTPClient(t *testing.T) {
	t.Parallel()

	// tests lists constructors for the clients to set, which build any mock
	// with the subtest's t, and whether the default client should replace them.
	tests := []struct {
		client      func(t *testing.T) types.HTTPClient
		name        string
		wantDefault bool
	}{
		{name: "custom client", client: func(t *testing.T) types.HTTPClient {
			t.Helper()

			return mocks.NewMockHTTPClient(t)
		}},
		{name: "nil", client: func(*testing.T) types.HTTPClient { return nil }, wantDefault: true},
		{name: "nil *http.Client", client: func(*testing.T) types.HTTPClient { return (*http.Client)(nil) }, wantDefault: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := tt.client(t)
			service := &Service{HTTPClient: mocks.NewMockHTTPClient(t)}
			service.SetHTTPClient(client)

			assert.Nil(t, service.HTTPClient, "SetHTTPClient clears the deprecated client field")

			if tt.wantDefault {
				assert.IsType(t, &http.Client{}, service.client.Prefer(service.HTTPClient))

				return
			}

			assert.Same(t, client, service.client.Prefer(service.HTTPClient))
		})
	}
}

// TestSendBudget verifies that each request of a send may use an equal share
// of the time left, so a recipient that does not answer cannot use up the
// others' time. Without a caller deadline, the time is the budget that
// ServiceTimeout reports for all recipients. A caller's deadline is shared the same
// way, and the last request keeps it as it is.
func TestSendBudget(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var deadlines []time.Time

		client := mocks.NewMockHTTPClient(t)
		client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
			deadline, _ := req.Context().Deadline()
			deadlines = append(deadlines, deadline)

			return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(messageBody))}, nil
		})

		service := newTestService(t, testServiceURL)
		service.SetHTTPClient(client)

		start := time.Now()
		budget := service.ServiceTimeout(nil)

		require.NoError(t, service.Send("Message", nil))
		assert.Equal(t, []time.Time{start.Add(budget / 2), start.Add(budget)}, deadlines,
			"the first recipient gets half of the budget and the last gets the rest")

		deadlines = nil

		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		defer cancel()

		require.NoError(t, service.SendContext(ctx, "Message", nil))
		assert.Equal(t, []time.Time{start.Add(30 * time.Minute), start.Add(time.Hour)}, deadlines,
			"the caller's deadline is shared, and the last request keeps it")
	})
}
