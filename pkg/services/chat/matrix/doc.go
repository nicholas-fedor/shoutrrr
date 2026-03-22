// Package matrix provides a notification service for sending messages via the Matrix protocol.
//
// The Matrix service enables sending notifications to Matrix rooms using the Matrix
// Client-Server API. Matrix is an open protocol for real-time communication that
// supports end-to-end encryption, room-based messaging, and integration with various
// clients. This service supports both password-based and token-based authentication.
//
// # URL Format
//
// The service URL follows the format:
//
//	matrix://[user[:password]@]host[:port][?query]
//
// Where:
//   - user: Matrix username (required for password authentication, omit when using access token)
//   - password: Matrix password or access token (required)
//   - host: Matrix server hostname (required)
//   - port: optional port number (default: 443)
//   - query: configuration parameters
//
// # Authentication
//
// The service supports two authentication methods:
//
// Password Authentication:
// When a username is provided along with a password, the service will attempt to
// authenticate using the m.login.password flow:
//
//	matrix://user:password@matrix.example.com
//
// Token Authentication:
// When no username is provided, the password field is treated as an access token:
//
//	matrix://:access_token@matrix.example.com
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - rooms: comma-separated list of room aliases (e.g., "#room:example.com") or
//     room IDs (e.g., "!roomid:example.com"). If not specified, messages are sent
//     to all joined rooms.
//   - title: notification title to prepend to the message
//   - disableTLS: set to "yes" or "true" to disable TLS (not recommended)
//
// # Templates
//
// Matrix does not use templates in the Shoutrrr sense, but supports adding a title
// prefix to messages through the title parameter.
//
// # Usage Examples
//
// ## Basic notification to all joined rooms
//
//	url := "matrix://:access_token@matrix.example.com"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification to specific rooms
//
//	url := "matrix://:access_token@matrix.example.com?rooms=#alerts:example.com,#general:example.com"
//	err := shoutrrr.Send(url, "Alert message")
//
// ## Notification with password authentication
//
//	url := "matrix://myuser:mypassword@matrix.example.com"
//	err := shoutrrr.Send(url, "Hello from myuser!")
//
// ## Notification with title
//
//	url := "matrix://:access_token@matrix.example.com?title=Alert"
//	err := shoutrrr.Send(url, "System is down!")
//
// ## Custom port and TLS disabled
//
//	url := "matrix://:access_token@matrix.example.com:8008?disableTLS=yes"
//	err := shoutrrr.Send(url, "Insecure notification")
//
// # Common Use Cases
//
// ## Monitoring System Alerts
//
// Send alerts from monitoring systems like Prometheus, Nagios, or Zabbix:
//
//	url := "matrix://:access_token@matrix.example.com?rooms=#alerts:example.com&title=Monitoring"
//	err := shoutrrr.Send(url, "Disk space critical: 95% used")
//
// ## CI/CD Pipeline Notifications
//
// Notify about build status from Jenkins, GitHub Actions, or GitLab CI:
//
//	url := "matrix://:access_token@matrix.example.com?rooms=#builds:example.com&title=Build%20Status"
//	err := shoutrrr.Send(url, "Build #123 passed")
//
// ## Team Notifications
//
// Send notifications to team rooms:
//
//	url := "matrix://:access_token@matrix.example.com?rooms=#team:example.com"
//	err := shoutrrr.Send(url, "Deployment started")
//
// ## Application Notifications
//
// Send notifications from applications:
//
//	url := "matrix://myuser:mypassword@matrix.example.com?rooms=#notifications:example.com"
//	err := shoutrrr.Send(url, "New order received: #12345")
//
// # Room Alias Resolution
//
// The service automatically resolves room aliases to room IDs. If a room alias
// doesn't start with # or !, # is automatically prepended:
//
//	// These are equivalent:
//	rooms=general
//	rooms=#general:example.com
//
// Room IDs must start with !:
//
//	rooms=!roomid:example.com
//
// # Error Handling
//
// The service returns errors for various failure scenarios. Always check the
// returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send Matrix notification: %v", err)
//	}
//
// Common error scenarios:
//   - ErrMissingHost: No Matrix server host provided
//   - ErrMissingCredentials: No password or access token provided
//   - ErrClientNotInitialized: Client failed to initialize
//   - ErrUnsupportedLoginFlows: Server doesn't support password or token login
//   - ErrUnexpectedStatus: Unexpected HTTP response from server
//
// # Security Considerations
//
// - Store access tokens securely, never in URLs in production
// - Prefer token authentication over password authentication when possible
// - Use TLS (default) for all production connections
// - Consider room access controls when sending to shared rooms
// - Access tokens provide full access to your Matrix account
//
// # Matrix API
//
// This service uses the Matrix Client-Server API v3. Key endpoints include:
//   - POST /_matrix/client/v3/login: Authentication
//   - POST /_matrix/client/v3/join/{roomIdOrAlias}: Join room
//   - GET /_matrix/client/v3/joined_rooms: List joined rooms
//   - PUT /_matrix/client/v3/rooms/{roomId}/send/m.room.message: Send message
//
// For more information about Matrix, see: https://matrix.org/
package matrix
