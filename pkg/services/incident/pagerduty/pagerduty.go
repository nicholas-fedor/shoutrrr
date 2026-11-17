package pagerduty

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	// maxDedupKeyLength is the maximum permitted length of the dedup key, in
	// characters.
	maxDedupKeyLength = 255
	// maxEventSize is the largest encoded event PagerDuty accepts, 512 KB.
	maxEventSize = 512 * 1024

	// actionTrigger is the event action that opens an alert.
	actionTrigger = "trigger"
	// actionAcknowledge is the event action that acknowledges the alert its
	// dedup key names.
	actionAcknowledge = "acknowledge"
	// actionResolve is the event action that resolves the alert its dedup key
	// names.
	actionResolve = "resolve"
	// detailsMessageKey is the custom details key that holds the message when
	// it is not the summary.
	detailsMessageKey = "message"

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

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send delivers a notification message to PagerDuty without a deadline of its own.
// See https://developer.pagerduty.com/docs/events-api-v2-overview.
//
// Parameters:
//   - message: the notification message, sent as the summary of a trigger event without a title.
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
//   - message: the notification message, sent as the summary of a trigger event without a title.
//   - params: per-send parameters for the event.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize. A params
//     error, the error from the config's checks, or an event that cannot be
//     built or is too large fails the send before any request. Otherwise the
//     request failure, or nil on success.
func (s *Service) SendContext(
	ctx context.Context,
	message string,
	params *types.Params,
) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	// Params apply to this send only, so they update a copy of the service
	// config.
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("failed to update config from params: %w", err)
	}

	s.applyLevel(&config, params)

	if err := config.prepare(); err != nil {
		return err
	}

	payload, err := newEventPayload(message, &config)
	if err != nil {
		return err
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	endpointURL := (&url.URL{
		Scheme: "https",
		Host:   util.JoinURLHostPort(config.Host, int(config.Port)),
		Path:   eventEndpointPath,
	}).String()

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
//   - params: the send params, or nil.
func (s *Service) applyLevel(config *Config, params *types.Params) {
	if params == nil {
		return
	}

	level, ok := params.Level()
	if !ok {
		return
	}

	if severity, mapped := levelSeverities[level]; mapped && s.pkr.KeepsDefault(config, params, "severity") {
		config.Severity = severity
	}
}

// newEventPayload builds an Events API v2 event from the message and the
// config of one send.
//
// An acknowledge or resolve event names only its alert, as PagerDuty's
// examples show, so it carries the routing key, the action, and the dedup key.
// A trigger event's summary is the title, or the message when there is no
// title, cut to 1024 characters. A message that is not the summary goes to the
// custom details as "message", unless the details prop has that key. The
// contexts prop becomes the links and images.
//
// Parameters:
//   - message: the notification message.
//   - payloadFields: the send's config, with its params applied and checked.
//
// Returns:
//   - EventPayload: the event.
//   - error: an error for a trigger event without a title or message, details
//     that are not a JSON object, or invalid contexts.
func newEventPayload(message string, payloadFields *Config) (EventPayload, error) {
	result := EventPayload{
		RoutingKey:  payloadFields.IntegrationKey,
		EventAction: payloadFields.Action,
		DedupKey:    payloadFields.DedupKey,
	}

	if payloadFields.Action != actionTrigger {
		return result, nil
	}

	summary := cmp.Or(payloadFields.Title, message)
	if summary == "" {
		return EventPayload{}, errMissingSummary
	}

	summary = cutSummary(summary)

	details, err := parseDetails(payloadFields.Details)
	if err != nil {
		return EventPayload{}, err
	}

	contexts, err := parseContexts(payloadFields.Contexts)
	if err != nil {
		return EventPayload{}, fmt.Errorf("parsing contexts: %w", err)
	}

	result.Payload = Payload{
		CustomDetails: customDetails(details, message, summary),
		Summary:       summary,
		Severity:      payloadFields.Severity,
		Source:        payloadFields.Source,
	}
	result.Client = payloadFields.Client
	result.ClientURL = payloadFields.ClientURL
	result.Links, result.Images = linksAndImages(contexts)

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
//   - error: an error for an encoded event over 512 KB, before any request. An
//     error for a status outside 2xx, with the message from up to
//     [httpbody.MaxSize] bytes of the body. Otherwise the encoding or request
//     error.
func (s *Service) sendAlert(ctx context.Context, endpoint string, payload *EventPayload) error {
	// Marshal the payload into JSON format
	jsonBody, err := json.Marshal(payload, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	if len(jsonBody) > maxEventSize {
		return fmt.Errorf("%w: %d bytes", errEventTooLarge, len(jsonBody))
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

	resp.Body = httpbody.Limit(resp.Body, resp.ContentLength)
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

// parseContexts parses contexts from either a JSON array format or legacy comma-separated string format.
// It first attempts to unmarshal the input as a JSON array of PagerDutyContext objects.
// If JSON unmarshaling fails, it falls back to parsing the legacy string format like "type:src,type2:src2".
// Legacy format supports:
// - "link:http://example.com" -> {Type: "link", Href: "http://example.com"}
// - "image:http://example.com/img.png" -> {Type: "image", Src: "http://example.com/img.png"}.
//
// Both formats take only link and image contexts.
//
// Parameters:
//   - contextsStr: the contexts, as a JSON array or in the legacy format.
//
// Returns:
//   - []PagerDutyContext: the contexts, or nil for an empty string.
//   - error: a format error, such as an unsupported context type or a legacy
//     entry without a colon.
func parseContexts(contextsStr string) ([]PagerDutyContext, error) {
	if contextsStr == "" {
		return nil, nil
	}

	// Input that is not a JSON array is read in the legacy comma-separated
	// format.
	var result []PagerDutyContext
	if err := json.Unmarshal([]byte(contextsStr), &result, jsonopts.UserInput); err != nil {
		result, err = parseLegacyContexts(contextsStr)
		if err != nil {
			return nil, err
		}
	}

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

// parseLegacyContexts reads contexts in the legacy comma-separated
// type:value format, skipping empty entries. An image entry's value is its
// source, and any other entry's value is its link, so the context checks
// treat the types of both formats alike.
//
// Parameters:
//   - contextsStr: the comma-separated type:value entries.
//
// Returns:
//   - []PagerDutyContext: the contexts, with their types as written.
//   - error: an error for an entry without a colon, or with an empty type or
//     value.
func parseLegacyContexts(contextsStr string) ([]PagerDutyContext, error) {
	contexts := strings.Split(contextsStr, ",")
	result := make([]PagerDutyContext, 0, len(contexts))

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

		//nolint:exhaustruct_v5 // A legacy entry has no text.
		ctxVar := PagerDutyContext{Type: contextType, Href: value}
		if contextType == contextTypeImage {
			//nolint:exhaustruct_v5 // A legacy entry has no text.
			ctxVar = PagerDutyContext{Type: contextType, Src: value}
		}

		result = append(result, ctxVar)
	}

	return result, nil
}

// parseDetails reads the details prop, a JSON object of custom details.
//
// Parameters:
//   - details: the details prop, or an empty string for none.
//
// Returns:
//   - map[string]any: the custom details, or nil for an empty prop.
//   - error: an error for details that are not a JSON object, such as an
//     array, a scalar, or null.
func parseDetails(details string) (map[string]any, error) {
	if details == "" {
		return nil, nil //nolint:nilnil // An empty prop means no custom details.
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(details), &result, jsonopts.UserInput); err != nil || result == nil {
		return nil, fmt.Errorf("%w: %q", errInvalidDetails, details)
	}

	return result, nil
}

// cutSummary cuts a summary to 1024 characters, PagerDuty's limit.
//
// Parameters:
//   - summary: the summary.
//
// Returns:
//   - string: the summary, or its first 1024 characters.
func cutSummary(summary string) string {
	// A summary of at most 1024 bytes holds at most 1024 characters.
	if len(summary) <= maxMessageLength {
		return summary
	}

	count := 0
	for i := range summary {
		if count == maxMessageLength {
			return summary[:i]
		}

		count++
	}

	return summary
}

// customDetails returns the custom details of a trigger event. A message
// that is not the summary goes to the details as "message", unless the
// details prop has that key.
//
// Parameters:
//   - details: the parsed details prop, or nil.
//   - message: the notification message.
//   - summary: the event summary, after cutting.
//
// Returns:
//   - any: the custom details, or nil when there are none.
func customDetails(details map[string]any, message, summary string) any {
	if message == "" || message == summary {
		if details == nil {
			return nil
		}

		return details
	}

	if details == nil {
		return messageDetails{Message: message}
	}

	if _, set := details[detailsMessageKey]; !set {
		details[detailsMessageKey] = message
	}

	return details
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
	switch severity {
	case "critical", "error", "warning", "info":
		return nil
	default:
		return errInvalidSeverity
	}
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
	switch action {
	case actionTrigger, actionAcknowledge, actionResolve:
		return nil
	default:
		return errInvalidEventAction
	}
}
