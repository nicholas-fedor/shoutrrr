// Package bark provides a notification service for sending push notifications via Bark.
//
// The Bark service allows sending push notifications to iOS devices through a self-hosted
// Bark server. Bark is an iOS push notification service that provides a simple REST API
// for sending custom notifications with titles, sounds, badges, and more. It supports
// device-specific keys for targeted notifications and offers rich notification features
// including custom icons (iOS 15+), notification grouping, and actionable notifications.
//
// For more information about the Bark server, see: https://github.com/Finb/Bark
//
// # URL Format
//
// The service URL follows the format:
//
//	bark://:devicekey@host[:port][/path][?query]
//
// Where:
//   - devicekey: device key for authentication (required, passed as password in URL)
//   - host: Bark server hostname (required)
//   - port: optional port number (default: 443 for HTTPS, 80 for HTTP)
//   - path: optional server path (default: "/")
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - title: notification title (default: empty, uses app name)
//   - sound: notification sound (default: empty, uses default system sound)
//     See: https://github.com/Finb/Bark/tree/master/Sounds
//   - badge: badge number displayed next to app icon (default: 0)
//   - icon: URL to notification icon (iOS 15+ only)
//   - group: notification group identifier
//   - url: URL to open when notification is tapped
//   - category: notification category (reserved for future use)
//   - copy: text to copy to clipboard when notification is tapped
//   - scheme: server protocol, "http" or "https" (default: "https")
//
// # Templates
//
// Bark does not use templates in the Shoutrrr sense, but supports rich notification
// formatting through its native features like titles, sounds, badges, and custom actions.
//
// # Usage Examples
//
// ## Basic notification
//
//	url := "bark://:devicekey@example.com"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification with custom title and sound
//
//	url := "bark://:devicekey@example.com?title=Alert&sound=alarm"
//	err := shoutrrr.Send(url, "System is down!")
//
// ## Notification with badge count
//
//	url := "bark://:devicekey@example.com?title=Messages&badge=5"
//	err := shoutrrr.Send(url, "You have 5 new messages")
//
// ## Notification with custom icon
//
//	url := "bark://:devicekey@example.com?icon=https://example.com/icon.png"
//	err := shoutrrr.Send(url, "Notification with custom icon")
//
// ## Notification with group and URL
//
//	url := "bark://:devicekey@example.com?group=alerts&url=https://example.com/details"
//	err := shoutrrr.Send(url, "Click to view alert details")
//
// ## Notification that copies to clipboard
//
//	url := "bark://:devicekey@example.com?copy=https://example.com/api/token"
//	err := shoutrrr.Send(url, "API token copied to clipboard")
//
// ## Notification to custom server path
//
//	url := "bark://:devicekey@example.com:2225/bark/push"
//	err := shoutrrr.Send(url, "Notification to custom path")
//
// ## Notification with HTTP (insecure)
//
//	url := "bark://:devicekey@example.com?scheme=http"
//	err := shoutrrr.Send(url, "Insecure notification (not recommended)")
//
// # Common Use Cases
//
// ## System Monitoring Alerts
//
// Send alerts from monitoring systems like Nagios, Prometheus, or Zabbix:
//
//	url := "bark://:devicekey@example.com?title=Alert&sound=alarm"
//	err := shoutrrr.Send(url, "Disk space critical: 95% used")
//
// ## CI/CD Pipeline Notifications
//
// Notify about build status from Jenkins, GitHub Actions, or GitLab CI:
//
//	url := "bark://:devicekey@example.com?title=Build%20Failed&sound=alert"
//	err := shoutrrr.Send(url, "Build #123 failed on main branch")
//
// ## Home Automation
//
// Integrate with home automation systems like Home Assistant:
//
//	url := "bark://:devicekey@example.com?title=Security&sound=alarm&group=home"
//	err := shoutrrr.Send(url, "Motion detected at front door")
//
// ## Backup Completion Notifications
//
// Notify when backups complete:
//
//	url := "bark://:devicekey@example.com?title=Backup&sound=complete"
//	err := shoutrrr.Send(url, "Daily backup finished successfully")
//
// ## Application Health Monitoring
//
// Send health check notifications from applications:
//
//	url := "bark://:devicekey@example.com?title=Health%20Check"
//	err := shoutrrr.Send(url, "All services are healthy")
//
// ## Incident Response
//
// Send urgent notifications for incident response:
//
//	url := "bark://:devicekey@example.com?title=INCIDENT&sound=alarm&group=ops"
//	err := shoutrrr.Send(url, "Database connection lost - immediate attention required")
//
// # Error Handling
//
// The service returns errors for various failure scenarios. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send Bark notification: %v", err)
//	}
//
// Common error scenarios:
//   - Invalid device key format
//   - Empty message content
//   - Network connectivity issues
//   - Invalid server hostname or port
//   - TLS certificate verification failures (when HTTPS is used)
//   - HTTP error status codes (4xx/5xx) from the Bark server
//   - Invalid icon URL
//   - Server-side errors (returned in API response message)
//
// # Security Considerations
//
// - Use HTTPS whenever possible (default behavior) to protect notification content in transit
// - Store device keys securely - avoid hardcoding them in source code or configuration files
// - Device keys are device-specific and act as authentication tokens
// - Be cautious with scheme=http - only use in trusted environments as it disables encryption
// - Validate icon URLs - ensure they point to trusted sources
// - Consider message content - avoid sending sensitive information in notification messages
// - Use appropriate notification sounds - avoid disruptive sounds in production environments
// - Implement proper access controls on your Bark server to prevent unauthorized key usage
//
// # Limitations and Behaviors
//
//   - Device key is required and must be provided in the URL password field
//   - Message content cannot be empty
//   - Icon is only available on iOS 15 or later
//   - Category is reserved for future use
//   - Badge number can be any non-negative integer
//   - Sound values must match available sounds in the Bark server
//     (see: https://github.com/Finb/Bark/tree/master/Sounds)
//   - Custom sounds can be added to the Bark server
//   - Group helps organize notifications but display depends on the client
//   - URL field specifies what URL to open when notification is tapped
//   - Copy field specifies text to copy to clipboard when notification is tapped
//   - Server path defaults to "/" if not specified
//   - Default scheme is "https" for secure communication
//   - HTTP client timeout is set for API requests
//   - Server response includes code, message, and timestamp upon delivery
//   - Failed requests return structured error responses with error codes and descriptions
package bark
