package twilio_test

import (
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

	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// capturedRequest records the parts of a request that the fake Twilio API
// received.
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

// apiReply is how the fake Twilio API answers one request.
type apiReply struct {
	// err fails the request instead of answering it when set.
	err error
	// body is the response body.
	body string
	// statusCode is the response status code.
	statusCode int
}

// Credentials, numbers, and endpoints in the formats Twilio uses.
const (
	// testAccountSID is an account SID.
	testAccountSID = "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	// testMessagingServiceSID is a Messaging Service SID.
	testMessagingServiceSID = "MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	// testFrom is a sender number in E.164 format.
	testFrom = "+15551234567"
	// testTo is a recipient number in E.164 format.
	testTo = "+15559876543"
	// testTo2 is a second recipient number in E.164 format.
	testTo2 = "+15551111111"
	// testServiceURL is a service URL with one recipient.
	testServiceURL = "twilio://" + testAccountSID + ":authToken@" + testFrom + "/" + testTo
	// testMultiURL is a service URL with two recipients.
	testMultiURL = testServiceURL + "/" + testTo2
	// messagesURL is the Messages endpoint of the test account.
	messagesURL = "https://api.twilio.com/2010-04-01/Accounts/" + testAccountSID + "/Messages.json"
	// createdBody is the body of a created message.
	createdBody = `{"sid":"SMb7c0a2ce80504485a6f653a7110836f5","status":"queued"}`
	// invalidToBody is the body of a message to an invalid number.
	invalidToBody = `{"code":21211,"message":"The 'To' number is not a valid phone number.","more_info":"https://www.twilio.com/docs/errors/21211","status":400}`
)

// errConnectionRefused is the transport failure the fake Twilio API returns.
var errConnectionRefused = errors.New("connection refused")

// created is the reply to an accepted message.
var created = apiReply{statusCode: http.StatusCreated, body: createdBody}

// newFakeAPI returns a mock HTTP client that stands in for the Twilio API. It
// expects one request for each reply, records the requests, and answers them
// in order.
//
// Parameters:
//   - t: the test that owns the mock.
//   - replies: the replies, one for each expected request.
//
// Returns:
//   - *mocks.MockHTTPClient: the client.
//   - *[]capturedRequest: the requests, appended as the client receives them.
func newFakeAPI(t *testing.T, replies ...apiReply) (*mocks.MockHTTPClient, *[]capturedRequest) {
	t.Helper()

	captured := &[]capturedRequest{}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).RunAndReturn(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)

		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)

		username, password, _ := req.BasicAuth()
		reply := replies[len(*captured)]
		*captured = append(*captured, capturedRequest{
			form:        form,
			method:      req.Method,
			url:         req.URL.String(),
			contentType: req.Header.Get("Content-Type"),
			username:    username,
			password:    password,
		})

		if reply.err != nil {
			return nil, reply.err
		}

		return &http.Response{
			Status:     strconv.Itoa(reply.statusCode) + " " + http.StatusText(reply.statusCode),
			StatusCode: reply.statusCode,
			Body:       io.NopCloser(strings.NewReader(reply.body)),
		}, nil
	}).Times(len(replies))

	return client, captured
}

// newTestService returns a service initialized from rawURL that logs to the
// test output and sends its requests through client.
//
// Parameters:
//   - t: the test.
//   - rawURL: the service URL.
//   - client: the HTTP client the service uses.
//
// Returns:
//   - *twilio.Service: the service.
func newTestService(t *testing.T, rawURL string, client types.HTTPClient) *twilio.Service {
	t.Helper()

	serviceURL, err := url.Parse(rawURL)
	require.NoError(t, err)

	service := &twilio.Service{}
	require.NoError(t, service.Initialize(serviceURL, log.New(t.Output(), "", 0)))
	service.SetHTTPClient(client)

	return service
}

