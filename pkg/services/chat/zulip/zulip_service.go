package zulip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTPClient defines the interface for HTTP operations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient is the default implementation using http.DefaultClient with timeout.
type DefaultHTTPClient struct {
	client *http.Client
}

// registerResponse represents the relevant fields from the Zulip register endpoint.
type registerResponse struct {
	MaxMessageLength int `json:"max_message_length"`
	MaxTopicLength   int `json:"max_topic_length"`
}

// Service sends notifications to a pre-configured Zulip channel or user.
type Service struct {
	standard.Standard

	HTTPClient HTTPClient

	Config *Config
	// limitsDone is closed when the limits request in flight ends, and is nil
	// when no request is in flight.
	limitsDone     chan struct{}
	pkr            format.PropKeyResolver
	contentMaxSize int
	topicMaxLength int

	// limitsMu guards limitsFetched, limitsDone, and the limits.
	limitsMu sync.Mutex
	// limitsFetched is true once the server answered the limits request.
	limitsFetched bool
}

// defaultHTTPTimeout is the default timeout for HTTP requests to Zulip.
const defaultHTTPTimeout = 30 * time.Second

// contentMaxSize defines the default maximum allowed message length in characters.
const contentMaxSize = 10000

// topicMaxLength defines the default maximum allowed topic length in characters.
const topicMaxLength = 60

// registerTimeout is the timeout for the register API call.
const registerTimeout = 10 * time.Second

// maxErrorBodySize limits how much of a non-OK response body we read to prevent
// memory exhaustion from huge error bodies returned by the server.
const maxErrorBodySize = 4 * 1024

// hostnameValidator matches a host name of one or more DNS labels.
var hostnameValidator = regexp.MustCompile(
	`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`,
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// validateHost checks that the host is a host name, an IPv4 address, or a
// bracketed IPv6 address, with an optional port from 1 to 65535.
//
// Parameters:
//   - host: the host, as in a URL authority.
//
// Returns:
//   - error: [ErrInvalidHost] naming the host, or nil.
func validateHost(host string) error {
	invalid := fmt.Errorf("%w: %q", ErrInvalidHost, host)
	bracketed := strings.HasPrefix(host, "[")

	name, port, err := net.SplitHostPort(host)
	if err != nil {
		name = host

		if bracketed {
			inner, ok := strings.CutSuffix(strings.TrimPrefix(host, "["), "]")
			if !ok {
				return invalid
			}

			name = inner
		}
	} else if number, parseErr := strconv.ParseUint(port, 10, 16); parseErr != nil || number == 0 {
		return invalid
	}

	if addr, parseErr := netip.ParseAddr(name); parseErr == nil {
		// URLs bracket IPv6 addresses and only IPv6 addresses.
		if addr.Is6() != bracketed {
			return invalid
		}

		return nil
	}

	if bracketed || !hostnameValidator.MatchString(name) {
		return invalid
	}

	return nil
}

func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Do performs the HTTP request.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", redact.URLError(err))
	}

	return resp, nil
}

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}

	s.pkr = format.NewPropKeyResolver(s.Config)
	if s.HTTPClient == nil {
		s.HTTPClient = NewDefaultHTTPClient()
	}

	s.contentMaxSize = contentMaxSize
	s.topicMaxLength = topicMaxLength

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	if err := s.Config.SetURL(serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Zulip without a deadline of its own.
//
// Parameters:
//   - message: the message to send.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: the send failure, or nil on success.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Zulip. The router calls it with
// its send deadline.
//
// Parameters:
//   - ctx: bounds the server limits lookup and the message request.
//   - message: the message to send.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: the send failure, or nil on success.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	// Clone the config to avoid modifying the original for this send operation.
	config := s.Config.Clone()

	// Use PropKeyResolver for consistent param handling
	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// If no topic is set, fall back to using the title as the topic
	// for backward compatibility with tools that only provide a title.
	if config.Topic == "" && config.Title != "" {
		config.Topic = config.Title
		config.Title = ""
	}

	if err := ValidateMessageType(config.Type); err != nil {
		return err
	}

	if err := validateHost(config.Host); err != nil {
		return err
	}

	contentMax, topicMax := s.limits(ctx)

	topicLength := len([]rune(config.Topic))
	if topicLength > topicMax {
		return fmt.Errorf(
			"%w: %d characters, got %d",
			ErrTopicTooLong,
			topicMax,
			topicLength,
		)
	}

	// Prepend title to message content when both topic and title are set.
	if config.Title != "" {
		message = fmt.Sprintf("%s\n\n%s", config.Title, message)
	}

	messageLength := utf8.RuneCountInString(message)
	if messageLength > contentMax {
		return fmt.Errorf(
			"%w: %d characters, got %d",
			ErrMessageTooLong,
			contentMax,
			messageLength,
		)
	}

	if config.Type == MessageTypeDirect {
		if err := validateDirectRecipients(parseRecipients(directRecipients(config))); err != nil {
			return err
		}
	} else if config.Stream == "" {
		return ErrMissingRecipient
	}

	return s.doSend(ctx, config, message)
}

