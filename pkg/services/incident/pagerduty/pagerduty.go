package pagerduty

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	eventEndpointTemplate = "https://%s:%d/v2/enqueue"
	defaultHTTPTimeout    = 30 * time.Second // defaultHTTPTimeout is the default timeout for HTTP requests.
	maxMessageLength      = 1024             // maxMessageLength is the maximum permitted length of the summary property.
)

// errPagerDutyNotificationFailed is returned when PagerDuty returns a non-2xx status code.
var errPagerDutyNotificationFailed = errors.New("PagerDuty notification failed")

// Service provides PagerDuty as a notification service.
type Service struct {
	standard.Standard
	Config     *Config
	pkr        format.PropKeyResolver
	httpClient *http.Client
}

// SetHTTPClient allows users to provide a custom HTTP client for enterprise environments
// requiring proxies, custom TLS configurations, etc.
func (service *Service) SetHTTPClient(client *http.Client) {
	service.httpClient = client
}

// sendAlert sends an alert payload to the specified PagerDuty endpoint URL.
func (service *Service) sendAlert(ctx context.Context, url string, payload EventPayload) error {
	// Marshal the payload into JSON format
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	jsonBuffer := bytes.NewBuffer(jsonBody)

	// Create a new HTTP POST request with the JSON body
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, jsonBuffer)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set the Content-Type header to application/json
	req.Header.Add("Content-Type", "application/json")

	// Use the custom HTTP client if provided, otherwise use the default
	client := service.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	// Send the HTTP request to PagerDuty
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification to PagerDuty: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status indicates success (2xx)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: %d", errPagerDutyNotificationFailed, resp.StatusCode)
	}

	return nil
}

// Initialize loads ServiceConfig from configURL and sets logger for this Service.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	if err := service.setDefaults(); err != nil {
		return err
	}

	if err := service.Config.setURL(&service.pkr, configURL); err != nil {
		return err
	}

	// Initialize HTTP client with timeout
	service.httpClient = &http.Client{
		Timeout: defaultHTTPTimeout,
	}

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// Send a notification message to PagerDuty
// See: https://developer.pagerduty.com/docs/events-api-v2-overview
func (service *Service) Send(message string, params *types.Params) error {
	return service.SendWithContext(context.Background(), message, params)
}

// SendWithContext sends a notification message to PagerDuty with context support
// See: https://developer.pagerduty.com/docs/events-api-v2-overview
func (service *Service) SendWithContext(
	ctx context.Context,
	message string,
	params *types.Params,
) error {
	config := service.Config
	endpointURL := fmt.Sprintf(eventEndpointTemplate, config.Host, config.Port)

	payload, err := service.newEventPayload(message, params)
	if err != nil {
		return err
	}

	return service.sendAlert(ctx, endpointURL, payload)
}

func (service *Service) newEventPayload(
	message string,
	params *types.Params,
) (EventPayload, error) {
	if params == nil {
		params = &types.Params{}
	}

	// Defensive copy
	payloadFields := *service.Config

	if err := service.pkr.UpdateConfigFromParams(&payloadFields, params); err != nil {
		return EventPayload{}, fmt.Errorf("failed to update config from params: %w", err)
	}

	// The maximum permitted length of this property is 1024 characters.
	runes := []rune(message)
	if len(runes) > maxMessageLength {
		message = string(runes[:maxMessageLength])
	}

	result := EventPayload{
		Payload: Payload{
			Summary:  message,
			Severity: payloadFields.Severity,
			Source:   payloadFields.Source,
		},
		RoutingKey:  payloadFields.IntegrationKey,
		EventAction: payloadFields.Action,
	}

	// Add optional fields if provided
	if payloadFields.Details != "" {
		var details any
		if err := json.Unmarshal([]byte(payloadFields.Details), &details); err != nil {
			return EventPayload{}, fmt.Errorf("failed to unmarshal details %q: %w", payloadFields.Details, err)
		}

		result.Details = details
	}

	if payloadFields.Client != "" {
		result.Client = payloadFields.Client
	}

	if payloadFields.ClientURL != "" {
		result.ClientURL = payloadFields.ClientURL
	}

	if payloadFields.Contexts != "" {
		contexts, err := parseContexts(payloadFields.Contexts)
		if err != nil {
			return EventPayload{}, fmt.Errorf("failed to parse contexts: %w", err)
		}

		result.Contexts = contexts
	}

	return result, nil
}

func (service *Service) setDefaults() error {
	if err := service.pkr.SetDefaultProps(service.Config); err != nil {
		return fmt.Errorf("failed to set default props: %w", err)
	}

	return nil
}

// parseContexts parses contexts from either a JSON array format or legacy comma-separated string format.
// It first attempts to unmarshal the input as a JSON array of PagerDutyContext objects.
// If JSON unmarshaling fails, it falls back to parsing the legacy string format like "type:src,type2:src2".
// Legacy format supports:
// - "link:http://example.com" -> {Type: "link", Href: "http://example.com"}
// - "image:http://example.com/img.png" -> {Type: "image", Src: "http://example.com/img.png"}
// - "text:Some description" -> {Type: "text", Text: "Some description"}.
func parseContexts(contextsStr string) ([]PagerDutyContext, error) {
	if contextsStr == "" {
		return nil, nil
	}

	// First, attempt to parse as JSON array
	var result []PagerDutyContext //nolint:prealloc // length is unknown for JSON case
	if err := json.Unmarshal([]byte(contextsStr), &result); err == nil {
		return result, nil
	}

	// Fall back to legacy comma-separated parsing
	// Split the input string by commas to get individual context entries
	contexts := strings.Split(contextsStr, ",")
	result = make([]PagerDutyContext, 0, len(contexts))

	for _, ctx := range contexts {
		// Trim whitespace from each context entry
		ctx = strings.TrimSpace(ctx)
		if ctx == "" {
			continue
		}

		const expectedParts = 2

		// Split each context by colon to separate type and value, limiting to 2 parts
		parts := strings.SplitN(ctx, ":", expectedParts)
		if len(parts) != expectedParts {
			return nil, fmt.Errorf("%w: %q", errInvalidContextFormat, ctx)
		}

		// Trim whitespace from type and value parts
		contextType := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Validate that neither type nor value is empty after trimming
		if contextType == "" || value == "" {
			return nil, fmt.Errorf("%w: %q", errEmptyContextTypeOrValue, ctx)
		}

		var context PagerDutyContext

		// Map context types to appropriate PagerDutyContext fields
		switch contextType {
		case "link":
			// Create a link context with href
			context = PagerDutyContext{Type: "link", Href: value}
		case "image":
			// Create an image context with src
			context = PagerDutyContext{Type: "image", Src: value}
		default:
			// For any other type, treat as text context
			context = PagerDutyContext{Type: contextType, Text: value}
		}

		// Add the parsed context to the result slice
		result = append(result, context)
	}

	return result, nil
}
