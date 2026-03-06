package zulip

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to a pre-configured Zulip channel or user.
type Service struct {
	standard.Standard

	Config *Config
}

// contentMaxSize defines the maximum allowed message size in bytes.
const (
	contentMaxSize = 10000 // bytes
	topicMaxLength = 60    // characters
)

// hostValidator ensures the host is a valid hostname or domain.
var hostValidator = regexp.MustCompile(
	`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`,
)

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}

	if err := s.Config.setURL(nil, configURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Zulip.
func (s *Service) Send(message string, params *types.Params) error {
	// Clone the config to avoid modifying the original for this send operation.
	config := s.Config.Clone()

	if params != nil {
		if stream, found := (*params)["stream"]; found {
			config.Stream = stream
		}

		if topic, found := (*params)["topic"]; found {
			config.Topic = topic
		}
	}

	topicLength := len([]rune(config.Topic))
	if topicLength > topicMaxLength {
		return fmt.Errorf("%w: %d characters, got %d", ErrTopicTooLong, topicMaxLength, topicLength)
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

	return s.doSend(config, message)
}

// doSend sends the notification to Zulip using the configured API URL.
func (s *Service) doSend(config *Config, message string) error {
	apiURL := s.getAPIURL(config)

	// Validate the host to mitigate SSRF risks
	if !hostValidator.MatchString(config.Host) {
		return fmt.Errorf("%w: %q", ErrInvalidHost, config.Host)
	}

	payload := CreatePayload(config, message)

	res, err := http.Post(
		apiURL,
		"application/x-www-form-urlencoded",
		strings.NewReader(payload.Encode()),
	)
	if err == nil && res.StatusCode != http.StatusOK {
		err = fmt.Errorf("%w: %s", ErrResponseStatusFailure, res.Status)
	}

	defer func() { _ = res.Body.Close() }()

	if err != nil {
		return fmt.Errorf("failed to send zulip message: %w", err)
	}

	return nil
}

// getAPIURL constructs the API URL for Zulip based on the Config.
func (s *Service) getAPIURL(config *Config) string {
	return (&url.URL{
		User:   url.UserPassword(config.BotMail, config.BotKey),
		Host:   config.Host,
		Path:   "api/v1/messages",
		Scheme: "https",
	}).String()
}
