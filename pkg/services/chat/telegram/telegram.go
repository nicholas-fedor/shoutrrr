package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
	"unicode/utf16"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/sequential"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to configured Telegram chats.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default clients.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// Config holds the token, the chats, and the message options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Settings for requests to the Bot API.
const (
	// defaultAPIURL is the base URL of Telegram's own Bot API server.
	defaultAPIURL = "https://api.telegram.org"
	// maxlength is the most UTF-16 code units a message can hold.
	maxlength = 4096
	// defaultHTTPTimeout is the part of the send budget for each chat, and the
	// timeout of the generator's requests for the bot info.
	defaultHTTPTimeout = 10 * time.Second
)

// Errors returned by the service.
var (
	// ErrMessageTooLong indicates that the message exceeds the maximum allowed length.
	ErrMessageTooLong = errors.New("Message exceeds the max length")
	// ErrUnexpectedResponse indicates that the Telegram API reported a failure
	// without an error description.
	ErrUnexpectedResponse = errors.New("telegram API reported a failure without an error description")
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetConfig returns the current configuration for the service.
//
// Returns:
//   - *Config: the service's configuration.
func (s *Service) GetConfig() *Config {
	return s.Config
}

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the scheme, telegram.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the service URL with the bot token and the chats.
//   - logger: the logger for the service's output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrInvalidToken].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.warnings.Reset()
	s.Config = &Config{
		Preview:      true,
		Notification: true,
	}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Telegram.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the
//     validation, params, or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to every configured Telegram chat.
//
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports, which the requests share. The first send
// to an http API URL logs a warning, and so does the first send that skips
// certificate verification through the default client.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, a params
//     error, [ErrMessageTooLong] for a message without a parse mode over 4096
//     UTF-16 code units with its title line, [ErrInvalidThreadID] for a chat
//     set by the params, [ErrInvalidAPIURL] for an API URL set by the params,
//     or the joined errors of the chats that failed, each naming its chat. A
//     chat that fails does not stop the chats after it. Once ctx ends, the
//     remaining chats are skipped and ctx's error is reported once. A request
//     error matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	targets, err := config.prepare()
	if err != nil {
		return err
	}

	if tooLong(message, &config) {
		return ErrMessageTooLong
	}

	payload := newMessagePayload(message, &config)

	ctx, cancel := httpclient.BoundContext(ctx, config.sendBudget())
	defer cancel()

	// The bot token is in every request path, so an http API URL sends it in
	// the clear, whichever client carries the request.
	if config.plaintextAPI() {
		s.warnings.Plaintext(ctx, s)
	}

	httpClient := s.sendClient(ctx, &config)

	return sequential.Send(
		ctx,
		targets,
		sequential.PerTarget(defaultHTTPTimeout),
		func(ctx context.Context, target chatTarget) error {
			if err := s.sendMessageToAPI(ctx, httpClient, &payload, target, &config); err != nil {
				return fmt.Errorf("sending to chat %q: %w", target.chat, err)
			}

			return nil
		},
	)
}

// ServiceTimeout reports the send budget the router gives Telegram. A send makes
// one request per chat in turn, and each request gets 10 seconds.
//
// Parameters:
//   - params: optional overrides for configuration fields, which may set the chats.
//     An invalid param fails the send itself, so the stored chats set the budget.
//
// Returns:
//   - time.Duration: 10 seconds for each chat, and at least 10 seconds.
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	if s.Config == nil {
		return defaultHTTPTimeout
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		config = *s.Config
	}

	return config.sendBudget()
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// sendClient returns the HTTP client for a send's requests to every chat.
//
// A send that skips certificate verification uses the insecure default client,
// and the first such send logs the shared warning. An injected client is used
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
		return s.client.InsecureClient(func() { s.warnings.Insecure(ctx, s) })
	}

	return s.client.Client()
}

// sendMessageToAPI sends a send's message to one chat.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - httpClient: the HTTP client of the send.
//   - payload: the send's request body without a chat, which is left
//     unchanged.
//   - target: the chat to send the message to.
//   - config: the configuration for this send.
//
// Returns:
//   - error: the Telegram API error or the request error.
func (*Service) sendMessageToAPI(
	ctx context.Context,
	httpClient types.HTTPClient,
	payload *SendMessagePayload,
	target chatTarget,
	config *Config,
) error {
	request := *payload
	request.ID = target.id
	request.MessageThreadID = target.threadID

	client := &Client{token: config.Token, apiURL: config.APIURL, httpClient: httpClient}
	_, err := client.SendMessageContext(ctx, &request)

	return err
}

// tooLong reports whether a message without a parse mode is longer than
// Telegram allows.
//
// Telegram counts the text after parsing, in UTF-16 code units, so the markup
// and escapes Shoutrrr adds do not count, and a title counts with its line break.
// A message written for a parse mode is not checked, since only Telegram's parser
// can tell how much of it is markup.
//
// Parameters:
//   - message: the message to send.
//   - config: the configuration for this send.
//
// Returns:
//   - bool: true for a message without a parse mode over the limit.
func tooLong(message string, config *Config) bool {
	if config.ParseMode != ParseModeNone {
		return false
	}

	length := utf16Length(message)

	if config.Title != "" {
		length += utf16Length(config.Title) + 1
	}

	return length > maxlength
}

// utf16Length returns the length of s in UTF-16 code units, the unit of
// Telegram's message length limit. A character outside the Basic
// Multilingual Plane counts as two units, and an invalid byte as one.
//
// Parameters:
//   - s: the text to measure.
//
// Returns:
//   - int: the number of UTF-16 code units.
func utf16Length(s string) int {
	length := 0

	for _, r := range s {
		length += max(1, utf16.RuneLen(r))
	}

	return length
}
