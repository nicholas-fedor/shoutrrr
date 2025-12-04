package gotify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

const (
	// HTTPTimeout defines the HTTP client timeout in seconds.
	HTTPTimeout = 10
	// TokenLength defines the expected length of a Gotify token, which must be exactly 15 characters and start with 'A'.
	TokenLength = 15
	// TokenChars specifies the valid characters for a Gotify token.
	TokenChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_"
	// HTTPClientErrorThreshold specifies the status code threshold for client errors (400+).
	HTTPClientErrorThreshold = 400
)

// ErrInvalidToken indicates an invalid Gotify token format or content.
var ErrInvalidToken = errors.New("invalid gotify token")

// ErrEmptyMessage indicates that the message to send is empty.
var ErrEmptyMessage = errors.New("message cannot be empty")

// ErrUnexpectedStatus indicates an unexpected HTTP response status.
var ErrUnexpectedStatus = errors.New("got unexpected HTTP status")

// Service implements a Gotify notification service that handles sending push notifications
// to Gotify servers. It manages HTTP client configuration, TLS settings, authentication,
// and payload construction for reliable message delivery.
type Service struct {
	standard.Standard                        // Embeds the standard service functionality including logging
	Config            *Config                // Holds the configuration settings for the Gotify service, including host, token, and other parameters
	pkr               format.PropKeyResolver // Property key resolver used to update configuration from URL parameters dynamically
	httpClient        *http.Client           // HTTP client instance configured with appropriate timeout and transport settings for API calls
	client            jsonclient.Client      // JSON client wrapper that handles JSON request/response marshaling and HTTP communication
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
		return err // Return error if URL parsing fails
	}

	// Create HTTP transport with TLS configuration based on DisableTLS setting
	transport := service.createTransport()

	// Create HTTP client with the configured transport and timeout settings
	service.httpClient = service.createHTTPClient(transport)

	// Initialize the JSON client wrapper for handling API requests and responses
	service.client = jsonclient.NewWithHTTPClient(service.httpClient)

	// Log a warning if TLS verification is disabled for security awareness
	if service.Config.DisableTLS {
		service.Log("Warning: TLS verification is disabled, making connections insecure")
	}

	return nil // Return success
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
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
	// Validate that the message is not empty before proceeding
	if message == "" {
		return ErrEmptyMessage
	}

	// Ensure params is not nil to avoid nil pointer dereferences
	if params == nil {
		params = &types.Params{}
	}

	// Lazy initialization of HTTP client if not already created
	// This allows the service to be initialized without immediate client creation
	if service.httpClient == nil {
		// Create transport with TLS settings
		transport := service.createTransport()
		// Create HTTP client with transport and timeout
		service.httpClient = service.createHTTPClient(transport)

		// Initialize JSON client wrapper
		service.client = jsonclient.NewWithHTTPClient(service.httpClient)
		// Log security warning for disabled TLS
		if service.Config.DisableTLS {
			service.Log("Warning: TLS verification is disabled, making connections insecure")
		}
	}

	// Get reference to current configuration
	config := service.Config

	// Begin parameter processing section
	// Filter out 'extras' parameter as it's handled separately from other config updates
	filteredParams := make(types.Params)

	// Iterate through all provided parameters
	for k, v := range *params {
		// Skip 'extras' key as it requires special JSON parsing
		if k != "extras" {
			filteredParams[k] = v
		}
	}

	// Update configuration with filtered parameters (title, priority, etc.)
	if err := service.pkr.UpdateConfigFromParams(config, &filteredParams); err != nil {
		return fmt.Errorf("failed to update config from params: %w", err)
	}

	// Parse extras from parameters or fall back to config extras
	extras := service.parseExtras(params, config)

	// Construct the complete API endpoint URL
	postURL, err := buildURL(config)
	if err != nil {
		return err
	}

	// Prepare the JSON request payload
	request := service.prepareRequest(message, config, extras)

	// Prepare headers for header-based authentication
	var headers http.Header
	if config.UseHeader {
		headers = make(http.Header)
		headers.Set("X-Gotify-Key", config.Token)
	}

	// Execute the HTTP request and return result
	return service.sendRequest(postURL, request, headers)
}

