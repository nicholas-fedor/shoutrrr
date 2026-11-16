package zulip

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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

// registerResponse represents the relevant fields from the Zulip register endpoint.
type registerResponse struct {
	// QueueID identifies the event queue the register request created, or is
	// empty when the response omits it.
	QueueID string `json:"queue_id"`
	// MaxMessageLength is the maximum message length in characters, or zero
	// when the response omits it.
	MaxMessageLength int `json:"max_message_length"`
	// MaxTopicLength is the maximum topic length in characters, or zero when
	// the response omits it.
	MaxTopicLength int `json:"max_topic_length"`
}

// Service sends notifications to a pre-configured Zulip channel or user.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// HTTPClient sends the requests when set. [Service.SetHTTPClient] clears
	// it, so whichever of the two the caller set last carries the sends.
	//
	// Deprecated: Use [Service.SetHTTPClient].
	HTTPClient types.HTTPClient

	// client holds the client from [Service.SetHTTPClient] and the default
	// clients.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// Config holds the settings read from the service URL. Each send works on
	// a copy with its params applied.
	Config *Config
	// limitsDone is closed when the limits request in flight ends, and is nil
	// when no request is in flight.
	limitsDone chan struct{}
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
	// contentMaxSize is the maximum message length in characters. It starts at
	// the package default and takes the server's limit once reported.
	contentMaxSize int
	// topicMaxLength is the maximum topic length in characters. It starts at
	// the package default and takes the server's limit once reported.
	topicMaxLength int

	// limitsMu guards limitsFetched, limitsDone, and the limits.
	limitsMu sync.Mutex
	// limitsFetched is true once the server answered the limits request.
	limitsFetched bool
}

// defaultHTTPTimeout is the timeout of [DefaultHTTPClient], and the part of
// the send budget for the message request.
const defaultHTTPTimeout = 30 * time.Second

// contentMaxSize defines the default maximum allowed message length in characters.
const contentMaxSize = 10000

// topicMaxLength defines the default maximum allowed topic length in characters.
const topicMaxLength = 60

// registerTimeout caps the limits lookup within the send budget. A slow lookup
// falls back to the default limits and leaves the message request the rest of
// the budget.
const registerTimeout = 10 * time.Second

// deleteQueueTimeout caps the event queue delete at the end of a send. The
// message has been sent by then, so a slow delete holds the send for at most
// this long.
const deleteQueueTimeout = 5 * time.Second

// maxErrorBodySize limits how much of a non-OK response body we read to prevent
// memory exhaustion from huge error bodies returned by the server.
const maxErrorBodySize = 4 * 1024

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// NewDefaultHTTPClient creates an HTTP client with a timeout.
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

