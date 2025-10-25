package pushover

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// hookURL is the Pushover API endpoint for sending messages.
const (
	hookURL            = "https://api.pushover.net/1/messages.json"
	contentType        = "application/x-www-form-urlencoded"
	defaultHTTPTimeout = 10 * time.Second // defaultHTTPTimeout is the default timeout for HTTP requests.
)

// ErrSendFailed indicates a failure in sending the notification to a Pushover device.
var ErrSendFailed = errors.New("failed to send notification to pushover device")

// Service provides the Pushover notification service.
type Service struct {
	standard.Standard
	Config *Config
	pkr    format.PropKeyResolver
	Client *http.Client
}

// Send delivers a notification message to Pushover.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config
	if err := service.pkr.UpdateConfigFromParams(config, params); err != nil {
		return fmt.Errorf("updating config from params: %w", err)
	}

	device := strings.Join(config.Devices, ",")
	if err := service.sendToDevice(device, message, config); err != nil {
		return fmt.Errorf("failed to send notifications to pushover devices: %w", err)
	}

	return nil
}

// sendToDevice sends a notification to a specific Pushover device.
func (service *Service) sendToDevice(device string, message string, config *Config) error {
	data := url.Values{}
	data.Set("device", device)
	data.Set("user", config.User)
	data.Set("token", config.Token)
	data.Set("message", message)

	if len(config.Title) > 0 {
		data.Set("title", config.Title)
	}

	if config.Priority >= -2 && config.Priority <= 1 {
		data.Set("priority", strconv.FormatInt(int64(config.Priority), 10))
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		hookURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	res, err := service.Client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request to Pushover API: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %q, response status %q", ErrSendFailed, device, res.Status)
	}

	return nil
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)
	service.Client = &http.Client{
		Timeout: defaultHTTPTimeout,
	}

	if err := service.Config.setURL(&service.pkr, configURL); err != nil {
		return err
	}

	return nil
}

// GetID returns the service identifier.
func (service *Service) GetID() string {
	return Scheme
}
