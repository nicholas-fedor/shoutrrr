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

// Send delivers an SMS message via Twilio to all configured recipients.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config

	err := service.pkr.UpdateConfigFromParams(config, params)
	if err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	var errs []error

	for _, toNumber := range config.ToNumbers {
		err := service.sendToRecipient(config, toNumber, message)
		if err != nil {
			errs = append(errs, fmt.Errorf("sending to %s: %w", toNumber, err))
		}
	}

	return errors.Join(errs...)
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)
	service.HTTPClient = DefaultHTTPClient()

	err := service.Config.setURL(&service.pkr, configURL)
	if err != nil {
		return err
	}

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}
