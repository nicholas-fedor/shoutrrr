package homeassistant

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Home Assistant.
type Service struct {
	// Standard provides base service functionality including logging.
	standard.Standard

	// Config holds Home Assistant credentials and notification options.
	Config *Config
	// pkr resolves property keys for configuration updates from URL parameters.
	pkr format.PropKeyResolver
	// httpClient is the HTTP client used for REST API requests.
	httpClient types.HTTPClient
}

// requestPayload is the JSON body posted to /api/services/{domain}/{service}.
type requestPayload struct {
	Message        string   `json:"message"`
	Title          string   `json:"title,omitempty"`
	NotificationID string   `json:"notification_id,omitempty"`
	Targets        []string `json:"target,omitempty"`
}

const (
	// contentType is the JSON encoding required by the REST API.
	contentType = "application/json"
	// defaultHTTPTimeout is the timeout applied to REST API requests.
	defaultHTTPTimeout = 10 * time.Second
)

var (
	_ types.Service          = (*Service)(nil)
	_ types.HTTPClientSetter = (*Service)(nil)
)

// GetID returns the service identifier.
//
// Returns:
//   - The scheme name "homeassistant".
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Parameters:
//   - serviceURL: The Home Assistant service URL.
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
		s.httpClient = s.newHTTPClient()
	}

	return nil
}

// Send delivers a notification message to Home Assistant.
//
// Parameters:
//   - message: The notification body.
//   - params: Optional runtime overrides for title, service, targets, and nid.
//
// Returns:
//   - An error if the message is empty, configuration updates fail, or delivery fails.
func (s *Service) Send(message string, params *types.Params) error {
	if message == "" {
		return ErrMessageEmpty
	}

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

// newHTTPClient returns the default HTTP client for Home Assistant requests.
//
// Returns:
//   - An HTTP client with a 10s timeout, TLS 1.2 minimum, and redirects disabled.
func (*Service) newHTTPClient() types.HTTPClient {
	return &http.Client{
		Timeout: defaultHTTPTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return ErrRedirect
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

// send posts a notification to the Home Assistant REST API.
//
// Parameters:
//   - message: The notification body sent as the JSON field message.
//   - config: The resolved configuration used to build the URL and payload.
//
// Returns:
//   - An error if the request cannot be created, sent, or the API returns a non-success status.
func (s *Service) send(message string, config *Config) error {
	payload, postURL, err := buildRequest(message, config)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		postURL,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+config.Token)
	req.Header.Set("User-Agent", "shoutrrr/"+meta.Version)

	client := s.httpClient
	if client == nil {
		client = s.newHTTPClient()
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

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return fmt.Errorf("%w: %s: %s", ErrUnexpectedStatus, res.Status, string(responseBody))
	}

	return nil
}

// buildRequest constructs the JSON payload and API URL for a notification.
//
// Parameters:
//   - message: The notification body.
//   - config: The resolved configuration.
//
// Returns:
//   - payload: The JSON body to post.
//   - postURL: The absolute API URL.
//   - err: An error if the service mapping is invalid.
func buildRequest(message string, config *Config) (requestPayload, string, error) {
	domain, service, err := config.domainService()
	if err != nil {
		return requestPayload{}, "", err
	}

	payload := requestPayload{Message: message}
	if config.Title != "" {
		payload.Title = config.Title
	}

	if config.isPersistent() {
		if config.Nid != "" {
			payload.NotificationID = config.Nid
		}
	} else if len(config.Targets) > 0 {
		payload.Targets = config.Targets
	}

	return payload, config.apiURL(domain, service), nil
}
