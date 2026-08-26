package signalgrid_test

import (
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
)

func TestAPIURLFormatCompliance(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validSignalgridURL)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		assertRequestMade(t, mockClient, signalgridAPIURL)

		mockClient.AssertExpectations(t)
	})
}

func TestContentTypeAndMethod(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validSignalgridURL)

		err := service.Send("Test message", nil)
		require.NoError(t, err)

		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Method == http.MethodPost &&
				req.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
		}, "POST form content type")

		mockClient.AssertExpectations(t)
	})
}

func TestUserAgentHeader(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(t, validSignalgridURL)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Header.Get("User-Agent") == "shoutrrr/"+meta.Version
		}, "User-Agent header")

		mockClient.AssertExpectations(t)
	})
}

func TestFormFieldNames(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validSignalgridURL+"?title=Alert&type=WARN&critical=true",
		)

		err := service.Send("payload", nil)
		require.NoError(t, err)

		form := requestForm(t, mockClient)
		require.Equal(t, "clientkey", form.Get("client_key"))
		require.Equal(t, "channeltoken", form.Get("channel"))
		require.Equal(t, "payload", form.Get("body"))
		require.Equal(t, "Alert", form.Get("title"))
		require.Equal(t, "WARN", form.Get("type"))
		require.Equal(t, "true", form.Get("critical"))

		mockClient.AssertExpectations(t)
	})
}