// SendWithContext delivers a notification message with the provided context.
//
// Parameters:
//   - ctx: bounds the send.
//   - message: the message to send.
//   - params: per-send parameters.
//
// Returns:
//   - error: the send failure, or nil on success.
//
// Deprecated: Use [Service.SendContext], which the router calls with its send deadline.
//
//go:fix inline
func (s *Service) SendWithContext(ctx context.Context, message string, params *types.Params) error {
	return s.SendContext(ctx, message, params)
}

// ServiceTimeout reports the send budget the router gives Zulip. The first send
// calls the register endpoint for server limits before the message request, so
// the budget covers both requests.
//
// Parameters:
//   - params: unused. The request timeouts do not depend on send parameters.
//
// Returns:
//   - time.Duration: the register timeout plus the message request timeout.
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return registerTimeout + defaultHTTPTimeout
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	if client == nil {
		client = NewDefaultHTTPClient()
	}

	s.HTTPClient = client
}

// currentLimits returns the message and topic limits as they stand.
//
// Returns:
//   - contentMax: the maximum message length in characters.
//   - topicMax: the maximum topic length in characters.
func (s *Service) currentLimits() (contentMax, topicMax int) { //nolint:nonamedreturns // Same-type returns need names.
	s.limitsMu.Lock()
	defer s.limitsMu.Unlock()

	return s.contentMaxSize, s.topicMaxLength
}

// doSend sends the notification to Zulip using the configured API URL.
func (s *Service) doSend(ctx context.Context, config *Config, message string) error {
	apiURL := s.getAPIURL(config)

	payload := CreatePayload(config, message)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(payload.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("making HTTP POST request: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(res.Body, maxErrorBodySize+1))
		if readErr == nil && len(body) > 0 {
			trunc := ""

			if len(body) > maxErrorBodySize {
				body = body[:maxErrorBodySize]
				trunc = " [truncated]"
			}

			return fmt.Errorf("failed to send zulip message: %w: %d, body: %s%s", ErrResponseStatusFailure, res.StatusCode, string(body), trunc)
		}

		return fmt.Errorf("failed to send zulip message: %w: %d", ErrResponseStatusFailure, res.StatusCode)
	}

	return nil
}

// fetchLimits calls the Zulip register endpoint to fetch server-side size
// limits.
//
// Parameters:
//   - ctx: bounds the request, further bounded by [registerTimeout].
//
// Returns:
//   - registerResponse: the limits the server reported, zero for any it did not.
//   - bool: whether the server answered with a 200 status and a JSON body.
func (s *Service) fetchLimits(ctx context.Context) (registerResponse, bool) {
	var reg registerResponse

	if err := validateHost(s.Config.Host); err != nil {
		return reg, false
	}

	registerURL := s.getRegisterURL()

	ctx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("fetch_event_types", `["realm"]`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, strings.NewReader(form.Encode()))
	if err != nil {
		return reg, false
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := s.HTTPClient.Do(req)
	if err != nil {
		return reg, false
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return reg, false
	}

	if err := json.NewDecoder(res.Body).Decode(&reg); err != nil {
		return reg, false
	}

	return reg, true
}

// getAPIURL constructs the API URL for Zulip based on the Config.
func (s *Service) getAPIURL(config *Config) string {
	return (&url.URL{
		User:   url.UserPassword(config.BotMail, config.BotKey),
		Host:   config.Host,
		Path:   "/api/v1/messages",
		Scheme: "https",
	}).String()
}

// getRegisterURL constructs the register API URL from the service config.
func (s *Service) getRegisterURL() string {
	return (&url.URL{
		User:   url.UserPassword(s.Config.BotMail, s.Config.BotKey),
		Host:   s.Config.Host,
		Path:   "/api/v1/register",
		Scheme: "https",
	}).String()
}

// limits returns the message and topic limits, asking the server on the first
// send and again on later sends until the server answers. One send makes the
// request while concurrent sends wait for it, or stop waiting when their own
// context ends, and then use the current limits.
//
// Parameters:
//   - ctx: bounds the limits request, or the wait for one in flight.
//
// Returns:
//   - contentMax: the maximum message length in characters.
//   - topicMax: the maximum topic length in characters.
func (s *Service) limits(ctx context.Context) (contentMax, topicMax int) { //nolint:nonamedreturns // Same-type returns need names.
	s.limitsMu.Lock()

	if s.limitsFetched {
		defer s.limitsMu.Unlock()

		return s.contentMaxSize, s.topicMaxLength
	}

	if done := s.limitsDone; done != nil {
		s.limitsMu.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
		}

		return s.currentLimits()
	}

	done := make(chan struct{})
	s.limitsDone = done
	s.limitsMu.Unlock()

	reg, fetched := s.fetchLimits(ctx)

	s.limitsMu.Lock()
	defer s.limitsMu.Unlock()

	if reg.MaxMessageLength > 0 {
		s.contentMaxSize = reg.MaxMessageLength
	}

	if reg.MaxTopicLength > 0 {
		s.topicMaxLength = reg.MaxTopicLength
	}

	s.limitsFetched = fetched
	s.limitsDone = nil

	close(done)

	return s.contentMaxSize, s.topicMaxLength
}
