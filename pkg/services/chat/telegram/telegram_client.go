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
	err := jc.Get(c.apiURL("getMe"), response)

	if !response.OK {
		return nil, GetErrorResponse(jsonclient.ErrorBody(err))
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
	err := jc.Post(c.apiURL("getUpdates"), request, response)

	if !response.OK {
		return nil, GetErrorResponse(jsonclient.ErrorBody(err))
	}

	return response.Result, nil
}

// SendMessage sends the specified Message.
func (c *Client) SendMessage(message *SendMessagePayload) (*Message, error) {
	response := &messageResponse{}
	jc := jsonclient.NewWithHTTPClient(c.httpClientOrDefault())
	err := jc.Post(c.apiURL("sendMessage"), message, response)

	if !response.OK {
		return nil, GetErrorResponse(jsonclient.ErrorBody(err))
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
func GetErrorResponse(body string) error {
	response := &responseError{}
	if err := json.Unmarshal([]byte(body), response); err == nil {
		return response
	}

	return nil
}
