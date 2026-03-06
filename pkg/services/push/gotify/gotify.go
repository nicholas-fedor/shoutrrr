package gotify

import (
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
	httpClient *http.Client           // HTTP client instance configured with appropriate timeout and transport settings for API calls
	client     jsonclient.Client      // JSON client wrapper that handles JSON request/response marshaling and HTTP communication

	// Interface dependencies (injected during initialization)
	httpClientManager HTTPClientManager
	urlBuilder        URLBuilder
	payloadBuilder    PayloadBuilder
	validator         Validator
	sender            Sender
}

// GetHTTPClient returns the HTTP client used by this service.
// This method implements the MockClientService interface for testing.
func (s *Service) GetHTTPClient() *http.Client {
	return s.httpClient
}

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
// This method sets up the entire service infrastructure including configuration parsing,
// HTTP client creation with appropriate TLS settings, and logging capabilities.
// Parameters:
//   - configURL: The URL containing Gotify server configuration (host, token, path, etc.)
//   - logger: Logger instance for recording service operations and warnings
//
// Returns: error if configuration parsing or setup fails, nil on success.
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	// Set the logger for this service instance to enable logging throughout the service lifecycle
	s.SetLogger(logger)

	// Initialize the configuration with default values
	s.Config = &Config{
		Title: "Shoutrrr notification", // Default notification title used when none specified
	}

	// Create a property key resolver to handle dynamic configuration updates from parameters
	s.pkr = format.NewPropKeyResolver(s.Config)

	// Parse the configuration URL to extract host, token, path, and other settings
	err := s.Config.SetURL(configURL)
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
// This is the main entry point for sending notifications. It handles message validation,
// parameter processing, configuration updates, URL construction, authentication setup,
// and HTTP request execution.
// Parameters:
//   - message: The notification message content to send (cannot be empty)
//   - params: Optional parameters that can override configuration settings or provide extras
//
// Returns: error if sending fails or validation fails, nil on successful delivery.
func (s *Service) Send(message string, params *types.Params) error {
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

	return s.sendRequest(postURL, request, headers)
}

// buildRequest constructs the URL, payload, and headers for the HTTP request.
func (s *Service) buildRequest(
	message string,
	config *Config,
	extras map[string]any,
) (string, *MessageRequest, http.Header, error) {
	// Validate token format before constructing URL
	if !s.validator.ValidateToken(config.Token) {
		return "", nil, nil, fmt.Errorf("%w: %q", ErrInvalidToken, config.Token)
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

// sendRequest handles the HTTP request.
// This function executes the actual HTTP POST request to the Gotify API endpoint,
// handling both successful responses and error conditions with appropriate error wrapping.
// Parameters:
//   - postURL: The complete API endpoint URL to send the request to
//   - request: The JSON payload to send in the request body
//   - headers: Optional headers to set on the request
//
// Returns: error if the request fails or server returns an error, nil on success.
func (s *Service) sendRequest(
	postURL string,
	request *MessageRequest,
	headers http.Header,
) error {
	if err := s.sender.SendRequest(
		s.httpClient,
		postURL,
		request,
		headers,
	); err != nil {
		return fmt.Errorf("%s: %w", ErrSendFailed.Error(), err)
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
