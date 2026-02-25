package twilio_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/sms/twilio"
)

const (
	validTwilioURL = "twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543"
	twilioAPIURL   = "https://api.twilio.com/2010-04-01/Accounts/ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/Messages.json"
)

func TestAPIURLFormatCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Verify the request was made to the correct Twilio API endpoint
		assertRequestMade(t, mockClient, twilioAPIURL)

		mockClient.AssertExpectations(t)
	})
}

func TestValidURLFormats(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		shouldError bool
	}{
		{"valid standard URL", validTwilioURL, false},
		{"valid with messaging service SID", "twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/+15559876543", false},
		{"valid with multiple recipients", "twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543/+15551111111", false},
		{"missing account SID", "twilio://:authToken@+15551234567/+15559876543", true},
		{"missing auth token", "twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:@+15551234567/+15559876543", true},
		{"missing from number", "twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@/+15559876543", true},
		{"missing to number", "twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/", true},
		{"same to and from", "twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15551234567", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				parsedURL, err := url.Parse(tt.url)
				if err != nil {
					if tt.shouldError {
						return
					}
					t.Fatalf("Unexpected URL parse error: %v", err)
				}

				service := &twilio.Service{}
				initErr := service.Initialize(parsedURL, &mockLogger{})

				if tt.shouldError {
					require.Error(t, initErr)
				} else {
					require.NoError(t, initErr)
				}
			})
		})
	}
}

func TestPayloadStructureCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Verify form-encoded body contains expected fields
		assertRequestContains(t, mockClient, "To=%2B15559876543")
		assertRequestContains(t, mockClient, "Body=Test+message")
		assertRequestContains(t, mockClient, "From=%2B15551234567")

		mockClient.AssertExpectations(t)
	})
}

func TestBasicAuthCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Verify Basic Auth is set correctly
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			username, password, ok := req.BasicAuth()
			return ok && username == "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX" && password == "authToken"
		}, "Basic Auth credentials")

		mockClient.AssertExpectations(t)
	})
}

func TestContentTypeCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Verify Content-Type header is set to form-urlencoded
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
		}, "Content-Type header")

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPMethodCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Verify POST method is used
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Method == http.MethodPost
		}, "HTTP POST method")

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPSRequirementCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Verify HTTPS scheme is used for API calls
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.URL.Scheme == "https"
		}, "HTTPS scheme")

		mockClient.AssertExpectations(t)
	})
}

func TestMessagingServiceSIDCompliance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/+15559876543",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		// Verify MessagingServiceSid is used instead of From
		assertRequestContains(t, mockClient, "MessagingServiceSid=MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")

		// Should NOT contain a From field
		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				return false
			}
			req.Body = io.NopCloser(bytes.NewReader(body))

			return !bytes.Contains(body, []byte("From="))
		}, "no From field when using MessagingServiceSid")

		mockClient.AssertExpectations(t)
	})
}
