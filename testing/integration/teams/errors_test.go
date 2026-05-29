package teams_test

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/chat/teams"
)

func TestSendWithHTTPError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(nil, errors.New("connection refused")).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, teams.ErrSendFailed)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWith400BadRequest(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusBadRequest, ""), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, teams.ErrSendFailedStatus)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWith500ServerError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(
			t,
			integrationBase,
			mockClient,
		)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusInternalServerError, ""), nil).
			Once()

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, teams.ErrSendFailedStatus)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithEmptyHost(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service := &teams.Service{}

		err := service.Send("Test message", nil)
		require.Error(t, err)
		require.ErrorIs(t, err, teams.ErrMissingHost)
	})
}

func TestSendWithInvalidWebhookURL(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		serviceURL := "teams://?host=" + url.QueryEscape("https://example.com/invalid")
		service := createTestService(t, serviceURL, mockClient)

		err := service.Send("Test message", nil)
		require.Error(t, err)

		mockClient.AssertNotCalled(t, "Do")
	})
}
