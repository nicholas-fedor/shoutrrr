package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Constants for the WeCom service configuration and limits.
const (
	apiURL      = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s"
	maxLength   = 4096 // Maximum message length in bytes
	defaultTime = 30 * time.Second
)

// Error variables for the WeCom service.
var (
	ErrLargeMessage = fmt.Errorf("message exceeds the max length of %d bytes", maxLength)
	ErrSendFailed   = errors.New("failed to send notification to WeCom")
	ErrKeyRequired  = errors.New("webhook key is required")
)

// httpClient is configured with a default timeout.
var httpClient = &http.Client{Timeout: defaultTime}

// Service sends notifications to WeCom.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to WeCom.
func (service *Service) Send(message string, params *types.Params) error {
	if len(message) > maxLength {
		return ErrLargeMessage
	}

	config := *service.Config
	if err := service.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating params: %w", err)
	}

	if config.Key == "" {
		return ErrKeyRequired
	}

	return service.doSend(config, message, params)
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	return service.Config.SetURL(configURL)
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// doSend sends the notification to WeCom using the configured API URL.
func (service *Service) doSend(config Config, message string, params *types.Params) error {
	postURL := fmt.Sprintf(apiURL, config.Key)

	payload, err := service.preparePayload(message, config, params)
	if err != nil {
		return err
	}

	return service.sendRequest(postURL, payload)
}

// preparePayload constructs and marshals the request payload for the WeCom API.
func (service *Service) preparePayload(
	message string,
	config Config,
	params *types.Params,
) ([]byte, error) {
	body := service.getRequestBody(message, config, params)

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	service.Logf("WeCom Request Body: %s", string(data))

	return data, nil
}

// sendRequest performs the HTTP POST request to the WeCom API and handles the response.
func (service *Service) sendRequest(postURL string, payload []byte) error {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		postURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: making HTTP request: %w", ErrSendFailed, err)
	}

	defer func() { _ = resp.Body.Close() }()

	return service.handleResponse(resp)
}

// handleResponse processes the API response and checks for errors.
func (service *Service) handleResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: unexpected status %s", ErrSendFailed, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	var response Response
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("unmarshaling response: %w", err)
	}

	if response.ErrCode != 0 {
		return fmt.Errorf(
			"%w: server returned error code %d: %s",
			ErrSendFailed,
			response.ErrCode,
			response.ErrMsg,
		)
	}

	service.Logf("Notification sent successfully to WeCom webhook")

	return nil
}

// getRequestBody constructs the request body for the WeCom API.
func (service *Service) getRequestBody(
	message string,
	config Config,
	_ *types.Params,
) *RequestBody {
	body := &RequestBody{
		MsgType: "text",
		Text: TextContent{
			Content: message,
		},
	}

	// Handle mentions from config
	if config.MentionedList != "" {
		// Parse comma-separated list
		body.Text.MentionedList = []string{config.MentionedList}
	}

	if config.MentionedMobileList != "" {
		body.Text.MentionedMobileList = []string{config.MentionedMobileList}
	}

	return body
}
