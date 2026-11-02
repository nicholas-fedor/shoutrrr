package ntfy

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Service sends notifications to ntfy.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default clients. A send
	// that skips certificate verification uses the insecure default client.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// Config holds the server, the topic, the credentials, and the message
	// options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// HTTPTimeout is a request timeout in seconds.
//
// Deprecated: A send is bounded by the budget that [Service.ServiceTimeout]
// reports, so the service does not use HTTPTimeout.
const HTTPTimeout = 10

// Deprecations of the scheme. Its replacement is another field, which the
// format package's deprecation tags cannot describe.
var (
	// schemeKey is the deprecated scheme key and its replacement.
	schemeKey = format.Deprecation{Name: "scheme", Replacement: "disabletls"}
	// schemeField is the deprecated Scheme field and its replacement.
	schemeField = format.Deprecation{Name: "Scheme", Replacement: "DisableTLS"}
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
// It warns once about each deprecated key the URL uses. It performs no network
// I/O.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from applying the default props or reading the URL,
//     such as [ErrTopicRequired], [ErrInvalidTopic], or [ErrInvalidScheme].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.warnings.Reset()

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	s.warnings.DeprecatedKeys(context.Background(), s, s.pkr.DeprecatedKeys())

	// The resolver records the keys the URL set, so a scheme the URL set
	// does not keep its default.
	if !s.pkr.KeepsDefault(s.Config, nil, "scheme") {
		s.warnings.DeprecatedKeys(context.Background(), s, []format.Deprecation{schemeKey})
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, the ntfy API error, or the
//     request error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to ntfy. Params apply to this send only,
// including disabletls and skiptlsverify. A context without a deadline is
// bounded by the budget that [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, the ntfy API error, or the
//     request error, which matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	config, err := s.sendConfig(ctx, params)
	if err != nil {
		return err
	}

	// Plain HTTP applies to an injected client too, because the URL decides it.
	if config.DisableTLS {
		s.warnings.Plaintext(ctx, s)
	}

	// Execute the API request to send the notification
	if err := s.sendAPI(ctx, &config, message); err != nil {
		return fmt.Errorf("failed to send ntfy notification: %w", err)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, restores the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
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
//   - ctx: the context of the send, passed to the client's warnings.
//   - config: the configuration for this send.
//
// Returns:
//   - jsonclient.ContextClient: a new client over the send's HTTP client.
func (s *Service) newAPIClient(ctx context.Context, config *Config) jsonclient.ContextClient {
	return jsonclient.NewContextClient(s.sendClient(ctx, config))
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

	client := s.newAPIClient(ctx, config)

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
		// Attempt to parse structured error response from API
		if client.ErrorResponse(err, &response) {
			return &response
		}

		return fmt.Errorf("posting to ntfy API: %w", err)
	}

	s.Debug(ctx, "notification sent")

	return nil
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

// sendConfig returns the config for one send. Deprecated config fields and
// the stored scheme move into their replacements before the params apply, so
// a param can still turn an option off. A scheme param of either protocol
// applies only when the params do not set disabletls. Each deprecated field
// and param key is warned about once.
//
// Parameters:
//   - ctx: the context of the send, passed to its warnings.
//   - params: the send params, or nil.
//
// Returns:
//   - Config: the config for this send.
//   - error: a params error, [ErrInvalidScheme], or the error from the
//     config's checks, such as [ErrTopicRequired].
func (s *Service) sendConfig(ctx context.Context, params *types.Params) (Config, error) {
	config := *s.Config

	deprecatedFields, err := format.SyncDeprecatedFields(&config)
	if err != nil {
		return config, fmt.Errorf("syncing deprecated config fields: %w", err)
	}

	s.warnings.DeprecatedFields(ctx, s, deprecatedFields)

	// A URL with an http scheme turns TLS off as it is read, so an http scheme
	// with TLS on was set on the deprecated field directly.
	if strings.EqualFold(config.Scheme, schemeHTTP) && !config.DisableTLS {
		s.warnings.DeprecatedFields(ctx, s, []format.Deprecation{schemeField})
	}

	s.warnings.DeprecatedKeys(ctx, s, s.pkr.DeprecatedParams(params))

	if hasParam(params, schemeKey.Name) {
		s.warnings.DeprecatedKeys(ctx, s, []format.Deprecation{schemeKey})
	}

	// The stored scheme turns TLS off before the params apply, so a disabletls
	// param can turn it back on. DisableTLS alone then decides the protocol.
	if err := config.applyScheme(); err != nil {
		return config, err
	}

	config.Scheme = schemeHTTPS

	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return config, fmt.Errorf("updating config from params: %w", err)
	}

	if hasParam(params, schemeKey.Name) {
		disableTLS := config.DisableTLS

		// The scheme param decides the protocol on its own, so https turns
		// TLS back on.
		config.DisableTLS = false

		if err := config.applyScheme(); err != nil {
			return config, err
		}

		// disabletls replaces scheme, so it wins when the params set both.
		if hasParam(params, schemeKey.Replacement) {
			config.DisableTLS = disableTLS
		}

		config.Scheme = schemeHTTPS
	}

	s.applyLevel(&config, params)

	if err := config.validate(); err != nil {
		return config, err
	}

	return config, nil
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

// hasParam reports whether params set key, matched case-insensitively.
//
// Parameters:
//   - params: the send params, or nil.
//   - key: the param key in lower case.
//
// Returns:
//   - bool: whether params hold the key.
func hasParam(params *types.Params, key string) bool {
	if params == nil {
		return false
	}

	for paramKey := range *params {
		if strings.EqualFold(paramKey, key) {
			return true
		}
	}

	return false
}
