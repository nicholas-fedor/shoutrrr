package signal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Signal recipients via signal-cli-rest-api.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

// HTTP request timeout duration.
const (
	defaultHTTPTimeout = 30 * time.Second
)

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - configURL: the configuration URL for the Signal service
//   - logger: the logger to use for logging
//
// Returns:
//   - error: if configuration fails, nil otherwise
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, configURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Signal recipients.
//
// Parameters:
//   - message: the text message to send
//   - params: optional parameters (e.g., attachments)
//
// Returns:
//   - error: if the send operation fails, nil otherwise
func (s *Service) Send(message string, params *types.Params) error {
	config := *s.Config

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
			if _, err := s.pkr.Get(key); err == nil {
				// It's a valid config key
				(*configParams)[key] = value
			} else {
				// It's a message parameter (like attachments)
				(*messageParams)[key] = value
			}
		}

		if err := s.pkr.UpdateConfigFromParams(&config, configParams); err != nil {
			return fmt.Errorf("updating config from params: %w", err)
		}
	}

	return s.sendMessage(message, &config, messageParams)
}

// buildAPIURL constructs the Signal API endpoint URL from the configuration.
//
// Parameters:
//   - config: the service configuration
//
// Returns:
//   - string: the full API endpoint URL
func (s *Service) buildAPIURL(config *Config) string {
	scheme := "https"
	if config.DisableTLS {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s:%d/v2/send", scheme, config.Host, config.Port)
}

// createPayload builds the JSON payload for the Signal API request.
//
// Parameters:
//   - message: the message text
//   - config: the service configuration
//   - params: optional parameters (may include attachments)
//
// Returns:
//   - sendMessagePayload: the payload struct to be sent
func (s *Service) createPayload(
	message string,
	config *Config,
	params *types.Params,
) sendMessagePayload {
	payload := sendMessagePayload{
		Message:           message,
		Number:            config.Source,
		Recipients:        config.Recipients,
		Base64Attachments: nil, // will be set if attachments provided
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
//
// Parameters:
//   - config: the service configuration
//   - payload: the payload to send (passed as pointer for efficiency)
//
// Returns:
//   - *http.Request: the constructed HTTP request
//   - context.CancelFunc: a function to cancel the request context
//   - error: if request creation fails, nil otherwise
func (s *Service) createRequest(
	config *Config,
	payload *sendMessagePayload,
) (*http.Request, context.CancelFunc, error) {
	apiURL := s.buildAPIURL(config)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		defaultHTTPTimeout,
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		cancel()

		return nil, nil, fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	s.setAuthentication(req, config)

	return req, cancel, nil
}

// parseResponse reads and logs the response from the Signal API.
//
// Parameters:
//   - resp: the HTTP response to parse
func (s *Service) parseResponse(resp *http.Response) {
	var response sendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		s.Logf("Warning: failed to parse response: %v", err)
	} else {
		s.Logf("Message sent successfully at timestamp %d", response.Timestamp)
	}
}

// sendMessage sends a message to all configured recipients.
//
// Parameters:
//   - message: the message text to send
//   - config: the service configuration
//   - params: optional parameters (e.g., attachments)
//
// Returns:
//   - error: if sending fails, nil otherwise
func (s *Service) sendMessage(message string, config *Config, params *types.Params) error {
	if len(config.Recipients) == 0 {
		return ErrNoRecipients
	}

	payload := s.createPayload(message, config, params)

	req, cancel, err := s.createRequest(config, &payload)
	if err != nil {
		return err
	}
	defer cancel()

	return s.sendRequest(req)
}

// sendRequest executes the HTTP request and processes the response.
//
// Parameters:
//   - req: the HTTP request to execute
//
// Returns:
//   - error: if the request fails or returns a non-success status, nil otherwise
func (s *Service) sendRequest(req *http.Request) error {
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
	s.parseResponse(resp)

	return nil
}

// setAuthentication configures HTTP authentication headers on the request.
//
// Parameters:
//   - req: the HTTP request to modify
//   - config: the service configuration containing credentials
func (s *Service) setAuthentication(req *http.Request, config *Config) {
	// Add authentication - prefer Bearer token over Basic Auth
	if config.Token != "" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	} else if config.User != "" {
		req.SetBasicAuth(config.User, config.Password)
	}
}
