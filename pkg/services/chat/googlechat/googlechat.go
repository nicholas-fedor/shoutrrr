package googlechat

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service implements a Google Chat notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// Config holds the webhook host, path, key, and token.
	Config *Config
	// client holds the injected HTTP client and the default client.
	client httpclient.Holder
}

// defaultHTTPTimeout bounds each request of the default client.
const defaultHTTPTimeout = 10 * time.Second

// ErrUnexpectedStatus indicates an unexpected HTTP status code from the Google Chat API.
var ErrUnexpectedStatus = errors.New("google chat api returned unexpected http status code")

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrMissingKey] or
//     [ErrMissingToken].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.client.SetDefault(defaultHTTPTimeout, nil)

	s.Config = &Config{}

	return s.Config.SetURL(serviceURL)
}

// Send delivers a notification message to Google Chat.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: unused, because Google Chat takes no send parameters.
//
// Returns:
//   - error: the request error, or [ErrUnexpectedStatus] for a non-success status.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Google Chat.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - _: unused, because Google Chat takes no send parameters.
//
// Returns:
//   - error: the request error, which matches ctx's error when ctx ends the
//     request, or [ErrUnexpectedStatus] for a non-success status.
func (s *Service) SendContext(ctx context.Context, message string, _ *types.Params) error {
	config := s.Config

	jsonBody, err := json.Marshal(JSON{Text: message}, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("marshaling message to JSON: %w", err)
	}

	postURL := getAPIURL(config)
	jsonBuffer := bytes.NewBuffer(jsonBody)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		postURL.String(),
		jsonBuffer,
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Client().Do(req)
	if err != nil {
		return fmt.Errorf("sending notification to Google Chat: %w", redact.URLError(err))
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// getAPIURL constructs the service URL for Google Chat notifications.
//
// Parameters:
//   - config: the configuration with the webhook host, path, key, and token.
//
// Returns:
//   - *url.URL: the https webhook URL, with the key and token in its query.
func getAPIURL(config *Config) *url.URL {
	query := url.Values{}
	query.Set("key", config.Key)
	query.Set("token", config.Token)

	return &url.URL{
		Path:     config.Path,
		Host:     config.Host,
		Scheme:   "https",
		RawQuery: query.Encode(),
	}
}
