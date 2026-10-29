package pagerduty

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Service provides PagerDuty as a notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder

	// Config holds the integration key, the host, and the event fields.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Settings and limits of the Events API v2.
const (
	// eventEndpointPath is the Events API v2 endpoint path for sending events.
	eventEndpointPath = "/v2/enqueue"
	// defaultHTTPTimeout is the send budget of PagerDuty.
	defaultHTTPTimeout = 30 * time.Second
	// maxMessageLength is the maximum permitted length of the summary property.
	maxMessageLength = 1024

	// contextTypeLink is the context type of a link.
	contextTypeLink = "link"
	// contextTypeImage is the context type of an image.
	contextTypeImage = "image"
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// levelSeverities maps each known message level to the PagerDuty severity it
// is sent with when the send keeps the default severity. Critical is left for
// events configured as critical.
var levelSeverities = map[types.MessageLevel]string{
	types.Debug:   "info",
	types.Info:    "info",
	types.Warning: "warning",
	types.Error:   "error",
}

// GetID returns the service identifier.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize loads ServiceConfig from serviceURL and sets logger for this Service.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from applying the default props or reading the URL.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.setDefaults(); err != nil {
		return err
	}

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send delivers a notification message to PagerDuty without a deadline of its own.
// See https://developer.pagerduty.com/docs/events-api-v2-overview.
//
// Parameters:
//   - message: the event summary.
//   - params: per-send parameters for the event.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the payload or
//     request failure, or nil on success.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to PagerDuty. The router calls it
// with its send deadline, and a context without a deadline is bounded by the
// budget that [Service.ServiceTimeout] reports.
// See https://developer.pagerduty.com/docs/events-api-v2-overview.
//
// Parameters:
//   - ctx: bounds the event request.
//   - message: the event summary.
//   - params: per-send parameters for the event.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the payload or
//     request failure, or nil on success.
func (s *Service) SendContext(
	ctx context.Context,
	message string,
	params *types.Params,
) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	config := s.Config
	endpointURL := (&url.URL{
		Scheme: "https",
		Host:   util.JoinURLHost(config.Host, strconv.FormatUint(uint64(config.Port), 10)),
		Path:   eventEndpointPath,
	}).String()

	payload, err := s.newEventPayload(message, params)
	if err != nil {
		return err
	}

	return s.sendAlert(ctx, endpointURL, &payload)
}

// SendWithContext delivers a notification message with the provided context.
//
// Parameters:
//   - ctx: bounds the send.
//   - message: the message to send.
//   - params: per-send parameters.
//
// Returns:
//   - error: the send failure, or nil on success.
//
// Deprecated: Use [Service.SendContext], which the router calls with its send deadline.
//
//go:fix inline
func (s *Service) SendWithContext(ctx context.Context, message string, params *types.Params) error {
	return s.SendContext(ctx, message, params)
}

// ServiceTimeout reports the send budget of PagerDuty.
//
// Returns:
//   - time.Duration: [defaultHTTPTimeout], whatever the params.
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return defaultHTTPTimeout
}

// SetHTTPClient sets a custom HTTP client for the service, such as one for an
// enterprise environment that needs a proxy or a custom TLS configuration. A nil
// client, including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// applyLevel sets the severity of an event from its level param, debug and
// info to info, warning to warning, and error to error. A severity set in the
// service URL or the send params takes precedence.
//
// Parameters:
//   - config: the config for this send, with params already applied.
//   - params: the send params, which are never nil here.
func (s *Service) applyLevel(config *Config, params *types.Params) {
	level, ok := params.Level()
	if !ok {
		return
	}

	if severity, mapped := levelSeverities[level]; mapped && s.pkr.KeepsDefault(config, params, "severity") {
		config.Severity = severity
	}
}

