package signal

import (
	"errors"
	"fmt"
	"net/url"

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

var (
	_ types.Service          = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
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
// Parameters:
//   - message: the text message to send
//   - params: optional configuration overrides including title and attachments
//
// Returns:
//   - error: if the send operation fails, nil otherwise
func (s *Service) Send(message string, params *types.Params) error {
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	return s.sendMessage(message, &config)
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
//   - message: the message text to send
//   - config: the service configuration
//
// Returns:
//   - error: if sending fails, nil otherwise
func (s *Service) sendMessage(message string, config *Config) error {
	if len(config.Recipients) == 0 {
		return ErrNoRecipients
	}

	var errs []error

	for _, batch := range batchRecipients(config.Recipients) {
		batchConfig := *config
		batchConfig.Recipients = batch

		payload := createPayload(message, &batchConfig)

		req, cancel, err := s.createRequest(&batchConfig, &payload)
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
