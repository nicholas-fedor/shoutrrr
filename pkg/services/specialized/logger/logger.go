package logger

import (
	"fmt"
	"maps"
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
func (s *Service) Send(message string, params *types.Params) error {
	data := types.Params{}

	if params != nil {
		maps.Copy(data, *params)
	}

	data["message"] = message

	return s.doSend(data)
}

func (s *Service) doSend(data types.Params) error {
	msg := data["message"]

	if tpl, found := s.GetTemplate("message"); found {
		wc := &strings.Builder{}
		if err := tpl.Execute(wc, data); err != nil {
			return fmt.Errorf("failed to write template to log: %w", err)
		}

		msg = wc.String()
	}

	s.Log(msg)

	return nil
}

// Initialize loads ServiceConfig from configURL and sets logger for this Service.
func (s *Service) Initialize(_ *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}

	return nil
}

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}