// TestIntegrationSend verifies that a service initialized from a URL sends one
// authenticated form POST to the account's Messages endpoint for each
// recipient, with normalized numbers, the sender as From or as
// MessagingServiceSid, and the title from the URL or params on its own line.
func TestIntegrationSend(t *testing.T) {
	t.Parallel()

	// tests lists service URLs, messages, and params with the forms the
	// requests should carry, in order.
	tests := []struct {
		params    *types.Params
		name      string
		rawURL    string
		message   string
		wantForms []url.Values
	}{
		{
			name:      "phone number sender",
			rawURL:    testServiceURL,
			message:   "Test message",
			wantForms: []url.Values{{"To": {testTo}, "Body": {"Test message"}, "From": {testFrom}}},
		},
		{
			name:      "Messaging Service sender",
			rawURL:    "twilio://" + testAccountSID + ":authToken@" + testMessagingServiceSID + "/" + testTo,
			message:   "Test message",
			wantForms: []url.Values{{"To": {testTo}, "Body": {"Test message"}, "MessagingServiceSid": {testMessagingServiceSID}}},
		},
		{
			name:    "several recipients",
			rawURL:  testMultiURL,
			message: "Message to multiple recipients",
			wantForms: []url.Values{
				{"To": {testTo}, "Body": {"Message to multiple recipients"}, "From": {testFrom}},
				{"To": {testTo2}, "Body": {"Message to multiple recipients"}, "From": {testFrom}},
			},
		},
		{
			name:    "formatted numbers",
			rawURL:  "twilio://" + testAccountSID + ":authToken@+1.555.123.4567/+1-555-987-6543/+1(555)1111111",
			message: "Test message",
			wantForms: []url.Values{
				{"To": {testTo}, "Body": {"Test message"}, "From": {testFrom}},
				{"To": {testTo2}, "Body": {"Test message"}, "From": {testFrom}},
			},
		},
		{
			name:      "title from the URL",
			rawURL:    testServiceURL + "?title=Alert",
			message:   "Something happened",
			wantForms: []url.Values{{"To": {testTo}, "Body": {"Alert\nSomething happened"}, "From": {testFrom}}},
		},
		{
			name:      "title from params",
			rawURL:    testServiceURL,
			message:   "Message body",
			params:    &types.Params{"title": "Dynamic Title"},
			wantForms: []url.Values{{"To": {testTo}, "Body": {"Dynamic Title\nMessage body"}, "From": {testFrom}}},
		},
		{
			name:      "unicode message",
			rawURL:    testServiceURL,
			message:   "Привет 🚨 ✓",
			wantForms: []url.Values{{"To": {testTo}, "Body": {"Привет 🚨 ✓"}, "From": {testFrom}}},
		},
		{
			name:      "empty message",
			rawURL:    testServiceURL,
			wantForms: []url.Values{{"To": {testTo}, "Body": {""}, "From": {testFrom}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			replies := make([]apiReply, len(tt.wantForms))
			for i := range replies {
				replies[i] = created
			}

			client, captured := newFakeAPI(t, replies...)
			service := newTestService(t, tt.rawURL, client)

			require.NoError(t, service.SendContext(t.Context(), tt.message, tt.params))

			require.Len(t, *captured, len(tt.wantForms))

			for i, request := range *captured {
				assert.Equal(t, http.MethodPost, request.method, "request %d", i)
				assert.Equal(t, messagesURL, request.url, "request %d", i)
				assert.Equal(t, "application/x-www-form-urlencoded", request.contentType, "request %d", i)
				assert.Equal(t, testAccountSID, request.username, "request %d", i)
				assert.Equal(t, "authToken", request.password, "request %d", i)
				assert.Equal(t, tt.wantForms[i], request.form, "request %d", i)
			}
		})
	}
}

// TestIntegrationSendErrors verifies the errors a caller sees when the Twilio
// API rejects a message or cannot be reached: the Twilio error message and
// code when the response carries them, the status otherwise, the transport
// error, and the failed recipient while the others still receive the message.
func TestIntegrationSendErrors(t *testing.T) {
	t.Parallel()

	// tests lists service URLs and API replies with the error the send should
	// return and the text it should and should not contain.
	tests := []struct {
		wantErr     error
		name        string
		rawURL      string
		replies     []apiReply
		wantText    []string
		notWantText []string
	}{
		{
			name:     "invalid recipient",
			rawURL:   testServiceURL,
			replies:  []apiReply{{statusCode: http.StatusBadRequest, body: invalidToBody}},
			wantErr:  twilio.ErrSendFailed,
			wantText: []string{"sending to " + testTo, "The 'To' number is not a valid phone number.", "code 21211"},
		},
		{
			name:     "unauthorized",
			rawURL:   testServiceURL,
			replies:  []apiReply{{statusCode: http.StatusUnauthorized, body: `{"code":20003,"message":"Authenticate","status":401}`}},
			wantErr:  twilio.ErrSendFailed,
			wantText: []string{"Authenticate (code 20003)"},
		},
		{
			name:     "forbidden",
			rawURL:   testServiceURL,
			replies:  []apiReply{{statusCode: http.StatusForbidden, body: `{"code":20003,"message":"Forbidden","status":403}`}},
			wantErr:  twilio.ErrSendFailed,
			wantText: []string{"Forbidden (code 20003)"},
		},
		{
			name:     "not found",
			rawURL:   testServiceURL,
			replies:  []apiReply{{statusCode: http.StatusNotFound, body: `{"code":20404,"message":"Not Found","status":404}`}},
			wantErr:  twilio.ErrSendFailed,
			wantText: []string{"Not Found (code 20404)"},
		},
		{
			name:     "server error without a body",
			rawURL:   testServiceURL,
			replies:  []apiReply{{statusCode: http.StatusInternalServerError}},
			wantErr:  twilio.ErrSendFailed,
			wantText: []string{`"500 Internal Server Error"`},
		},
		{
			name:     "error body that is not JSON",
			rawURL:   testServiceURL,
			replies:  []apiReply{{statusCode: http.StatusBadRequest, body: "not valid json"}},
			wantErr:  twilio.ErrSendFailed,
			wantText: []string{`"400 Bad Request"`},
		},
		{
			name:     "network error",
			rawURL:   testServiceURL,
			replies:  []apiReply{{err: errConnectionRefused}},
			wantErr:  errConnectionRefused,
			wantText: []string{"sending to " + testTo, "connection refused"},
		},
		{
			name:        "second recipient fails",
			rawURL:      testMultiURL,
			replies:     []apiReply{created, {err: errConnectionRefused}},
			wantErr:     errConnectionRefused,
			wantText:    []string{"sending to " + testTo2},
			notWantText: []string{"sending to " + testTo + ":"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, captured := newFakeAPI(t, tt.replies...)
			service := newTestService(t, tt.rawURL, client)

			err := service.SendContext(t.Context(), "Test message", nil)

			require.ErrorIs(t, err, tt.wantErr)

			for _, want := range tt.wantText {
				assert.Contains(t, err.Error(), want)
			}

			for _, notWant := range tt.notWantText {
				assert.NotContains(t, err.Error(), notWant)
			}

			assert.Len(t, *captured, len(tt.replies))
		})
	}
}
