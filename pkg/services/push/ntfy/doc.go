// Package ntfy provides a notification service for sending push notifications via ntfy.sh.
//
// The ntfy service allows sending notifications to ntfy topics, which can be subscribed to
// by mobile apps, web browsers, and other clients. It supports rich notifications with
// titles, priorities, tags, attachments, actions, and more. ntfy is a simple HTTP-based
// pub-sub notification service that works with any programming language.
//
// # URL Format
//
// The service URL follows the format:
//
//	ntfy://[user:password@]host[:port]/topic[?query]
//
// Where:
//   - user: optional username for authentication
//   - password: optional password for authentication
//   - host: ntfy server hostname (default: ntfy.sh)
//   - port: optional port number (default: 443 for HTTPS, 80 for HTTP)
//   - topic: target topic name (required)
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - title: notification title
//   - priority: message priority (1=min, 2=low, 3=default, 4=high, 5=max/urgent)
//   - tags: comma-separated list of tags (may map to emojis)
//   - actions: semicolon-separated list of user action buttons
//   - click: URL to open when notification is clicked
//   - attach: URL of attachment to include
//   - filename: filename for attachment
//   - delay: timestamp or duration for delayed delivery (e.g., "30m", "1h", "2023-12-25T10:00:00Z")
//   - email: email address for email notifications
//   - icon: URL to use as notification icon
//   - cache: whether to cache messages (default: yes)
//   - firebase: whether to send via Firebase (default: yes)
//   - disabletls: set to "yes" to disable TLS verification
//
// # Templates
//
// ntfy does not use templates in the Shoutrrr sense, but supports rich message formatting
// through its native features like titles, tags, priorities, and actions.
//
// # Usage Examples
//
// ## Basic notification
//
//	url := "ntfy://mytopic"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification with title and high priority
//
//	url := "ntfy://mytopic?title=Alert&priority=high"
//	err := shoutrrr.Send(url, "System is down!")
//
// ## Notification with tags and actions
//
//	url := "ntfy://mytopic?tags=warning,server&actions=view,%20View%20logs,%20https://logs.example.com"
//	err := shoutrrr.Send(url, "Server error occurred")
//
// ## Notification with attachment
//
//	url := "ntfy://mytopic?title=Backup&attach=https://files.example.com/backup.zip&filename=backup.zip"
//	err := shoutrrr.Send(url, "Backup completed successfully")
//
// ## Delayed notification
//
//	url := "ntfy://mytopic?delay=1h"
//	err := shoutrrr.Send(url, "Reminder: Meeting in 1 hour")
//
// ## Notification with authentication
//
//	url := "ntfy://user:password@ntfy.example.com/mytopic"
//	err := shoutrrr.Send(url, "Authenticated notification")
//
// # Common Use Cases
//
// ## System Monitoring Alerts
//
// Send alerts from monitoring systems like Nagios, Prometheus, or Zabbix:
//
//	url := "ntfy://alerts?priority=high&tags=warning,server"
//	err := shoutrrr.Send(url, "Disk space critical: 95% used")
//
// ## CI/CD Pipeline Notifications
//
// Notify about build status from Jenkins, GitHub Actions, or GitLab CI:
//
//	url := "ntfy://builds?tags=success,github&title=Build%20Successful"
//	err := shoutrrr.Send(url, "Build #123 passed")
//
// ## Home Automation
//
// Integrate with home automation systems like Home Assistant:
//
//	url := "ntfy://home?tags=home,security&priority=urgent"
//	err := shoutrrr.Send(url, "Motion detected at front door")
//
// ## Backup Completion Notifications
//
// Notify when backups complete:
//
//	url := "ntfy://backups?tags=backup,done&title=Backup%20Complete"
//	err := shoutrrr.Send(url, "Daily backup finished successfully")
//
// ## Scheduled Reminders
//
// Send delayed notifications for reminders:
//
//	url := "ntfy://reminders?delay=9am&title=Morning%20Reminder"
//	err := shoutrrr.Send(url, "Don't forget the team meeting!")
//
// # Error Handling
//
// The service returns errors for network failures, HTTP error status codes (4xx/5xx),
// authentication failures, and configuration issues. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send ntfy notification: %v", err)
//	}
//
// Common error scenarios:
//   - Invalid topic name
//   - Authentication failure (wrong credentials)
//   - Network connectivity issues
//   - Server rate limiting
//   - Invalid attachment URLs
//
// # Security Considerations
//
// - Use HTTPS whenever possible (default behavior)
// - Store credentials securely, not in URLs if possible
// - Consider topic names as passwords - use unique, hard-to-guess names
// - Be cautious with attachment URLs - ensure they are from trusted sources
// - Use authentication for private topics
// - Consider message content - avoid sending sensitive information in notifications
// - Rate limiting may apply depending on server configuration
//
// # Limitations and Behaviors
//
// - Topic names are case-sensitive
// - Maximum message size depends on server configuration (typically 4KB)
// - Attachments are referenced by URL, not uploaded directly
// - Actions are limited to view, broadcast, and http types
// - Delayed delivery minimum is 10 seconds, maximum is 3 days
// - Messages may be cached on the server for up to 12 hours after delivery
// - Firebase notifications may not work in all regions or for all clients
// - Some features (like actions) are client-dependent and may not work on all platforms
package ntfy
