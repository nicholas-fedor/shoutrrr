package bark

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

// Service sends notifications to Bark.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

var (
	ErrFailedAPIRequest   = errors.New("failed to make API request")
	ErrUnexpectedStatus   = errors.New("unexpected status code")
	ErrUpdateParamsFailed = errors.New("failed to update config from params")
)

// GetID returns the identifier for the Bark service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize sets up the Service with configuration from configURL and assigns a logger.
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	_ = s.pkr.SetDefaultProps(s.Config)

	return s.Config.setURL(&s.pkr, configURL)
}

// Send transmits a notification message to Bark.
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("%w: %w", ErrUpdateParamsFailed, err)
	}

	if err := s.sendAPI(config, message); err != nil {
		return fmt.Errorf("failed to send bark notification: %w", err)
	}

	return nil
}

func (s *Service) sendAPI(config *Config, message string) error {
	response := APIResponse{}
	request := PushPayload{
		Body:      message,
		DeviceKey: config.DeviceKey,
		Title:     config.Title,
		Category:  config.Category,
		Copy:      config.Copy,
		Sound:     config.Sound,
		Group:     config.Group,
		Badge:     &config.Badge,
		Icon:      config.Icon,
		URL:       config.URL,
	}
	jsonClient := jsonclient.NewClient()

	if err := jsonClient.Post(config.GetAPIURL("push"), &request, &response); err != nil {
		if jsonClient.ErrorResponse(err, &response) {
			return &response
		}

		return fmt.Errorf("%w: %w", ErrFailedAPIRequest, err)
	}

	if response.Code != http.StatusOK {
		if response.Message != "" {
			return &response
		}

		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, response.Code)
	}

	return nil
}
