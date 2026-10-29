package gotify

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service implements a Gotify notification service that handles sending push notifications
// to Gotify servers. It manages HTTP client configuration, TLS settings, authentication,
// and payload construction for reliable message delivery.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default clients. A send
	// that skips certificate verification uses the insecure default client.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// urlBuilder builds the message endpoint URL. It and the builders below
	// are set by Initialize.
	urlBuilder URLBuilder
	// payloadBuilder builds the request body and parses the extras.
	payloadBuilder PayloadBuilder
	// validator checks the message, priority, date, and token.
	validator Validator
	// sender sends the request and checks the response.
	sender contextSender

	// Config holds the server, the token, and the message options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Gotify priorities that message levels map to, one from each band of the
// Gotify Android app.
const (
	// priorityIconOnly shows only an icon in the notification bar.
	priorityIconOnly = 2
	// prioritySound adds a sound to the icon.
	prioritySound = 5
	// prioritySoundAndVibration adds a sound and vibration to the icon.
	prioritySoundAndVibration = 8
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// levelPriorities maps message levels to the Gotify priority they are sent
// with when neither the URL nor the send params set the priority. Info has no
// entry, so the application's default priority applies.
var levelPriorities = map[types.MessageLevel]int{
	types.Debug:   priorityIconOnly,
	types.Warning: prioritySound,
	types.Error:   prioritySoundAndVibration,
}

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
// It parses the URL and warns once about each deprecated key the URL uses.
// It performs no network I/O.
//
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

	// The resolver parses the URL and applies the params of each send. It
	// records the deprecated keys the URL uses.
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.warnings.Reset()

	// Parse the configuration URL to extract host, token, path, and other settings
	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return fmt.Errorf("failed to set URL: %w", err)
	}

	s.warnings.DeprecatedKeys(context.Background(), s, s.pkr.DeprecatedKeys())

	// Inject default implementations for interfaces
	s.urlBuilder = &DefaultURLBuilder{}
	s.payloadBuilder = &DefaultPayloadBuilder{}
	s.validator = &DefaultValidator{}
	s.sender = &DefaultSender{}

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
//   - error: [types.ErrServiceNotInitialized] before Initialize, the
//     validation, config, or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Gotify.
//
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the notification message content to send, which cannot be empty.
//   - params: optional parameters that override configuration settings or provide extras.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the
//     validation, config, or send error. A request error matches ctx's error
//     when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	if err := s.validateInputs(message, params); err != nil {
		return fmt.Errorf("input validation failed: %w", err)
	}

	config, extras, err := s.processConfig(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to process config: %w", err)
	}

	postURL, request, headers, err := s.buildRequest(message, &config, extras)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	// Plain HTTP applies to an injected client too, because the URL decides it.
	if config.DisableTLS {
		s.warnings.Plaintext(ctx, s)
	}

	return s.sendRequest(ctx, s.sendClient(ctx, &config), postURL, request, headers)
}

// SetHTTPClient allows external injection of a custom HTTP client (for router propagation).
// A nil client, including a nil *http.Client, restores the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// buildRequest constructs the URL, payload, and headers for the HTTP request.
//
// Parameters:
//   - message: the message.
//   - config: the configuration for this send.
//   - extras: the extras to send.
//
// Returns:
//   - string: the message endpoint URL.
//   - *MessageRequest: the request body.
//   - http.Header: the token header for header authentication, or nil.
//   - error: [ErrInvalidToken], or the error from building the URL.
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

