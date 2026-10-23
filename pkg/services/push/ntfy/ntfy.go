package ntfy

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/internal/transport"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Service sends notifications to ntfy.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// httpClient is the client for sends whose certificate verification
	// setting matches the config: an injected client, or the default client.
	httpClient types.HTTPClient

	// Config holds the server, the topic, the credentials, and the message
	// options.
	Config *Config
	// otherClient is the default client for sends whose certificate verification
	// setting differs from the stored config, built on first use.
	otherClient *http.Client
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
	// otherClientMu guards otherClient.
	otherClientMu sync.Mutex
	// defaultClient reports whether httpClient was built by the service rather than
	// supplied through SetHTTPClient, so Initialize rebuilds it for the new config.
	defaultClient bool
}

// HTTPTimeout defines the HTTP client timeout in seconds.
const HTTPTimeout = 10

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
// Unless a client is injected, it builds the default HTTP client for the URL's
// certificate verification setting, with a warning when verification is off.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from applying the default props or reading the URL,
//     such as [ErrTopicRequired] or [ErrInvalidTopic].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	err := s.Config.setURL(&s.pkr, serviceURL)
	if err != nil {
		return err
	}

	if s.httpClient == nil || s.defaultClient {
		s.setDefaultHTTPClient()

		if s.Config.DisableTLSVerification {
			s.Log("Warning: TLS verification is disabled, making connections insecure")
		}
	}

	return nil
}

// Send delivers a notification message to ntfy. Params apply to this send only.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: the params error, the ntfy API error, or the request error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to ntfy. Params apply to this send only,
// including disabletls and disabletlsverification.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: the params error, the ntfy API error, or the request error, which
//     matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	config := *s.Config

	// Update this send's config with runtime parameters
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	s.applyLevel(&config, params)

	// Execute the API request to send the notification
	if err := s.sendAPI(ctx, &config, message); err != nil {
		return fmt.Errorf("failed to send ntfy notification: %w", err)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client, which Initialize builds when the service is not yet configured
// and rebuilds whenever it applies a new config.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	if client != nil {
		s.httpClient = client
		s.defaultClient = false

		return
	}

	if s.Config == nil {
		s.httpClient = nil
		s.defaultClient = true

		return
	}

	s.setDefaultHTTPClient()
}

// applyLevel sets the priority of a send from its level param, debug to low,
// info to default, warning to high, and error to max. A priority set in the
// service URL or the send params takes precedence.
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

	if levelPriority, mapped := levelPriorities[level]; mapped && s.pkr.KeepsDefault(config, params, "priority") {
		config.Priority = levelPriority
	}
}

// newAPIClient returns the JSON client for one send, so every send has its own
// request headers.
//
// Parameters:
//   - config: the configuration for this send.
//
// Returns:
//   - jsonclient.ContextClient: a new client over the send's HTTP client.
func (s *Service) newAPIClient(config *Config) jsonclient.ContextClient {
	return jsonclient.NewContextClient(s.sendHTTPClient(config))
}

// newDefaultHTTPClient builds the client used when none is injected. It enforces
// TLS 1.2 or later and skips certificate verification only when asked to.
//
// Parameters:
//   - skipVerify: whether to skip certificate verification.
//
// Returns:
//   - *http.Client: the default client.
func (s *Service) newDefaultHTTPClient(skipVerify bool) *http.Client {
	return &http.Client{
		Timeout: HTTPTimeout * time.Second,
		Transport: transport.New(&tls.Config{
			InsecureSkipVerify: skipVerify,
			MinVersion:         tls.VersionTLS12,
		}),
	}
}

// sendAPI sends a notification to the ntfy API.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - message: the message to send.
//
// Returns:
//   - error: the ntfy API error or the request error.
func (s *Service) sendAPI(ctx context.Context, config *Config, message string) error {
	response := apiResponseError{}
	request := message

	client := s.newAPIClient(config)

	// Prepare request headers
	headers := client.Headers()
	if config.Markdown {
		headers.Set("Content-Type", "text/markdown")
	} else {
		headers.Set("Content-Type", "text/plain; charset=utf-8")
	}

	headers.Set("User-Agent", meta.UserAgent())
	addHeaderIfNotEmpty(&headers, "Title", config.Title)
	addHeaderIfNotEmpty(&headers, "Priority", config.Priority.String())
	addHeaderIfNotEmpty(&headers, "Tags", strings.Join(config.Tags, ","))
	addHeaderIfNotEmpty(&headers, "Delay", config.Delay)
	addHeaderIfNotEmpty(&headers, "Actions", strings.Join(config.Actions, ";"))
	addHeaderIfNotEmpty(&headers, "Click", config.Click)
	addHeaderIfNotEmpty(&headers, "Attach", config.Attach)
	addHeaderIfNotEmpty(&headers, "X-Icon", config.Icon)
	addHeaderIfNotEmpty(&headers, "Filename", config.Filename)
	addHeaderIfNotEmpty(&headers, "Email", config.Email)

	if !config.Cache {
		headers.Add("Cache", "no")
	}

	if !config.Firebase {
		headers.Add("Firebase", "no")
	}

	// Access tokens use Bearer auth and take precedence over username and password.
	if config.Token != "" {
		headers.Set("Authorization", "Bearer "+config.Token)
	} else if config.Username != "" || config.Password != "" {
		headers.Set(
			"Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(config.Username+":"+config.Password)),
		)
	}

	// Send the HTTP request
	if err := client.PostContext(ctx, config.GetAPIURL(), request, &response); err != nil {
		s.Logf("NTFY API request failed with error: %v", err)
		// Attempt to parse structured error response from API
		if client.ErrorResponse(err, &response) {
			return &response
		}

		return fmt.Errorf("posting to ntfy API: %w", err)
	}

	s.Logf("NTFY API request succeeded")

	return nil
}

// sendHTTPClient returns the HTTP client for one send. A custom client is used
// as is. The default client serves sends with the stored certificate
// verification setting, and sends that change it share another default client,
// built on first use.
//
// Parameters:
//   - config: the configuration for this send.
//
// Returns:
//   - types.HTTPClient: the client for the send.
func (s *Service) sendHTTPClient(config *Config) types.HTTPClient {
	if !s.defaultClient || config.DisableTLSVerification == s.Config.DisableTLSVerification {
		return s.httpClient
	}

	s.otherClientMu.Lock()
	defer s.otherClientMu.Unlock()

	if s.otherClient == nil {
		if config.DisableTLSVerification {
			s.Log("Warning: TLS verification is disabled, making connections insecure")
		}

		s.otherClient = s.newDefaultHTTPClient(config.DisableTLSVerification)
	}

	return s.otherClient
}

// setDefaultHTTPClient builds the default client for the stored config and
// drops the default client for the other certificate verification setting.
func (s *Service) setDefaultHTTPClient() {
	s.httpClient = s.newDefaultHTTPClient(s.Config.DisableTLSVerification)
	s.defaultClient = true

	s.otherClientMu.Lock()
	s.otherClient = nil
	s.otherClientMu.Unlock()
}

// addHeaderIfNotEmpty adds a header to the request if the value is non-empty.
//
// Parameters:
//   - headers: the request headers.
//   - key: the header name.
//   - value: the header value, which is skipped when empty.
func addHeaderIfNotEmpty(headers *http.Header, key, value string) {
	if value != "" {
		headers.Add(key, value)
	}
}
