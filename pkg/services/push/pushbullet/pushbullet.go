package pushbullet

import (
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
	standard.Standard

	client     jsonclient.Client
	Config     *Config
	pkr        format.PropKeyResolver
	httpClient types.HTTPClient
}

// Constants.
const (
	pushesEndpoint = "https://api.pushbullet.com/v2/pushes"
	// defaultHTTPTimeout is the timeout of the HTTP client used when none is injected.
	defaultHTTPTimeout = 10 * time.Second
)

// Static errors for push validation.
var (
	ErrUnexpectedResponseType = errors.New("unexpected response type, expected note")
	ErrResponseBodyMismatch   = errors.New("response body mismatch")
	ErrResponseTitleMismatch  = errors.New("response title mismatch")
	ErrPushNotActive          = errors.New("push notification is not active")
)

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize loads ServiceConfig from serviceURL and sets logger for this Service.
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
func (s *Service) Send(message string, params *types.Params) error {
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	for _, target := range config.Targets {
		if err := s.doSend(&config, target, message); err != nil {
			return err
		}
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	s.httpClient = client
	s.client = s.newJSONClient()
}

// doSend sends a push notification to a specific target and validates the response.
func (s *Service) doSend(config *Config, target, message string) error {
	push := NewNotePush(message, config.Title)
	push.SetTarget(target)

	response := PushResponse{}
	if err := s.client.Post(pushesEndpoint, push, &response); err != nil {
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
//   - jsonclient.Client: the API client.
func (s *Service) newJSONClient() jsonclient.Client {
	httpClient := s.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	client := jsonclient.NewWithHTTPClient(httpClient)
	if s.Config != nil {
		client.Headers().Set("Access-Token", s.Config.Token)
	}

	return client
}
