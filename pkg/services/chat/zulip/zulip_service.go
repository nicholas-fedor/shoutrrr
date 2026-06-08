package zulip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

// registerResponse represents the relevant fields from the Zulip register endpoint.
type registerResponse struct {
	MaxMessageLength int `json:"max_message_length"`
	MaxTopicLength   int `json:"max_topic_length"`
}

// Service sends notifications to a pre-configured Zulip channel or user.
type Service struct {
	standard.Standard

	Config     *Config
	pkr        format.PropKeyResolver
	HTTPClient HTTPClient

	mu             sync.Once
	contentMaxSize int
	topicMaxLength int
}

// defaultHTTPTimeout is the default timeout for HTTP requests to Zulip.
const defaultHTTPTimeout = 30 * time.Second

// contentMaxSize defines the default maximum allowed message size in bytes.
const contentMaxSize = 10000

// topicMaxLength defines the default maximum allowed topic length in characters.
const topicMaxLength = 60

// registerTimeout is the timeout for the register API call.
const registerTimeout = 10 * time.Second

// maxErrorBodySize limits how much of a non-OK response body we read to prevent
// memory exhaustion from huge error bodies returned by the server.
const maxErrorBodySize = 4 * 1024

// hostValidator ensures the host is a valid hostname or domain,
// optionally followed by a colon and port number (1-65535).
var hostValidator = regexp.MustCompile(
	`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}(:[0-9]{1,5})?$`,
)

// validateHost checks that the host string is a valid hostname with optional port (1-65535).
func validateHost(host string) error {
	if !hostValidator.MatchString(host) {
		return fmt.Errorf("%w: %q", ErrInvalidHost, host)
	}

	if _, portStr, ok := strings.Cut(host, ":"); ok && portStr != "" {
		port, parseErr := strconv.Atoi(portStr)
		if parseErr != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: %q", ErrInvalidHost, host)
		}
	}

	return nil
}

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
	s.contentMaxSize = contentMaxSize
	s.topicMaxLength = topicMaxLength

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

	// If no topic is set, fall back to using the title as the topic
	// for backward compatibility with tools that only provide a title.
	if config.Topic == "" && config.Title != "" {
		config.Topic = config.Title
		config.Title = ""
	}

	if err := ValidateMessageType(config.Type); err != nil {
		return err
	}

	if err := validateHost(config.Host); err != nil {
		return err
	}

	s.mu.Do(func() { s.fetchLimits(ctx) })

	topicLength := len([]rune(config.Topic))
	if topicLength > s.topicMaxLength {
		return fmt.Errorf(
			"%w: %d characters, got %d",
			ErrTopicTooLong,
			s.topicMaxLength,
			topicLength,
		)
	}

	// Prepend title to message content when both topic and title are set.
	if config.Title != "" {
		message = fmt.Sprintf("%s\n\n%s", config.Title, message)
	}

	messageSize := len(message)
	if messageSize > s.contentMaxSize {
		return fmt.Errorf(
			"%w: %d bytes, got %d bytes",
			ErrMessageTooLong,
			s.contentMaxSize,
			messageSize,
		)
	}

	if config.Type == MessageTypeDirect {
		recipients := config.To
		if recipients == "" && config.Stream != "" {
			recipients = config.Stream
		}

		if recipients == "" {
			return ErrMissingRecipient
		}
	} else if config.Stream == "" {
		return ErrMissingRecipient
	}

	return s.doSend(ctx, config, message)
}

// doSend sends the notification to Zulip using the configured API URL.
func (s *Service) doSend(ctx context.Context, config *Config, message string) error {
	apiURL := s.getAPIURL(config)

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
		body, readErr := io.ReadAll(io.LimitReader(res.Body, maxErrorBodySize+1))
		if readErr == nil && len(body) > 0 {
			trunc := ""

			if len(body) > maxErrorBodySize {
				body = body[:maxErrorBodySize]
				trunc = " [truncated]"
			}

			return fmt.Errorf("failed to send zulip message: %w: %d, body: %s%s", ErrResponseStatusFailure, res.StatusCode, string(body), trunc)
		}

		return fmt.Errorf("failed to send zulip message: %w: %d", ErrResponseStatusFailure, res.StatusCode)
	}

	return nil
}

// fetchLimits calls the Zulip register endpoint to fetch server-side size limits.
// Uses defaults if the call fails.
func (s *Service) fetchLimits(ctx context.Context) {
	if err := validateHost(s.Config.Host); err != nil {
		return
	}

	registerURL := s.getRegisterURL()

	ctx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("fetch_event_types", `["realm"]`)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := s.HTTPClient.Do(req)
	if err != nil {
		return
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return
	}

	var reg registerResponse
	if err := json.NewDecoder(res.Body).Decode(&reg); err != nil {
		return
	}

	if reg.MaxMessageLength > 0 {
		s.contentMaxSize = reg.MaxMessageLength
	}

	if reg.MaxTopicLength > 0 {
		s.topicMaxLength = reg.MaxTopicLength
	}
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

// getRegisterURL constructs the register API URL from the service config.
func (s *Service) getRegisterURL() string {
	return (&url.URL{
		User:   url.UserPassword(s.Config.BotMail, s.Config.BotKey),
		Host:   s.Config.Host,
		Path:   "/api/v1/register",
		Scheme: "https",
	}).String()
}
