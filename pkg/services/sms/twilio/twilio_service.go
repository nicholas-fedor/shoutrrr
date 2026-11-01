package twilio

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/sequential"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service provides the Twilio SMS notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// HTTPClient sends the requests to the Twilio API when set. It stays nil
	// after Initialize until the caller sets it, and the sends use the default
	// client meanwhile. [Service.SetHTTPClient] clears it, so whichever of the
	// two the caller set last carries the sends.
	//
	// Deprecated: Use [Service.SetHTTPClient].
	HTTPClient types.HTTPClient

	// Config holds the credentials, the sender, the recipients, and the title.
	Config *Config
	// client holds the client from [Service.SetHTTPClient] and the default
	// client.
	client httpclient.Holder
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the service identifier.
//
// Returns:
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger. It keeps an HTTP
// client that is already set.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for the service.
//
// Returns:
//   - error: the error from parsing or validating the service URL.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

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
//   - error: [types.ErrServiceNotInitialized] before Initialize, a params
//     error, or the joined errors of the failed recipients.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers an SMS message via Twilio to all configured recipients.
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports, which the requests share.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, a params
//     error, or the joined errors of the failed recipients. A request error
//     matches ctx's error when ctx ends the request. When ctx ends, the
//     remaining recipients are skipped and ctx's error is included.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	err := s.pkr.UpdateConfigFromParams(config, params)
	if err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.validate(); err != nil {
		return err
	}

	return sequential.Send(
		ctx,
		config.ToNumbers,
		sequential.PerTarget(defaultHTTPTimeout),
		func(ctx context.Context, toNumber string) error {
			if err := s.sendToRecipient(ctx, config, toNumber, message); err != nil {
				return fmt.Errorf("sending to %s: %w", toNumber, err)
			}

			return nil
		},
	)
}

// ServiceTimeout reports the send budget of Twilio. A send makes one request
// per recipient, one after another, so the budget allows [defaultHTTPTimeout]
// for each.
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

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client. It clears the
// deprecated HTTPClient field, so the sends use this client until that field
// is set again.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
	s.HTTPClient = nil
}
