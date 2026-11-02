package bark

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTPClient defines the interface for HTTP operations.
//
// Deprecated: The service takes a [types.HTTPClient], which HTTPClient is an
// alias of. Use [types.HTTPClient].
//
//go:fix inline
type HTTPClient = types.HTTPClient

// DefaultHTTPClient is an HTTP client whose requests time out after
// [defaultHTTPTimeout]. The zero value is ready to use.
//
// Deprecated: The service builds its own default client and bounds each send
// by its send budget. Inject an *http.Client with [Service.SetHTTPClient]
// instead.
type DefaultHTTPClient struct {
	// client sends the requests, or is nil to send through a new client with
	// [defaultHTTPTimeout] as its timeout.
	client *http.Client
}

// Service sends push notifications to Bark-enabled iOS devices.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// HTTPClient sends the requests to the Bark server when set.
	// [Service.SetHTTPClient] clears it, so whichever of the two the caller
	// set last carries the sends.
	//
	// Deprecated: Use [Service.SetHTTPClient].
	HTTPClient types.HTTPClient

	// Config holds the server, the device key, and the notification options.
	Config *Config
	// client holds the client from [Service.SetHTTPClient] and the default
	// client.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// defaultHTTPTimeout is the send budget, and the timeout of
// [DefaultHTTPClient].
const defaultHTTPTimeout = 30 * time.Second

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

// NewDefaultHTTPClient creates a new HTTP client with default timeout settings.
//
// Returns:
//   - *DefaultHTTPClient: a client whose requests time out after
//     [defaultHTTPTimeout].
//
// Deprecated: The service builds its own default client and bounds each send
// by its send budget. Inject an *http.Client with [Service.SetHTTPClient]
// instead.
func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Do executes the HTTP request and returns the response.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the server's response.
//   - error: the request failure, with the request URL redacted.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	client := &http.Client{Timeout: defaultHTTPTimeout}
	if c != nil && c.client != nil {
		client = c.client
	}

	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("executing HTTP request: %w", redact.URLError(err))
	}

	return resp, nil
}

// GetID returns the scheme identifier for the Bark service.
//
// Returns:
//   - The string "bark" representing this notification service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize sets up the Bark service with configuration from the provided URL.
// It keeps an HTTP client that is already set, and warns once when the URL
// uses the deprecated scheme key. It performs no network I/O.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from applying the default props or reading the URL,
//     such as [ErrMissingDeviceKey] or [ErrInvalidScheme].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.warnings.Reset()

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	// The resolver records the keys the URL set, so a scheme the URL set
	// does not keep its default.
	if !s.pkr.KeepsDefault(s.Config, nil, schemeKey.Name) {
		s.warnings.DeprecatedKeys(context.Background(), s, []format.Deprecation{schemeKey})
	}

	return nil
}

// Send transmits a notification message to the Bark server.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: The notification body text to send.
//   - params: Additional parameters for notification customization.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the error
//     from the params, the config's checks, or the send.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext transmits a notification message to the Bark server. Params
// apply to this send only, including disabletls. A context without a deadline
// is bounded by the budget that [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the notification body text to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the error
//     from the params or the config's checks, such as [ErrInvalidScheme] or
//     [ErrMissingDeviceKey], or the send error. A request error
//     matches ctx's error when ctx ends the request.
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

	if err := s.sendAPI(ctx, &config, message); err != nil {
		return fmt.Errorf("failed to send bark notification: %w", err)
	}

	return nil
}

// SendItems converts message items to plain text and sends as a notification.
// This method handles rich message items by extracting plain text content.
//
// Parameters:
//   - items: Slice of message items to send.
//   - params: Additional parameters for notification customization.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the send
//     failure, or nil on success.
//
// Deprecated: Use [Service.Send] with [types.ItemsToPlain]. The router already
// sends rich messages to Bark as plain text.
//
//go:fix inline
func (s *Service) SendItems(items []types.MessageItem, params *types.Params) error {
	return s.Send(types.ItemsToPlain(items), params)
}

// ServiceTimeout reports the send budget of Bark.
//
// Parameters:
//   - params: unused. The budget does not depend on send parameters.
//
// Returns:
//   - time.Duration: [defaultHTTPTimeout].
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return defaultHTTPTimeout
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client. It clears the
// deprecated HTTPClient field, so the sends use this client until that field
// is set again.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
	s.HTTPClient = nil
}

