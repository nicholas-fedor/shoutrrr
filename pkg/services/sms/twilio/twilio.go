package twilio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	apiBaseURL         = "https://api.twilio.com/2010-04-01/Accounts"
	contentType        = "application/x-www-form-urlencoded"
	defaultHTTPTimeout = 10 * time.Second
	msgServicePrefix   = "MG"
)

// ErrSendFailed indicates a failure in sending the SMS via Twilio.
var ErrSendFailed = errors.New("failed to send SMS via Twilio")

// Service provides the Twilio SMS notification service.
type Service struct {
	standard.Standard
	Config     *Config
	pkr        format.PropKeyResolver
	httpClient *http.Client
}

// Send delivers an SMS message via Twilio to all configured recipients.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config

	err := service.pkr.UpdateConfigFromParams(config, params)
	if err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	var errs []error

	for _, toNumber := range config.ToNumbers {
		err := service.sendToRecipient(config, toNumber, message)
		if err != nil {
			errs = append(errs, fmt.Errorf("sending to %s: %w", toNumber, err))
		}
	}

	return errors.Join(errs...)
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)
	service.httpClient = &http.Client{
		Timeout: defaultHTTPTimeout,
	}

	err := service.Config.setURL(&service.pkr, configURL)
	if err != nil {
		return err
	}

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// sendToRecipient sends an SMS message to a single recipient via the Twilio API.
func (service *Service) sendToRecipient(config *Config, toNumber string, message string) error {
	body := message
	if config.Title != "" {
		body = config.Title + "\n" + message
	}

	apiURL := fmt.Sprintf("%s/%s/Messages.json", apiBaseURL, config.AccountSID)

	data := url.Values{}
	data.Set("To", toNumber)
	data.Set("Body", body)

	// Use MessagingServiceSid if the sender looks like a Messaging Service SID,
	// otherwise use it as a regular From number.
	if strings.HasPrefix(config.FromNumber, msgServicePrefix) {
		data.Set("MessagingServiceSid", config.FromNumber)
	} else {
		data.Set("From", config.FromNumber)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(config.AccountSID, config.AuthToken)

	res, err := service.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request to Twilio API: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return parseAPIError(res)
	}

	return nil
}

// parseAPIError reads the Twilio API error response body and returns a descriptive error.
func parseAPIError(res *http.Response) error {
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("%w: response status %q (failed to read body)", ErrSendFailed, res.Status)
	}

	var apiErr apiErrorResponse

	err = json.Unmarshal(respBody, &apiErr)
	if err == nil && apiErr.Message != "" {
		return fmt.Errorf("%w: %s (code %d)", ErrSendFailed, apiErr.Message, apiErr.Code)
	}

	return fmt.Errorf("%w: response status %q", ErrSendFailed, res.Status)
}

// apiErrorResponse represents an error response from the Twilio REST API.
type apiErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}
