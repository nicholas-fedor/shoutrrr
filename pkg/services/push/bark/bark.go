package bark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// HTTPClient defines the interface for HTTP operations required by the Bark service.
// This interface allows for dependency injection of HTTP clients for testing purposes.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient is the default HTTP client implementation used when no
// custom client is provided. It uses a reasonable timeout to prevent hanging requests.
type DefaultHTTPClient struct {
	client *http.Client
}

// Service sends push notifications to Bark-enabled iOS devices.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	HTTPClient HTTPClient
}

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 30 * time.Second

// NewDefaultHTTPClient creates a new HTTP client with default timeout settings.
//
// Returns:
//   - A configured DefaultHTTPClient instance ready for use.
func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Do executes the HTTP request and returns the response.
//
// Parameters:
//   - req: The HTTP request to execute.
//
// Returns:
//   - The HTTP response from the server.
//   - An error if the request fails.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("executing HTTP request: %w", err)
	}

	return resp, nil
}

// GetID returns the scheme identifier for the Bark service.
//
// Returns:
//   - The string "bark" representing this notification service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize sets up the Bark service with configuration from the provided URL.
//
// Parameters:
//   - serviceURL: URL containing configuration for the Bark service.
//   - logger: Standard logger for output messages.
//
// Returns:
//   - An error if initialization fails.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.HTTPClient = NewDefaultHTTPClient()

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send transmits a notification message to the Bark server.
//
// Parameters:
//   - message: The notification body text to send.
//   - params: Additional parameters for notification customization.
//
// Returns:
//   - An error if the notification fails to send.
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("%w: %w", ErrUpdateParamsFailed, err)
	}

	if err := s.sendAPI(config, message); err != nil {
		return fmt.Errorf("failed to send bark notification: %w", err)
	}

	return nil
}

// SendItems converts message items to plain text and sends as a notification.
// This method handles rich message items by extracting plain text content.
//
// Parameters:
//   - items: Slice of message items to send.
//   - params: Additional parameters for notification customization.
//
// Returns:
//   - An error if the notification fails to send.
func (s *Service) SendItems(items []types.MessageItem, params *types.Params) error {
	// Convert message items to plain text
	message := types.ItemsToPlain(items)

	return s.Send(message, params)
}

// sendAPI sends a notification to the Bark server using the configured endpoint.
// This method handles JSON serialization, HTTP request creation, and response parsing.
//
// Parameters:
//   - config: The Bark service configuration containing API settings.
//   - message: The notification body text to send.
//
// Returns:
//   - An error if the API request fails.
func (s *Service) sendAPI(config *Config, message string) error {
	// Use background context for the request - services can add context support
	// by wrapping this method or using a custom HTTP client
	ctx := context.Background()

	response := APIResponse{}
	request := PushPayload{
		Body:      message,
		DeviceKey: config.DeviceKey,
		Title:     config.Title,
		Category:  config.Category,
		Copy:      config.Copy,
		Sound:     config.Sound,
		Group:     config.Group,
		Badge:     &config.Badge,
		Icon:      config.Icon,
		URL:       config.URL,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshaling request to JSON: %w", err)
	}

	apiURL := config.GetAPIURL("push")

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		// Try to parse error response
		jsonClient := jsonclient.NewClient()
		if jsonClient.ErrorResponse(err, &response) {
			return &response
		}

		return fmt.Errorf("%w: %w", ErrFailedAPIRequest, err)
	}

	defer func() { _ = httpResp.Body.Close() }()

	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if response.Code != http.StatusOK {
		if response.Message != "" {
			return &response
		}

		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.Code)
	}

	return nil
}
