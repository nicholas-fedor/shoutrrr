package mattermost

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/internal/transport"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to a pre-configured Mattermost channel or user.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// httpClient is the HTTP client for webhook requests, set to the default
	// client by Initialize.
	httpClient types.HTTPClient

	// Config holds the host, the webhook token, and the message options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Send params that override URL fields. The config resolver does not accept
// them as keys, so [CreateJSONPayload] reads them from the params.
const (
	// userNameParam overrides [Config.UserName] for one send.
	userNameParam = "username"
	// channelParam overrides [Config.Channel] for one send.
	channelParam = "channel"
)

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
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// It builds the default HTTP client, whose transport honors the proxy
// environment variables and enforces TLS 1.2 or later unless TLS is disabled.
//
// Parameters:
//   - serviceURL: the Mattermost service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error when the URL is invalid.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	err := s.Config.setURL(&s.pkr, serviceURL)
	if err != nil {
		return err
	}

	var tlsConfig *tls.Config
	if !s.Config.DisableTLS {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: false,            // Explicitly safe when TLS is enabled
			MinVersion:         tls.VersionTLS12, // Enforce TLS 1.2 or higher
		}
	}

	s.httpClient = &http.Client{Transport: transport.New(tlsConfig)}

	return nil
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
//   - error: the params, payload, or request error, or [ErrSendFailed] for an
//     error status.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Mattermost.
//
// Parameters:
//   - ctx: cancellation and deadline for the request, further bounded by [defaultHTTPTimeout].
//   - message: the message to send.
//   - params: optional overrides for configuration fields, including the
//     username and channel params and disabletls.
//
// Returns:
//   - error: the params, payload, or request error, or [ErrSendFailed] for an
//     error status. A request error matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	if err := s.pkr.UpdateConfigFromParams(config, configParams(params)); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

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
		serviceURL.String(),
		bytes.NewReader(json),
	)
	if err != nil {
		return fmt.Errorf("creating POST request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/json")

	client := s.httpClient
	if client == nil {
		client = &http.Client{Transport: transport.New(&tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		})}
	}

	res, err := client.Do(req)
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
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	s.httpClient = client
}

// configParams returns the send params the config resolver accepts as keys.
//
// It leaves out the username and channel params, which override URL fields
// and are read by [CreateJSONPayload]. The resolver still rejects any other
// unknown key.
//
// Parameters:
//   - params: the send params, or nil for none.
//
// Returns:
//   - *types.Params: a copy of params without the URL field overrides, or nil
//     when params is nil.
func configParams(params *types.Params) *types.Params {
	if params == nil {
		return nil
	}

	keyParams := maps.Clone(*params)
	delete(keyParams, userNameParam)
	delete(keyParams, channelParam)

	return &keyParams
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
