package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTPClient defines the interface for HTTP operations.
type HTTPClient = types.HTTPClient

// defaultHTTPClient implements HTTPClient using http.Client with a timeout.
type defaultHTTPClient struct {
	client types.HTTPClient
}

// Service sends notifications to Microsoft Teams via Power Automate workflow webhooks.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient HTTPClient
}

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 30 * time.Second

// adaptiveCardVersion is the Adaptive Card schema version used in payloads.
const adaptiveCardVersion = "1.2"

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// Do performs the HTTP request.
func (c *defaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", redact.URLError(err))
	}

	return resp, nil
}

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.httpClient = &defaultHTTPClient{
		client: &http.Client{Timeout: defaultHTTPTimeout},
	}

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	return s.Config.SetURL(serviceURL)
}

// Send delivers a notification message to Microsoft Teams.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [ErrMissingHost], a params or validation error, or the send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Microsoft Teams as an Adaptive Card.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [ErrMissingHost], a params or validation error, or the send error,
//     which matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return ErrMissingHost
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	return s.doSend(ctx, &config, message)
}

// ServiceTimeout returns the HTTP timeout used for a Teams send.
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return defaultHTTPTimeout
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	if client == nil {
		s.httpClient = &defaultHTTPClient{
			client: &http.Client{Timeout: defaultHTTPTimeout},
		}

		return
	}

	switch c := client.(type) {
	case *http.Client:
		s.httpClient = &defaultHTTPClient{client: c}
	case HTTPClient:
		s.httpClient = c
	}
}

// colorToEnum maps user-provided color values to valid Adaptive Card TextBlock.color enum values.
func colorToEnum(color string) string {
	switch strings.ToLower(color) {
	case "red", "attention", "danger":
		return "attention"
	case "orange", "yellow", "warning", "warn":
		return "warning"
	case "green", "good", "success":
		return "good"
	case "blue", "accent":
		return "accent"
	case "dark":
		return "dark"
	case "light":
		return "light"
	case "default", "":
		return "default"
	default:
		return "default"
	}
}

// doSend sends the notification to Teams as an Adaptive Card payload.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - message: the message to send.
//
// Returns:
//   - error: [ErrMissingHost], a validation error, [ErrSendFailed] wrapping the
//     request error, or [ErrSendFailedStatus] for a non-success status.
func (s *Service) doSend(ctx context.Context, config *Config, message string) error {
	if config.Host == "" {
		return ErrMissingHost
	}

	if err := ValidateWebhookURL(config.Host); err != nil {
		return err
	}

	lines := strings.Split(message, "\n")
	body := make([]adaptiveBlock, 0, len(lines)+1)

	if config.Title != "" {
		//nolint:exhaustruct_v5 // Color, Wrap are optional and set conditionally
		titleBlock := adaptiveBlock{
			Type:   "TextBlock",
			Text:   config.Title,
			Weight: "Bolder",
			Size:   "Medium",
			Color:  colorToEnum(config.Color),
		}

		body = append(body, titleBlock)
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		//nolint:exhaustruct_v5 // Color, Weight, Size are optional and default to zero values
		body = append(body, adaptiveBlock{
			Type: "TextBlock",
			Text: line,
			Wrap: true,
		})
	}

	payload := adaptivePayload{
		Type: "message",
		Attachments: []adaptiveAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				ContentURL:  nil,
				Content: adaptiveCardContent{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: adaptiveCardVersion,
					Body:    body,
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload to JSON: %w", err)
	}

	res, err := s.postJSON(ctx, config.Host, jsonBytes)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusAccepted {
		return fmt.Errorf("%w: %s", ErrSendFailedStatus, res.Status)
	}

	return nil
}

// postJSON performs an HTTP POST with a JSON payload.
//
// Parameters:
//   - ctx: cancellation and deadline for the request, further bounded by [defaultHTTPTimeout].
//   - serviceURL: the webhook URL.
//   - payload: the JSON payload.
//
// Returns:
//   - *http.Response: the response, whose body the caller closes.
//   - error: the request error.
func (s *Service) postJSON(ctx context.Context, serviceURL string, payload []byte) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(
		ctx,
		defaultHTTPTimeout,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serviceURL,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making HTTP POST request: %w", redact.URLError(err))
	}

	return res, nil
}
