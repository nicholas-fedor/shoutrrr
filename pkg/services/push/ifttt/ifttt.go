package ifttt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to an IFTTT webhook.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

// apiURLFormat defines the IFTTT webhook URL template.
const (
	apiURLFormat = "https://maker.ifttt.com/trigger/%s/with/key/%s"
)

// ErrSendFailed indicates a failure to send an IFTTT event notification.
var (
	ErrSendFailed       = errors.New("failed to send IFTTT event")
	ErrUnexpectedStatus = errors.New("got unexpected response status code")
)

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
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
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config
	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	payload, err := createJSONToSend(config, message, params)
	if err != nil {
		return err
	}

	for _, event := range config.Events {
		apiURL := s.createAPIURLForEvent(event)
		if err := doSend(payload, apiURL); err != nil {
			return fmt.Errorf("%w: event %q: %w", ErrSendFailed, event, err)
		}
	}

	return nil
}

// createAPIURLForEvent builds an IFTTT webhook URL for a specific event.
func (s *Service) createAPIURLForEvent(event string) string {
	return fmt.Sprintf(apiURLFormat, event, s.Config.WebHookID)
}

// doSend executes an HTTP POST request to send the payload to the IFTTT webhook.
func doSend(payload []byte, postURL string) error {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		postURL,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request to IFTTT webhook: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
	}

	return nil
}
