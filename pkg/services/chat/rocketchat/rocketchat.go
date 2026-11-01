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
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
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
	// warnings logs each warning about the service's settings once.
	warnings logging.Once
}

// Send params that override URL fields. The config resolver skips them, and
// [CreateJSONPayload] reads them from the params.
const (
	// userNameParam overrides [Config.UserName] for one send.
	userNameParam = "username"
	// channelParam overrides [Config.Channel] for one send.
	channelParam = "channel"
)

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
	s.warnings.Reset()

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
//   - params: optional username and channel overrides and config keys, such
//     as disabletls and skiptlsverify.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, the payload or request error, or
//     an error for a non-success status.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Rocket.Chat. A context
// without a deadline is bounded by the budget that [Service.ServiceTimeout]
// reports. The first send without TLS logs a warning, and so does the first
// send that skips certificate verification through the default client.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional username and channel overrides and config keys, such
//     as disabletls and skiptlsverify. Any other key is an error.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, the payload or request error, or
//     an error for a non-success status. A request error matches ctx's error
//     when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	// Params apply to this send only, so they update a copy of the service
	// config.
	configCopy := *s.Config
	config := &configCopy

	resolver := format.NewPropKeyResolver(config)
	if err := resolver.UpdateConfigFromParams(config, params, userNameParam, channelParam); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.validate(); err != nil {
		return err
	}

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

	// Plain HTTP applies to an injected client too, because the URL decides it.
	if config.DisableTLS {
		s.warnings.Plaintext(ctx, s)
	}

	res, err := s.sendClient(ctx, config).Do(req)
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

// sendClient returns the client for a send. A send that skips certificate
// verification uses the insecure default client, and the first such send logs
// the shared warning. A client set through [Service.Client] or
// [Service.SetHTTPClient] is used unchanged and never warned about, because
// its owner controls its TLS settings.
//
// Parameters:
//   - ctx: the context of the send, passed to its warnings.
//   - config: the config for this send.
//
// Returns:
//   - types.HTTPClient: the client to send through.
func (s *Service) sendClient(ctx context.Context, config *Config) types.HTTPClient {
	if config.SkipTLSVerify {
		return s.client.PreferInsecure(s.Client, func() { s.warnings.Insecure(ctx, s) })
	}

	return s.client.Prefer(s.Client)
}

// buildURL constructs the API URL for Rocket.Chat based on the Config.
//
// Parameters:
//   - config: the configuration with the host, port, tokens, and TLS setting.
//
// Returns:
//   - The complete Rocket.Chat webhook URL as a string, over HTTP when TLS is
//     disabled and over HTTPS otherwise.
func buildURL(config *Config) string {
	scheme := "https"
	if config.DisableTLS {
		scheme = "http"
	}

	// The URL type escapes an IPv6 zone in the host. Each token is escaped as
	// one path segment, so a slash, question mark, or hash in it cannot change
	// the webhook path or start a query or fragment.
	base := (&url.URL{Scheme: scheme, Host: util.JoinURLHost(config.Host, config.Port)}).String()

	return base + "/hooks/" + url.PathEscape(config.TokenA) + "/" + url.PathEscape(config.TokenB)
}
