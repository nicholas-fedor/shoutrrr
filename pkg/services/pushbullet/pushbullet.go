package pushbullet

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

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

// Service providing Pushbullet as a notification service.
type Service struct {
	standard.Standard
	client jsonclient.Client
	Config *Config
	pkr    format.PropKeyResolver
}

// Initialize loads ServiceConfig from configURL and sets logger for this Service.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)

	service.Config = &Config{
		Title: "Shoutrrr notification", // Explicitly set default
	}
	service.pkr = format.NewPropKeyResolver(service.Config)

	if err := service.Config.setURL(&service.pkr, configURL); err != nil {
		return err
	}

	service.client = jsonclient.NewClient()
	service.client.Headers().Set("Access-Token", service.Config.Token)

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// Send a push notification via Pushbullet.
func (service *Service) Send(message string, params *types.Params) error {
	config := *service.Config
	if err := service.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	for _, target := range config.Targets {
		if err := doSend(&config, target, message, service.client); err != nil {
			return err
		}
	}

	return nil
}

// doSend sends a push notification to a specific target and validates the response.
func doSend(config *Config, target string, message string, client jsonclient.Client) error {
	push := NewNotePush(message, config.Title)
	push.SetTarget(target)

	response := PushResponse{}
	if err := client.Post(pushesEndpoint, push, &response); err != nil {
		errorResponse := &ResponseError{}
		if client.ErrorResponse(err, errorResponse) {
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
