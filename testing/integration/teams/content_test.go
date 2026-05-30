package teams_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSendPlainTextMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, ""), nil).
			Once()

		err := service.Send("Test plain text message", nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, `"type":"message"`)
		assertRequestContains(t, mockClient, `"type":"AdaptiveCard"`)
		assertRequestContains(t, mockClient, `"text":"Test plain text message"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmptyMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, ""), nil).
			Once()

		err := service.Send("", nil)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

func TestSendMultiLineMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, ""), nil).
			Once()

		err := service.Send("Line 1\nLine 2\nLine 3", nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, `"text":"Line 1"`)
		assertRequestContains(t, mockClient, `"text":"Line 2"`)
		assertRequestContains(t, mockClient, `"text":"Line 3"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendMessageWithTitleInCard(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase+"&title=MyTitle",
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, ""), nil).
			Once()

		err := service.Send("Body text", nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, `"text":"MyTitle"`)
		assertRequestContains(t, mockClient, `"weight":"Bolder"`)
		assertRequestContains(t, mockClient, `"text":"Body text"`)

		mockClient.AssertExpectations(t)
	})
}

func TestAdaptiveCardStructure(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, ""), nil).
			Once()

		err := service.Send("Test", nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, `"$schema":"http://adaptivecards.io/schemas/adaptive-card.json"`)
		assertRequestContains(t, mockClient, `"version":"1.2"`)
		assertRequestContains(t, mockClient, `"contentType":"application/vnd.microsoft.card.adaptive"`)

		mockClient.AssertExpectations(t)
	})
}
