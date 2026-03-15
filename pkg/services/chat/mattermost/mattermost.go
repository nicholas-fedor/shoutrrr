package mattermost

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to a pre-configured Mattermost channel or user.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	httpClient *http.Client
}

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 10 * time.Second

// GetHTTPClient returns the service's HTTP client for testing purposes.
func (s *Service) GetHTTPClient() *http.Client {
	return s.httpClient
}

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	err := s.Config.setURL(&s.pkr, serviceURL)
	if err != nil {
		return err
	}

	var transport *http.Transport
	if s.Config.DisableTLS {
		transport = &http.Transport{} // Plain HTTP
	} else {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,            // Explicitly safe when TLS is enabled
				MinVersion:         tls.VersionTLS12, // Enforce TLS 1.2 or higher
			},
		}
	}

	s.httpClient = &http.Client{Transport: transport}

	return nil
}

// Send delivers a notification message to Mattermost.
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config
	serviceURL := buildURL(config)

	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	json, err := CreateJSONPayload(config, message, params)
	if err != nil {
		return fmt.Errorf("creating JSON payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		defaultHTTPTimeout,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		serviceURL.String(),
		bytes.NewReader(json),
	)
	if err != nil {
		return fmt.Errorf("creating POST request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := s.httpClient.Do(req)
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
// Returns a *url.URL that is valid by construction.
func buildURL(config *Config) *url.URL {
	scheme := "https"
	if config.DisableTLS {
		scheme = "http"
	}

	return &url.URL{
		Scheme: scheme,
		Host:   config.Host,
		Path:   "/hooks/" + config.Token,
	}
}
