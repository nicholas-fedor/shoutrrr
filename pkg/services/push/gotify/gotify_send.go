package gotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// sendRequest posts the message to the Gotify API and checks the response.
// With header authentication, the token travels in the X-Gotify-Key header.
// Otherwise postURL carries it in its query.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - client: the HTTP client to send through.
//   - config: the configuration for this send.
//   - postURL: the complete API endpoint URL to send the request to.
//   - request: the JSON payload to send in the request body.
//
// Returns:
//   - error: [ErrSendFailed] wrapping the Gotify error in the body of an
//     error status, or the request error. The request error is a
//     [jsonclient.Error] for a status outside 200 to 299 or a reply that is
//     not JSON, and matches ctx's error when ctx ends the request.
func (s *Service) sendRequest(
	ctx context.Context,
	client types.HTTPClient,
	config *Config,
	postURL string,
	request *MessageRequest,
) error {
	jsonClient := jsonclient.NewContextClient(client)
	if config.UseHeader {
		jsonClient.Headers().Set("X-Gotify-Key", config.Token)
	}

	// The reply is the created message. It is decoded only to reject a reply
	// that is not JSON, such as a page from an intermediate server.
	err := jsonClient.PostContext(ctx, postURL, request, &struct{}{})
	if err == nil {
		return nil
	}

	return fmt.Errorf("%w: %w", ErrSendFailed, serverError(jsonClient, err))
}

// serverError returns the Gotify error in the body of an error status,
// labeled as a client or server error by status.
//
// Parameters:
//   - client: the API client that made the request.
//   - err: the request error.
//
// Returns:
//   - error: the labeled Gotify error, or err when its body holds none.
func serverError(client jsonclient.Client, err error) error {
	errorRes := &responseError{}
	if !client.ErrorResponse(err, errorRes) || errorRes.Name == "" {
		return err
	}

	// ErrorResponse matched, so err is a jsonclient.Error.
	jsonErr, _ := errors.AsType[jsonclient.Error](err)
	if jsonErr.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("server error (%d): %w", jsonErr.StatusCode, errorRes)
	}

	return fmt.Errorf("client error (%d): %w", jsonErr.StatusCode, errorRes)
}
