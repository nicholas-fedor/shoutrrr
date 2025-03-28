package zulip

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// contentMaxSize defines the maximum allowed message size in bytes.
const (
	contentMaxSize = 10000 // bytes
	topicMaxLength = 60    // characters
)

// ErrTopicTooLong indicates the topic exceeds the maximum allowed length.
var (
	ErrTopicTooLong          = errors.New("topic exceeds max length")
	ErrMessageTooLong        = errors.New("message exceeds max size")
	ErrResponseStatusFailure = errors.New("response status code unexpected")
	ErrInvalidHost           = errors.New("invalid host format")
)

// hostValidator ensures the host is a valid hostname or domain.
var hostValidator = regexp.MustCompile(
	`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`,
)

// Service sends notifications to a pre-configured Zulip channel or user.
type Service struct {
	standard.Standard
	Config *Config
}

// Send delivers a notification message to Zulip.
func (service *Service) Send(message string, params *types.Params) error {
	// Clone the config to avoid modifying the original for this send operation.
	config := service.Config.Clone()

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

	return service.doSend(config, message)
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}

	if err := service.Config.setURL(nil, configURL); err != nil {
		return err
	}

	return nil
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}

// doSend sends the notification to Zulip using the configured API URL.
//
//nolint:gosec,noctx // Ignoring G107: Potential HTTP request made with variable url
func (service *Service) doSend(config *Config, message string) error {
	apiURL := service.getAPIURL(config)

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

	defer res.Body.Close()

	if err != nil {
		return fmt.Errorf("failed to send zulip message: %w", err)
	}

	return nil
}

// getAPIURL constructs the API URL for Zulip based on the Config.
func (service *Service) getAPIURL(config *Config) string {
	return (&url.URL{
		User:   url.UserPassword(config.BotMail, config.BotKey),
		Host:   config.Host,
		Path:   "api/v1/messages",
		Scheme: "https",
	}).String()
}
