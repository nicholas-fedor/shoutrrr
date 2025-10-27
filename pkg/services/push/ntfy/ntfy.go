package ntfy

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
	"github.com/nicholas-fedor/shoutrrr/pkg/util/jsonclient"
)

// HTTPTimeout defines the HTTP client timeout in seconds.
const HTTPTimeout = 10

// Service sends notifications to ntfy.
type Service struct {
	standard.Standard
	Config     *Config
	pkr        format.PropKeyResolver
	httpClient *http.Client
	client     jsonclient.Client
}

// SetHTTPClient sets a custom HTTP client for the service.
func (service *Service) SetHTTPClient(httpClient *http.Client) {
	service.httpClient = httpClient
	service.client = jsonclient.NewWithHTTPClient(service.httpClient)
}

// Send delivers a notification message to ntfy.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config

	// Update config with runtime parameters
	if err := service.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// Execute the API request to send the notification
	if err := service.sendAPI(config, message); err != nil {
		return fmt.Errorf("failed to send ntfy notification: %w", err)
	}

	return nil
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	_ = service.pkr.SetDefaultProps(service.Config)

	err := service.Config.setURL(&service.pkr, configURL)
	if err != nil {
		return err
	}

	service.httpClient = &http.Client{
		Timeout: HTTPTimeout * time.Second,
	}

	// Configure HTTP transport for TLS verification and enforce TLS 1.2
	if service.Config.DisableTLSVerification {
		service.httpClient.Transport = &http.Transport{
			//nolint:gosec // TLS verification intentionally disabled
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
		}
		service.Log("Warning: TLS verification is disabled, making connections insecure")
	} else {
		service.httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		}
		service.Log("Using custom HTTP transport with TLS verification enabled and TLS 1.2 enforced")
	}

	service.client = jsonclient.NewWithHTTPClient(service.httpClient)

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}

// sendAPI sends a notification to the ntfy API.
func (service *Service) sendAPI(config *Config, message string) error {
	response := apiResponse{}
	request := message

	// Prepare request headers
	headers := service.client.Headers()
	headers.Del("Content-Type")
	headers.Set("Content-Type", "text/plain; charset=utf-8")
	headers.Set("User-Agent", "shoutrrr/"+meta.Version)
	addHeaderIfNotEmpty(&headers, "Title", config.Title)
	addHeaderIfNotEmpty(&headers, "Priority", config.Priority.String())
	addHeaderIfNotEmpty(&headers, "Tags", strings.Join(config.Tags, ","))
	addHeaderIfNotEmpty(&headers, "Delay", config.Delay)
	addHeaderIfNotEmpty(&headers, "Actions", strings.Join(config.Actions, ";"))
	addHeaderIfNotEmpty(&headers, "Click", config.Click)
	addHeaderIfNotEmpty(&headers, "Attach", config.Attach)
	addHeaderIfNotEmpty(&headers, "X-Icon", config.Icon)
	addHeaderIfNotEmpty(&headers, "Filename", config.Filename)
	addHeaderIfNotEmpty(&headers, "Email", config.Email)

	if !config.Cache {
		headers.Add("Cache", "no")
	}

	if !config.Firebase {
		headers.Add("Firebase", "no")
	}

	// Add Basic Auth header if username or password is provided
	if config.Username != "" || config.Password != "" {
		headers.Set(
			"Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(config.Username+":"+config.Password)),
		)
	}

	// Send the HTTP request
	if err := service.client.Post(config.GetAPIURL(), request, &response); err != nil {
		service.Logf("NTFY API request failed with error: %v", err)
		// Attempt to parse structured error response from API
		if service.client.ErrorResponse(err, &response) {
			return &response
		}

		return fmt.Errorf("posting to ntfy API: %w", err)
	}

	service.Logf("NTFY API request succeeded")

	return nil
}

// addHeaderIfNotEmpty adds a header to the request if the value is non-empty.
func addHeaderIfNotEmpty(headers *http.Header, key string, value string) {
	if value != "" {
		headers.Add(key, value)
	}
}
