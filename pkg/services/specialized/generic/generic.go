package generic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// JSONTemplate identifies the JSON format for webhook payloads.
const (
	JSONTemplate = "JSON"
)

// ErrSendFailed indicates a failure to send a notification to the generic webhook.
var (
	ErrSendFailed        = errors.New("failed to send notification to generic webhook")
	ErrUnexpectedStatus  = errors.New("server returned unexpected response status code")
	ErrTemplateNotLoaded = errors.New("template has not been loaded")
)

// Service implements a generic notification service for custom webhooks.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to a generic webhook endpoint.
func (service *Service) Send(message string, paramsPtr *types.Params) error {
	// Create a copy of the config to avoid modifying the original
	config := *service.Config

	var params types.Params
	if paramsPtr == nil {
		// Handle nil params by creating empty map
		params = types.Params{}
	} else {
		params = *paramsPtr
	}

	if err := service.pkr.UpdateConfigFromParams(&config, &params); err != nil {
		// Update config with runtime parameters
		service.Logf("Failed to update params: %v", err)
	}

	// Prepare parameters for sending
	sendParams := createSendParams(&config, params, message)
	if err := service.doSend(&config, sendParams); err != nil {
		// Execute the HTTP request to send the notification
		return fmt.Errorf("%w: %s", ErrSendFailed, err.Error())
	}

	return nil
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	// Set the logger for the service
	service.SetLogger(logger)

	// Get default config and property key resolver
	config, pkr := DefaultConfig()
	// Assign config to service
	service.Config = config
	// Assign resolver
	service.pkr = pkr

	// Set URL and return any error
	return service.Config.setURL(&service.pkr, configURL)
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// GetConfigURLFromCustom converts a custom webhook URL into a standard service URL.
func (*Service) GetConfigURLFromCustom(customURL *url.URL) (*url.URL, error) {
	// Copy the URL to modify
	webhookURL := *customURL
	if strings.HasPrefix(webhookURL.Scheme, Scheme) {
		// Remove the scheme prefix if present
		webhookURL.Scheme = webhookURL.Scheme[len(Scheme)+1:]
	}

	// Parse config from webhook URL
	config, pkr, err := ConfigFromWebhookURL(webhookURL)
	if err != nil {
		return nil, err
	}

	// Generate and return the service URL
	return config.getURL(&pkr), nil
}

// doSend executes the HTTP request to send a notification to the webhook.
func (service *Service) doSend(config *Config, params types.Params) error {
	// Get the webhook URL as string
	postURL := config.WebhookURL().String()

	// Prepare the request payload
	payload, err := service.GetPayload(config, params)
	if err != nil {
		return err
	}

	// Create background context for the request
	ctx := context.Background()

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, config.RequestMethod, postURL, payload)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
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
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", err)
	}

	if res != nil && res.Body != nil {
		// Read and log response body if available
		defer func() {
			_ = res.Body.Close()
		}()

		if body, err := io.ReadAll(res.Body); err == nil {
			service.Log("Server response: ", string(body))
		}
	}

	if res.StatusCode >= http.StatusBadRequest {
		// Check for error status codes (4xx and 5xx)
		return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
	}

	return nil
}

// GetPayload prepares the request payload based on the configured template.
func (service *Service) GetPayload(config *Config, params types.Params) (io.Reader, error) {
	switch config.Template {
	case "":
		// No template, send message directly
		return bytes.NewBufferString(params[config.MessageKey]), nil
	case "json", JSONTemplate:
		// JSON template, marshal params to JSON
		for key, value := range config.extraData {
			// Add extra data to params
			params[key] = value
		}

		// Marshal to JSON
		jsonBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params to JSON: %w", err)
		}

		return bytes.NewBuffer(jsonBytes), nil
	}

	// Get the template
	tpl, found := service.GetTemplate(config.Template)
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

// createSendParams constructs parameters for sending a notification.
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
