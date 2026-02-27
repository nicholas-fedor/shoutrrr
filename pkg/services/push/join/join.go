package join

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

const (
	// hookURL defines the Join API endpoint for sending push notifications.
	hookURL     = "https://joinjoaomgcd.appspot.com/_ah/api/messaging/v1/sendPush"
	contentType = "text/plain"
)

// ErrSendFailed indicates a failure to send a notification to Join devices.
var ErrSendFailed = errors.New("failed to send notification to join devices")

// Service sends notifications to Join devices.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

// Send delivers a notification message to Join devices.
func (service *Service) Send(message string, params *types.Params) error {
	config := service.Config

	if params == nil {
		params = &types.Params{}
	}

	title, found := (*params)["title"]
	if !found {
		title = config.Title
	}

	icon, found := (*params)["icon"]
	if !found {
		icon = config.Icon
	}

	devices := strings.Join(config.Devices, ",")

	return service.sendToDevices(devices, message, title, icon)
}

func (service *Service) sendToDevices(devices, message, title, icon string) error {
	config := service.Config

	apiURL, err := url.Parse(hookURL)
	if err != nil {
		return fmt.Errorf("parsing Join API URL: %w", err)
	}

	data := url.Values{}
	data.Set("deviceIds", devices)
	data.Set("apikey", config.APIKey)
	data.Set("text", message)

	if len(title) > 0 {
		data.Set("title", title)
	}

	if len(icon) > 0 {
		data.Set("icon", icon)
	}

	apiURL.RawQuery = data.Encode()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, apiURL.String(), http.NoBody)
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending HTTP request to Join: %w", err)
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %q, response status %q", ErrSendFailed, devices, res.Status)
	}

	return nil
}

// Initialize configures the service with a URL and logger.
func (service *Service) Initialize(configURL *url.URL, logger types.StdLogger) error {
	service.SetLogger(logger)
	service.Config = &Config{}
	service.pkr = format.NewPropKeyResolver(service.Config)

	if err := service.Config.setURL(&service.pkr, configURL); err != nil {
		return err
	}

	return nil
}

// GetID returns the identifier for this service.
func (service *Service) GetID() string {
	return Scheme
}
