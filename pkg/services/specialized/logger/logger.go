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
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// Config is the service configuration, set by Initialize.
	Config *Config
}

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service        = (*Service)(nil)
	_ types.ServiceTimeout = (*Service)(nil)
)

// GetID returns the service identifier.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize sets logger for this Service and gives it an empty Config. The
// service URL is ignored, since the logger service has no settings.
//
// Parameters:
//   - logger: the logger that receives the messages.
//
// Returns:
//   - error: always nil.
func (s *Service) Initialize(_ *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}

	return nil
}

// Send writes a notification message to the log, through the message template
// when one is set.
//
// Parameters:
//   - message: the message to log.
//   - params: optional values for the message template.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, or the
//     template error.
func (s *Service) Send(message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	data := types.Params{}

	if params != nil {
		maps.Copy(data, *params)
	}

	data["message"] = message

	return s.doSend(data)
}

// doSend logs the message in data, rendered through the message template when
// one is set.
//
// Parameters:
//   - data: the template values, with the message under the message key.
//
// Returns:
//   - error: the template execution error.
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
