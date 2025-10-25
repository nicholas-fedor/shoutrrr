package signal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTP request timeout duration.
const (
	defaultHTTPTimeout = 30 * time.Second
)

// ErrSendFailed indicates a failure to send a Signal message.
var (
	ErrSendFailed = errors.New("failed to send Signal message")
)

// Service sends notifications to Signal recipients via signal-cli-rest-api.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to Signal recipients.
func (service *Service) Send(message string, params *types.Params) error {
	config := *service.Config

	// Separate config params from message params (like attachments)
	var (
		configParams  *types.Params
		messageParams *types.Params
	)

	if params != nil {
		configParams = &types.Params{}
		messageParams = &types.Params{}

		for key, value := range *params {
			// Check if this is a config parameter
			if _, err := service.pkr.Get(key); err == nil {
				// It's a valid config key
				(*configParams)[key] = value
			} else {
				// It's a message parameter (like attachments)
				(*messageParams)[key] = value
			}
		}

		if err := service.pkr.UpdateConfigFromParams(&config, configParams); err != nil {
			return fmt.Errorf("updating config from params: %w", err)
		}
	}

	return service.sendMessage(message, &config, messageParams)
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	if err := service.Config.setURL(&service.pkr, configURL); err != nil {
		return err
	}

	return nil
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// sendMessage sends a message to all configured recipients.
func (service *Service) sendMessage(message string, config *Config, params *types.Params) error {
	if len(config.Recipients) == 0 {
		return ErrNoRecipients
	}

	payload := service.createPayload(message, config, params)

	req, cancel, err := service.createRequest(config, payload)
	if err != nil {
		return err
	}
	defer cancel()

	return service.sendRequest(req)
}

// createPayload builds the JSON payload for the Signal API request.
func (service *Service) createPayload(
	message string,
	config *Config,
	params *types.Params,
) sendMessagePayload {
	payload := sendMessagePayload{
		Message:    message,
		Number:     config.Source,
		Recipients: config.Recipients,
	}

	// Check for attachments in params (passed during Send call)
	// Note: Shoutrrr doesn't have a standard attachment interface,
	// so we check for "attachments" parameter with base64 data
	if params != nil {
		if attachments, ok := (*params)["attachments"]; ok && attachments != "" {
			// Parse comma-separated base64 attachments
			attachmentList := strings.Split(attachments, ",")
			for i, attachment := range attachmentList {
				attachmentList[i] = strings.TrimSpace(attachment)
			}

			payload.Base64Attachments = attachmentList
		}
	}

	return payload
}

// createRequest builds the HTTP request for the Signal API.
func (service *Service) createRequest(
	config *Config,
	payload sendMessagePayload,
) (*http.Request, context.CancelFunc, error) {
	apiURL := service.buildAPIURL(config)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		cancel()

		return nil, nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	service.setAuthentication(req, config)

	return req, cancel, nil
}

// buildAPIURL constructs the Signal API endpoint URL.
func (service *Service) buildAPIURL(config *Config) string {
	scheme := "https"
	if config.DisableTLS {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s:%d/v2/send", scheme, config.Host, config.Port)
}

// setAuthentication configures HTTP authentication headers.
func (service *Service) setAuthentication(req *http.Request, config *Config) {
	// Add authentication - prefer Bearer token over Basic Auth
	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	} else if config.User != "" {
		req.SetBasicAuth(config.User, config.Password)
	}
}

// sendRequest executes the HTTP request and handles the response.
func (service *Service) sendRequest(req *http.Request) error {
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: server returned status %d", ErrSendFailed, resp.StatusCode)
	}

	// Parse response (optional, for logging)
	service.parseResponse(resp)

	return nil
}

// parseResponse extracts and logs response information.
func (service *Service) parseResponse(resp *http.Response) {
	var response sendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		service.Logf("Warning: failed to parse response: %v", err)
	} else {
		service.Logf("Message sent successfully at timestamp %d", response.Timestamp)
	}
}
