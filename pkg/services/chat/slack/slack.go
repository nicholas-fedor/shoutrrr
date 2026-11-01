package slack

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/internal/httpbody"
	"github.com/nicholas-fedor/shoutrrr/internal/httpclient"
	"github.com/nicholas-fedor/shoutrrr/internal/jsonopts"
	"github.com/nicholas-fedor/shoutrrr/internal/logging"
	"github.com/nicholas-fedor/shoutrrr/internal/redact"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Service sends notifications to a pre-configured Slack channel or user.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder

	// Config holds the token, the channel, and the message options.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Settings for requests to the Slack API and webhooks.
const (
	// apiPostMessage is the Slack API endpoint for sending messages.
	apiPostMessage = "https://slack.com/api/chat.postMessage"
)

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

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrInvalidToken].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}

	s.pkr = format.NewPropKeyResolver(s.Config)

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send delivers a notification message to Slack.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the params
//     error or the send error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext delivers a notification message to Slack through the API or a webhook.
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the params
//     error or the send error, which matches ctx's error when ctx ends the
//     request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	ctx, cancel := httpclient.BoundContext(ctx, s.ServiceTimeout(params))
	defer cancel()

	// Params apply to this send only, so they update a copy of the service config.
	configCopy := *s.Config
	config := &configCopy

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.validate(); err != nil {
		return err
	}

	payload := CreateJSONPayload(config, message)

	var err error
	if config.Token.IsAPIToken() {
		err = s.sendAPI(ctx, config, &payload)
	} else {
		err = s.sendWebhook(ctx, config, &payload)
	}

	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client,
// including a nil *http.Client, selects the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// sendAPI sends a notification using the Slack API.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - payload: the message payload.
//
// Returns:
//   - error: the request error or the error the API reported.
func (s *Service) sendAPI(ctx context.Context, config *Config, payload *MessagePayload) error {
	response := APIResponse{}
	jsonClient := jsonclient.NewContextClient(s.client.Client())
	jsonClient.Headers().Set("Authorization", config.Token.Authorization())

	if err := jsonClient.PostContext(ctx, apiPostMessage, payload, &response); err != nil {
		return fmt.Errorf("posting to Slack API: %w", err)
	}

	if !response.Ok {
		if response.Error != "" {
			return fmt.Errorf("%w: %v", ErrAPIResponseFailure, response.Error)
		}

		return ErrUnknownAPIError
	}

	if response.Warning != "" {
		s.LogAttrs(ctx, slog.LevelWarn, "API returned a warning", slog.String(logging.KeyWarning, response.Warning))
	}

	return nil
}

// sendWebhook sends a notification using a Slack webhook.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - payload: the message payload.
//
// Returns:
//   - error: the request error, or the error the webhook reported with up to
//     [httpbody.MaxSize] bytes of its response.
func (s *Service) sendWebhook(ctx context.Context, config *Config, payload *MessagePayload) error {
	payloadBytes, err := json.Marshal(payload, jsonopts.Default)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		config.Token.WebhookURL(),
		bytes.NewBuffer(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", redact.URLError(err))
	}

	req.Header.Set("Content-Type", jsonclient.ContentType)

	client := s.client.Client()

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to invoke webhook: %w", redact.URLError(err))
	}

	defer func() { _ = res.Body.Close() }()

	// An oversized body is never "ok", so its first bytes are reported as the
	// webhook's error.
	resBytes, err := httpbody.Read(res.Body, httpbody.MaxSize)
	if err != nil && !errors.Is(err, httpbody.ErrTooLarge) {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	response := string(resBytes)

	switch response {
	case "":
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("%w: %v", ErrWebhookStatusFailure, res.Status)
		}

		fallthrough
	case "ok":
		return nil
	default:
		return fmt.Errorf("%w: %v", ErrWebhookResponseFailure, response)
	}
}
