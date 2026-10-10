package rocketchat

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to a pre-configured Rocket.Chat channel or user.
// It implements the types.Service interface for Rocket.Chat integration.
type Service struct {
	standard.Standard

	// Config holds the Rocket.Chat service configuration.
	Config     *Config
	Client     *http.Client
	httpClient types.HTTPClient
}

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 10 * time.Second

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Params:
//   - serviceURL: The configuration URL containing Rocket.Chat connection details
//   - logger: The logger to use for this service
//
// Returns:
//   - error: An error if configuration fails, nil otherwise
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{}
	if s.Client == nil {
		s.Client = &http.Client{
			Timeout: defaultHTTPTimeout, // Set a default timeout
		}
	}

	if err := s.Config.SetURL(serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Rocket.Chat.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional username and channel overrides.
//
// Returns:
//   - error: the payload or request error, or an error for a non-success status.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Rocket.Chat.
//
// Parameters:
//   - ctx: cancellation and deadline for the request, further bounded by [defaultHTTPTimeout].
//   - message: the message to send.
//   - params: optional username and channel overrides.
//
// Returns:
//   - error: the payload or request error, or an error for a non-success status.
//     A request error matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	var res *http.Response

	var err error

	config := s.Config
	serviceURL := buildURL(config)

	json, err := CreateJSONPayload(config, message, params)
	if err != nil {
		return fmt.Errorf("creating JSON payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		ctx,
		defaultHTTPTimeout,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serviceURL,
		bytes.NewReader(json),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err = s.httpClientOrDefault().Do(req)
	if err != nil {
		return fmt.Errorf(
			"posting to URL: %w\nHOST: %s\nPORT: %s",
			redact.URLError(err),
			config.Host,
			config.Port,
		)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf(
				"%w: %d",
				ErrNotificationFailed,
				res.StatusCode,
			)
		}

		return fmt.Errorf("%w: %d %s",
			ErrNotificationFailed,
			res.StatusCode,
			resBody,
		)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.httpClient = client
	if s.Client != nil {
		if c, ok := client.(*http.Client); ok {
			s.Client = c
		}
	}
}

// httpClientOrDefault returns the custom client or the internal Client.
func (s *Service) httpClientOrDefault() types.HTTPClient {
	if s.httpClient != nil {
		return s.httpClient
	}

	return s.Client
}

// buildURL constructs the API URL for Rocket.Chat based on the Config.
//
// Params:
//   - config: The configuration containing host, port, and token information
//
// Returns:
//   - The complete Rocket.Chat webhook URL as a string
func buildURL(config *Config) string {
	base := config.Host
	if config.Port != "" {
		base = net.JoinHostPort(config.Host, config.Port)
	}

	return fmt.Sprintf(
		"https://%s/hooks/%s/%s",
		base,
		config.TokenA,
		config.TokenB,
	)
}
