// Package pushover provides a notification service for sending push notifications via Pushover.
//
// The Pushover service allows sending push notifications to devices running the Pushover
// application on iOS, Android, and Desktop. Pushover is a simple notification service
// that delivers instant notifications from scripts, applications, and devices to your
// phone or desktop. It supports different priority levels, custom sounds, and
// notification titles.
//
// # URL Format
//
// The service URL follows the format:
//
//	pushover://userKey:apiToken@host[?query]
//
// Where:
//   - userKey: Pushover user key (required, found in your Pushover dashboard)
//   - apiToken: Pushover application API token (required, created in your application)
//   - host: optional, but typically not used (included for URL scheme compatibility)
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - devices: comma-separated list of target device names (optional, sends to all devices if not specified)
//   - priority: message priority (-2 to 1, default: 0)
//   - title: notification title (optional, defaults to application name)
//
// Priority levels:
//   - -2: lowest - no sound, no notification, stored for later
//   - -1: low - no sound, vibration only
//   - 0: normal (default) - normal notification behavior
//   - 1: high - bypasses quiet hours, plays sound
//
// # Templates
//
// Pushover does not use templates in the Shoutrrr sense, but supports notification
// customization through its native features like titles and priority levels.
//
// # Usage Examples
//
// ## Basic notification to all devices
//
//	url := "pushover://userkey:apitoken@"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification to specific devices
//
//	url := "pushover://userkey:apitoken@?devices=phone,tablet"
//	err := shoutrrr.Send(url, "Hello on specific devices!")
//
// ## Notification with custom title
//
//	url := "pushover://userkey:apitoken@?title=Alert"
//	err := shoutrrr.Send(url, "System is down!")
//
// ## High priority notification
//
//	url := "pushover://userkey:apitoken@?priority=1"
//	err := shoutrrr.Send(url, "Critical alert - immediate attention required!")
//
// ## Quiet notification (no sound)
//
//	url := "pushover://userkey:apitoken@?priority=-1"
//	err := shoutrrr.Send(url, "Background notification")
//
// # Common Use Cases
//
// ## System Monitoring Alerts
//
// Send alerts from monitoring systems like Nagios, Prometheus, or Zabbix:
//
//	url := "pushover://userkey:apitoken@?title=Alert&priority=1"
//	err := shoutrrr.Send(url, "Disk space critical: 95% used")
//
// ## CI/CD Pipeline Notifications
//
// Notify about build status from Jenkins, GitHub Actions, or GitLab CI:
//
//	url := "pushover://userkey:apitoken@?title=Build%20Failed&priority=1"
//	err := shoutrrr.Send(url, "Build #123 failed on main branch")
//
// ## Home Automation
//
// Integrate with home automation systems:
//
//	url := "pushover://userkey:apitoken@?title=Security&priority=1"
//	err := shoutrrr.Send(url, "Motion detected at front door")
//
// ## Backup Completion Notifications
//
// Notify when backups complete:
//
//	url := "pushover://userkey:apitoken@?title=Backup"
//	err := shoutrrr.Send(url, "Daily backup finished successfully")
//
// ## Application Health Monitoring
//
// Send health check notifications from applications:
//
//	url := "pushover://userkey:apitoken@?title=Health%20Check&priority=-1"
//	err := shoutrrr.Send(url, "All services are healthy")
//
// ## Incident Response
//
// Send urgent notifications for incident response:
//
//	url := "pushover://userkey:apitoken@?title=INCIDENT&priority=1"
//	err := shoutrrr.Send(url, "Database connection lost - immediate attention required")
//
// # Error Handling
//
// The service returns errors for various failure scenarios. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send Pushover notification: %v", err)
//	}
//
// Common error scenarios:
//   - Invalid or missing user key
//   - Invalid or missing API token
//   - Network connectivity issues
//   - Invalid device names (devices that don't exist in your Pushover account)
//   - HTTP error status codes from the Pushover API
//   - Empty message content
//
// # Security Considerations
//
// - Store API tokens securely - avoid hardcoding them in source code or configuration files
// - Use appropriate priority levels - high priority notifications bypass quiet hours and should be reserved for critical events
// - Consider device-specific targeting when needed using the devices parameter
// - The user key is not secret, but the API token should be treated as a secret
// - Pushover notifications may contain sensitive information - consider this when sending notifications
// - Review Pushover's terms of service for any usage restrictions
//
// # Limitations and Behaviors
//
// - Message content cannot be empty
// - Priority can be set to values between -2 and 1
//   - Priority 2 (emergency) requires additional parameters (retry, expire) which are not currently supported
//
// - Device names must match exactly how they appear in your Pushover account
// - HTTP client timeout is set to 10 seconds for API requests
// - The Pushover API endpoint is https://api.pushover.net/1/messages.json
// - Notifications are delivered instantly under normal conditions
// - Priority -2 messages are stored for 7 days, -1 and 0 for 30 days, 1 for 7 days
// - Maximum message length is 1024 characters
package pushover
