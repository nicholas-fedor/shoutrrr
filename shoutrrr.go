// Package shoutrrr provides a simple API for sending notifications to various services.
//
// The package supports multiple notification services including Slack, Discord,
// Telegram, and many others. Notifications are sent using service-specific URLs
// that contain all necessary configuration and authentication details.
//
// Basic usage:
//
//	if err := shoutrrr.Send("slack://webhook/...", "Hello, World!"); err != nil {
//		// handle error
//	}
//
// For more complex scenarios, create a sender with multiple service URLs:
//
//	sender, err := shoutrrr.CreateSender("slack://webhook/...", "discord://webhook/...")
//
// The package uses a default router, but you can also create custom routers
// using router.NewWithOptions for more control over the notification pipeline.
package shoutrrr

import (
	"fmt"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// defaultRouter manages the creation and routing of notification services.
var defaultRouter = router.ServiceRouter{
	Timeout: router.DefaultTimeout,
}

// Send delivers a notification message using the specified URL.
func Send(rawURL, message string) error {
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
//
// Deprecated: Use CreateSenderWithOptions.
func CreateSender(rawURLs ...string) (*router.ServiceRouter, error) {
	serviceRouter, err := router.NewWithOptions(nil, types.SenderOptions{}, rawURLs...)
	if err != nil {
		return nil, fmt.Errorf("creating sender for URLs %v: %w", rawURLs, err)
	}

	return serviceRouter, nil
}

// CreateSenderWithOptions constructs a new service router for the given URLs
// and SenderOptions without a logger.
func CreateSenderWithOptions(opts types.SenderOptions, rawURLs ...string) (*router.ServiceRouter, error) {
	serviceRouter, err := router.NewWithOptions(nil, opts, rawURLs...)
	if err != nil {
		return nil, fmt.Errorf("creating sender with options for URLs %v: %w", rawURLs, err)
	}

	return serviceRouter, nil
}

// NewSender constructs a new service router with a logger for the given URLs.
//
// Deprecated: Use NewSenderWithOptions.
func NewSender(logger types.StdLogger, serviceURLs ...string) (*router.ServiceRouter, error) {
	serviceRouter, err := router.NewWithOptions(logger, types.SenderOptions{}, serviceURLs...)
	if err != nil {
		return nil, fmt.Errorf("creating sender with logger for URLs %v: %w", serviceURLs, err)
	}

	return serviceRouter, nil
}

// NewSenderWithOptions constructs a new service router using the given logger,
// SenderOptions, and URLs. Use this to supply a custom HTTPClient for all
// outbound requests (e.g. for SSRF protection or custom proxies/TLS).
func NewSenderWithOptions(logger types.StdLogger, opts types.SenderOptions, serviceURLs ...string) (*router.ServiceRouter, error) {
	serviceRouter, err := router.NewWithOptions(logger, opts, serviceURLs...)
	if err != nil {
		return nil, fmt.Errorf("creating sender with options for URLs %v: %w", serviceURLs, err)
	}

	return serviceRouter, nil
}

// SetLogger configures the logger for all services in the default router.
func SetLogger(logger types.StdLogger) {
	defaultRouter.SetLogger(logger)
}

// Version returns the current Shoutrrr version.
func Version() string {
	return meta.Version
}