// GetHTTPClient returns the HTTP client for testing purposes.
func (service *Service) GetHTTPClient() *http.Client {
	return service.httpClient
}

// createTransport sets up the HTTP transport with TLS configuration and proxy settings.
// This method configures the underlying HTTP transport layer with appropriate security settings
// based on the service configuration, particularly handling TLS verification preferences.
// Returns: *http.Transport configured with TLS settings for secure or insecure connections.
func (service *Service) createTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: service.Config.DisableTLS, //nolint:gosec // Intentionally allow insecure connections when explicitly configured
		},
		// Proxy settings can be added here if needed in future enhancements
	}
}

// createHTTPClient creates an HTTP client with timeout and transport.
// This method assembles an HTTP client with the configured transport and timeout settings
// to ensure reliable API communication with appropriate performance characteristics.
// Parameters:
//   - transport: The HTTP transport with TLS and proxy configuration
//
// Returns: *http.Client ready for making API requests with proper timeout and transport settings.
func (service *Service) createHTTPClient(transport *http.Transport) *http.Client {
	return &http.Client{
		Transport: transport,                 // Use the configured transport for TLS and proxy handling
		Timeout:   HTTPTimeout * time.Second, // Set timeout to prevent hanging requests
	}
}

// validateToken checks if a Gotify token meets length and character requirements.
// This function implements Gotify's token validation rules to ensure tokens are properly formatted
// before attempting API calls. Tokens must be exactly 15 characters, start with 'A', and contain
// only valid characters from the allowed set.
// Parameters:
//   - token: The token string to validate
//
// Returns: true if token is valid according to Gotify's rules, false otherwise.
func validateToken(token string) bool {
	// First check: token must be exactly 15 characters long and start with 'A'
	if len(token) != TokenLength || token[0] != 'A' {
		return false
	}

	// Second check: iterate through each character to ensure only valid characters are used
	for _, c := range token {
		// Check if current character exists in the allowed character set
		if !strings.ContainsRune(TokenChars, c) {
			return false
		}
	}

	// All validation checks passed
	return true
}

// buildURL constructs the Gotify API URL with scheme, host, path, and token.
// This function builds the complete endpoint URL for the Gotify message API, handling
// different authentication methods (header vs query parameter) and TLS settings.
// The URL format depends on whether header authentication is enabled.
// Parameters:
//   - config: Configuration containing host, path, token, and authentication settings
//
// Returns: complete API URL string, or error if token validation fails.
func buildURL(config *Config) (string, error) {
	// Extract token from config for validation
	token := config.Token

	// Validate token format before constructing URL
	if !validateToken(token) {
		return "", fmt.Errorf("%w: %q", ErrInvalidToken, token)
	}

	// Determine URL scheme based on TLS settings
	scheme := "https"
	if config.DisableTLS {
		scheme = "http" // Use HTTP scheme when TLS verification is disabled
	}

	// Construct URL based on authentication method
	if config.UseHeader {
		// Header authentication: token sent in X-Gotify-Key header, not in URL
		return fmt.Sprintf("%s://%s%s/message", scheme, config.Host, config.Path), nil
	}

	// Query parameter authentication: include token in URL query string
	return fmt.Sprintf("%s://%s%s/message?token=%s", scheme, config.Host, config.Path, token), nil
}

