package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// apiPostMessage is the Slack API endpoint for sending messages.
const (
	apiPostMessage     = "https://slack.com/api/chat.postMessage"
	defaultHTTPTimeout = 10 * time.Second // defaultHTTPTimeout is the default timeout for HTTP requests.
)

// Service sends notifications to a pre-configured Slack channel or user.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
	client *http.Client
}

// Send delivers a notification message to Slack.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config

	if err := service.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	payload := CreateJSONPayload(config, message)

	var err error
	if config.Token.IsAPIToken() {
		err = service.sendAPI(config, payload)
	} else {
		err = service.sendWebhook(config, payload)
	}

	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}

	return nil
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)
	service.client = &http.Client{
		Timeout: defaultHTTPTimeout,
	}

	return service.Config.setURL(&service.pkr, configURL)
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// sendAPI sends a notification using the Slack API.
func (service *Service) sendAPI(config *Config, payload any) error {
	response := APIResponse{}
	jsonClient := jsonclient.NewClient()
	jsonClient.Headers().Set("Authorization", config.Token.Authorization())

	if err := jsonClient.Post(apiPostMessage, payload, &response); err != nil {
		return fmt.Errorf("posting to Slack API: %w", err)
	}

	if !response.Ok {
		if response.Error != "" {
			return fmt.Errorf("%w: %v", ErrAPIResponseFailure, response.Error)
		}

		return ErrUnknownAPIError
	}

	if response.Warning != "" {
		service.Logf("Slack API warning: %q", response.Warning)
	}

	return nil
}

// sendWebhook sends a notification using a Slack webhook.
func (service *Service) sendWebhook(config *Config, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		config.Token.WebhookURL(),
		bytes.NewBuffer(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", jsonclient.ContentType)

	res, err := service.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to invoke webhook: %w", err)
	}

	defer res.Body.Close()

	resBytes, _ := io.ReadAll(res.Body)
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
