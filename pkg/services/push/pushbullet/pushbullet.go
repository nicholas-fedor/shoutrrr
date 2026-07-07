package pushbullet

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

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
)

// Static errors for push validation.
var (
	ErrUnexpectedResponseType = errors.New("unexpected response type, expected note")
	ErrResponseBodyMismatch   = errors.New("response body mismatch")
	ErrResponseTitleMismatch  = errors.New("response title mismatch")
	ErrPushNotActive          = errors.New("push notification is not active")
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

	s.client = jsonclient.NewWithHTTPClient(s.httpClientOrDefault())
	s.client.Headers().Set("Access-Token", s.Config.Token)

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

// SetHTTPClient sets a custom HTTP client for the service.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.httpClient = client
	if client != nil {
		s.client = jsonclient.NewWithHTTPClient(client)
	}
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

// httpClientOrDefault returns the injected client or a default Client.
func (s *Service) httpClientOrDefault() *http.Client {
	if s.httpClient != nil {
		if c, ok := s.httpClient.(*http.Client); ok {
			return c
		}
	}

	if s.httpClient == nil {
		return &http.Client{}
	}

	return &http.Client{}
}
