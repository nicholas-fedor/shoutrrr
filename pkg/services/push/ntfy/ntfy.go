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
func (s *Service) SetHTTPClient(httpClient *http.Client) {
	s.httpClient = httpClient
	s.client = jsonclient.NewWithHTTPClient(s.httpClient)
}

// Send delivers a notification message to ntfy.
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config

	// Update config with runtime parameters
	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// Execute the API request to send the notification
	if err := s.sendAPI(config, message); err != nil {
		return fmt.Errorf("failed to send ntfy notification: %w", err)
	}

	return nil
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	_ = s.pkr.SetDefaultProps(s.Config)

	err := s.Config.setURL(&s.pkr, configURL)
	if err != nil {
		return err
	}

	// Force HTTP scheme when DisableTLS is true
	if s.Config.DisableTLS {
		s.Config.Scheme = "http"
	}

	s.httpClient = &http.Client{
		Timeout: HTTPTimeout * time.Second,
	}

	// Configure HTTP transport: skip TLS verification if disabled, enforce TLS 1.2 minimum
	if s.Config.DisableTLSVerification {
		s.httpClient.Transport = &http.Transport{
			//nolint:gosec // TLS verification intentionally disabled
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
		}
		s.Log("Warning: TLS verification is disabled, making connections insecure")
	} else {
		s.httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		}
		s.Log(
			"Using custom HTTP transport with TLS verification enabled and TLS 1.2 enforced",
		)
	}

	s.client = jsonclient.NewWithHTTPClient(s.httpClient)

	return nil
}

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// sendAPI sends a notification to the ntfy API.
func (s *Service) sendAPI(config *Config, message string) error {
	response := apiResponse{}
	request := message

	// Prepare request headers
	headers := s.client.Headers()
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
	if err := s.client.Post(config.GetAPIURL(), request, &response); err != nil {
		s.Logf("NTFY API request failed with error: %v", err)
		// Attempt to parse structured error response from API
		if s.client.ErrorResponse(err, &response) {
			return &response
		}

		return fmt.Errorf("posting to ntfy API: %w", err)
	}

	s.Logf("NTFY API request succeeded")

	return nil
}

// addHeaderIfNotEmpty adds a header to the request if the value is non-empty.
func addHeaderIfNotEmpty(headers *http.Header, key, value string) {
	if value != "" {
		headers.Add(key, value)
	}
}
