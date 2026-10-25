package rocketchat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util"
)

// Service sends notifications to a pre-configured Rocket.Chat channel or user.
// It implements the types.Service interface for Rocket.Chat integration.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// Config holds the Rocket.Chat service configuration.
	Config *Config
	// Client sends the requests when set. It stays nil after Initialize until
	// the caller sets it, and the sends use the default client meanwhile.
	// [Service.SetHTTPClient] clears it, so whichever of the two the caller
	// set last carries the sends.
	//
	// Deprecated: Use [Service.SetHTTPClient].
	Client *http.Client
	// client holds the client from [Service.SetHTTPClient] and the default
	// client.
	client httpclient.Holder
}

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the service identifier.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger. It keeps an HTTP
// client that is already set.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrNotEnoughArguments].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{}

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
//   - error: [types.ErrServiceNotInitialized] before Initialize, the payload or
//     request error, or an error for a non-success status.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Rocket.Chat. A context
// without a deadline is bounded by the budget that [Service.ServiceTimeout]
// reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional username and channel overrides.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the payload or
//     request error, or an error for a non-success status. A request error
//     matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	var res *http.Response

	var err error

	config := s.Config
	serviceURL := buildURL(config)

	json, err := CreateJSONPayload(config, message, params)
	if err != nil {
		return fmt.Errorf("creating JSON payload: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serviceURL,
		bytes.NewReader(json),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/json")

	res, err = s.client.Prefer(s.Client).Do(req)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", util.JoinURLHost(config.Host, config.Port), redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		resBody, err := httpbody.Read(res.Body, httpbody.MaxSize)
		if err != nil && !errors.Is(err, httpbody.ErrTooLarge) {
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

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client. It clears the
// deprecated Client field, so the sends use this client until that field is
// set again.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
	s.Client = nil
}

// buildURL constructs the API URL for Rocket.Chat based on the Config.
//
// Parameters:
//   - config: The configuration containing host, port, and token information
//
// Returns:
//   - The complete Rocket.Chat webhook URL as a string
func buildURL(config *Config) string {
	// The URL type escapes an IPv6 zone in the host, and the tokens are kept as
	// written.
	base := (&url.URL{Scheme: "https", Host: util.JoinURLHost(config.Host, config.Port)}).String()

	return fmt.Sprintf(
		"%s/hooks/%s/%s",
		base,
		config.TokenA,
		config.TokenB,
	)
}