// newEventPayload builds an Events API v2 event from the message, the config,
// and the send params, which apply to this event only.
//
// The summary is cut to 1024 characters. The details prop becomes the custom
// details, and the contexts prop becomes the links and images.
//
// Parameters:
//   - message: the event summary.
//   - params: the send params, or nil for none.
//
// Returns:
//   - EventPayload: the event.
//   - error: a params error, an invalid severity or action, details that are not
//     a JSON object, or invalid contexts.
func (s *Service) newEventPayload(
	message string,
	params *types.Params,
) (EventPayload, error) {
	if params == nil {
		params = &types.Params{}
	}

	// Defensive copy
	payloadFields := *s.Config

	if err := s.pkr.UpdateConfigFromParams(&payloadFields, params); err != nil {
		return EventPayload{}, fmt.Errorf("failed to update config from params: %w", err)
	}

	s.applyLevel(&payloadFields, params)

	// Validate severity
	if err := validateSeverity(payloadFields.Severity); err != nil {
		return EventPayload{}, err
	}

	// Validate event action
	if err := validateEventAction(payloadFields.Action); err != nil {
		return EventPayload{}, err
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

	// Add optional dedup_key if provided
	if payloadFields.DedupKey != "" {
		result.DedupKey = payloadFields.DedupKey
	}

	// Custom details must be a JSON object, so arrays, scalars, and null are rejected.
	if payloadFields.Details != "" {
		var details map[string]any
		if err := json.Unmarshal([]byte(payloadFields.Details), &details, jsonopts.UserInput); err != nil || details == nil {
			return EventPayload{}, fmt.Errorf("%w: %q", errInvalidDetails, payloadFields.Details)
		}

		result.Payload.CustomDetails = details
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

		result.Links, result.Images = linksAndImages(contexts)
	}

	return result, nil
}

// sendAlert sends an alert payload to the specified PagerDuty endpoint URL.
//
// Parameters:
//   - ctx: cancellation for the request.
//   - endpoint: the PagerDuty events URL.
//   - payload: the event to send.
//
// Returns:
//   - error: [errPagerDutyNotificationFailed] for a status outside 2xx, with the
//     message from up to [httpbody.MaxSize] bytes of the body, or the
//     encoding or request error.
func (s *Service) sendAlert(ctx context.Context, endpoint string, payload *EventPayload) error {
	// Marshal the payload into JSON format
	jsonBody, err := json.Marshal(payload, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	jsonBuffer := bytes.NewBuffer(jsonBody)

	// Create a new HTTP POST request with the JSON body
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		jsonBuffer,
	)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", redact.URLError(err))
	}

	// Set the Content-Type header to application/json
	req.Header.Add("Content-Type", "application/json")

	// Send the HTTP request to PagerDuty
	resp, err := s.client.Client().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification to PagerDuty: %w", redact.URLError(err))
	}

	defer func() { _ = resp.Body.Close() }()

	// Check if the response status indicates success (2xx)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Parse error response body for better error reporting
		errorMsg := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if resp.Body != nil {
			bodyBytes, err := httpbody.Read(resp.Body, httpbody.MaxSize)
			if (err == nil || errors.Is(err, httpbody.ErrTooLarge)) && len(bodyBytes) > 0 {
				// Try to parse as PagerDuty error response
				var errorResponse struct {
					Status  string   `json:"status"`
					Message string   `json:"message"`
					Error   string   `json:"error"`
					Errors  []string `json:"errors"`
				}
				if jsonErr := json.Unmarshal(bodyBytes, &errorResponse, jsonopts.Default); jsonErr == nil {
					switch {
					case errorResponse.Message != "":
						errorMsg = errorResponse.Message
					case errorResponse.Error != "":
						errorMsg = errorResponse.Error
					case len(errorResponse.Errors) > 0:
						errorMsg = strings.Join(errorResponse.Errors, "; ")
					}
				} else {
					// Fallback to raw body if JSON parsing fails
					errorMsg = string(bodyBytes)
				}
			}
		}

		return fmt.Errorf("%w: %s", errPagerDutyNotificationFailed, errorMsg)
	}

	return nil
}

// setDefaults applies the default props to the service config.
//
// Returns:
//   - error: the error from applying a default prop.
func (s *Service) setDefaults() error {
	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("failed to set default props: %w", err)
	}

	return nil
}

