package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to configured Telegram chats.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient types.HTTPClient
}

// apiFormat defines the Telegram API endpoint template.
const (
	apiFormat          = "https://api.telegram.org/bot%s/%s"
	maxlength          = 4096
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
func (s *Service) GetConfig() *Config {
	return s.Config
}

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
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
//   - error: the validation, params, or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to every configured Telegram chat.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [ErrMessageTooLong], a params error, or the first send error,
//     which matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if len(message) > maxlength {
		return ErrMessageTooLong
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	return s.sendMessageForChatIDs(ctx, message, &config)
}

// ServiceTimeout reports the send budget the router gives Telegram. A send makes
// one request per chat, one after another, each bounded by the client timeout.
//
// Parameters:
//   - params: optional overrides for configuration fields, which may set the chats.
//     An invalid param fails the send itself, so the stored chats set the budget.
//
// Returns:
//   - time.Duration: the client timeout multiplied by the number of chats, and at
//     least one client timeout.
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	if s.Config == nil {
		return defaultHTTPTimeout
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		config = *s.Config
	}

	return defaultHTTPTimeout * time.Duration(max(1, len(config.Chats)))
}

// SetHTTPClient sets a custom HTTP client for the service.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.httpClient = client
}

// httpClientOrDefault returns the custom client or a default client.
func (s *Service) httpClientOrDefault() types.HTTPClient {
	if s.httpClient != nil {
		return s.httpClient
	}

	return &http.Client{Timeout: defaultHTTPTimeout}
}

// sendMessageForChatIDs sends the message to every chat in config, which includes
// any chats set by the send params.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the message to send.
//   - config: the configuration for this send.
//
// Returns:
//   - error: the first send error.
func (s *Service) sendMessageForChatIDs(ctx context.Context, message string, config *Config) error {
	for _, chat := range config.Chats {
		if err := s.sendMessageToAPI(ctx, message, chat, config); err != nil {
			return err
		}
	}

	return nil
}

// sendMessageToAPI sends a message to the Telegram API for a specific chat.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - chat: the chat to send the message to.
//   - config: the configuration for this send.
//
// Returns:
//   - error: the Telegram API error or the request error.
func (s *Service) sendMessageToAPI(ctx context.Context, message, chat string, config *Config) error {
	client := &Client{token: config.Token, httpClient: s.httpClientOrDefault()}
	payload := createSendMessagePayload(message, chat, config)
	_, err := client.SendMessageContext(ctx, &payload)

	return err
}
