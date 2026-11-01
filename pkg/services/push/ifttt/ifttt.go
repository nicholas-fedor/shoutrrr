package ifttt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/internal/sequential"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to an IFTTT webhook.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder

	// Config holds the webhook key, the events, and the value fields.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Settings for requests to the IFTTT webhooks.
const (
	// apiURLFormat is the webhook URL template, formatted with the event name
	// and the webhook key.
	apiURLFormat = "https://maker.ifttt.com/trigger/%s/with/key/%s"
	// defaultTimeout is the part of the send budget for each event.
	defaultTimeout = 10 * time.Second
)

// Errors returned when sending.
var (
	// ErrSendFailed indicates a failure to send an IFTTT event notification.
	ErrSendFailed = errors.New("failed to send IFTTT event")
	// ErrUnexpectedStatus indicates a non-success response status.
	ErrUnexpectedStatus = errors.New("got unexpected response status code")
)

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
//   - string: the service scheme, [Scheme].
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrMissingEvents] or
//     [ErrMissingWebhookID].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{
		UseMessageAsValue: DefaultMessageValue,
	}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to an IFTTT webhook.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields and payload values.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the params,
//     payload, or send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to the IFTTT webhook of every
// configured event, one after another. A context without a deadline is bounded
// by the budget that [Service.ServiceTimeout] reports, which the requests
// share. An event that fails does not stop the events after it, and once ctx
// ends, the remaining events are skipped.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the message to send.
//   - params: optional overrides for configuration fields and payload values.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the params,
//     value field, or payload error, or the joined errors of the events that
//     failed, each wrapping [ErrSendFailed] and naming its event, and ctx's
//     error when it ended the send.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	ctx, cancel := httpclient.BoundContext(ctx, config.sendBudget())
	defer cancel()

	// Params can change the value fields, so they get the checks the URL gets.
	if err := config.validateValueFields(); err != nil {
		return fmt.Errorf("validating params: %w", err)
	}

	payload, err := createJSONToSend(config, message)
	if err != nil {
		return err
	}

	return sequential.Send(
		ctx,
		config.Events,
		sequential.PerTarget(defaultTimeout),
		func(ctx context.Context, event string) error {
			if err := s.doSend(ctx, payload, s.createAPIURLForEvent(event)); err != nil {
				return fmt.Errorf("%w: event %q: %w", ErrSendFailed, event, err)
			}

			return nil
		},
	)
}

// ServiceTimeout reports the send budget of IFTTT. A send makes one request per
// event in turn, so the budget allows 10 seconds for each.
//
// Parameters:
//   - params: optional overrides for configuration fields, which may set the
//     events. An invalid param fails the send itself, so the stored events set
//     the budget.
//
// Returns:
//   - time.Duration: 10 seconds for each event, and at least 10 seconds.
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	if s.Config == nil {
		return defaultTimeout
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		config = *s.Config
	}

	return config.sendBudget()
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// createAPIURLForEvent builds an IFTTT webhook URL for a specific event.
//
// Parameters:
//   - event: the event name.
//
// Returns:
//   - string: the webhook URL, with the configured webhook key.
func (s *Service) createAPIURLForEvent(event string) string {
	return fmt.Sprintf(apiURLFormat, event, s.Config.WebHookID)
}

// doSend executes an HTTP POST request to send the payload to the IFTTT webhook.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - payload: the JSON payload.
//   - postURL: the webhook URL for the event.
//
// Returns:
//   - error: the request error, or [ErrUnexpectedStatus] for a non-success status.
func (s *Service) doSend(ctx context.Context, payload []byte, postURL string) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		postURL,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := s.client.Client().Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request to IFTTT webhook: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
	}

	return nil
}
