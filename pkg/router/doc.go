// Package router provides service routing functionality for shoutrrr notifications.
//
// The router package is responsible for routing notification messages to specific
// notification services based on service URLs. It manages the lifecycle of service
// instances, handles URL parsing, and provides both synchronous and asynchronous
// message delivery.
//
// # Main Components
//
// ServiceRouter (router.go)
//
// The primary type that manages notification services and routes messages.
// ServiceRouter provides methods for:
//   - Initializing services from URLs
//   - Sending messages synchronously and asynchronously
//   - Managing service lifecycles
//   - Queueing and flushing batched messages
//
// Service Factory (servicemap.go)
//
// Maps service schemes to their factory functions, enabling dynamic service
// instantiation. Supports over 20 notification services including:
//   - Chat: Discord, Slack, Telegram, Matrix, Mattermost, Teams, etc.
//   - Email: SMTP
//   - Push: Gotify, Pushover, Pushbullet, ntfy, etc.
//   - SMS: Twilio
//   - Incident: PagerDuty, OpsGenie
//
// Basic usage:
//
//	router, err := router.New(logger, "slack://webhook/...", "discord://webhook/...")
//	if err != nil {
//	    // handle error
//	}
//
//	errors := router.Send("Hello, World!", nil)
//
// For more control, use individual methods:
//
//	service, err := router.Locate("slack://webhook/...")
//	if err != nil {
//	    // handle error
//	}
//
//	err := service.Send("Hello, World!", nil)
package router
