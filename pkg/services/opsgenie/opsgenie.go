package opsgenie

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// alertEndpointTemplate is the OpsGenie API endpoint template for sending alerts.
const (
	alertEndpointTemplate = "https://%s:%d/v2/alerts"
	MaxMessageLength      = 130              // MaxMessageLength is the maximum length of the alert message field in OpsGenie.
	httpSuccessMax        = 299              // httpSuccessMax is the maximum HTTP status code for a successful response.
	defaultHTTPTimeout    = 10 * time.Second // defaultHTTPTimeout is the default timeout for HTTP requests.
)

// ErrUnexpectedStatus indicates that OpsGenie returned an unexpected HTTP status code.
var ErrUnexpectedStatus = errors.New("OpsGenie notification returned unexpected HTTP status code")

// Service provides OpsGenie as a notification service.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
}

// sendAlert sends an alert to OpsGenie using the specified URL and API key.
func (service *Service) sendAlert(url string, apiKey string, payload AlertPayload) error {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling alert payload to JSON: %w", err)
	}

	jsonBuffer := bytes.NewBuffer(jsonBody)

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, jsonBuffer)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Add("Authorization", "GenieKey "+apiKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification to OpsGenie: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > httpSuccessMax {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf(
				"%w: %d, cannot read body: %w",
				ErrUnexpectedStatus,
				resp.StatusCode,
				err,
			)
		}

		return fmt.Errorf("%w: %d - %s", ErrUnexpectedStatus, resp.StatusCode, body)
	}

	return nil
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	return service.Config.setURL(&service.pkr, configURL)
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// Send delivers a notification message to OpsGenie.
// See: https://docs.opsgenie.com/docs/alert-api#create-alert
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config
	endpointURL := fmt.Sprintf(alertEndpointTemplate, config.Host, config.Port)

	payload, err := service.newAlertPayload(message, params)
	if err != nil {
		return err
	}

	return service.sendAlert(endpointURL, config.APIKey, payload)
}

// newAlertPayload creates a new alert payload for OpsGenie based on the message and parameters.
func (service *Service) newAlertPayload(
	message string,
	params *types.Params,
) (AlertPayload, error) {
	if params == nil {
		params = &types.Params{}
	}

	// Defensive copy
	payloadFields := *service.Config

	if err := service.pkr.UpdateConfigFromParams(&payloadFields, params); err != nil {
		return AlertPayload{}, fmt.Errorf("updating payload fields from params: %w", err)
	}

	// Use `Message` for the title if available, or if the message is too long
	// Use `Description` for the message in these scenarios
	title := payloadFields.Title
	description := message

	if title == "" {
		if len(message) > MaxMessageLength {
			title = message[:MaxMessageLength]
		} else {
			title = message
			description = ""
		}
	}

	if payloadFields.Description != "" && description != "" {
		description += "\n"
	}

	result := AlertPayload{
		Message:     title,
		Alias:       payloadFields.Alias,
		Description: description + payloadFields.Description,
		Responders:  payloadFields.Responders,
		VisibleTo:   payloadFields.VisibleTo,
		Actions:     payloadFields.Actions,
		Tags:        payloadFields.Tags,
		Details:     payloadFields.Details,
		Entity:      payloadFields.Entity,
		Source:      payloadFields.Source,
		Priority:    payloadFields.Priority,
		User:        payloadFields.User,
		Note:        payloadFields.Note,
	}

	return result, nil
}
