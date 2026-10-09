package gotify

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Service implements a Gotify notification service that handles sending push notifications
// to Gotify servers. It manages HTTP client configuration, TLS settings, authentication,
// and payload construction for reliable message delivery.
type Service struct {
	standard.Standard // Embeds the standard service functionality including logging

	Config     *Config                // Holds the configuration settings for the Gotify service, including host, token, and other parameters
	pkr        format.PropKeyResolver // Property key resolver used to update configuration from URL parameters dynamically
	mu         sync.Mutex             // Protects HTTP client initialization for thread safety
	httpClient types.HTTPClient       // HTTP client instance configured with appropriate timeout and transport settings for API calls
	client     jsonclient.Client      // JSON client wrapper that handles JSON request/response marshaling and HTTP communication

	// Interface dependencies (injected during initialization)
	httpClientManager HTTPClientManager
	urlBuilder        URLBuilder
	payloadBuilder    PayloadBuilder
	validator         Validator
	sender            contextSender
}

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
// This method sets up the entire service infrastructure including configuration parsing,
// HTTP client creation with appropriate TLS settings, and logging capabilities.
// Parameters:
//   - serviceURL: The URL containing Gotify server configuration (host, token, path, etc.)
//   - logger: Logger instance for recording service operations and warnings
//
// Returns: error if configuration parsing or setup fails, nil on success.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	// Set the logger for this service instance to enable logging throughout the service lifecycle
	s.SetLogger(logger)

	// Initialize the configuration with default values
	s.Config = &Config{
		Title: "Shoutrrr notification", // Default notification title used when none specified
	}

	// Create a property key resolver to handle dynamic configuration updates from parameters
	s.pkr = format.NewPropKeyResolver(s.Config)

	// Parse the configuration URL to extract host, token, path, and other settings
	err := s.Config.SetURL(serviceURL)
	if err != nil {
		return fmt.Errorf("failed to set URL: %w", err)
	}

	// Inject default implementations for interfaces
	s.httpClientManager = &DefaultHTTPClientManager{}
	s.urlBuilder = &DefaultURLBuilder{}
	s.payloadBuilder = &DefaultPayloadBuilder{}
	s.validator = &DefaultValidator{}
	s.sender = &DefaultSender{}

	// Initialize HTTP client and related components in a thread-safe manner
	s.initClient()

	return nil // Return success
}

// Send delivers a notification message to Gotify.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the notification message content to send, which cannot be empty.
//   - params: optional parameters that override configuration settings or provide extras.
//
// Returns:
//   - error: the validation, config, or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Gotify.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the notification message content to send, which cannot be empty.
//   - params: optional parameters that override configuration settings or provide extras.
//
// Returns:
//   - error: the validation, config, or send error. A request error matches
//     ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if err := s.validateInputs(message, params); err != nil {
		return fmt.Errorf("input validation failed: %w", err)
	}

	s.initClient()

	config, extras, err := s.processConfig(params)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	postURL, request, headers, err := s.buildRequest(message, &config, extras)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	return s.sendRequest(ctx, postURL, request, headers)
}

// SetHTTPClient allows external injection of a custom HTTP client (for router propagation).
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if client == nil {
		s.httpClient = nil
		s.client = nil

		return
	}

	if c, ok := client.(*http.Client); ok && c == nil {
		s.httpClient = nil
		s.client = nil

		return
	}

	s.httpClient = client
	s.client = jsonclient.NewWithHTTPClient(client)
}

// buildRequest constructs the URL, payload, and headers for the HTTP request.
func (s *Service) buildRequest(
	message string,
	config *Config,
	extras map[string]any,
) (string, *MessageRequest, http.Header, error) {
	// Validate token format before constructing URL
	if !s.validator.ValidateToken(config.Token) {
		return "", nil, nil, ErrInvalidToken
	}

	// Construct the complete API endpoint URL
	postURL, err := s.urlBuilder.BuildURL(config)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Prepare the JSON request payload
	request := s.payloadBuilder.PrepareRequest(message, config, extras, config.Date)

	// Prepare headers for header-based authentication
	var headers http.Header
	if config.UseHeader {
		headers = make(http.Header)
		headers.Set("X-Gotify-Key", config.Token)
	}

	return postURL, request, headers, nil
}

// initClient initializes the HTTP client and related components.
// This method ensures that the transport, HTTP client, JSON client,
// and TLS warning logging are performed when needed, allowing re-initialization if the client becomes nil.
func (s *Service) initClient() {
	initClient(s, s.httpClientManager)
}

// processConfig handles configuration processing including parameter updates, validation, and extras parsing.
func (s *Service) processConfig(params *types.Params) (Config, map[string]any, error) {
	// Get reference to current configuration
	config := *s.Config

	// Filter out 'extras' parameter as it's handled separately from other config updates
	filteredParams := filterParams(params)

	// Update configuration with filtered parameters (title, priority, etc.)
	if err := s.pkr.UpdateConfigFromParams(&config, &filteredParams); err != nil {
		return config, nil, fmt.Errorf("failed to update config from params: %w", err)
	}

	// Validate priority is within valid range (-2 to 10)
	if err := s.validator.ValidatePriority(config.Priority); err != nil {
		return config, nil, fmt.Errorf("priority validation failed: %w", err)
	}

	// Validate and convert date format
	validatedDate, err := s.validator.ValidateDate(config.Date)
	if err != nil {
		s.Logf("Invalid date format: %v", err)

		config.Date = ""
	} else {
		config.Date = validatedDate
	}

	// Parse extras from parameters or fall back to config extras
	extras, err := s.payloadBuilder.ParseExtras(params, &config)
	if err != nil {
		s.Logf("Failed to parse extras from params: %v", err)

		extras = config.Extras
	}

	return config, extras, nil
}

// sendRequest executes the HTTP POST request to the Gotify API endpoint.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - postURL: the complete API endpoint URL to send the request to.
//   - request: the JSON payload to send in the request body.
//   - headers: optional headers to set on the request.
//
// Returns:
//   - error: [ErrSendFailed] wrapping the request error or the error the server reported.
func (s *Service) sendRequest(
	ctx context.Context,
	postURL string,
	request *MessageRequest,
	headers http.Header,
) error {
	if err := s.sender.SendRequestContext(
		ctx,
		s.httpClient,
		postURL,
		request,
		headers,
	); err != nil {
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	return nil
}

// validateInputs performs initial validation checks for the Send method.
func (s *Service) validateInputs(message string, _ *types.Params) error {
	if err := s.validator.ValidateMessage(message); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}

	if err := s.validator.ValidateServiceInitialized(s.Config); err != nil {
		return fmt.Errorf("service initialization validation failed: %w", err)
	}

	return nil
}

// filterParams filters out 'extras' parameters from the given params.
func filterParams(params *types.Params) types.Params {
	if params == nil {
		return types.Params{}
	}

	filtered := make(types.Params)

	for k, v := range *params {
		if k != "extras" {
			filtered[k] = v
		}
	}

	return filtered
}
