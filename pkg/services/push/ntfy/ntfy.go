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

// Service sends notifications to ntfy.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient types.HTTPClient
	// apiClient creates the JSON client for one send. Nil builds one over
	// httpClient, so every send has its own request headers.
	apiClient func() jsonclient.Client
	// defaultClient reports whether httpClient was built by the service rather than
	// supplied through SetHTTPClient, so Initialize rebuilds it for the new config.
	defaultClient bool
}

// HTTPTimeout defines the HTTP client timeout in seconds.
const HTTPTimeout = 10

// Compile-time checks that Service implements the interfaces the router relies on.
var (
	_ types.Service          = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
	_ types.ServiceTimeout   = (*Service)(nil)
)

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	err := s.Config.setURL(&s.pkr, serviceURL)
	if err != nil {
		return err
	}

	// Force HTTP scheme when DisableTLS is true
	if s.Config.DisableTLS {
		s.Config.Scheme = "http"
	}

	if s.httpClient == nil || s.defaultClient {
		s.httpClient = s.newDefaultHTTPClient()
		s.defaultClient = true

		if s.Config.DisableTLSVerification {
			s.Log("Warning: TLS verification is disabled, making connections insecure")
		}
	}

	return nil
}

// Send delivers a notification message to ntfy. Params apply to this send only.
func (s *Service) Send(message string, params *types.Params) error {
	config := *s.Config

	// Update this send's config with runtime parameters
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	// Execute the API request to send the notification
	if err := s.sendAPI(&config, message); err != nil {
		return fmt.Errorf("failed to send ntfy notification: %w", err)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service. A nil client restores
// the default client, which Initialize builds when the service is not yet configured
// and rebuilds whenever it applies a new config.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	if c, ok := client.(*http.Client); ok && c == nil {
		client = nil
	}

	s.defaultClient = client == nil

	if client == nil {
		if s.Config == nil {
			s.httpClient = nil

			return
		}

		client = s.newDefaultHTTPClient()
	}

	s.httpClient = client
}

// newAPIClient returns the JSON client for one send.
//
// Returns:
//   - jsonclient.Client: a new client over the service's HTTP client.
func (s *Service) newAPIClient() jsonclient.Client {
	if s.apiClient != nil {
		return s.apiClient()
	}

	return jsonclient.NewWithHTTPClient(s.httpClient)
}

// newDefaultHTTPClient builds the client used when none is injected. It enforces
// TLS 1.2 or later and skips certificate verification only when the config
// disables it.
//
// Returns:
//   - *http.Client: the default client.
func (s *Service) newDefaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: HTTPTimeout * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: s.Config.DisableTLSVerification,
				MinVersion:         tls.VersionTLS12,
			},
		},
	}
}

// sendAPI sends a notification to the ntfy API.
func (s *Service) sendAPI(config *Config, message string) error {
	response := apiResponseError{}
	request := message

	client := s.newAPIClient()

	// Prepare request headers
	headers := client.Headers()
	if config.Markdown {
		headers.Set("Content-Type", "text/markdown")
	} else {
		headers.Set("Content-Type", "text/plain; charset=utf-8")
	}

	headers.Set("User-Agent", meta.UserAgent())
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

	// Access tokens use Bearer auth and take precedence over username and password.
	if config.Token != "" {
		headers.Set("Authorization", "Bearer "+config.Token)
	} else if config.Username != "" || config.Password != "" {
		headers.Set(
			"Authorization",
			"Basic "+base64.StdEncoding.EncodeToString([]byte(config.Username+":"+config.Password)),
		)
	}

	// Send the HTTP request
	if err := client.Post(config.GetAPIURL(), request, &response); err != nil {
		s.Logf("NTFY API request failed with error: %v", err)
		// Attempt to parse structured error response from API
		if client.ErrorResponse(err, &response) {
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
