package twilio_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendBasicMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Hello from Twilio integration test", nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, "Body=Hello+from+Twilio+integration+test")

		mockClient.AssertExpectations(t)
	})
}

func TestSendToMultipleRecipients(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			"twilio://ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:authToken@+15551234567/+15559876543/+15551111111",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Times(2)

		err := service.Send("Message to multiple recipients", nil)
		require.NoError(t, err)

		mockClient.AssertNumberOfCalls(t, "Do", 2)
		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			validTwilioURL+"?title=Alert",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Something happened", nil)
		require.NoError(t, err)

		// Title should be prepended to the body
		assertRequestContains(t, mockClient, "Body=Alert")
		assertRequestContains(t, mockClient, "Something")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitleParam(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		params := types.Params{"title": "Dynamic Title"}
		err := service.Send("Message body", &params)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, "Body=Dynamic+Title")
		assertRequestContains(t, mockClient, "Message")

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithUnicodeContent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("Hello World", nil)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmptyMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusCreated, `{"sid": "SM123"}`), nil).
			Once()

		err := service.Send("", nil)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestServiceInitialization(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validTwilioURL, mockClient)

		require.NotNil(t, service)
		require.Equal(t, "twilio", service.GetID())

		mockClient.AssertExpectations(t)
	})
}
