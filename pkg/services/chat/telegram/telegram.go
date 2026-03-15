package telegram

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to configured Telegram chats.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

// apiFormat defines the Telegram API endpoint template.
const (
	apiFormat = "https://api.telegram.org/bot%s/%s"
	maxlength = 4096
)

// ErrMessageTooLong indicates that the message exceeds the maximum allowed length.
var (
	ErrMessageTooLong = errors.New("Message exceeds the max length")
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
func (s *Service) Send(message string, params *types.Params) error {
	if len(message) > maxlength {
		return ErrMessageTooLong
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	return s.sendMessageForChatIDs(message, &config)
}

// sendMessageForChatIDs sends the message to all configured chat IDs.
func (s *Service) sendMessageForChatIDs(message string, config *Config) error {
	for _, chat := range s.Config.Chats {
		if err := sendMessageToAPI(message, chat, config); err != nil {
			return err
		}
	}

	return nil
}

// sendMessageToAPI sends a message to the Telegram API for a specific chat.
func sendMessageToAPI(message, chat string, config *Config) error {
	client := &Client{token: config.Token}
	payload := createSendMessagePayload(message, chat, config)
	_, err := client.SendMessage(&payload)

	return err
}
