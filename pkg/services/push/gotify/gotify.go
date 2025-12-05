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
	standard.Standard                        // Embeds the standard service functionality including logging
	Config            *Config                // Holds the configuration settings for the Gotify service, including host, token, and other parameters
	pkr               format.PropKeyResolver // Property key resolver used to update configuration from URL parameters dynamically
	mu                sync.Mutex             // Protects HTTP client initialization for thread safety
	httpClient        *http.Client           // HTTP client instance configured with appropriate timeout and transport settings for API calls
	client            jsonclient.Client      // JSON client wrapper that handles JSON request/response marshaling and HTTP communication

	// Interface dependencies (injected during initialization)
	httpClientManager HTTPClientManager
	urlBuilder        URLBuilder
	payloadBuilder    PayloadBuilder
	validator         Validator
	sender            Sender
}

// Initialize configures the service with a URL and logger.
// This method sets up the entire service infrastructure including configuration parsing,
// HTTP client creation with appropriate TLS settings, and logging capabilities.
// Parameters:
//   - configURL: The URL containing Gotify server configuration (host, token, path, etc.)
//   - logger: Logger instance for recording service operations and warnings
//
// Returns: error if configuration parsing or setup fails, nil on success.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	// Set the logger for this service instance to enable logging throughout the service lifecycle
	service.SetLogger(logger)

	// Initialize the configuration with default values
	service.Config = &Config{
		Title: "Shoutrrr notification", // Default notification title used when none specified
	}

	// Create a property key resolver to handle dynamic configuration updates from parameters
	service.pkr = format.NewPropKeyResolver(service.Config)

	// Parse the configuration URL to extract host, token, path, and other settings
	err := service.Config.SetURL(configURL)
	if err != nil {
		return fmt.Errorf("failed to set URL: %w", err)
	}

	// Inject default implementations for interfaces
	service.httpClientManager = &DefaultHTTPClientManager{}
	service.urlBuilder = &DefaultURLBuilder{}
	service.payloadBuilder = &DefaultPayloadBuilder{}
	service.validator = &DefaultValidator{}
	service.sender = &DefaultSender{}

	// Initialize HTTP client and related components in a thread-safe manner
	service.initClient()

	return nil // Return success
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// GetHTTPClient returns the HTTP client used by this service.
// This method implements the MockClientService interface for testing.
func (service *Service) GetHTTPClient() *http.Client {
	return service.httpClient
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
func (service *Service) Send(message string, params *types.Params) error {
	if err := service.validateInputs(message, params); err != nil {
		return fmt.Errorf("input validation failed: %w", err)
	}

	service.initClient()

	config, extras, err := service.processConfig(params)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	postURL, request, headers, err := service.buildRequest(message, &config, extras)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	return service.sendRequest(postURL, request, headers)
}

// validateInputs performs initial validation checks for the Send method.
func (service *Service) validateInputs(message string, _ *types.Params) error {
	if err := service.validator.ValidateMessage(message); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}

	if err := service.validator.ValidateServiceInitialized(service.Config); err != nil {
		return fmt.Errorf("service initialization validation failed: %w", err)
	}

	return nil
}

// processConfig handles configuration processing including parameter updates, validation, and extras parsing.
func (service *Service) processConfig(params *types.Params) (Config, map[string]any, error) {
	// Get reference to current configuration
	config := *service.Config

	// Filter out 'extras' parameter as it's handled separately from other config updates
	filteredParams := filterParams(params)

	// Update configuration with filtered parameters (title, priority, etc.)
	if err := service.pkr.UpdateConfigFromParams(&config, &filteredParams); err != nil {
		return config, nil, fmt.Errorf("failed to update config from params: %w", err)
	}

	// Validate priority is within valid range (-2 to 10)
	if err := service.validator.ValidatePriority(config.Priority); err != nil {
		return config, nil, fmt.Errorf("priority validation failed: %w", err)
	}

	// Validate and convert date format
	validatedDate, err := service.validator.ValidateDate(config.Date)
	if err != nil {
		service.Logf("Invalid date format: %v", err)

		config.Date = ""
	} else {
		config.Date = validatedDate
	}

	// Parse extras from parameters or fall back to config extras
	extras, err := service.payloadBuilder.ParseExtras(params, &config)
	if err != nil {
		service.Logf("Failed to parse extras from params: %v", err)

		extras = config.Extras
	}

	return config, extras, nil
}

// buildRequest constructs the URL, payload, and headers for the HTTP request.
func (service *Service) buildRequest(
	message string,
	config *Config,
	extras map[string]any,
) (string, *MessageRequest, http.Header, error) {
	// Validate token format before constructing URL
	if !service.validator.ValidateToken(config.Token) {
		return "", nil, nil, fmt.Errorf("%w: %q", ErrInvalidToken, config.Token)
	}

	// Construct the complete API endpoint URL
	postURL, err := service.urlBuilder.BuildURL(config)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to build URL: %w", err)
	}

	// Prepare the JSON request payload
	request := service.payloadBuilder.PrepareRequest(message, config, extras, config.Date)

	// Prepare headers for header-based authentication
	var headers http.Header
	if config.UseHeader {
		headers = make(http.Header)
		headers.Set("X-Gotify-Key", config.Token)
	}

	return postURL, request, headers, nil
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

// initClient initializes the HTTP client and related components.
// This method ensures that the transport, HTTP client, JSON client,
// and TLS warning logging are performed when needed, allowing re-initialization if the client becomes nil.
func (service *Service) initClient() {
	initClient(service, service.httpClientManager)
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
func (service *Service) sendRequest(
	postURL string,
	request *MessageRequest,
	headers http.Header,
) error {
	if err := service.sender.SendRequest(service.httpClient, postURL, request, headers); err != nil {
		return fmt.Errorf("%s: %w", ErrSendFailed.Error(), err)
	}

	return nil
}
