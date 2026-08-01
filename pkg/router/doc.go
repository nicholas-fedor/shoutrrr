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
//   - Dispatching structured MessageItems to services that implement RichSender
//   - Propagating context.Context to services that implement ContextSender
//
// Errors returned from Send/SendAsync/SendItems are wrapped in *types.TargetError
// so callers can identify which service failed and use errors.Is/errors.As against
// the underlying error.
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
// Schema Registry (schema_registry.go)
//
// SupportedSchemas and SupportsSchema expose the set of registered service
// schemes, enabling discovery without constructing a router.
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
//
// For SSRF protection or custom egress control, supply a custom HTTP client:
//
//	opts := types.SenderOptions{
//	    HTTPClient: myClient,
//	}
//	router, err := router.NewWithOptions(logger, opts, "slack://webhook/...")
//	if err != nil {
//	    // handle error
//	}
//
//	errors := router.Send("Hello, World!", nil)
package router
