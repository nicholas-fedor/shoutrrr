package matrix

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications via the Matrix protocol.
type Service struct {
	standard.Standard

	Config *Config
	client *client
	pkr    format.PropKeyResolver
}

// Scheme identifies this service in configuration URLs.
const Scheme = "matrix"

// ErrClientNotInitialized indicates that the client is not initialized for sending messages.
var ErrClientNotInitialized = errors.New("client not initialized; cannot send message")

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{
		EnumlessConfig: standard.EnumlessConfig{},
		User:           "",
		Password:       "",
		DisableTLS:     false,
		Host:           "",
		Rooms:          nil,
		Title:          "",
	}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	if serviceURL.String() != "matrix://dummy@dummy.com" {
		s.client = newClient(s.Config.Host, s.Config.DisableTLS, logger)
		if s.Config.User != "" {
			return s.client.login(s.Config.User, s.Config.Password)
		}

		s.client.useToken(s.Config.Password)
	}

	return nil
}

// Send delivers a notification message to Matrix rooms.
func (s *Service) Send(message string, params *types.Params) error {
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if s.client == nil {
		return ErrClientNotInitialized
	}

	sendErrors := s.client.sendMessage(message, s.Config.Rooms)
	if len(sendErrors) > 0 {
		for _, err := range sendErrors {
			s.Logf("error sending message: %w", err)
		}

		return fmt.Errorf(
			"%v error(s) sending message, with initial error: %w",
			len(sendErrors),
			sendErrors[0],
		)
	}

	return nil
}
