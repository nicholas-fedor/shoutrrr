package telegram

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// apiFormat defines the Telegram API endpoint template.
const (
	apiFormat = "https://api.telegram.org/bot%s/%s"
	maxlength = 4096
)

// ErrMessageTooLong indicates that the message exceeds the maximum allowed length.
var (
	ErrMessageTooLong = errors.New("Message exceeds the max length")
)

// Service sends notifications to configured Telegram chats.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to Telegram.
func (service *Service) Send(message string, params *types.Params) error {
	if len(message) > maxlength {
		return ErrMessageTooLong
	}

	config := *service.Config
	if err := service.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	return service.sendMessageForChatIDs(message, &config)
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{
		Preview:      true,
		Notification: true,
	}
	service.pkr = format.NewPropKeyResolver(service.Config)

	if err := service.Config.setURL(&service.pkr, configURL); err != nil {
		return err
	}

	return nil
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// sendMessageForChatIDs sends the message to all configured chat IDs.
func (service *Service) sendMessageForChatIDs(message string, config *Config) error {
	for _, chat := range service.Config.Chats {
		if err := sendMessageToAPI(message, chat, config); err != nil {
			return err
		}
	}

	return nil
}

// GetConfig returns the current configuration for the service.
func (service *Service) GetConfig() *Config {
	return service.Config
}

// sendMessageToAPI sends a message to the Telegram API for a specific chat.
func sendMessageToAPI(message string, chat string, config *Config) error {
	client := &Client{token: config.Token}
	payload := createSendMessagePayload(message, chat, config)
	_, err := client.SendMessage(&payload)

	return err
}
