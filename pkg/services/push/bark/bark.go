package bark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/redact"
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

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

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
		return resp, fmt.Errorf("executing HTTP request: %w", redact.URLError(err))
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
	if s.HTTPClient == nil {
		s.HTTPClient = NewDefaultHTTPClient()
	}

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send transmits a notification message to the Bark server.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: The notification body text to send.
//   - params: Additional parameters for notification customization.
//
// Returns:
//   - error: the params or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext transmits a notification message to the Bark server.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: The notification body text to send.
//   - params: Additional parameters for notification customization.
//
// Returns:
//   - error: the params or send error. A request error matches ctx's error when
//     ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("%w: %w", ErrUpdateParamsFailed, err)
	}

	if err := s.sendAPI(ctx, config, message); err != nil {
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
//   - error: the send failure, or nil on success.
//
// Deprecated: Use [Service.Send] with [types.ItemsToPlain]. The router already
// sends rich messages to Bark as plain text.
//
//go:fix inline
func (s *Service) SendItems(items []types.MessageItem, params *types.Params) error {
	return s.Send(types.ItemsToPlain(items), params)
}

// ServiceTimeout returns the HTTP timeout used for a Bark send.
//
// Parameters:
//   - params: Unused. The budget does not depend on send parameters.
//
// Returns:
//   - [defaultHTTPTimeout].
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return defaultHTTPTimeout
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	if client == nil {
		s.HTTPClient = NewDefaultHTTPClient()

		return
	}

	s.HTTPClient = client
}

// sendAPI sends a notification to the Bark server using the configured endpoint.
// This method handles JSON serialization, HTTP request creation, and response parsing.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: The Bark service configuration containing API settings.
//   - message: The notification body text to send.
//
// Returns:
//   - error: the request error, or the error the Bark server reported.
func (s *Service) sendAPI(ctx context.Context, config *Config, message string) error {
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
		if jsonErr, ok := errors.AsType[jsonclient.Error](err); ok {
			if json.Unmarshal([]byte(jsonErr.Body), &response) == nil {
				return &response
			}
		}

		return fmt.Errorf("%w: %w", ErrFailedAPIRequest, redact.URLError(err))
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
