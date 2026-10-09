package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Client for Telegram API.
type Client struct {
	token      string
	httpClient types.HTTPClient
}

// GetBotInfo returns the bot User info.
func (c *Client) GetBotInfo() (*User, error) {
	response := &userResponse{}
	jc := jsonclient.NewWithHTTPClient(c.httpClientOrDefault())

	if err := jc.Get(c.apiURL("getMe"), response); err != nil || !response.OK {
		return nil, fmt.Errorf("getting bot info: %w", responseErr(err, response.ErrorCode, response.Description))
	}

	return &response.Result, nil
}

// GetUpdates retrieves the latest updates.
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

	if err := jc.Post(c.apiURL("getUpdates"), request, response); err != nil || !response.OK {
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

	if err := jc.PostContext(ctx, c.apiURL("sendMessage"), message, response); err != nil || !response.OK {
		return nil, fmt.Errorf("sending message: %w", responseErr(err, response.ErrorCode, response.Description))
	}

	return response.Result, nil
}

func (c *Client) apiURL(endpoint string) string {
	return fmt.Sprintf(apiFormat, c.token, endpoint)
}

// httpClientOrDefault returns the injected client or a default Client.
func (c *Client) httpClientOrDefault() types.HTTPClient {
	if c.httpClient != nil {
		return c.httpClient
	}

	return &http.Client{Timeout: defaultHTTPTimeout}
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
	if json.Unmarshal([]byte(body), response) == nil &&
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
