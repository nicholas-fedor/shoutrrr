package generic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// JSONTemplate identifies the JSON format for webhook payloads.
const (
	JSONTemplate = "JSON"
)

// ErrSendFailed indicates a failure to send a notification to the generic webhook.
var (
	ErrSendFailed        = errors.New("failed to send notification to generic webhook")
	ErrUnexpectedStatus  = errors.New("server returned unexpected response status code")
	ErrTemplateNotLoaded = errors.New("template has not been loaded")
)

// Service implements a generic notification service for custom webhooks.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to a generic webhook endpoint.
func (service *Service) Send(message string, paramsPtr *types.Params) error {
	config := *service.Config

	var params types.Params
	if paramsPtr == nil {
		params = types.Params{}
	} else {
		params = *paramsPtr
	}

	if err := service.pkr.UpdateConfigFromParams(&config, &params); err != nil {
		service.Logf("Failed to update params: %v", err)
	}

	sendParams := createSendParams(&config, params, message)
	if err := service.doSend(&config, sendParams); err != nil {
		return fmt.Errorf("%w: %s", ErrSendFailed, err.Error())
	}

	return nil
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)

	config, pkr := DefaultConfig()
	service.Config = config
	service.pkr = pkr

	return service.Config.setURL(&service.pkr, configURL)
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// GetConfigURLFromCustom converts a custom webhook URL into a standard service URL.
func (*Service) GetConfigURLFromCustom(customURL *url.URL) (*url.URL, error) {
	webhookURL := *customURL
	if strings.HasPrefix(webhookURL.Scheme, Scheme) {
		webhookURL.Scheme = webhookURL.Scheme[len(Scheme)+1:]
	}

	config, pkr, err := ConfigFromWebhookURL(webhookURL)
	if err != nil {
		return nil, err
	}

	return config.getURL(&pkr), nil
}

// doSend executes the HTTP request to send a notification to the webhook.
func (service *Service) doSend(config *Config, params types.Params) error {
	postURL := config.WebhookURL().String()

	payload, err := service.GetPayload(config, params)
	if err != nil {
		return err
	}

	ctx := context.Background()

	req, err := http.NewRequestWithContext(ctx, config.RequestMethod, postURL, payload)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", config.ContentType)
	req.Header.Set("Accept", config.ContentType)

	for key, value := range config.headers {
		req.Header.Set(key, value)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request: %w", err)
	}

	if res != nil && res.Body != nil {
		defer res.Body.Close()

		if body, err := io.ReadAll(res.Body); err == nil {
			service.Log("Server response: ", string(body))
		}
	}

	if res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %s", ErrUnexpectedStatus, res.Status)
	}

	return nil
}

// GetPayload prepares the request payload based on the configured template.
func (service *Service) GetPayload(config *Config, params types.Params) (io.Reader, error) {
	switch config.Template {
	case "":
		return bytes.NewBufferString(params[config.MessageKey]), nil
	case "json", JSONTemplate:
		for key, value := range config.extraData {
			params[key] = value
		}

		jsonBytes, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshaling params to JSON: %w", err)
		}

		return bytes.NewBuffer(jsonBytes), nil
	}

	tpl, found := service.GetTemplate(config.Template)
	if !found {
		return nil, fmt.Errorf("%w: %q", ErrTemplateNotLoaded, config.Template)
	}

	bb := &bytes.Buffer{}
	if err := tpl.Execute(bb, params); err != nil {
		return nil, fmt.Errorf("executing template %q: %w", config.Template, err)
	}

	return bb, nil
}

// createSendParams constructs parameters for sending a notification.
func createSendParams(config *Config, params types.Params, message string) types.Params {
	sendParams := types.Params{}

	for key, val := range params {
		if key == types.TitleKey {
			key = config.TitleKey
		}

		sendParams[key] = val
	}

	sendParams[config.MessageKey] = message

	return sendParams
}