// processConfig handles configuration processing including parameter updates, validation, and extras parsing.
// A priority the params set is sent even when it is 0. A date is ignored, because Gotify sets it.
//
// Deprecated config fields move into their replacements before the params
// apply, so a param can still turn an option off. Each deprecated field, param
// key, and the date is warned about once.
//
// Parameters:
//   - ctx: the context of the send, passed to its warnings.
//   - params: the send params.
//
// Returns:
//   - Config: the config for this send.
//   - map[string]any: the extras for this send.
//   - error: a params or priority error.
func (s *Service) processConfig(ctx context.Context, params *types.Params) (Config, map[string]any, error) {
	// Get reference to current configuration
	config := *s.Config

	deprecatedFields, err := format.SyncDeprecatedFields(&config)
	if err != nil {
		return config, nil, fmt.Errorf("failed to sync deprecated config fields: %w", err)
	}

	s.warnings.DeprecatedFields(ctx, s, deprecatedFields)

	// Filter out 'extras' parameter as it's handled separately from other config updates
	filteredParams := filterParams(params)
	s.warnings.DeprecatedKeys(ctx, s, s.pkr.DeprecatedParams(&filteredParams))

	// Update configuration with filtered parameters (title, priority, etc.)
	if err := s.pkr.UpdateConfigFromParams(&config, &filteredParams); err != nil {
		return config, nil, fmt.Errorf("failed to update config from params: %w", err)
	}

	for key := range filteredParams {
		if strings.EqualFold(key, "priority") {
			config.prioritySet = true
		}
	}

	applyLevel(&config, params)

	// Validate priority is within valid range (-2 to 10)
	if err := s.validator.ValidatePriority(config.Priority); err != nil {
		return config, nil, fmt.Errorf("priority validation failed: %w", err)
	}

	if config.Date != "" {
		s.warnings.Do("date", func() {
			s.Log("Warning: ignoring the date param, because Gotify sets the date of each message")
		})
	}

	// Parse extras from parameters or fall back to config extras
	extras, err := s.payloadBuilder.ParseExtras(params, &config)
	if err != nil {
		s.Logf("Failed to parse extras from params: %v", err)

		extras = config.Extras
	}

	return config, extras, nil
}

// sendClient returns the client for a send. A send that skips certificate
// verification uses the insecure default client, and the first such send logs
// the shared warning. An injected client is used unchanged and never warned
// about, because its owner controls its TLS settings.
//
// Parameters:
//   - ctx: the context of the send, passed to its warnings.
//   - config: the config for this send.
//
// Returns:
//   - types.HTTPClient: the client to send through.
func (s *Service) sendClient(ctx context.Context, config *Config) types.HTTPClient {
	if !config.SkipTLSVerify {
		return s.client.Client()
	}

	return s.client.InsecureClient(func() { s.warnings.Insecure(ctx, s) })
}

// sendRequest executes the HTTP POST request to the Gotify API endpoint.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - client: the HTTP client to send through.
//   - postURL: the complete API endpoint URL to send the request to.
//   - request: the JSON payload to send in the request body.
//   - headers: optional headers to set on the request.
//
// Returns:
//   - error: [ErrSendFailed] wrapping the request error or the error the server reported.
func (s *Service) sendRequest(
	ctx context.Context,
	client types.HTTPClient,
	postURL string,
	request *MessageRequest,
	headers http.Header,
) error {
	if err := s.sender.SendRequestContext(
		ctx,
		client,
		postURL,
		request,
		headers,
	); err != nil {
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	return nil
}

// validateInputs performs initial validation checks for the Send method.
//
// Parameters:
//   - message: the message to send.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] when Initialize has not
//     succeeded, or a message validation error.
func (s *Service) validateInputs(message string, _ *types.Params) error {
	if s.validator == nil {
		return types.ErrServiceNotInitialized
	}

	if err := s.validator.ValidateMessage(message); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}

	if err := s.validator.ValidateServiceInitialized(s.Config); err != nil {
		return fmt.Errorf("service initialization validation failed: %w", err)
	}

	return nil
}

// filterParams filters out 'extras' parameters from the given params.
//
// Parameters:
//   - params: the send params, or nil.
//
// Returns:
//   - types.Params: a copy of params without extras.
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

// applyLevel sets the priority of a message from its level param when neither
// the service URL nor the send params set it. Debug uses 2, which shows only an
// icon, warning uses 5, which adds a sound, and error uses 8, which adds
// vibration. Info leaves the application's default priority.
//
// Parameters:
//   - config: the config for this send, with params already applied.
//   - params: the send params, or nil.
func applyLevel(config *Config, params *types.Params) {
	if params == nil || config.prioritySet {
		return
	}

	level, ok := params.Level()
	if !ok {
		return
	}

	if levelPriority, mapped := levelPriorities[level]; mapped {
		config.Priority = levelPriority
		config.prioritySet = true
	}
}
