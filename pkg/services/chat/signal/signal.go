package signal

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Signal recipients via signal-cli-rest-api.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default clients. A send
	// that skips certificate verification uses the insecure default client.
	client httpclient.Holder
	// warnings logs each warning about the service's settings once.
	warnings logging.Once

	// Config holds the server, the credentials, the source and recipients,
	// and the message options.
	Config *Config
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

// GetID returns the identifier for this service.
//
// Returns:
//   - string: the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger. It performs no
// network I/O.
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
	s.warnings.Reset()

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	return s.Config.setURL(&s.pkr, serviceURL)
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
//   - error: [types.ErrServiceNotInitialized] before Initialize, a params
//     error, [ErrNoRecipients], or the joined errors of the failed requests.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Signal recipients.
//
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the text message to send.
//   - params: optional configuration overrides including title and attachments.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, a params
//     error, [ErrNoRecipients], or the joined errors of the failed requests. A
//     request error matches ctx's error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// Plain HTTP applies to an injected client too, because the URL decides it.
	if config.DisableTLS {
		s.warnings.Plaintext(ctx, s)
	}

	return s.sendMessage(ctx, message, &config)
}

// ServiceTimeout reports Signal's send budget. A send makes one request per
// recipient batch, one after another, and each batch adds [batchBudget].
//
// Parameters:
//   - params: optional overrides for configuration fields, which may set the
//     recipients. An invalid param fails the send itself, so the stored
//     recipients set the budget.
//
// Returns:
//   - time.Duration: [batchBudget] multiplied by the number of recipient
//     batches, and at least [batchBudget].
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	if s.Config == nil {
		return batchBudget
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		config = *s.Config
	}

	return batchBudget * time.Duration(max(1, len(batchRecipients(config.Recipients))))
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
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
//   - error: [ErrNoRecipients], [ErrInvalidRecipient], or the joined errors of
//     the failed requests. When ctx ends, the remaining batches are skipped
//     and ctx's error is included.
func (s *Service) sendMessage(ctx context.Context, message string, config *Config) error {
	if len(config.Recipients) == 0 {
		return ErrNoRecipients
	}

	// Params set recipients without the URL's validation.
	if err := validateRecipients(config.Recipients); err != nil {
		return err
	}

	var errs []error

	for _, batch := range batchRecipients(config.Recipients) {
		// Once ctx ends, every remaining request would fail with its error, so the
		// send stops and reports that error once.
		if err := ctx.Err(); err != nil {
			if !errors.Is(errors.Join(errs...), err) {
				errs = append(errs, err)
			}

			break
		}

		batchConfig := *config
		batchConfig.Recipients = batch

		payload := createPayload(message, &batchConfig)

		req, err := s.createRequest(ctx, &batchConfig, &payload)
		if err != nil {
			errs = append(errs, err)

			continue
		}

		if err := s.sendRequest(req, &batchConfig); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