// parseContexts parses contexts from either a JSON array format or legacy comma-separated string format.
// It first attempts to unmarshal the input as a JSON array of PagerDutyContext objects.
// If JSON unmarshaling fails, it falls back to parsing the legacy string format like "type:src,type2:src2".
// Legacy format supports:
// - "link:http://example.com" -> {Type: "link", Href: "http://example.com"}
// - "image:http://example.com/img.png" -> {Type: "image", Src: "http://example.com/img.png"}.
//
// Parameters:
//   - contextsStr: the contexts, as a JSON array or in the legacy format.
//
// Returns:
//   - []PagerDutyContext: the contexts, or nil for an empty string.
//   - error: a format error, such as [errInvalidContextType] for a JSON entry
//     or [errInvalidContextFormat] for a legacy entry.
func parseContexts(contextsStr string) ([]PagerDutyContext, error) {
	if contextsStr == "" {
		return nil, nil
	}

	// First, attempt to parse as JSON array
	var result []PagerDutyContext
	if err := json.Unmarshal([]byte(contextsStr), &result, jsonopts.UserInput); err == nil {
		// Validate Type field and required fields for each context in JSON format
		for _, ctx := range result {
			if ctx.Type != contextTypeLink && ctx.Type != contextTypeImage {
				return nil, fmt.Errorf("%w: found %q", errInvalidContextType, ctx.Type)
			}

			if ctx.Type == contextTypeLink && ctx.Href == "" {
				return nil, fmt.Errorf("%w: %+v", errMissingHrefForLinkContext, ctx)
			}

			if ctx.Type == contextTypeImage && ctx.Src == "" {
				return nil, fmt.Errorf("%w: %+v", errMissingSrcForImageContext, ctx)
			}
		}

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

		var ctxVar PagerDutyContext

		// Map context types to appropriate PagerDutyContext fields
		switch contextType {
		case "link":
			// Create a link context with href
			//nolint:exhaustruct_v5 // link type only needs Type+Href
			ctxVar = PagerDutyContext{Type: "link", Href: value}
		case "image":
			// Create an image context with src
			//nolint:exhaustruct_v5 // image type only needs Type+Src
			ctxVar = PagerDutyContext{Type: "image", Src: value}
		case "text":
			// Skip text contexts
			continue
		default:
			return nil, fmt.Errorf(
				"%w: unsupported context type %q",
				errInvalidContextFormat,
				contextType,
			)
		}

		// Add the parsed context to the result slice
		result = append(result, ctxVar)
	}

	return result, nil
}

// linksAndImages converts parsed contexts into the links and images of an
// Events API v2 event. A context's text becomes the link text or the image's
// alternative text.
//
// Parameters:
//   - contexts: The parsed link and image contexts.
//
// Returns:
//   - links: The link contexts as links, or nil when there are none.
//   - images: The image contexts as images, or nil when there are none.
func linksAndImages(contexts []PagerDutyContext) ([]Link, []Image) {
	var (
		links  []Link
		images []Image
	)

	for _, ctx := range contexts {
		switch ctx.Type {
		case contextTypeLink:
			links = append(links, Link{Href: ctx.Href, Text: ctx.Text})
		case contextTypeImage:
			images = append(images, Image{Src: ctx.Src, Href: ctx.Href, Alt: ctx.Text})
		default:
		}
	}

	return links, images
}

// validateSeverity checks if the provided severity is one of the allowed values.
//
// Parameters:
//   - severity: the severity to check.
//
// Returns:
//   - error: [errInvalidSeverity] for a value other than critical, error,
//     warning, or info.
func validateSeverity(severity string) error {
	validSeverities := map[string]bool{
		"critical": true,
		"error":    true,
		"warning":  true,
		"info":     true,
	}

	if !validSeverities[severity] {
		return errInvalidSeverity
	}

	return nil
}

// validateEventAction checks if the provided event action is one of the allowed values.
//
// Parameters:
//   - action: the event action to check.
//
// Returns:
//   - error: [errInvalidEventAction] for a value other than trigger,
//     acknowledge, or resolve.
func validateEventAction(action string) error {
	validActions := map[string]bool{
		"trigger":     true,
		"acknowledge": true,
		"resolve":     true,
	}

	if !validActions[action] {
		return errInvalidEventAction
	}

	return nil
}
