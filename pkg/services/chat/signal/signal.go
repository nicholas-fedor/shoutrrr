package signal

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

// Service sends notifications to Signal recipients via signal-cli-rest-api.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient types.HTTPClient
	// injectedHTTPClient is true when SetHTTPClient supplied the client.
	injectedHTTPClient bool
}

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.ContextSender    = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the configuration URL for the Signal service
//   - logger: the logger to use for logging
//
// Returns:
//   - error: if configuration fails, nil otherwise
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	if s.httpClient == nil {
		s.httpClient = s.newHTTPClient(s.Config.SkipTLSVerify)
	}

	return nil
}

// Send delivers a notification message to Signal recipients.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the text message to send.
//   - params: optional configuration overrides including title and attachments.
//
// Returns:
//   - error: a params error, [ErrNoRecipients], or the joined errors of the
//     failed requests.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Signal recipients.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests, each further bounded by
//     [defaultHTTPTimeout].
//   - message: the text message to send.
//   - params: optional configuration overrides including title and attachments.
//
// Returns:
//   - error: a params error, [ErrNoRecipients], or the joined errors of the
//     failed requests. A request error matches ctx's error when ctx ends the
//     request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	return s.sendMessage(ctx, message, &config)
}

// ServiceTimeout reports the send budget the router gives Signal. A send makes
// one request per recipient batch, one after another, each bounded by
// [defaultHTTPTimeout].
//
// Parameters:
//   - params: optional overrides for configuration fields, which may set the
//     recipients. An invalid param fails the send itself, so the stored
//     recipients set the budget.
//
// Returns:
//   - time.Duration: [defaultHTTPTimeout] multiplied by the number of recipient
//     batches, and at least [defaultHTTPTimeout].
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	if s.Config == nil {
		return defaultHTTPTimeout
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		config = *s.Config
	}

	return defaultHTTPTimeout * time.Duration(max(1, len(batchRecipients(config.Recipients))))
}

// SetHTTPClient sets a custom HTTP client for the service.
//
// Parameters:
//   - client: the HTTP client to use for API requests
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.httpClient = client
	s.injectedHTTPClient = client != nil
}

// sendMessage sends a message to all configured recipients.
// Mixed recipient types are sent as separate /v2/send calls because the REST API
// rejects phones, groups, and usernames in the same request.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the message text to send.
//   - config: the service configuration.
//
// Returns:
//   - error: [ErrNoRecipients], or the joined errors of the failed requests.
func (s *Service) sendMessage(ctx context.Context, message string, config *Config) error {
	if len(config.Recipients) == 0 {
		return ErrNoRecipients
	}

	var errs []error

	for _, batch := range batchRecipients(config.Recipients) {
		batchConfig := *config
		batchConfig.Recipients = batch

		payload := createPayload(message, &batchConfig)

		req, cancel, err := s.createRequest(ctx, &batchConfig, &payload)
		if err != nil {
			if cancel != nil {
				cancel()
			}

			errs = append(errs, err)

			continue
		}

		err = s.sendRequest(req, &batchConfig)

		cancel()

		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
