package pushbullet_test

import (
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// roundTripFunc adapts a function to an [http.RoundTripper].
type roundTripFunc func(req *http.Request) (*http.Response, error)

// testToken is an access token of the length Pushbullet issues.
const testToken = "tokentokentokentokentokentokentoke"

// RoundTrip calls f with the request.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response from f.
//   - error: the error from f.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// TestIntegrationRouterInjectedClientAuthenticates verifies that a push sent
// through a client the router injects carries the access token. The fake API
// answers 401 without it, like the Pushbullet API.
func TestIntegrationRouterInjectedClientAuthenticates(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Access-Token") != testToken {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Access token is missing or invalid.","type":"invalid_request"}}`)),
			}, nil
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"type":"note","body":"Message","title":"Shoutrrr notification","active":true}`)),
		}, nil
	})

	serviceRouter, err := router.NewWithOptions(
		log.New(t.Output(), "", 0),
		types.SenderOptions{HTTPClient: &http.Client{Transport: transport}},
	)
	require.NoError(t, err)

	service, err := serviceRouter.Locate("pushbullet://" + testToken + "/device")
	require.NoError(t, err)

	assert.NoError(t, service.Send("Message", nil))
}
