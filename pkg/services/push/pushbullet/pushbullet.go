package pushbullet

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
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Service providing Pushbullet as a notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client holds the injected HTTP client and the default client.
	client httpclient.Holder
	// Config holds the access token, the title, and the targets.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Settings for requests to the Pushbullet API.
const (
	// pushesEndpoint is the API endpoint for creating pushes.
	pushesEndpoint = "https://api.pushbullet.com/v2/pushes"
	// defaultHTTPTimeout is the part of the send budget for each target.
	defaultHTTPTimeout = 10 * time.Second
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

// Initialize loads ServiceConfig from serviceURL and sets logger for this Service.
//
// Parameters:
//   - serviceURL: the service URL.
//   - logger: the logger for service output.
//
// Returns:
//   - error: the error from reading the URL, such as [ErrTokenIncorrectSize].
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{
		Title: "Shoutrrr notification", // Explicitly set default
	}
	s.pkr = format.NewPropKeyResolver(s.Config)

	return s.Config.setURL(&s.pkr, serviceURL)
}

// Send a push notification via Pushbullet.
//
// It delegates to [Service.SendContext] with [context.Background].
//
// Parameters:
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the params
//     error, or the joined errors of the targets that failed.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext sends a push notification to every configured Pushbullet target.
// A context without a deadline is bounded by the budget that
// [Service.ServiceTimeout] reports, which the requests share. A target that
// fails does not stop the targets after it, and once ctx ends, the remaining
// targets are skipped.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: [types.ErrServiceNotInitialized] before Initialize, the params
//     error, or the joined errors of the targets that failed, each naming its
//     target, and ctx's error when it ended the send.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	if s.Config == nil {
		return types.ErrServiceNotInitialized
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := config.prepare(); err != nil {
		return err
	}

	ctx, cancel := httpclient.BoundContext(ctx, config.sendBudget())
	defer cancel()

	client := s.newJSONClient()

	return sequential.Send(
		ctx,
		config.sendTargets(),
		sequential.PerTarget(defaultHTTPTimeout),
		func(ctx context.Context, target string) error {
			if err := doSend(ctx, client, &config, target, message); err != nil {
				return fmt.Errorf("sending to target %q: %w", target, err)
			}

			return nil
		},
	)
}

// ServiceTimeout reports the send budget of Pushbullet. A send makes one request
// per target in turn, so the budget allows 10 seconds for each.
//
// Parameters:
//   - params: optional overrides for configuration fields. An invalid param fails
//     the send itself, so the stored targets set the budget.
//
// Returns:
//   - time.Duration: 10 seconds for each target, and at least 10 seconds.
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	if s.Config == nil {
		return defaultHTTPTimeout
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		config = *s.Config
	}

	return config.sendBudget()
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.client.Set(client)
}

// newJSONClient builds the API client from the injected or default HTTP client,
// with the access token set, so every request is authenticated whichever HTTP
// client it goes through. The service must be initialized.
//
// Returns:
//   - jsonclient.ContextClient: the API client.
func (s *Service) newJSONClient() jsonclient.ContextClient {
	client := jsonclient.NewContextClient(s.client.Client())
	client.Headers().Set("Access-Token", s.Config.Token)

	return client
}

// doSend sends a push notification to a specific target. The API answers
// every failure with an error status, so a push counts as sent once the API
// accepts it, whatever its copy of the push says.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - client: the API client of the send.
//   - config: the configuration for this send.
//   - target: the device, channel, or email to push to.
//   - message: the message to send.
//
// Returns:
//   - error: the API error or the request error.
func doSend(ctx context.Context, client jsonclient.ContextClient, config *Config, target, message string) error {
	push := NewNotePush(message, config.Title)
	push.SetTarget(target)

	// The reply is the created push. It is decoded only to reject a reply that
	// is not JSON, such as a page from an intermediate server.
	if err := client.PostContext(ctx, pushesEndpoint, push, &struct{}{}); err != nil {
		// Every Pushbullet error has a message. A JSON body without one, such
		// as the reply a redirect carries, is not the API's error.
		errorResponse := &ResponseError{}
		if client.ErrorResponse(err, errorResponse) && errorResponse.ErrorData.Message != "" {
			return fmt.Errorf("API error: %w", errorResponse)
		}

		return fmt.Errorf("failed to push: %w", err)
	}

	return nil
}
