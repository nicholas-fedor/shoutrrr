// Package pushbullet provides a notification service for sending push notifications via Pushbullet.
//
// The Pushbullet service allows sending push notifications to devices, email addresses,
// or channels that are connected to a Pushbullet account. Pushbullet is a cross-platform
// notification service that enables sending push notifications between devices, with
// support for note-type messages containing a title and body. It uses API token-based
// authentication and supports targeting specific devices, email contacts, or channels.
//
// # URL Format
//
// The service URL follows the format:
//
//	pushbullet://token[/targets][?query]
//
// Where:
//   - token: Pushbullet API token for authentication (required, must be 34 characters)
//   - targets: optional path components specifying recipients (devices, emails, or channels)
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - title: notification title (default: "Shoutrrr notification")
//
// # Target Types
//
// Targets can be specified in the URL path and support three formats:
//
//   - Device: a device identifier (e.g., "mydevice")
//   - Email: an email address (e.g., "user@example.com")
//   - Channel: a channel tag prefixed with # (e.g., "#mychannel")
//
// Multiple targets can be specified as path components:
//
//	pushbullet://token/device1/device2?title=Alert
//	pushbullet://token/user@example.com?title=Email%20Alert
//	pushbullet://token/#mychannel?title=Channel%20Alert
//
// # Templates
//
// Pushbullet does not use templates in the Shoutrrr sense, but supports note-type
// push notifications with titles and body content.
//
// # Usage Examples
//
// ## Basic notification to all devices
//
//	url := "pushbullet://o.34charactertokenstringhere1234"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification with custom title
//
//	url := "pushbullet://o.34charactertokenstringhere1234?title=Alert"
//	err := shoutrrr.Send(url, "System notification message")
//
// ## Notification to specific device
//
//	url := "pushbullet://o.34charactertokenstringhere1234/mydevice"
//	err := shoutrrr.Send(url, "Message to specific device")
//
// ## Notification to email address
//
//	url := "pushbullet://o.34charactertokenstringhere1234/user@example.com"
//	err := shoutrrr.Send(url, "Message to email recipient")
//
// ## Notification to channel
//
//	url := "pushbullet://o.34charactertokenstringhere1234/#mychannel"
//	err := shoutrrr.Send(url, "Message to channel subscribers")
//
// ## Notification to multiple targets
//
//	url := "pushbullet://o.34charactertokenstringhere1234/device1/device2/#channel"
//	err := shoutrrr.Send(url, "Message to multiple targets")
//
// # Common Use Cases
//
// ## System Monitoring Alerts
//
// Send alerts from monitoring systems like Nagios, Prometheus, or Zabbix:
//
//	url := "pushbullet://o.34charactertokenstringhere1234?title=Alert"
//	err := shoutrrr.Send(url, "Disk space critical: 95% used")
//
// ## CI/CD Pipeline Notifications
//
// Notify about build status from Jenkins, GitHub Actions, or GitLab CI:
//
//	url := "pushbullet://o.34charactertokenstringhere1234?title=Build%20Status"
//	err := shoutrrr.Send(url, "Build #123 completed successfully")
//
// ## Cross-Device Notifications
//
// Send notifications that appear on all your connected devices:
//
//	url := "pushbullet://o.34charactertokenstringhere1234"
//	err := shoutrrr.Send(url, "Reminder from my server")
//
// ## Team Notifications via Channel
//
// Share notifications with team members subscribed to a channel:
//
//	url := "pushbullet://o.34charactertokenstringhere1234/#operations"
//	err := shoutrrr.Send(url, "Deployment started")
//
// ## Email-Based Notifications
//
// Send notifications to specific email addresses:
//
//	url := "pushbullet://o.34charactertokenstringhere1234/admin@example.com?title=Admin%20Alert"
//	err := shoutrrr.Send(url, "Server requires attention")
//
// # Error Handling
//
// The service returns errors for various failure scenarios. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send Pushbullet notification: %v", err)
//	}
//
// Common error scenarios:
//   - Invalid token format (must be exactly 34 characters)
//   - Empty message content
//   - Network connectivity issues
//   - Invalid or non-existent target device
//   - Invalid email address format
//   - Invalid channel (channel must exist and be accessible)
//   - HTTP error status codes (4xx/5xx) from the Pushbullet API
//   - Authentication failures (invalid or expired token)
//   - Push notification marked as inactive by the server
//   - Response validation failures (mismatched title or body)
//
// # Security Considerations
//
// - API tokens are sensitive credentials - treat them like passwords
// - Store tokens securely - avoid hardcoding them in source code or public repositories
// - Use HTTPS (default) to protect token and message content in transit
// - Be cautious about who has access to your Pushbullet account and tokens
// - Consider message content - avoid sending sensitive information in notifications
// - Review Pushbullet's privacy policy and data handling practices
// - Rotate tokens periodically if supported
// - Use channel-based notifications for group communications instead of sharing tokens
//
// # Limitations and Behaviors
//
// - API token must be exactly 34 characters long
// - Message content cannot be empty
// - Title has a maximum length (typically truncated by the Pushbullet service)
// - Targets are processed sequentially - if one fails, the service returns an error
// - Channel names must be prefixed with # in the URL path
// - Email targets must be valid email addresses
// - Device targets must match the device identifier exactly
// - The service validates API responses to ensure message integrity
// - Push notifications are sent as "note" type messages
// - HTTP client timeout is set for API requests
// - Successful responses are validated for correctness (type, title, body, active status)
//
// # Related Links
//
// - Pushbullet Official Website: https://www.pushbullet.com
// - Pushbullet API Documentation: https://docs.pushbullet.com
// - Pushbullet Account Settings: https://www.pushbullet.com/settings
package pushbullet
