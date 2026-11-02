package homeassistant

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Home Assistant.
type Service struct {
	// Standard provides base service functionality including logging.
	standard.Standard

	// client holds the injected HTTP client and the default clients. A send
	// that skips certificate verification uses the insecure default client.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// Config holds Home Assistant credentials and notification options.
	Config *Config
	// pkr resolves property keys for configuration updates from URL parameters.
	pkr format.PropKeyResolver
}

// requestPayload is the JSON body posted to /api/services/{domain}/{service}.
type requestPayload struct {
	// Message is the notification body.
	Message string `json:"message"`
	// Title is the notification title, left out when empty.
	Title string `json:"title,omitempty"`
	// NotificationID identifies a persistent notification, so a later one
	// with the same ID replaces it.
	NotificationID string `json:"notification_id,omitempty"`
	// Targets lists the notify service targets, left out when empty.
	Targets []string `json:"target,omitempty"`
}

// contentType is the JSON encoding required by the REST API.
const contentType = "application/json"

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
//   - The scheme name "homeassistant".
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger. It performs no
// network I/O.
//
// Parameters:
//   - serviceURL: The Home Assistant service URL.
//   - logger: Logger used for service output.
//
// Returns:
//   - An error if default properties cannot be applied or the URL is invalid.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.warnings.Reset()

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send delivers a notification message to Home Assistant.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: The notification body.
//   - params: Optional runtime overrides for title, service, targets, and nid.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize,
//     [ErrMessageEmpty], the error from the params or the config's checks, or
//     the send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Home Assistant.
//
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: The notification body.
//   - params: Optional runtime overrides for title, service, targets, and nid.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize,
//     [ErrMessageEmpty], the error from the params or the config's checks, or
//     the send error, which matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	if message == "" {
		return ErrMessageEmpty
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

	// Plain HTTP applies to an injected client too, because the URL decides it.
	if config.DisableTLS {
		s.warnings.Plaintext(ctx, s)
	}

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

// send posts a notification to the Home Assistant REST API.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: The notification body sent as the JSON field message.
//   - config: The resolved configuration used to build the URL and payload.
//
// Returns:
//   - error: the request error, or an error for a non-success status.
func (s *Service) send(ctx context.Context, message string, config *Config) error {
	payload, postURL, err := buildRequest(message, config)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		postURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+config.Token)
	req.Header.Set("User-Agent", meta.UserAgent())

	res, err := s.sendClient(ctx, config).Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	responseBody, err := httpbody.Read(res.Body, httpbody.MaxSize)
	if err != nil && !errors.Is(err, httpbody.ErrTooLarge) {
		return fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return fmt.Errorf("%w: %s: %s", ErrUnexpectedStatus, res.Status, string(responseBody))
	}

	return nil
}

// buildRequest constructs the JSON payload and API URL for a notification.
//
// Notify actions get the non-empty targets only. An empty target list leaves
// the field out, so the action delivers to its own default recipients.
//
// Parameters:
//   - message: The notification body.
//   - config: The resolved configuration.
//
// Returns:
//   - payload: The JSON body to post.
//   - postURL: The absolute API URL.
//   - err: An error if the service mapping is invalid.
func buildRequest(message string, config *Config) (requestPayload, string, error) {
	domain, service, err := config.domainService()
	if err != nil {
		return requestPayload{}, "", err
	}

	payload := requestPayload{Message: message}
	if config.Title != "" {
		payload.Title = config.Title
	}

	if domain == persistentDomain && service == persistentService {
		if config.Nid != "" {
			payload.NotificationID = config.Nid
		}
	} else if targets := nonEmptyTargets(config.Targets); len(targets) > 0 {
		payload.Targets = targets
	}

	return payload, config.apiURL(domain, service), nil
}

// nonEmptyTargets returns the targets without empty entries, such as the one
// an empty targets prop or a trailing comma produces.
//
// Parameters:
//   - targets: The configured targets, which the result does not modify.
//
// Returns:
//   - []string: A new slice with the non-empty targets in order.
func nonEmptyTargets(targets []string) []string {
	return slices.DeleteFunc(slices.Clone(targets), func(target string) bool {
		return target == ""
	})
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