// sendAPI sends a notification to the Bark server using the configured endpoint.
// This method handles JSON serialization, HTTP request creation, and response parsing.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: The Bark service configuration containing API settings.
//   - message: The notification body text to send.
//
// Returns:
//   - error: the request error, the error the Bark server reported, or an
//     error wrapping [ErrUnexpectedStatus] for an error status whose body is
//     not JSON or reports success.
func (s *Service) sendAPI(ctx context.Context, config *Config, message string) error {
	response := APIResponse{}
	request := PushPayload{
		Body:      message,
		DeviceKey: config.DeviceKey,
		Title:     config.Title,
		Category:  config.Category,
		Copy:      config.Copy,
		Sound:     config.Sound,
		Group:     config.Group,
		Badge:     &config.Badge,
		Icon:      config.Icon,
		URL:       config.URL,
	}

	requestBody, err := json.Marshal(request, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("marshaling request to JSON: %w", err)
	}

	apiURL := config.GetAPIURL("push")

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.sendClient(ctx, config).Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFailedAPIRequest, redact.URLError(err))
	}

	defer func() { _ = httpResp.Body.Close() }()

	body, err := httpbody.Read(httpResp.Body, httpbody.MaxSize)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if err := json.Unmarshal(body, &response, jsonopts.Default); err != nil {
		// A proxy in front of the server can answer an error status with a
		// body that is not JSON, such as an HTML page.
		if httpResp.StatusCode >= http.StatusBadRequest {
			return fmt.Errorf("%w: %d", ErrUnexpectedStatus, httpResp.StatusCode)
		}

		return fmt.Errorf("decoding response: %w", err)
	}

	if response.Code != http.StatusOK {
		if response.Message != "" {
			return &response
		}

		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.Code)
	}

	// A body that reports success does not override an error status.
	if httpResp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, httpResp.StatusCode)
	}

	return nil
}

// sendClient returns the client for a send. A send that skips certificate
// verification uses the insecure default client, and the first such send logs
// the shared warning. A client set through [Service.HTTPClient] or
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
		return s.client.PreferInsecure(s.HTTPClient, func() { s.warnings.Insecure(ctx, s) })
	}

	return s.client.Prefer(s.HTTPClient)
}

// sendConfig returns the config for one send. The stored scheme moves into
// DisableTLS before the params apply, so a disabletls param can turn TLS back
// on. A scheme param of either protocol applies only when the params do not
// set disabletls. The deprecated Scheme field and scheme param are each warned
// about once.
//
// Parameters:
//   - ctx: the context of the send, passed to its warnings.
//   - params: the send params, or nil.
//
// Returns:
//   - Config: the config for this send.
//   - error: a params error wrapping [ErrUpdateParamsFailed],
//     [ErrInvalidScheme], or the error from the config's checks, such as
//     [ErrMissingDeviceKey].
func (s *Service) sendConfig(ctx context.Context, params *types.Params) (Config, error) {
	config := *s.Config

	// A URL with an http scheme turns TLS off as it is read, so an http scheme
	// with TLS on was set on the deprecated field directly.
	if strings.EqualFold(config.Scheme, schemeHTTP) && !config.DisableTLS {
		s.warnings.DeprecatedFields(ctx, s, []format.Deprecation{schemeField})
	}

	if params != nil && hasKey(*params, schemeKey.Name) {
		s.warnings.DeprecatedKeys(ctx, s, []format.Deprecation{schemeKey})
	}

	// The stored scheme turns TLS off before the params apply, so a disabletls
	// param can turn it back on. DisableTLS alone then decides the protocol.
	if err := config.applyScheme(); err != nil {
		return config, err
	}

	config.Scheme = schemeHTTPS

	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return config, fmt.Errorf("%w: %w", ErrUpdateParamsFailed, err)
	}

	if params != nil && hasKey(*params, schemeKey.Name) {
		disableTLS := config.DisableTLS

		// The scheme param decides the protocol on its own, so https turns
		// TLS back on.
		config.DisableTLS = false

		if err := config.applyScheme(); err != nil {
			return config, err
		}

		// disabletls replaces scheme, so it wins when the params set both.
		if hasKey(*params, schemeKey.Replacement) {
			config.DisableTLS = disableTLS
		}

		config.Scheme = schemeHTTPS
	}

	if err := config.validate(); err != nil {
		return config, err
	}

	return config, nil
}
