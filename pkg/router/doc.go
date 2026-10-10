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
//   - Propagating context.Context to services that implement ContextSender, through
//     SendContext and SendItemsContext
//   - Closing services that hold connections between sends, through Close
//
// Send, SendContext, SendItems, and SendItemsContext return one entry per
// configured URL, in the order the URLs were given, and the entry is nil when
// that send succeeded. SendAsync's channel receives one result per service, nil
// for a successful send, in the order the sends finish, and then closes. A failed
// send is, or wraps, a *types.TargetError, so callers can identify which service
// failed and use errors.Is/errors.As against the underlying error.
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
//	r, err := router.NewWithOptions(logger, types.SenderOptions{}, "slack://webhook/...", "discord://webhook/...")
//	if err != nil {
//	    // handle error
//	}
//	defer r.Close()
//
//	errs := r.Send("Hello, World!", nil)
//
// For more control, use individual methods:
//
//	service, err := r.Locate("slack://webhook/...")
//	if err != nil {
//	    // handle error
//	}
//
//	err := service.Send("Hello, World!", nil)
//
// For SSRF protection or custom egress control, set HTTPClient for HTTP
// services such as this slack:// URL. DialContext applies only to non-HTTP
// TCP services (SMTP, XMPP, and MQTT).
//
//	opts := types.SenderOptions{
//	    HTTPClient:  myClient,
//	    DialContext: myDial,
//	}
//	r, err := router.NewWithOptions(logger, opts, "slack://webhook/...")
//	if err != nil {
//	    // handle error
//	}
//	defer r.Close()
//
//	errs := r.Send("Hello, World!", nil)
package router