// Do performs the HTTP request.
//
// Parameters:
//   - req: the request to send.
//
// Returns:
//   - *http.Response: the response, which the caller closes.
//   - error: the request error, with URLs redacted by [redact.URLError].
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	client := &http.Client{Timeout: defaultHTTPTimeout}
	if c != nil && c.client != nil {
		client = c.client
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", redact.URLError(err))
	}

	return resp, nil
}

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger. It keeps an HTTP
// client that is already set, and drops the limits of an earlier server, so
// the next send looks them up again.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for the service.
//
// Returns:
//   - error: the error from setting the default props or reading the URL, or
//     nil.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.warnings.Reset()
	s.Config = &Config{}

	s.pkr = format.NewPropKeyResolver(s.Config)

	s.limitsMu.Lock()
	s.contentMaxSize = contentMaxSize
	s.topicMaxLength = topicMaxLength
	s.limitsFetched = false
	s.limitsMu.Unlock()

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
//   - error: [types.ErrServiceNotInitialized] before Initialize, the send
//     failure, or nil on success.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Zulip. The router calls it with
// its send deadline. A context without a deadline is bounded by the budget
// that [Service.ServiceTimeout] reports. The first send that skips certificate
// verification through the default client logs a warning.
//
// Parameters:
//   - ctx: bounds the server limits lookup and the message request.
//   - message: the message to send.
//   - params: per-send parameters, applied to this send only.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize. A params
//     error, the error from the config's checks, such as [ErrMissingAPIKey],
//     or a missing recipient ([ErrMissingRecipient] or [ErrMixedRecipients])
//     fails the send before any request. Otherwise the send failure, or nil on
//     success.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

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

	if err := config.validate(); err != nil {
		return err
	}

	if config.Type == MessageTypeDirect {
		if err := validateDirectRecipients(parseRecipients(directRecipients(config))); err != nil {
			return err
		}
	} else if config.Stream == "" {
		return ErrMissingRecipient
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	// The limits lookup and the message go through the same client.
	httpClient := s.sendClient(ctx, config)

	contentMax, topicMax, queueID := s.limits(ctx, httpClient, config)
	if queueID != "" {
		// The queue is deleted after the message, so the message does not
		// wait for the delete.
		defer deleteQueue(ctx, httpClient, config, queueID)
	}

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

	return s.doSend(ctx, httpClient, config, message)
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

// ServiceTimeout reports the send budget of Zulip. The first send calls the
// register endpoint for server limits before the message request, and deletes
// the event queue the register request creates after it. The budget covers
// all of them.
//
// Parameters:
//   - params: unused. The budget does not depend on send parameters.
//
// Returns:
//   - time.Duration: [registerTimeout] plus [defaultHTTPTimeout].
func (*Service) ServiceTimeout(*types.Params) time.Duration {
	return registerTimeout + defaultHTTPTimeout
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client. It clears the deprecated HTTPClient field, so the sends
// use this client until that field is set again.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
	s.HTTPClient = nil
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

// deleteQueue deletes an event queue, which Zulip would otherwise keep for 10
// minutes without a poll. A failed delete is not reported, since Zulip then
// removes the queue on its own.
//
// Parameters:
//   - ctx: bounds the request, further bounded to 5 seconds.
//   - httpClient: the HTTP client of the send.
//   - config: the checked configuration of the send.
//   - queueID: the ID of the queue to delete.
func deleteQueue(ctx context.Context, httpClient types.HTTPClient, config *Config, queueID string) {
	ctx, cancel := context.WithTimeout(ctx, deleteQueueTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("queue_id", queueID)

	req, err := newRequest(ctx, http.MethodDelete, config, "/api/v1/events", form)
	if err != nil {
		return
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return
	}

	// Reading the short response to its end lets the next send reuse the
	// connection.
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxErrorBodySize)) //nolint:errcheck // A failed read only keeps the connection from being reused.
	_ = res.Body.Close()
}

// doSend sends the notification to Zulip using the configured API URL.
//
// Parameters:
//   - ctx: cancellation for the request.
//   - httpClient: the HTTP client of the send.
//   - config: the configuration for this send.
//   - message: the message to send.
//
// Returns:
//   - error: [ErrResponseStatusFailure] for a status other than 200, with up to
//     [maxErrorBodySize] bytes of the body, or the request error.
func (s *Service) doSend(ctx context.Context, httpClient types.HTTPClient, config *Config, message string) error {
	req, err := newRequest(ctx, http.MethodPost, config, "/api/v1/messages", CreatePayload(config, message))
	if err != nil {
		return err
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("making HTTP POST request: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		body, readErr := httpbody.Read(res.Body, maxErrorBodySize)

		truncated := errors.Is(readErr, httpbody.ErrTooLarge)
		if (readErr == nil || truncated) && len(body) > 0 {
			trunc := ""

			if truncated {
				trunc = " [truncated]"
			}

			return fmt.Errorf("failed to send zulip message: %w: %d, body: %s%s", ErrResponseStatusFailure, res.StatusCode, string(body), trunc)
		}

		return fmt.Errorf("failed to send zulip message: %w: %d", ErrResponseStatusFailure, res.StatusCode)
	}

	return nil
}

// fetchLimits calls the Zulip register endpoint to fetch server-side size
// limits. The endpoint always creates an event queue, so the request
// subscribes it to no events.
//
// Parameters:
//   - ctx: bounds the request, further bounded by [registerTimeout].
//   - httpClient: the HTTP client of the send that looks the limits up.
//   - config: the checked configuration of the send that looks the limits up.
//
// Returns:
//   - registerResponse: the limits the server reported, zero for any it did
//     not, and the ID of the queue the request created.
//   - bool: whether the server answered with a 200 status and a JSON body.
func (s *Service) fetchLimits(ctx context.Context, httpClient types.HTTPClient, config *Config) (registerResponse, bool) {
	var reg registerResponse

	ctx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("event_types", "[]")
	form.Set("fetch_event_types", `["realm"]`)

	req, err := newRequest(ctx, http.MethodPost, config, "/api/v1/register", form)
	if err != nil {
		return reg, false
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return reg, false
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return reg, false
	}

	body, err := httpbody.Read(res.Body, httpbody.MaxSize)
	if err != nil {
		return reg, false
	}

	if err := json.Unmarshal(body, &reg, jsonopts.Default); err != nil {
		return reg, false
	}

	return reg, true
}

// limits returns the message and topic limits, asking the server on the first
// send and again on later sends until the server answers. One send makes the
// request while concurrent sends wait for it, or stop waiting when their own
// context ends, and then use the current limits.
//
// Parameters:
//   - ctx: bounds the limits request, or the wait for one in flight.
//   - httpClient: the HTTP client of the send, used when it makes the
//     request.
//   - config: the checked configuration of the send, used when it makes the
//     request.
//
// Returns:
//   - contentMax: the maximum message length in characters.
//   - topicMax: the maximum topic length in characters.
//   - queueID: the event queue that this send's limits request created,
//     which the send deletes, or empty when it made no request or the server
//     named no queue.
func (s *Service) limits(
	ctx context.Context,
	httpClient types.HTTPClient,
	config *Config,
) (contentMax, topicMax int, queueID string) { //nolint:nonamedreturns // Same-type returns need names.
	s.limitsMu.Lock()

	if s.limitsFetched {
		defer s.limitsMu.Unlock()

		return s.contentMaxSize, s.topicMaxLength, ""
	}

	if done := s.limitsDone; done != nil {
		s.limitsMu.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
		}

		contentMax, topicMax = s.currentLimits()

		return contentMax, topicMax, ""
	}

	done := make(chan struct{})
	s.limitsDone = done
	s.limitsMu.Unlock()

	reg, fetched := s.fetchLimits(ctx, httpClient, config)

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

	return s.contentMaxSize, s.topicMaxLength, reg.QueueID
}

// newRequest builds a form-encoded request to a Zulip API endpoint. The bot
// credentials go in the URL's userinfo, which net/http sends as basic auth.
//
// Parameters:
//   - ctx: bounds the request.
//   - method: the HTTP method.
//   - config: the configuration for this send.
//   - path: the endpoint path, such as /api/v1/messages.
//   - form: the request form, sent as the body.
//
// Returns:
//   - *http.Request: the request.
//   - error: the error from building the request, with its URL redacted by
//     [redact.URLError].
func newRequest(ctx context.Context, method string, config *Config, path string, form url.Values) (*http.Request, error) {
	endpoint := &url.URL{
		User:   url.UserPassword(config.BotMail, config.BotKey),
		Host:   config.Host,
		Path:   path,
		Scheme: "https",
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return req, nil
}

// sendClient returns the HTTP client for a send's requests, the limits lookup
// included. A send that skips certificate verification uses the insecure
// default client, and the first such send logs the shared warning. A client
// set through [Service.HTTPClient] or [Service.SetHTTPClient] is used
// unchanged and never warned about, because its owner controls its TLS
// settings.
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
