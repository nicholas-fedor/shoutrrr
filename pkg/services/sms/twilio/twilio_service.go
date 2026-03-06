package twilio

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service provides the Twilio SMS notification service.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	HTTPClient HTTPClient
}

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.HTTPClient = DefaultHTTPClient()

	err := s.Config.setURL(&s.pkr, configURL)
	if err != nil {
		return err
	}

	return nil
}

// Send delivers an SMS message via Twilio to all configured recipients.
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config

	err := s.pkr.UpdateConfigFromParams(config, params)
	if err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	var errs []error

	for _, toNumber := range config.ToNumbers {
		err := s.sendToRecipient(config, toNumber, message)
		if err != nil {
			errs = append(errs, fmt.Errorf("sending to %s: %w", toNumber, err))
		}
	}

	return errors.Join(errs...)
}
