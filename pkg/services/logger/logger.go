package logger

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service is the Logger service struct.
type Service struct {
	standard.Standard
	Config *Config
}

// Send a notification message to log.
func (service *Service) Send(message string, params *types.Params) error {
	data := types.Params{}

	if params != nil {
		for key, value := range *params {
			data[key] = value
		}
	}

	data["message"] = message

	return service.doSend(data)
}

func (service *Service) doSend(data types.Params) error {
	msg := data["message"]

	if tpl, found := service.GetTemplate("message"); found {
		wc := &strings.Builder{}
		if err := tpl.Execute(wc, data); err != nil {
			return fmt.Errorf("failed to write template to log: %w", err)
		}

		msg = wc.String()
	}

	service.Log(msg)

	return nil
}

// Initialize loads ServiceConfig from configURL and sets logger for this Service.
func (service *Service) Initialize(_ *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}
