package twilio

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio/mocks"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// failingReader is a response body whose reads fail.
type failingReader struct{}

// trackedBody is a response body that records whether it was read to its end
// before it was closed.
type trackedBody struct {
	// Reader supplies the body.
	*strings.Reader

	// drainedAtClose is true when Close found nothing left to read.
	drainedAtClose bool
}

// errRead is the read failure returned by failingReader.
var errRead = errors.New("read failure")

// Read fails with errRead.
//
// Parameters:
//   - p: the buffer, which stays unchanged.
//
// Returns:
//   - int: zero.
//   - error: errRead.
func (failingReader) Read([]byte) (int, error) {
	return 0, errRead
}

// Close records whether the body was read to its end first.
//
// Returns:
//   - error: always nil.
func (b *trackedBody) Close() error {
	b.drainedAtClose = b.Len() == 0

	return nil
}

// TestSendToRecipientReadsReply verifies that a reply that no Debug record
// reads is still read to its end before it is closed, so the next
// recipient's request can reuse the connection.
func TestSendToRecipientReadsReply(t *testing.T) {
	t.Parallel()

	body := &trackedBody{Reader: strings.NewReader(messageBody), drainedAtClose: false}
	client := mocks.NewMockHTTPClient(t)
	client.EXPECT().Do(mock.Anything).Return(&http.Response{StatusCode: http.StatusCreated, Body: body}, nil).Once()

	service := &Service{HTTPClient: client}
	service.SetLogger(types.NewSlogLogger(slog.New(slog.DiscardHandler)))

	config := &Config{AccountSID: testAccountSID, AuthToken: "authToken", FromNumber: testFrom}
	require.NoError(t, service.sendToRecipient(t.Context(), config, testTo, encodeMessageForm(config, "Test")))
	assert.True(t, body.drainedAtClose)
}

// TestDefaultHTTPClient verifies that the default client has the request
// timeout.
func TestDefaultHTTPClient(t *testing.T) {
	t.Parallel()

	client, ok := DefaultHTTPClient().(*http.Client)

	require.True(t, ok)
	assert.Equal(t, defaultHTTPTimeout, client.Timeout)
}

// TestSendToRecipient verifies the request for one recipient. It is an
// authenticated form POST to the account's Messages endpoint with the
// recipient and the body. The sender goes as MessagingServiceSid for a
// Messaging Service SID and as From for any other sender.
func TestSendToRecipient(t *testing.T) {
	t.Parallel()

	// tests lists configs with the form the request should carry.
	tests := []struct {
		name     string
		wantForm url.Values
		config   Config
	}{
		{
			name:     "phone number sender",
			config:   Config{AccountSID: testAccountSID, AuthToken: "authToken", FromNumber: testFrom},
			wantForm: url.Values{"To": {testTo}, "Body": {"Test"}, "From": {testFrom}},
		},
		{
			name:     "Messaging Service sender",
			config:   Config{AccountSID: testAccountSID, AuthToken: "authToken", FromNumber: testMessagingServiceSID},
			wantForm: url.Values{"To": {testTo}, "Body": {"Test"}, "MessagingServiceSid": {testMessagingServiceSID}},
		},
		{
			name:     "MG that is not a Messaging Service SID",
			config:   Config{AccountSID: testAccountSID, AuthToken: "authToken", FromNumber: "MGMHotels"},
			wantForm: url.Values{"To": {testTo}, "Body": {"Test"}, "From": {"MGMHotels"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, captured := newRespondingClient(t, cannedResponse{statusCode: http.StatusCreated, body: messageBody})
			service := &Service{HTTPClient: client}

			require.NoError(t, service.sendToRecipient(t.Context(), &tt.config, testTo, encodeMessageForm(&tt.config, "Test")))

			require.Len(t, *captured, 1)
			request := (*captured)[0]
			assert.Equal(t, http.MethodPost, request.method)
			assert.Equal(t, "https://api.twilio.com/2010-04-01/Accounts/"+testAccountSID+"/Messages.json", request.url)
			assert.Equal(t, contentType, request.contentType)
			assert.Equal(t, testAccountSID, request.username)
			assert.Equal(t, "authToken", request.password)
			assert.Equal(t, tt.wantForm, request.form)
		})
	}
}

// TestSendToRecipientRedactsUnparsableURL verifies that an account SID that
// makes the API URL unparsable fails before any request, without the SID in
// the error.
func TestSendToRecipientRedactsUnparsableURL(t *testing.T) {
	t.Parallel()

	// The mock has no expectations, so a request fails the test.
	service := &Service{HTTPClient: mocks.NewMockHTTPClient(t)}
	config := &Config{AccountSID: "ACSECRETsid%zz", AuthToken: "token", FromNumber: testFrom, ToNumbers: []string{testTo}}

	err := service.sendToRecipient(t.Context(), config, testTo, "message")

	_, isURLErr := errors.AsType[*url.Error](err)
	require.True(t, isURLErr, "error %v is not a *url.Error", err)
	assert.NotContains(t, err.Error(), "ACSECRETsid")
}

// TestParseAPIError verifies that an error response reports the Twilio error
// message and code from a JSON body, and the status otherwise.
func TestParseAPIError(t *testing.T) {
	t.Parallel()

	// tests lists error responses with the text the error should report.
	tests := []struct {
		body   io.Reader
		name   string
		status string
		want   []string
	}{
		{
			name:   "Twilio error",
			status: "400 Bad Request",
			body:   strings.NewReader(invalidToBody),
			want:   []string{"The 'To' number +15559876543 is not a valid phone number.", "code 21211"},
		},
		{name: "JSON without a message", status: "400 Bad Request", body: strings.NewReader(`{"code":0}`), want: []string{`"400 Bad Request"`}},
		{name: "not JSON", status: "500 Internal Server Error", body: strings.NewReader("Internal Server Error"), want: []string{`"500 Internal Server Error"`}},
		{name: "empty body", status: "400 Bad Request", body: strings.NewReader(""), want: []string{`"400 Bad Request"`}},
		{name: "unreadable body", status: "502 Bad Gateway", body: failingReader{}, want: []string{`"502 Bad Gateway"`, "failed to read body"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := parseAPIError(&http.Response{Status: tt.status, Body: io.NopCloser(tt.body)})

			require.ErrorIs(t, err, ErrSendFailed)

			for _, want := range tt.want {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}
