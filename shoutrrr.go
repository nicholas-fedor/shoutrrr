package shoutrrr

import (
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// defaultRouter manages the creation and routing of notification services.
var defaultRouter = router.ServiceRouter{}

// SetLogger configures the logger for all services in the default router.
func SetLogger(logger types.StdLogger) {
	defaultRouter.SetLogger(logger)
}

// Send delivers a notification message using the specified URL.
func Send(rawURL string, message string) error {
	service, err := defaultRouter.Locate(rawURL)
	if err != nil {
		return fmt.Errorf("locating service for URL %q: %w", rawURL, err)
	}

	if err := service.Send(message, &types.Params{}); err != nil {
		return fmt.Errorf("sending message via service at %q: %w", rawURL, err)
	}

	return nil
}

// CreateSender constructs a new service router for the given URLs without a logger.
func CreateSender(rawURLs ...string) (*router.ServiceRouter, error) {
	sr, err := router.New(nil, rawURLs...)
	if err != nil {
		return nil, fmt.Errorf("creating sender for URLs %v: %w", rawURLs, err)
	}

	return sr, nil
}

// NewSender constructs a new service router with a logger for the given URLs.
func NewSender(logger types.StdLogger, serviceURLs ...string) (*router.ServiceRouter, error) {
	sr, err := router.New(logger, serviceURLs...)
	if err != nil {
		return nil, fmt.Errorf("creating sender with logger for URLs %v: %w", serviceURLs, err)
	}

	return sr, nil
}

// Version returns the current Shoutrrr version.
func Version() string {
	return meta.GetVersion()
}
