package generic

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service implements a generic notification service for custom webhooks.
type Service struct {
	// Standard provides logging, templates, and the other shared service
	// functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// Config holds the webhook URL, the custom headers and extra data, and
	// the payload options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Payload template names and send settings.
const (
	// JSONTemplate identifies the JSON format for webhook payloads.
	JSONTemplate = "JSON"
)

// Errors returned by the generic webhook service.
var (
	// ErrSendFailed indicates a failure to send a notification to the generic webhook.
	ErrSendFailed = errors.New("failed to send notification to generic webhook")
	// ErrUnexpectedStatus indicates that the webhook answered with an error status, 400 or above.
	ErrUnexpectedStatus = errors.New("server returned unexpected response status code")
	// ErrTemplateNotLoaded indicates that the configured template is not loaded.
	ErrTemplateNotLoaded = errors.New("template has not been loaded")
	// ErrHostMissing indicates that the service URL has no webhook host.
	ErrHostMissing = errors.New("host missing from service URL")
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
	_ types.CustomURLService = (*Service)(nil)
)

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// GetPayload prepares the request payload based on the configured template.
//
// Without a template the payload is the message. The JSON template encodes the
// params and the extra data as a JSON object. Any other template name is a
// template loaded into the service, executed with the params.
//
// Parameters:
//   - config: the configuration for this send, with the template name.
//   - params: the send params, with the message under the message key.
//
// Returns:
//   - io.Reader: the payload.
//   - error: an encoding error, [ErrTemplateNotLoaded], or the template's
//     error.
func (s *Service) GetPayload(config *Config, params types.Params) (io.Reader, error) {
	switch config.Template {
	case "":
		// No template, send message directly
		return bytes.NewBufferString(params[config.MessageKey]), nil
	case "json", JSONTemplate:
		// JSON template, marshal params to JSON
		// Add extra data to params
		maps.Copy(params, config.extraData)

		// Marshal to JSON
		jsonBytes, err := json.Marshal(params, jsonopts.Default)
		if err != nil {
			return nil, fmt.Errorf("marshaling params to JSON: %w", err)
		}

		return bytes.NewBuffer(jsonBytes), nil
	}

	// Get the template
	tpl, found := s.GetTemplate(config.Template)
	if !found {
		return nil, fmt.Errorf("%w: %q", ErrTemplateNotLoaded, config.Template)
	}

	// Buffer for template execution
	bb := &bytes.Buffer{}
	if err := tpl.Execute(bb, params); err != nil {
		return nil, fmt.Errorf("executing template %q: %w", config.Template, err)
	}

	return bb, nil
}

// GetServiceURLFromCustom converts a custom webhook URL into a standard service URL.
//
// Parameters:
//   - customURL: the webhook URL, with or without the generic+ scheme prefix.
//
// Returns:
//   - *url.URL: the service URL.
//   - error: the error from reading the webhook URL's query.
func (*Service) GetServiceURLFromCustom(customURL *url.URL) (*url.URL, error) {
	// Copy the URL to modify
	webhookURL := *customURL
	if strings.HasPrefix(webhookURL.Scheme, Scheme) {
		// Remove the scheme prefix if present
		webhookURL.Scheme = webhookURL.Scheme[len(Scheme)+1:]
	}

	// Parse config from webhook URL
	config, pkr, err := ConfigFromWebhookURL(&webhookURL)
	if err != nil {
		return nil, err
	}

	// Generate and return the service URL
	return config.getURL(&pkr), nil
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrHostMissing].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	// Set the logger for the service
	s.SetLogger(logger)
	s.warnings.Reset()

	// Get default config and property key resolver
	config, pkr := DefaultConfig()
	// Assign config to service
	s.Config = config
	// Assign resolver
	s.pkr = pkr

	// Set URL and return any error
	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send delivers a notification message to a generic webhook endpoint.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - paramsPtr: optional send params, which fill the payload template and may
//     override the configuration fields their keys name.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     for an invalid config value in params, or [ErrSendFailed] wrapping the
//     payload or request error.
func (s *Service) Send(message string, paramsPtr *types.Params) error {
	return s.SendContext(context.Background(), message, paramsPtr)
}

// SendContext delivers a notification message to a generic webhook endpoint.
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports. The first send without TLS logs a warning,
// and so does the first send that skips certificate verification through the
// default client.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - paramsPtr: optional send params, which fill the payload template and may
//     override the configuration fields their keys name.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     for an invalid config value in params, or [ErrSendFailed] wrapping the
//     payload or request error, which matches ctx's error when ctx ends the
//     request.
func (s *Service) SendContext(ctx context.Context, message string, paramsPtr *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(paramsPtr))
	defer cancel()

	// Create a copy of the config to avoid modifying the original
	config := *s.Config

	var params types.Params
	if paramsPtr == nil {
		// Handle nil params by creating empty map
		params = types.Params{}
	} else {
		params = *paramsPtr
	}

	// Only config keys update the config. The other params fill the payload.
	if err := s.pkr.UpdateConfigFromKnownParams(&config, paramsPtr); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := validateWebhook(config.webhookURL); err != nil {
		return err
	}

	// Plain HTTP applies to an injected client too, because the URL decides it.
	if config.DisableTLS {
		s.warnings.Plaintext(ctx, s)
	}

	// Prepare parameters for sending
	sendParams := createSendParams(&config, params, message)
	if err := s.doSend(ctx, &config, sendParams); err != nil {
		// Execute the HTTP request to send the notification
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// doSend executes the HTTP request to send a notification to the webhook.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - params: the send params, which fill the payload template.
//
// Returns:
//   - error: the payload or request error, or [ErrUnexpectedStatus] for an error status.
func (s *Service) doSend(ctx context.Context, config *Config, params types.Params) error {
	// Get the webhook URL as string
	postURL := config.WebhookURL().String()

	// Prepare the request payload
	payload, err := s.GetPayload(config, params)
	if err != nil {
		return err
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, config.RequestMethod, postURL, payload)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	// Determine content type based on payload format
	contentType := config.ContentType
	if config.Template == "" {
		// When no template is specified, payload is plain text
		contentType = "text/plain"
	}

	// Set content type header
	req.Header.Set("Content-Type", contentType)
	// Set accept header
	req.Header.Set("Accept", contentType)

	for key, value := range config.headers {
		// Add custom headers
		req.Header.Set(key, value)
	}

	// Send the HTTP request
	res, err := s.sendClient(ctx, config).Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", redact.URLError(err))
	}

	if res != nil && res.Body != nil {
		defer func() {
			_ = res.Body.Close()
		}()

		// The body is not logged because it can echo the request, including its credentials.
		s.LogAttrs(ctx, slog.LevelDebug, "response received", slog.String(logging.KeyStatus, res.Status))
	}

	if res.StatusCode >= http.StatusBadRequest {
		// Check for error status codes (4xx and 5xx)
		return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
	}

	return nil
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
	if config.SkipTLSVerify {
		return s.client.InsecureClient(func() { s.warnings.Insecure(ctx, s) })
	}

	return s.client.Client()
}

// createSendParams constructs parameters for sending a notification.
//
// Parameters:
//   - config: the configuration with the title and message keys.
//   - params: the send params, whose title param moves to the title key.
//   - message: the message, set under the message key.
//
// Returns:
//   - types.Params: the params for the payload.
func createSendParams(config *Config, params types.Params, message string) types.Params {
	// Initialize new params map
	sendParams := types.Params{}

	for key, val := range params {
		// Copy params, mapping title key if necessary
		if key == types.TitleKey {
			key = config.TitleKey
		}

		sendParams[key] = val
	}

	// Add the message
	sendParams[config.MessageKey] = message

	return sendParams
}