// parseExtras handles extras parsing from params.
// This function processes the 'extras' parameter which contains additional JSON data
// to be sent with the notification. It attempts to parse JSON from parameters first,
// falling back to configuration extras if parsing fails or no parameter extras exist.
// Parameters:
//   - params: Request parameters that may contain 'extras' JSON string
//   - config: Configuration that may contain default extras
//
// Returns: map of extra data to include in the notification payload.
func (service *Service) parseExtras(params *types.Params, config *Config) map[string]any {
	// Initialize variable to hold parsed extras
	var requestExtras map[string]any

	// Check if parameters exist and contain extras
	if params != nil {
		// Look for 'extras' key in parameters
		if extrasStr, exists := (*params)["extras"]; exists && extrasStr != "" {
			// Attempt to parse the JSON string into a map
			if err := json.Unmarshal([]byte(extrasStr), &requestExtras); err != nil {
				// Log parsing failure but don't fail the entire operation
				service.Logf("Failed to parse extras from params: %v", err)
			}
		}
	}

	// Fall back to configuration extras if no valid parameter extras were found
	if requestExtras == nil {
		requestExtras = config.Extras
	}

	// Return the resolved extras (either from params or config)
	return requestExtras
}

// prepareRequest builds the request payload.
// This function constructs the JSON payload that will be sent to the Gotify API,
// combining the message content with configuration settings and any additional extras.
// Parameters:
//   - message: The main notification message text
//   - config: Configuration containing title, priority, and other settings
//   - extras: Additional key-value pairs to include in the notification
//
// Returns: *messageRequest containing all data to be sent to the API.
func (service *Service) prepareRequest(
	message string,
	config *Config,
	extras map[string]any,
) *messageRequest {
	return &messageRequest{
		Message:  message,         // The notification message content
		Title:    config.Title,    // Notification title from configuration
		Priority: config.Priority, // Priority level for the notification
		Extras:   extras,          // Additional metadata or custom fields
	}
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
	request *messageRequest,
	headers http.Header,
) error {
	// Prepare response structure to capture API response
	response := &messageResponse{}

	var err error
	if len(headers) > 0 {
		// Use HTTP client directly when custom headers are needed
		err = service.sendRequestWithHeaders(postURL, request, response, headers)
	} else {
		// Use JSON client for standard requests
		err = service.client.Post(postURL, request, response)
	}

	if err != nil {
		// Attempt to extract structured error from response
		errorRes := &responseError{}
		if service.client.ErrorResponse(err, errorRes) {
			return errorRes // Return structured API error
		}

		// Return generic error with context
		return fmt.Errorf("failed to send notification to Gotify: %w", err)
	}

	// Request completed successfully
	return nil
}

// sendRequestWithHeaders sends a request with custom headers using the underlying HTTP client.
// This method is used when per-request headers are needed, bypassing the jsonclient
// to avoid modifying shared header state.
// Parameters:
//   - postURL: The complete API endpoint URL to send the request to
//   - request: The JSON payload to send in the request body
//   - response: The response structure to unmarshal into
//   - headers: Custom headers to set on the request
//
// Returns: error if the request fails or server returns an error, nil on success.
func (service *Service) sendRequestWithHeaders(
	postURL string,
	request *messageRequest,
	response *messageResponse,
	headers http.Header,
) error {
	// Marshal the request to JSON
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshaling request to JSON: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		postURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating POST request for %q: %w", postURL, err)
	}

	// Set Content-Type header
	req.Header.Set("Content-Type", "application/json")

	// Set custom headers
	for key, values := range headers {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	// Execute the request
	res, err := service.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending POST request to %q: %w", postURL, err)
	}

	defer func() { _ = res.Body.Close() }()

	// Parse the response
	return parseResponse(res, response)
}

// parseResponse parses the HTTP response and unmarshals it into the provided object.
// This is a simplified version for internal use.
func parseResponse(res *http.Response, response any) error {
	body, err := io.ReadAll(res.Body)

	if res.StatusCode >= HTTPClientErrorThreshold {
		if err == nil {
			err = fmt.Errorf("%w: %v", ErrUnexpectedStatus, res.Status)
		}
	}

	if err == nil {
		err = json.Unmarshal(body, response)
	}

	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}
