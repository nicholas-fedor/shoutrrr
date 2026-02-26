package mattermost

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 10 * time.Second

// ErrSendFailed indicates that the notification failed due to an unexpected response status code.
var ErrSendFailed = errors.New(
	"failed to send notification to service, response status code unexpected",
)

// Service sends notifications to a pre-configured Mattermost channel or user.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient *http.Client
}

// GetHTTPClient returns the service's HTTP client for testing purposes.
func (service *Service) GetHTTPClient() *http.Client {
	return service.httpClient
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	err := service.Config.setURL(&service.pkr, configURL)
	if err != nil {
		return err
	}

	var transport *http.Transport
	if service.Config.DisableTLS {
		transport = &http.Transport{
			TLSClientConfig: nil, // Plain HTTP
		}
	} else {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,            // Explicitly safe when TLS is enabled
				MinVersion:         tls.VersionTLS12, // Enforce TLS 1.2 or higher
			},
		}
	}

	service.httpClient = &http.Client{Transport: transport}

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// Send delivers a notification message to Mattermost.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config
	apiURL := buildURL(config)

	if err := service.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	json, _ := CreateJSONPayload(config, message, params)

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(json))
	if err != nil {
		return fmt.Errorf("creating POST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := service.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing POST request to Mattermost API: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s", ErrSendFailed, res.Status)
	}

	return nil
}

// buildURL constructs the API URL for Mattermost based on the Config.
func buildURL(config *Config) string {
	scheme := "https"
	if config.DisableTLS {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s/hooks/%s", scheme, config.Host, config.Token)
}
