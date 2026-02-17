package matrix

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Scheme identifies this service in configuration URLs.
const Scheme = "matrix"

// ErrClientNotInitialized indicates that the client is not initialized for sending messages.
var ErrClientNotInitialized = errors.New("client not initialized; cannot send message")

// Service sends notifications via the Matrix protocol.
type Service struct {
	standard.Standard
	Config *Config
	client *client
	pkr    format.PropKeyResolver
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, configURL); err != nil {
		return err
	}

	if configURL.String() != "matrix://dummy@dummy.com" {
		s.client = newClient(s.Config.Host, s.Config.DisableTLS, logger)
		if s.Config.User != "" {
			return s.client.login(s.Config.User, s.Config.Password)
		}

		s.client.accessToken = s.Config.Password
	}

	return nil
}

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
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
	errors := s.client.sendMessage(message, s.Config.Rooms)
	if len(errors) > 0 {
		for _, err := range errors {
			s.Logf("error sending message: %w", err)
		}

		return fmt.Errorf(
			"%v error(s) sending message, with initial error: %w",
			len(errors),
			errors[0],
		)
	}

	return nil
}
