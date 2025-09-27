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

var (
	ErrFailedAPIRequest   = errors.New("failed to make API request")
	ErrUnexpectedStatus   = errors.New("unexpected status code")
	ErrUpdateParamsFailed = errors.New("failed to update config from params")
)

// Service sends notifications to Bark.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send transmits a notification message to Bark.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config

	if err := service.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("%w: %w", ErrUpdateParamsFailed, err)
	}

	if err := service.sendAPI(config, message); err != nil {
		return fmt.Errorf("failed to send bark notification: %w", err)
	}

	return nil
}

// Initialize sets up the Service with configuration from configURL and assigns a logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	_ = service.pkr.SetDefaultProps(service.Config)

	return service.Config.setURL(&service.pkr, configURL)
}

// GetID returns the identifier for the Bark service.
func (service *Service) GetID() string {
	return Scheme
}

func (service *Service) sendAPI(config *Config, message string) error {
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
