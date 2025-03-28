package googlechat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// ErrUnexpectedStatus indicates an unexpected HTTP status code from the Google Chat API.
var ErrUnexpectedStatus = errors.New("google chat api returned unexpected http status code")

// Service implements a Google Chat notification service.
type Service struct {
	standard.Standard
	Config *Config
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}

	return service.Config.SetURL(configURL)
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// Send delivers a notification message to Google Chat.
func (service *Service) Send(message string, _ *types.Params) error {
	config := service.Config

	jsonBody, err := json.Marshal(JSON{Text: message})
	if err != nil {
		return fmt.Errorf("marshaling message to JSON: %w", err)
	}

	postURL := getAPIURL(config)
	jsonBuffer := bytes.NewBuffer(jsonBody)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		postURL.String(),
		jsonBuffer,
	)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending notification to Google Chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	return nil
}

// getAPIURL constructs the API URL for Google Chat notifications.
func getAPIURL(config *Config) *url.URL {
	query := url.Values{}
	query.Set("key", config.Key)
	query.Set("token", config.Token)

	return &url.URL{
		Path:     config.Path,
		Host:     config.Host,
		Scheme:   "https",
		RawQuery: query.Encode(),
	}
}
