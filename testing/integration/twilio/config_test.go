package twilio_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConfigURLRoundTrip(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		configURL := service.Config.GetURL()
		assert.Equal(t, "twilio", configURL.Scheme)
		assert.Equal(t, "ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", configURL.User.Username())

		password, hasPassword := configURL.User.Password()
		assert.True(t, hasPassword)
		assert.Equal(t, "authToken", password)
		assert.Equal(t, "+15551234567", configURL.Host)

		mockClient.AssertExpectations(t)
	})
}

func TestConfigMultipleRecipients(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543/+15551111111",
			mockClient,
		)

		assert.Equal(t, []string{"+15559876543", "+15551111111"}, service.Config.ToNumbers)

		configURL := service.Config.GetURL()
		assert.Equal(t, "/+15559876543/+15551111111", configURL.Path)

		mockClient.AssertExpectations(t)
	})
}

func TestConfigPhoneNumberNormalization(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedFrom string
		expectedTo   []string
	}{
		{
			"strips dashes",
			"twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+1-555-123-4567/+1-555-987-6543",
			"+15551234567",
			[]string{"+15559876543"},
		},
		{
			"strips parentheses",
			"twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+1(555)1234567/+1(555)9876543",
			"+15551234567",
			[]string{"+15559876543"},
		},
		{
			"strips dots",
			"twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+1.555.123.4567/+1.555.987.6543",
			"+15551234567",
			[]string{"+15559876543"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				mockClient := &MockHTTPClient{}
				service := createTestService(t, tt.url, mockClient)

				assert.Equal(t, tt.expectedFrom, service.Config.FromNumber)
				assert.Equal(t, tt.expectedTo, service.Config.ToNumbers)

				mockClient.AssertExpectations(t)
			})
		})
	}
}

func TestConfigMessagingServiceSID(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/+15559876543",
			mockClient,
		)

		assert.Equal(t, "MGXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX", service.Config.FromNumber)

		mockClient.AssertExpectations(t)
	})
}

func TestConfigTitleParameter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validTwilioURL+"?title=MyTitle",
			mockClient,
		)

		assert.Equal(t, "MyTitle", service.Config.Title)

		// Verify title appears in sent message
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Test body", nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, "Body=MyTitle")

		mockClient.AssertExpectations(t)
	})
}

func TestConfigEmptyTitle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		assert.Equal(t, "", service.Config.Title)

		mockClient.AssertExpectations(t)
	})
}
