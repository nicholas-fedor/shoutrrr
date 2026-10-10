package join

import (
	"context"
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

// Service sends notifications to Join devices.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient types.HTTPClient
}

const (
	// hookURL defines the Join API endpoint for sending push notifications.
	hookURL        = "https://joinjoaomgcd.appspot.com/_ah/api/messaging/v1/sendPush"
	contentType    = "text/plain"
	defaultTimeout = 10 * time.Second
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Join devices.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional title and icon overrides.
//
// Returns:
//   - error: the send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Join devices in one request.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional title and icon overrides.
//
// Returns:
//   - error: the send error, which matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	config := s.Config

	if params == nil {
		params = &types.Params{}
	}

	title, found := (*params)["title"]
	if !found {
		title = config.Title
	}

	icon, found := (*params)["icon"]
	if !found {
		icon = config.Icon
	}

	devices := strings.Join(config.Devices, ",")

	return s.sendToDevices(ctx, devices, message, title, icon)
}

// SetHTTPClient sets a custom HTTP client for the service.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.httpClient = client
}

// sendToDevices sends the message to the given devices through the Join API.
//
// Parameters:
//   - ctx: cancellation and deadline for the request, further bounded by [defaultTimeout].
//   - devices: the comma-separated device IDs.
//   - message: the message to send.
//   - title: the notification title, or empty for none.
//   - icon: the notification icon URL.
//
// Returns:
//   - error: the request error, or [ErrSendFailed] for a non-success status.
func (s *Service) sendToDevices(ctx context.Context, devices, message, title, icon string) error {
	config := s.Config

	apiURL, err := url.Parse(hookURL)
	if err != nil {
		return fmt.Errorf("parsing Join API URL: %w", err)
	}

	data := url.Values{}
	data.Set("deviceIds", devices)
	data.Set("apikey", config.APIKey)
	data.Set("text", message)

	if title != "" {
		data.Set("title", title)
	}

	if title != "" {
		data.Set("icon", icon)
	}

	apiURL.RawQuery = data.Encode()

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL.String(),
		http.NoBody,
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request to Join: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"%w: %q, response status %q",
			ErrSendFailed,
			devices,
			res.Status,
		)
	}

	return nil
}
