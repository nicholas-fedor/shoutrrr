package mattermost

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to a pre-configured Mattermost channel or user.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client for webhook
	// requests.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// Config holds the host, the webhook token, and the message options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
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

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the Mattermost service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error when the URL is invalid.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.warnings.Reset()
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send delivers a notification message to Mattermost.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, the payload or request error, or
//     [ErrSendFailed] for an error status.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Mattermost. A context without
// a deadline is bounded by the budget that [Service.ServiceTimeout] reports.
// The first send without TLS logs a warning, and so does the first send that
// skips certificate verification through the default client.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields, including the
//     username and channel params, disabletls, and skiptlsverify.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, the payload or request error, or
//     [ErrSendFailed] for an error status. A request error matches ctx's error
//     when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	if err := s.pkr.UpdateConfigFromParams(config, params, userNameParam, channelParam); err != nil {
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

	// Plain HTTP applies to an injected client too, because the URL decides it.
	if config.DisableTLS {
		s.warnings.Plaintext(ctx, s)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serviceURL.String(),
		bytes.NewReader(json),
	)
	if err != nil {
		return fmt.Errorf("creating POST request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := s.sendClient(ctx, config).Do(req)
	if err != nil {
		return fmt.Errorf("executing POST request to Mattermost API: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", ErrSendFailed, res.Status)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// sendClient returns the client for a send. A send that skips certificate
// verification uses the insecure default client, and the first such send logs
// the shared warning. An injected client is used unchanged and never warned
// about, because its owner controls its TLS settings.
//
// Parameters:
//   - ctx: the context of the send, passed to its warnings.
//   - config: the config for this send.
//
// Returns:
//   - types.HTTPClient: the client to send through.
func (s *Service) sendClient(ctx context.Context, config *Config) types.HTTPClient {
	if config.SkipTLSVerify {
		return s.client.InsecureClient(func() { s.warnings.Insecure(ctx, s) })
	}

	return s.client.Client()
}

// buildURL constructs the API URL for Mattermost based on the Config.
//
// Parameters:
//   - config: the configuration with the host, the webhook token, and the TLS
//     setting.
//
// Returns:
//   - *url.URL: the webhook URL, http when TLS is disabled and https
//     otherwise. It is valid by construction.
func buildURL(config *Config) *url.URL {
	scheme := "https"
	if config.DisableTLS {
		scheme = "http"
	}

	return &url.URL{
		Scheme: scheme,
		Host:   config.Host,
		Path:   "/hooks/" + config.Token,
	}
}
