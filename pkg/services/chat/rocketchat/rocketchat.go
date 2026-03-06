package rocketchat

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to a pre-configured Rocket.Chat channel or user.
// It implements the types.Service interface for Rocket.Chat integration.
type Service struct {
	standard.Standard

	// Config holds the Rocket.Chat service configuration.
	Config *Config
	// Client is the HTTP client used for API requests.
	Client *http.Client
}

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 10 * time.Second

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
//
// Params:
//   - configURL: The configuration URL containing Rocket.Chat connection details
//   - logger: The logger to use for this service
//
// Returns:
//   - error: An error if configuration fails, nil otherwise
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)

	s.Config = &Config{}
	if s.Client == nil {
		s.Client = &http.Client{
			Timeout: defaultHTTPTimeout, // Set a default timeout
		}
	}

	if err := s.Config.SetURL(configURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Rocket.Chat.
//
// Params:
//   - message: The message text to send
//   - params: Optional parameters that can override configuration (username, channel)
//
// Returns:
//   - error: An error if sending fails, nil on success
func (s *Service) Send(message string, params *types.Params) error {
	var res *http.Response

	var err error

	config := s.Config
	apiURL := buildURL(config)

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
		apiURL,
		bytes.NewReader(json),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err = s.Client.Do(req)
	if err != nil {
		return fmt.Errorf(
			"posting to URL: %w\nHOST: %s\nPORT: %s",
			err,
			config.Host,
			config.Port,
		)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		resBody, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf(
				"%w: %d",
				ErrNotificationFailed,
				res.StatusCode,
			)
		}

		return fmt.Errorf("%w: %d %s",
			ErrNotificationFailed,
			res.StatusCode,
			resBody,
		)
	}

	return nil
}

// buildURL constructs the API URL for Rocket.Chat based on the Config.
//
// Params:
//   - config: The configuration containing host, port, and token information
//
// Returns:
//   - The complete Rocket.Chat webhook URL as a string
func buildURL(config *Config) string {
	base := config.Host
	if config.Port != "" {
		base = net.JoinHostPort(config.Host, config.Port)
	}

	return fmt.Sprintf(
		"https://%s/hooks/%s/%s",
		base,
		config.TokenA,
		config.TokenB,
	)
}
