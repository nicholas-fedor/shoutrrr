package signalgrid_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

func TestSendBasicMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validSignalgridURL)

		err := service.Send("Hello from Signalgrid", nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, "body=Hello+from+Signalgrid")
		assertRequestMade(t, mockClient, signalgridAPIURL)

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitle(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validSignalgridURL+"?title=Alert",
		)

		err := service.Send("Something happened", nil)
		require.NoError(t, err)

		form := requestForm(t, mockClient)
		require.Equal(t, "Alert", form.Get("title"))
		require.Equal(t, "Something happened", form.Get("body"))

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTitleParam(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validSignalgridURL)

		params := types.Params{"title": "Override"}
		err := service.Send("updated", &params)
		require.NoError(t, err)

		require.Equal(t, "Override", requestForm(t, mockClient).Get("title"))

		mockClient.AssertExpectations(t)
	})
}

func TestSendOmitsEmptyTitleAndCritical(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validSignalgridURL)

		err := service.Send("body only", nil)
		require.NoError(t, err)

		form := requestForm(t, mockClient)
		require.Empty(t, form.Get("title"))
		require.False(t, form.Has("title"))
		require.False(t, form.Has("critical"))
		require.Equal(t, "INFO", form.Get("type"))

		mockClient.AssertExpectations(t)
	})
}

func TestSendWithTypeAndCritical(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validSignalgridURL+"?type=CRIT&critical=true",
		)

		err := service.Send("outage", nil)
		require.NoError(t, err)

		form := requestForm(t, mockClient)
		require.Equal(t, "CRIT", form.Get("type"))
		require.Equal(t, "true", form.Get("critical"))

		mockClient.AssertExpectations(t)
	})
}

func TestSendEmptyMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mockClient := &MockHTTPClient{}
		service := createTestService(t, validSignalgridURL, mockClient)

		mockClient.On("Do", mock.Anything).
			Return(createMockResponse(http.StatusOK, `{"code":"200","text":"OK"}`), nil).
			Once()

		err := service.Send("", nil)
		require.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}
