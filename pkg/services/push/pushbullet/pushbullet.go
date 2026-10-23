package pushbullet

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// Service providing Pushbullet as a notification service.
type Service struct {
	// Standard provides logging and the other shared service functionality.
	standard.Standard

	// client is the JSON client the requests go through, built on
	// httpClient.
	client jsonclient.ContextClient
	// httpClient is the injected HTTP client, or nil for a default client.
	httpClient types.HTTPClient
	// Config holds the access token, the title, and the targets.
	Config *Config
	// pkr reads and writes the config props by their query keys.
	pkr format.PropKeyResolver
}

// Settings for requests to the Pushbullet API.
const (
	// pushesEndpoint is the API endpoint for creating pushes.
	pushesEndpoint = "https://api.pushbullet.com/v2/pushes"
	// defaultHTTPTimeout is the timeout of the HTTP client used when none is injected.
	defaultHTTPTimeout = 10 * time.Second
)

// Static errors for push validation.
var (
	// ErrUnexpectedResponseType indicates a created push that is not a note.
	ErrUnexpectedResponseType = errors.New("unexpected response type, expected note")
	// ErrResponseBodyMismatch indicates a created push whose body differs
	// from the message.
	ErrResponseBodyMismatch = errors.New("response body mismatch")
	// ErrResponseTitleMismatch indicates a created push whose title differs
	// from the one sent.
	ErrResponseTitleMismatch = errors.New("response title mismatch")
	// ErrPushNotActive indicates a created push that is already inactive.
	ErrPushNotActive = errors.New("push notification is not active")
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

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	s.client = s.newJSONClient()

	return nil
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
//   - error: the params error or the first push error.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendContext(context.Background(), message, params)
}

// SendContext sends a push notification to every configured Pushbullet target.
//
// Parameters:
//   - ctx: cancellation and deadline for the requests.
//   - message: the message to send.
//   - params: optional overrides for configuration fields.
//
// Returns:
//   - error: the params error or the first push error, which matches ctx's
//     error when ctx ends the request.
func (s *Service) SendContext(ctx context.Context, message string, params *types.Params) error {
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	for _, target := range config.Targets {
		if err := s.doSend(ctx, &config, target, message); err != nil {
			return err
		}
	}

	return nil
}

// ServiceTimeout reports the send budget the router gives Pushbullet. A send makes
// one request per target, one after another, each bounded by the client timeout.
//
// Parameters:
//   - params: optional overrides for configuration fields. An invalid param fails
//     the send itself, so the stored targets set the budget.
//
// Returns:
//   - time.Duration: the client timeout multiplied by the number of targets, and
//     at least one client timeout.
func (s *Service) ServiceTimeout(params *types.Params) time.Duration {
	if s.Config == nil {
		return defaultHTTPTimeout
	}

	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		config = *s.Config
	}

	return defaultHTTPTimeout * time.Duration(max(1, len(config.Targets)))
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client.
//
// Parameters:
//   - client: the HTTP client to use for requests, or nil for the default.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	s.httpClient = client
	s.client = s.newJSONClient()
}

// doSend sends a push notification to a specific target and validates the response.
//
// Parameters:
//   - ctx: cancellation and deadline for the request.
//   - config: the configuration for this send.
//   - target: the device, channel, or email to push to.
//   - message: the message to send.
//
// Returns:
//   - error: the API error, the request error, or a response validation error.
func (s *Service) doSend(ctx context.Context, config *Config, target, message string) error {
	push := NewNotePush(message, config.Title)
	push.SetTarget(target)

	response := PushResponse{}
	if err := s.client.PostContext(ctx, pushesEndpoint, push, &response); err != nil {
		errorResponse := &ResponseError{}
		if s.client.ErrorResponse(err, errorResponse) {
			return fmt.Errorf("API error: %w", errorResponse)
		}

		return fmt.Errorf("failed to push: %w", err)
	}

	// Validate response fields
	if response.Type != "note" {
		return fmt.Errorf("%w: got %s", ErrUnexpectedResponseType, response.Type)
	}

	if response.Body != message {
		return fmt.Errorf(
			"%w: got %s, expected %s",
			ErrResponseBodyMismatch,
			response.Body,
			message,
		)
	}

	if response.Title != config.Title {
		return fmt.Errorf(
			"%w: got %s, expected %s",
			ErrResponseTitleMismatch,
			response.Title,
			config.Title,
		)
	}

	if !response.Active {
		return ErrPushNotActive
	}

	return nil
}

// newJSONClient builds the API client from the injected or default HTTP client.
// It sets the access token on every client it builds, so replacing the HTTP client
// keeps requests authenticated.
//
// Returns:
//   - jsonclient.ContextClient: the API client.
func (s *Service) newJSONClient() jsonclient.ContextClient {
	httpClient := s.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	client := jsonclient.NewContextClient(httpClient)
	if s.Config != nil {
		client.Headers().Set("Access-Token", s.Config.Token)
	}

	return client
}
