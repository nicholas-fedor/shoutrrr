package pushover

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service provides the Pushover notification service.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	Client     *http.Client
	httpClient types.HTTPClient
}

// hookURL is the Pushover API endpoint for sending messages.
const (
	hookURL            = "https://api.pushover.net/1/messages.json"
	contentType        = "application/x-www-form-urlencoded"
	defaultHTTPTimeout = 10 * time.Second // defaultHTTPTimeout is the default timeout for HTTP requests.
)

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
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}

	s.pkr = format.NewPropKeyResolver(s.Config)
	if s.Client == nil {
		s.Client = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Pushover.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: the params, encryption key, or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to the configured Pushover devices
// in one request.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: the params, encryption key, or send error. A request error matches
//     ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if _, err := parseEncryptionKey(config.EncryptionKey); err != nil {
		return err
	}

	device := strings.Join(config.Devices, ",")
	if err := s.sendToDevice(ctx, device, message, config); err != nil {
		return fmt.Errorf("failed to send notifications to pushover devices: %w", err)
	}

	return nil
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

	s.httpClient = client
}

// httpClientOrDefault returns the custom client or the default Client.
func (s *Service) httpClientOrDefault() types.HTTPClient {
	if s.httpClient != nil {
		return s.httpClient
	}

	return s.Client
}

// sendToDevice sends a notification to the given Pushover devices.
//
// Parameters:
//   - ctx: cancellation and deadline for the request, further bounded by [defaultHTTPTimeout].
//   - device: the comma-separated device names, or empty for all devices.
//   - message: the message to send.
//   - config: the configuration for this send.
//
// Returns:
//   - error: an encryption error, the request error, or [ErrSendFailed] for a
//     non-success status.
func (s *Service) sendToDevice(ctx context.Context, device, message string, config *Config) error {
	key, err := parseEncryptionKey(config.EncryptionKey)
	if err != nil {
		return err
	}

	if key != nil {
		message, err = encryptField(message, key)
		if err != nil {
			return err
		}
	}

	data := url.Values{}
	data.Set("device", device)
	data.Set("user", config.User)
	data.Set("token", config.Token)
	data.Set("message", message)

	if config.Title != "" {
		title := config.Title
		if key != nil {
			title, err = encryptField(title, key)
			if err != nil {
				return err
			}
		}

		data.Set("title", title)
	}

	if key != nil {
		data.Set("encrypted", "1")
	}

	if config.Priority >= -2 && config.Priority <= 1 {
		data.Set("priority", strconv.FormatInt(int64(config.Priority), 10))
	}

	ctx, cancel := context.WithTimeout(ctx, defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		hookURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	client := s.httpClientOrDefault()

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request to Pushover API: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %q, response status %q", ErrSendFailed, device, res.Status)
	}

	return nil
}
