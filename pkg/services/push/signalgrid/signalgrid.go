package signalgrid

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Signalgrid.
type Service struct {
	// Standard provides base service functionality including logging.
	standard.Standard

	// Config holds Signalgrid credentials and notification options.
	Config *Config
	// pkr resolves property keys for configuration updates from URL parameters.
	pkr format.PropKeyResolver
	// httpClient is the HTTP client used for Push API requests.
	httpClient types.HTTPClient
}

const (
	// apiURL is the Signalgrid Push API endpoint.
	apiURL = "https://api.signalgrid.co/v1/push"
	// contentType is the form encoding required by the Push API.
	contentType = "application/x-www-form-urlencoded"
	// defaultHTTPTimeout is the timeout applied to Push API requests.
	defaultHTTPTimeout = 10 * time.Second
)

// GetID returns the service identifier.
//
// Returns:
//   - The scheme name "signalgrid".
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: The Signalgrid service URL.
//   - logger: Logger used for service output.
//
// Returns:
//   - An error if default properties cannot be applied or the URL is invalid.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default props: %w", err)
	}

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	if s.httpClient == nil {
		s.httpClient = &http.Client{
			Timeout: defaultHTTPTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			},
		}
	}

	return nil
}

// Send delivers a notification message to Signalgrid.
//
// Parameters:
//   - message: The notification body.
//   - params: Optional runtime overrides for title, type, and critical.
//
// Returns:
//   - An error if configuration updates or delivery fail.
func (s *Service) Send(message string, params *types.Params) error {
	config := *s.Config
	if err := s.pkr.UpdateConfigFromParams(&config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	if err := s.send(message, &config); err != nil {
		return fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	return nil
}

// SetHTTPClient sets a custom HTTP client for the service.
//
// Parameters:
//   - client: The HTTP client to use for API requests.
func (s *Service) SetHTTPClient(client types.HTTPClient) {
	s.httpClient = client
}

// send posts a notification to the Signalgrid Push API.
//
// Parameters:
//   - message: The notification body sent as the form field body.
//   - config: The resolved configuration used to populate remaining form fields.
//
// Returns:
//   - An error if the request cannot be created, sent, or the API returns a non-success status.
func (s *Service) send(message string, config *Config) error {
	data := url.Values{}
	data.Set("client_key", config.ClientKey)
	data.Set("channel", config.Channel)
	data.Set("body", message)
	data.Set("type", config.Type.String())

	if config.Title != "" {
		data.Set("title", config.Title)
	}

	if config.Critical {
		data.Set("critical", "true")
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
	req.Header.Set("User-Agent", "shoutrrr/"+meta.Version)

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: %s: %s", ErrUnexpectedStatus, res.Status, string(responseBody))
	}

	return nil
}
