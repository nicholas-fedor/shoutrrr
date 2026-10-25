package telegram

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Client for Telegram API.
type Client struct {
	// httpClient sends the requests, or is nil to use [fallbackClient].
	httpClient types.HTTPClient
	// token is the bot token, which is part of every request URL.
	token string
	// apiURL is the base URL of the Bot API server, or empty for
	// https://api.telegram.org.
	apiURL string
}

// fallbackClient sends the requests of every [Client] without an HTTP client
// of its own, so they share connections. Its timeout bounds the requests that
// take no context.
var fallbackClient = &http.Client{Timeout: defaultHTTPTimeout}

// GetBotInfo returns the bot User info.
//
// Returns:
//   - *User: the bot's user, from getMe.
//   - error: the Telegram API error or the request error.
func (c *Client) GetBotInfo() (*User, error) {
	response := &userResponse{}
	jc := jsonclient.NewWithHTTPClient(c.httpClientOrDefault())

	if err := jc.Get(c.endpointURL("getMe"), response); err != nil || !response.OK {
		return nil, fmt.Errorf("getting bot info: %w", responseErr(err, response.ErrorCode, response.Description))
	}

	return &response.Result, nil
}

// GetUpdates retrieves the latest updates.
//
// Parameters:
//   - offset: the identifier of the first update to return. Earlier updates
//     are confirmed and no longer returned.
//   - limit: the most updates to return, from 1 to 100.
//   - timeout: how many seconds to long poll, or 0 for none.
//   - allowedUpdates: the update types to receive, or nil to keep the previous
//     setting.
//
// Returns:
//   - []Update: the updates.
//   - error: the Telegram API error or the request error.
func (c *Client) GetUpdates(
	offset int,
	limit int,
	timeout int,
	allowedUpdates []string,
) ([]Update, error) {
	request := &updatesRequest{
		Offset:         offset,
		Limit:          limit,
		Timeout:        timeout,
		AllowedUpdates: allowedUpdates,
	}
	response := &updatesResponse{}
	jc := jsonclient.NewWithHTTPClient(c.httpClientOrDefault())

	if err := jc.Post(c.endpointURL("getUpdates"), request, response); err != nil || !response.OK {
		return nil, fmt.Errorf("getting updates: %w", responseErr(err, response.ErrorCode, response.Description))
	}

	return response.Result, nil
}

// SendMessage sends the specified Message.
//
// It delegates to [Client.SendMessageContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//
// Returns:
//   - *Message: the message Telegram created.
//   - error: the Telegram API error or the request error.
func (c *Client) SendMessage(message *SendMessagePayload) (*Message, error) {
	return c.SendMessageContext(context.Background(), message)
}

// SendMessageContext sends the specified Message.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//
// Returns:
//   - *Message: the message Telegram created.
//   - error: the Telegram API error or the request error, which matches ctx's
//     error when ctx ends the request.
func (c *Client) SendMessageContext(ctx context.Context, message *SendMessagePayload) (*Message, error) {
	response := &messageResponse{}
	jc := jsonclient.NewContextClient(c.httpClientOrDefault())

	if err := jc.PostContext(ctx, c.endpointURL("sendMessage"), message, response); err != nil || !response.OK {
		return nil, fmt.Errorf("sending message: %w", responseErr(err, response.ErrorCode, response.Description))
	}

	return response.Result, nil
}

// endpointURL returns the URL of a Bot API method, which is the server's base
// URL followed by /bot<token>/<method>.
//
// Parameters:
//   - endpoint: the Bot API method, such as sendMessage.
//
// Returns:
//   - string: the method's URL.
func (c *Client) endpointURL(endpoint string) string {
	base := strings.TrimSuffix(cmp.Or(c.apiURL, defaultAPIURL), "/")

	return base + "/bot" + c.token + "/" + endpoint
}

// httpClientOrDefault returns the injected client or the shared fallback
// client.
//
// Returns:
//   - types.HTTPClient: the injected client, or [fallbackClient].
func (c *Client) httpClientOrDefault() types.HTTPClient {
	if c.httpClient != nil {
		return c.httpClient
	}

	return fallbackClient
}

// GetErrorResponse retrieves the error message from a failed request.
//
// Parameters:
//   - body: the response body of the failed request.
//
// Returns:
//   - error: the Telegram API error, or nil when body is not a Telegram error response.
func GetErrorResponse(body string) error {
	response := &responseError{}
	if json.Unmarshal([]byte(body), response, jsonopts.Default) == nil &&
		(response.ErrorCode != 0 || response.Description != "") {
		return response
	}

	return nil
}

// responseErr returns the error for a failed API call.
//
// Parameters:
//   - err: the error from the JSON client, which may be nil.
//   - errorCode: the error code decoded from a successful HTTP response.
//   - description: the error description decoded from a successful HTTP response.
//
// Returns:
//   - error: the Telegram API error from the error body or the decoded fields,
//     wrapped with err when there is one, otherwise err, or [ErrUnexpectedResponse]
//     when the call reported no error.
func responseErr(err error, errorCode int, description string) error {
	apiErr := GetErrorResponse(jsonclient.ErrorBody(err))
	if apiErr == nil && (errorCode != 0 || description != "") {
		apiErr = &responseError{OK: false, ErrorCode: errorCode, Description: description}
	}

	switch {
	case apiErr != nil && err != nil:
		return fmt.Errorf("%w: %w", err, apiErr)
	case apiErr != nil:
		return apiErr
	case err != nil:
		return err
	default:
		return ErrUnexpectedResponse
	}
}
