package teams_test

import (
	"net/http"
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

var integrationBase = "teams://?host=" + url.QueryEscape(workflowURL)

func TestServiceInitialization(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase,
			mockClient,
		)

		require.NotNil(t, service)
		require.Equal(t, "teams", service.GetID())

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitle(t *testing.T) {
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

		params := &types.Params{"title": "Custom Title"}
		err := service.Send("Message with custom title", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"weight":"Bolder"`)
		assertRequestContains(t, mockClient, `"text":"Custom Title"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithColor(t *testing.T) {
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

		params := &types.Params{"color": "ff0000"}
		err := service.Send("Message with color", params)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"accentColor":"ff0000"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmptyParameters(t *testing.T) {
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

		emptyParams := &types.Params{}
		err := service.Send("Message with empty params", emptyParams)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"text":"Message with empty params"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithNilParameters(t *testing.T) {
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

		err := service.Send("Message with nil params", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"text":"Message with nil params"`)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithURLParameters(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, ""), nil).
			Once()

		serviceURL := integrationBase + "&title=URLTitle&color=00ff00"
		service := createTestService(t, serviceURL, mockClient)

		err := service.Send("Message with URL params", nil)

		require.NoError(t, err)
		assertRequestContains(t, mockClient, `"text":"URLTitle"`)
		assertRequestContains(t, mockClient, `"weight":"Bolder"`)

		mockClient.AssertExpectations(t)
	})
}
