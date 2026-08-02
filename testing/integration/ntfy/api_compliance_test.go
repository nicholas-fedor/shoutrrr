package ntfy_test

import (
	"encoding/base64"
	"net/http"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
)

func TestAPIURLFormat(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"ntfy://ntfy.example.com/mytopic",
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestMade(t, mockClient, "https://ntfy.example.com/mytopic")

		mockClient.AssertExpectations(t)
	})
}

func TestAPIURLFormatWithNestedTopic(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"ntfy://ntfy.example.com/alerts/critical",
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestMade(t, mockClient, "https://ntfy.example.com/alerts/critical")

		mockClient.AssertExpectations(t)
	})
}

func TestHTTPMethodIsPost(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			return req.Method == http.MethodPost
		}, "POST method")

		mockClient.AssertExpectations(t)
	})
}

func TestContentTypeHeader(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestHeaderContains(t, mockClient, "Content-Type", "text/plain; charset=utf-8")

		mockClient.AssertExpectations(t)
	})
}

func TestContentTypeHeaderMarkdown(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL+"?markdown=yes",
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestHeaderContains(t, mockClient, "Content-Type", "text/markdown")

		mockClient.AssertExpectations(t)
	})
}

func TestUserAgentHeader(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			userAgent := req.Header.Get("User-Agent")

			return userAgent == "shoutrrr/"+meta.Version
		}, "User-Agent header")

		mockClient.AssertExpectations(t)
	})
}

func TestBasicAuthHeader(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"ntfy://myuser:mypass@ntfy.example.com/mytopic",
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			authHeader := req.Header.Get("Authorization")
			expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("myuser:mypass"))

			return authHeader == expected
		}, "Basic Auth header")

		mockClient.AssertExpectations(t)
	})
}

func TestBasicAuthHeaderUsernameOnly(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"ntfy://myuser@ntfy.example.com/mytopic",
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			authHeader := req.Header.Get("Authorization")
			expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("myuser:"))

			return authHeader == expected
		}, "Basic Auth header with username only")

		mockClient.AssertExpectations(t)
	})
}

func TestDisableTLSUsesHTTP(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			"ntfy://ntfy.example.com/mytopic?disabletls=yes",
		)

		err := service.Send("test", nil)
		require.NoError(t, err)

		assertRequestMade(t, mockClient, "http://ntfy.example.com/mytopic")

		mockClient.AssertExpectations(t)
	})
}

func TestRequestBodyIsPlainText(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		err := service.Send("plain text body", nil)
		require.NoError(t, err)

		assertRequestMatches(t, mockClient, func(req *http.Request) bool {
			contentType := req.Header.Get("Content-Type")

			return contentType == "text/plain; charset=utf-8"
		}, "plain text Content-Type")

		mockClient.AssertExpectations(t)
	})
}

func TestRequestBodyContainsExactMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, mockClient := createTestServiceWithMock(
			t,
			validNtfyURL,
		)

		message := "Exact message body"
		err := service.Send(message, nil)
		require.NoError(t, err)

		assertRequestContains(t, mockClient, message)

		mockClient.AssertExpectations(t)
	})
}
