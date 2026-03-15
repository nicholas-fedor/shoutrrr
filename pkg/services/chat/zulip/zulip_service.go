package zulip

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// HTTPClient defines the interface for HTTP operations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient is the default implementation using http.DefaultClient with timeout.
type DefaultHTTPClient struct {
	client *http.Client
}

// Service sends notifications to a pre-configured Zulip channel or user.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	HTTPClient HTTPClient
}

// defaultHTTPTimeout is the default timeout for HTTP requests to Zulip.
const defaultHTTPTimeout = 30 * time.Second

// contentMaxSize defines the maximum allowed message size in bytes.
const contentMaxSize = 10000

// topicMaxLength defines the maximum allowed topic length in characters.
const topicMaxLength = 60

// hostValidator ensures the host is a valid hostname or domain.
var hostValidator = regexp.MustCompile(
	`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`,
)

// NewDefaultHTTPClient creates a new default HTTP client with a reasonable timeout.
func NewDefaultHTTPClient() *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Do performs the HTTP request.
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing HTTP request: %w", err)
	}

	return resp, nil
}

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)
	s.HTTPClient = NewDefaultHTTPClient()

	if err := s.pkr.SetDefaultProps(s.Config); err != nil {
		return fmt.Errorf("setting default properties: %w", err)
	}

	if err := s.Config.SetURL(serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Zulip.
func (s *Service) Send(message string, params *types.Params) error {
	return s.SendWithContext(context.Background(), message, params)
}

// SendWithContext delivers a notification message to Zulip with context support.
func (s *Service) SendWithContext(ctx context.Context, message string, params *types.Params) error {
	// Clone the config to avoid modifying the original for this send operation.
	config := s.Config.Clone()

	// Use PropKeyResolver for consistent param handling
	if err := s.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	topicLength := len([]rune(config.Topic))
	if topicLength > topicMaxLength {
		return fmt.Errorf(
			"%w: %d characters, got %d",
			ErrTopicTooLong,
			topicMaxLength,
			topicLength,
		)
	}

	messageSize := len(message)
	if messageSize > contentMaxSize {
		return fmt.Errorf(
			"%w: %d bytes, got %d bytes",
			ErrMessageTooLong,
			contentMaxSize,
			messageSize,
		)
	}

	return s.doSend(ctx, config, message)
}

// doSend sends the notification to Zulip using the configured API URL.
func (s *Service) doSend(ctx context.Context, config *Config, message string) error {
	apiURL := s.getAPIURL(config)

	// Validate the host to mitigate SSRF risks
	if !hostValidator.MatchString(config.Host) {
		return fmt.Errorf("%w: %q", ErrInvalidHost, config.Host)
	}

	payload := CreatePayload(config, message)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		apiURL,
		strings.NewReader(payload.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("making HTTP POST request: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send zulip message: %w: %d", ErrResponseStatusFailure, res.StatusCode)
	}

	return nil
}

// getAPIURL constructs the API URL for Zulip based on the Config.
func (s *Service) getAPIURL(config *Config) string {
	return (&url.URL{
		User:   url.UserPassword(config.BotMail, config.BotKey),
		Host:   config.Host,
		Path:   "/api/v1/messages",
		Scheme: "https",
	}).String()
}
