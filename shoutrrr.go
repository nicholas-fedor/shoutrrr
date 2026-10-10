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
// For more complex scenarios, create a sender with multiple service URLs. Its
// Send returns one error entry per URL, in the order the URLs were given:
//
//	sender, err := shoutrrr.CreateSenderWithOptions(types.SenderOptions{}, "slack://webhook/...", "discord://webhook/...")
//	if err != nil {
//		// handle error
//	}
//	defer sender.Close()
//
//	errs := sender.Send("Hello, World!", nil)
//
// Use SendContext, or the sender's SendContext, to cancel a send or bound it with
// a deadline.
// Send and SendContext deliver a single message through a one-shot router, so the
// service's send budget applies. For more control over the notification pipeline,
// create a router with router.NewWithOptions.
package shoutrrr

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/nicholas-fedor/shoutrrr/internal/meta"
	"github.com/nicholas-fedor/shoutrrr/pkg/router"
	"github.com/nicholas-fedor/shoutrrr/pkg/types"
)

// rootLogger holds the logger that [SetLogger] configures for [Send] and
// [SendContext]. A nil value discards service output.
var rootLogger atomic.Pointer[types.StdLogger]

// Send delivers a notification message using the specified URL. It is
// [SendContext] without a caller context or params.
//
// Parameters:
//   - rawURL: the service URL to send to.
//   - message: the message to send.
//
// Returns:
//   - error: the locate or send failure, or nil on success.
func Send(rawURL, message string) error {
	return SendContext(context.Background(), rawURL, message, nil)
}

// SendContext delivers a notification message using the specified URL. It sends
// through a one-shot router, so the service's send budget applies and a failure
// is a *types.TargetError, and then closes the service, releasing resources such
// as an MQTT connection. A failure to close after the send is not reported.
//
// Parameters:
//   - ctx: cancels the send and bounds how long SendContext waits.
//   - rawURL: the service URL to send to.
//   - message: the message to send.
//   - params: per-send parameters, which may be nil.
//
// Returns:
//   - error: the locate or send failure, or nil on success.
func SendContext(ctx context.Context, rawURL, message string, params *types.Params) error {
	serviceRouter, err := router.NewWithOptions(loadRootLogger(), types.SenderOptions{})
	if err != nil {
		return fmt.Errorf("creating sender: %w", err)
	}

	if err := serviceRouter.AddService(rawURL); err != nil {
		return fmt.Errorf("locating service: %w", err)
	}

	// The message has been sent or has failed by the time Close runs, so a close
	// failure does not change the outcome.
	defer func() { _ = serviceRouter.Close() }()

	if err := serviceRouter.SendContext(ctx, message, params)[0]; err != nil {
		return fmt.Errorf("sending message: %w", err)
	}

	return nil
}

// CreateSender constructs a new service router for the given URLs without a logger.
//
// Deprecated: Use CreateSenderWithOptions.
//
//go:fix inline
func CreateSender(rawURLs ...string) (*router.ServiceRouter, error) {
	return CreateSenderWithOptions(types.SenderOptions{}, rawURLs...)
}

// CreateSenderWithOptions constructs a new service router for the given URLs
// and SenderOptions without a logger.
func CreateSenderWithOptions(opts types.SenderOptions, rawURLs ...string) (*router.ServiceRouter, error) {
	serviceRouter, err := router.NewWithOptions(nil, opts, rawURLs...)
	if err != nil {
		return nil, fmt.Errorf("creating sender: %w", err)
	}

	return serviceRouter, nil
}

// NewSender constructs a new service router with a logger for the given URLs.
//
// Deprecated: Use NewSenderWithOptions.
//
//go:fix inline
func NewSender(logger types.StdLogger, serviceURLs ...string) (*router.ServiceRouter, error) {
	return NewSenderWithOptions(logger, types.SenderOptions{}, serviceURLs...)
}

// NewSenderWithOptions constructs a new service router using the given logger,
// SenderOptions, and URLs. Use this to supply a custom HTTPClient for HTTP
// services and DialContext for TCP services (e.g. for SSRF protection or
// custom proxies/TLS).
func NewSenderWithOptions(logger types.StdLogger, opts types.SenderOptions, serviceURLs ...string) (*router.ServiceRouter, error) {
	serviceRouter, err := router.NewWithOptions(logger, opts, serviceURLs...)
	if err != nil {
		return nil, fmt.Errorf("creating sender: %w", err)
	}

	return serviceRouter, nil
}

// SetLogger configures the logger that [Send] and [SendContext] give their services.
// It is safe to call while sends are in progress, and applies to later sends.
//
// Parameters:
//   - logger: the logger for service output, or nil to discard it.
func SetLogger(logger types.StdLogger) {
	rootLogger.Store(&logger)
}

// Version returns the current Shoutrrr version.
func Version() string {
	return meta.Version
}

// loadRootLogger returns the logger that [SetLogger] configured.
//
// Returns:
//   - types.StdLogger: the configured logger, or nil when none is set.
func loadRootLogger() types.StdLogger {
	if logger := rootLogger.Load(); logger != nil {
		return *logger
	}

	return nil
}
