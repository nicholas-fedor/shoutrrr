package telegram

import (
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
		return nil, fmt.Errorf("getting bot info: %w", responseErr(err))
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
		return nil, fmt.Errorf("getting updates: %w", responseErr(err))
	}

	return response.Result, nil
}

// SendMessage sends the specified Message.
func (c *Client) SendMessage(message *SendMessagePayload) (*Message, error) {
	response := &messageResponse{}
	jc := jsonclient.NewWithHTTPClient(c.httpClientOrDefault())

	if err := jc.Post(c.apiURL("sendMessage"), message, response); err != nil || !response.OK {
		return nil, fmt.Errorf("sending message: %w", responseErr(err))
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
//
// Returns:
//   - error: err wrapped with the Telegram API error from the response body when
//     the body has one, otherwise err, or [ErrUnexpectedResponse] when the call
//     reported no error.
func responseErr(err error) error {
	if apiErr := GetErrorResponse(jsonclient.ErrorBody(err)); apiErr != nil {
		return fmt.Errorf("%w: %w", err, apiErr)
	}

	if err != nil {
		return err
	}

	return ErrUnexpectedResponse
}
