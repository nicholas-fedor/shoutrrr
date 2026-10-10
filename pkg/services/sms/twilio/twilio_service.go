package twilio

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

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

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.HTTPClient = DefaultHTTPClient()

	err := s.Config.setURL(&s.pkr, serviceURL)
	if err != nil {
		return err
	}

	return nil
}

// Send delivers an SMS message via Twilio to all configured recipients.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: a params error, or the joined errors of the failed recipients.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers an SMS message via Twilio to all configured recipients.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests, each further bounded by
//     [defaultHTTPTimeout].
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: a params error, or the joined errors of the failed recipients. A
//     request error matches ctx's error when ctx ends the request. When ctx
//     ends, the remaining recipients are skipped and ctx's error is included.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	err := s.pkr.UpdateConfigFromParams(config, params)
	if err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	var errs []error

	for _, toNumber := range config.ToNumbers {
		// Once ctx ends, every remaining request would fail with its error, so the
		// send stops and reports that error once.
		if err := ctx.Err(); err != nil {
			if !errors.Is(errors.Join(errs...), err) {
				errs = append(errs, err)
			}

			break
		}

		err := s.sendToRecipient(ctx, config, toNumber, message)
		if err != nil {
			errs = append(errs, fmt.Errorf("sending to %s: %w", toNumber, err))
		}
	}

	return errors.Join(errs...)
}

// ServiceTimeout reports the send budget the router gives Twilio. A send makes
// one request per recipient, one after another, each bounded by
// [defaultHTTPTimeout].
//
// Parameters:
//   - _: unused, because the recipients come only from the service URL.
//
// Returns:
//   - time.Duration: [defaultHTTPTimeout] multiplied by the number of
//     recipients, and at least [defaultHTTPTimeout].
func (s *Service) ServiceTimeout(_ *types.Params) time.Duration {
	if s.Config == nil {
		return defaultHTTPTimeout
	}

	return defaultHTTPTimeout * time.Duration(max(1, len(s.Config.ToNumbers)))
}

// SetHTTPClient sets a custom HTTP client for the service.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if client == nil {
		s.HTTPClient = DefaultHTTPClient()

		return
	}

	s.HTTPClient = client
}
