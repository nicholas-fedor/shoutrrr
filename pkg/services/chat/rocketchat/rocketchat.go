package rocketchat

import (
	"bytes"
	"context"
	"errors"
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
type Service struct {
	standard.Standard

	Config *Config
	Client *http.Client
}

// defaultHTTPTimeout is the default timeout for HTTP requests.
const defaultHTTPTimeout = 10 * time.Second

// ErrNotificationFailed indicates a failure in sending the notification.
var ErrNotificationFailed = errors.New("notification failed")

// GetID returns the service identifier.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
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
func (s *Service) Send(message string, params *types.Params) error {
	var res *http.Response

	var err error

	config := s.Config
	apiURL := buildURL(config)
	json, _ := CreateJSONPayload(config, message, params)

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(json))
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
		resBody, _ := io.ReadAll(res.Body)

		return fmt.Errorf("%w: %d %s", ErrNotificationFailed, res.StatusCode, resBody)
	}

	return nil
}

// buildURL constructs the API URL for Rocket.Chat based on the Config.
func buildURL(config *Config) string {
	base := config.Host
	if config.Port != "" {
		base = net.JoinHostPort(config.Host, config.Port)
	}

	return fmt.Sprintf("https://%s/hooks/%s/%s", base, config.TokenA, config.TokenB)
}
