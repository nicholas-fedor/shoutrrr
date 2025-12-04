// Package gotify provides a notification service for sending push notifications via Gotify servers.
//
// The Gotify service allows sending push notifications to Gotify servers, which can be received
// by mobile apps, web browsers, and other clients. Gotify is a self-hosted notification service
// that provides a simple REST API for sending messages with titles, priorities, and custom extras.
// It supports both token-based authentication in query parameters or HTTP headers, and offers
// flexible configuration for secure and reliable message delivery.
//
// # URL Format
//
// The service URL follows the format:
//
//	gotify://host[:port][/path]/token[?query]
//
// Where:
//   - host: Gotify server hostname (required)
//   - port: optional port number (default: 443 for HTTPS, 80 for HTTP)
//   - path: optional subpath for Gotify installation (e.g., "/gotify")
//   - token: application token for authentication (required, must be 15 characters starting with 'A')
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - title: notification title (default: "Shoutrrr notification")
//   - priority: message priority (0 to 10, where higher numbers indicate higher priority; negative values have special meanings in some clients)
//   - disabletls: set to "yes" to disable TLS certificate verification (insecure, use with caution)
//   - useheader: set to "yes" to send token in X-Gotify-Key header instead of URL query parameter
//   - extras: JSON string containing additional key-value pairs to include in the notification
//
// # Templates
//
// Gotify does not use templates in the Shoutrrr sense, but supports rich message formatting
// through its native features like titles, priorities, and custom extras fields that can
// contain structured data for enhanced client-side processing.
//
// # Usage Examples
//
// ## Basic notification
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification with custom title and high priority
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?title=Alert&priority=1"
//	err := shoutrrr.Send(url, "System is down!")
//
// ## Notification with extras
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?title=Backup&extras=%7B%22action%22%3A%22view%22%2C%22url%22%3A%22https%3A%2F%2Fexample.com%2Flogs%22%7D"
//	err := shoutrrr.Send(url, "Backup completed successfully")
//
// ## Notification with header authentication
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?useheader=yes"
//	err := shoutrrr.Send(url, "Authenticated notification")
//
// ## Notification with disabled TLS (insecure)
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?disabletls=yes"
//	err := shoutrrr.Send(url, "Insecure notification")
//
// ## Notification with custom path
//
//	url := "gotify://gotify.example.com/gotify/Aaa.bbb.ccc.ddd"
//	err := shoutrrr.Send(url, "Notification to subpath installation")
//
// # Common Use Cases
//
// ## System Monitoring Alerts
//
// Send alerts from monitoring systems like Nagios, Prometheus, or Zabbix:
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?title=Alert&priority=1"
//	err := shoutrrr.Send(url, "Disk space critical: 95% used")
//
// ## CI/CD Pipeline Notifications
//
// Notify about build status from Jenkins, GitHub Actions, or GitLab CI:
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?title=Build%20Failed&priority=0"
//	err := shoutrrr.Send(url, "Build #123 failed on main branch")
//
// ## Home Automation
//
// Integrate with home automation systems like Home Assistant:
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?title=Security&priority=1&extras=%7B%22sensor%22%3A%22motion%22%2C%22location%22%3A%22front-door%22%7D"
//	err := shoutrrr.Send(url, "Motion detected at front door")
//
// ## Backup Completion Notifications
//
// Notify when backups complete:
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?title=Backup&extras=%7B%22type%22%3A%22daily%22%2C%22size%22%3A%225GB%22%7D"
//	err := shoutrrr.Send(url, "Daily backup finished successfully")
//
// ## Application Health Monitoring
//
// Send health check notifications from applications:
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?title=Health%20Check&priority=-1"
//	err := shoutrrr.Send(url, "All services are healthy")
//
// ## Incident Response
//
// Send urgent notifications for incident response:
//
//	url := "gotify://gotify.example.com/Aaa.bbb.ccc.ddd?title=INCIDENT&priority=1&extras=%7B%22severity%22%3A%22critical%22%2C%22team%22%3A%22ops%22%7D"
//	err := shoutrrr.Send(url, "Database connection lost - immediate attention required")
//
// # Error Handling
//
// The service returns errors for various failure scenarios. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send Gotify notification: %v", err)
//	}
//
// Common error scenarios:
//   - Invalid token format (must be 15 characters starting with 'A', valid characters only)
//   - Empty message content
//   - Network connectivity issues
//   - Invalid server hostname or port
//   - TLS certificate verification failures (when TLS is enabled)
//   - HTTP error status codes (4xx/5xx) from the Gotify server
//   - Invalid JSON in extras parameter
//   - Authentication failures (invalid or expired token)
//
// # Security Considerations
//
// - Use HTTPS whenever possible (default behavior) to protect notification content in transit
// - Store application tokens securely - avoid hardcoding them in source code or configuration files
// - Use header-based authentication (useheader=yes) to avoid exposing tokens in server logs
// - Consider token entropy - Gotify tokens are application-specific and should be treated as secrets
// - Be cautious with disabletls=yes - only use in trusted environments as it disables certificate verification
// - Validate extras JSON content to prevent injection of malicious data
// - Use appropriate priority levels - high priority notifications should be reserved for critical events
// - Consider message content - avoid sending sensitive information in notification messages
// - Implement proper access controls on your Gotify server to prevent unauthorized token usage
//
// # Limitations and Behaviors
//
// - Application tokens must be exactly 15 characters long and start with 'A'
// - Valid token characters are: a-z, A-Z, 0-9, ., -, _
// - Message content cannot be empty
// - Priority can be set to a value between 0 and 10, where 0 is the lowest and 10 is the highest priority. Negative values have special meanings in some clients.
// - Extras field accepts valid JSON objects for custom data
// - HTTP client timeout is set to 10 seconds for API requests
// - TLS certificate verification can be disabled but is not recommended for production use
// - Header authentication sends token in X-Gotify-Key header and cleans up after each request
// - Server response includes message ID, app ID, and timestamp upon successful delivery
// - Failed requests return structured error responses with error codes and descriptions
package gotify
