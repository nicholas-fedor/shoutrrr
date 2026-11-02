package signalgrid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Signalgrid.
type Service struct {
	// Standard provides base service functionality including logging.
	standard.Standard

	// client holds the injected HTTP client and the default client for Push
	// API requests.
	client httpclient.Holder

	// Config holds Signalgrid credentials and notification options.
	Config *Config
	// pkr resolves property keys for configuration updates from URL parameters.
	pkr format.PropKeyResolver
}

// Settings for requests to the Push API.
const (
	// apiURL is the Signalgrid Push API endpoint.
	apiURL = "https://api.signalgrid.co/v1/push"
	// contentType is the form encoding required by the Push API.
	contentType = "application/x-www-form-urlencoded"
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// levelTypes maps each known message level to the Signalgrid type it is sent
// with when the send keeps the default type.
var levelTypes = map[types.MessageLevel]notificationType{
	types.Debug:   TypeINFO,
	types.Info:    TypeINFO,
	types.Warning: TypeWARN,
	types.Error:   TypeCRIT,
}

// GetID returns the service identifier.
//
// Returns:
//   - The scheme name "signalgrid".
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: The Signalgrid service URL.
//   - logger: Logger used for service output.
//
// Returns:
//   - error: the error when default properties cannot be applied or the URL is invalid.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Signalgrid.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: The notification body.
//   - params: Optional runtime overrides for title, type, and critical.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the error
//     from the params, the config's checks, or the send.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Signalgrid. A context without
// a deadline is bounded by the budget that [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: The notification body.
//   - params: Optional runtime overrides for title, type, and critical.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the error
//     from the params, the config's checks, or the send. A request error
//     matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.validate(); err != nil {
		return err
	}

	s.applyLevel(&config, params)

	if err := s.send(ctx, message, &config); err != nil {
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
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

// applyLevel sets the type of a notification from its level param, debug and
// info to INFO, warning to WARN, and error to CRIT. A type set in the service
// URL or the send params takes precedence.
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

	if levelType, mapped := levelTypes[level]; mapped && s.pkr.KeepsDefault(config, params, "type") {
		config.Type = levelType
	}
}

// send posts a notification to the Signalgrid Push API.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: The notification body sent as the form field body.
//   - config: The resolved configuration used to populate remaining form fields.
//
// Returns:
//   - error: the request error, or an error for a non-success status.
func (s *Service) send(ctx context.Context, message string, config *Config) error {
	data := url.Values{}
	data.Set("client_key", config.ClientKey)
	data.Set("channel", config.Channel)
	data.Set("body", message)
	data.Set("type", config.Type.String())

	if config.Title != "" {
		data.Set("title", config.Title)
	}

	if config.Critical {
		data.Set("critical", "true")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", meta.UserAgent())

	res, err := s.client.Client().Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	responseBody, err := httpbody.Read(res.Body, httpbody.MaxSize)
	if err != nil && !errors.Is(err, httpbody.ErrTooLarge) {
		return fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %s: %s", ErrUnexpectedStatus, res.Status, string(responseBody))
	}

	return nil
}
