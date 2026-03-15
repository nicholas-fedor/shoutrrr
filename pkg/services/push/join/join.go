package join

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/nicholas-fedor/shoutrrr/pkg/format"
	"github.com/nicholas-fedor/shoutrrr/pkg/services/standard"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// Service sends notifications to Join devices.
type Service struct {
	standard.Standard

	Config *Config
	pkr    format.PropKeyResolver
}

const (
	// hookURL defines the Join API endpoint for sending push notifications.
	hookURL     = "https://joinjoaomgcd.appspot.com/_ah/api/messaging/v1/sendPush"
	contentType = "text/plain"
)

// GetID returns the identifier for this service.
func (s *Service) GetID() string {
	return Scheme
}

// Initialize configures the service with a URL and logger.
func (s *Service) Initialize(serviceURL *url.URL, logger types.StdLogger) error {
	s.SetLogger(logger)
	s.Config = &Config{}
	s.pkr = format.NewPropKeyResolver(s.Config)

	if err := s.Config.setURL(&s.pkr, serviceURL); err != nil {
		return err
	}

	return nil
}

// Send delivers a notification message to Join devices.
func (s *Service) Send(message string, params *types.Params) error {
	config := s.Config

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

	return s.sendToDevices(devices, message, title, icon)
}

func (s *Service) sendToDevices(devices, message, title, icon string) error {
	config := s.Config

	apiURL, err := url.Parse(hookURL)
	if err != nil {
		return fmt.Errorf("parsing Join API URL: %w", err)
	}

	data := url.Values{}
	data.Set("deviceIds", devices)
	data.Set("apikey", config.APIKey)
	data.Set("text", message)

	if title != "" {
		data.Set("title", title)
	}

	if title != "" {
		data.Set("icon", icon)
	}

	apiURL.RawQuery = data.Encode()

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		apiURL.String(),
		http.NoBody,
	)
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
		return fmt.Errorf(
			"%w: %q, response status %q",
			ErrSendFailed,
			devices,
			res.Status,
		)
	}

	return nil
}
