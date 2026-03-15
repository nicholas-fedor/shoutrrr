// Package join provides a notification service for sending push notifications via Join.
//
// The Join service allows sending push notifications to devices running the Join app
// (by Joaoapps). Join is a cross-platform notification service that works on Android,
// iOS, Windows, and other platforms. It enables you to send notifications, SMS,
// and other messages to your devices from any application that can make HTTP requests.
//
// For more information about Join, visit https://joaoapps.com/join/
//
// For more information about the Join API, visit https://joinjoaomgcd.appspot.com/
//
// # URL Format
//
// The service URL follows the format:
//
//	join://:apikey@join[?query]
//
// Where:
//   - apikey: Join API key/token (required) - found in the Join app settings
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - devices: comma-separated list of device IDs to send notification to (required)
//   - title: notification title (optional)
//   - icon: URL of icon to display with the notification (optional)
//
// # Templates
//
// Join does not use templates in the Shoutrrr sense, but supports native notification
// features like titles and custom icons that can be configured via query parameters.
//
// # Usage Examples
//
// ## Basic notification to specific device
//
//	url := "join://:my-api-key@join?devices=device-id-123"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification with title
//
//	url := "join://:my-api-key@join?devices=device-id-123&title=Alert"
//	err := shoutrrr.Send(url, "System is down!")
//
// ## Notification with custom icon
//
//	url := "join://:my-api-key@join?devices=device-id-123&title=Backup&icon=https://example.com/icon.png"
//	err := shoutrrr.Send(url, "Backup completed successfully")
//
// ## Notification to multiple devices
//
//	url := "join://:my-api-key@join?devices=device1,device2,device3&title=Multi-device"
//	err := shoutrrr.Send(url, "Sent to multiple devices!")
//
// # Common Use Cases
//
// ## System Monitoring Alerts
//
// Send alerts from monitoring systems like Nagios, Prometheus, or Zabbix:
//
//	url := "join://:my-api-key@join?devices=monitoring-device&title=Alert"
//	err := shoutrrr.Send(url, "Disk space critical: 95% used")
//
// ## CI/CD Pipeline Notifications
//
// Notify about build status from Jenkins, GitHub Actions, or GitLab CI:
//
//	url := "join://:my-api-key@join?devices=ci-device&title=Build%20Status"
//	err := shoutrrr.Send(url, "Build #123 completed successfully")
//
// ## Cross-Device Notifications
//
// Send notifications that appear on multiple devices:
//
//	url := "join://:my-api-key@join?devices=phone,laptop,tablet&title=Reminder"
//	err := shoutrrr.Send(url, "Don't forget the meeting!")
//
// ## Home Automation
//
// Integrate with home automation systems like Home Assistant:
//
//	url := "join://:my-api-key@join?devices=automation-device&title=Security"
//	err := shoutrrr.Send(url, "Motion detected at front door")
//
// ## Remote Device Commands
//
// Send commands to devices running the Join app:
//
//	url := "join://:my-api-key@join?devices=remote-device&title=Command"
//	err := shoutrrr.Send(url, "Run backup script now")
//
// # Error Handling
//
// The service returns errors for various failure scenarios. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send Join notification: %v", err)
//	}
//
// Common error scenarios:
//   - Missing API key (required)
//   - Missing device IDs (required)
//   - Network connectivity issues
//   - Invalid API key
//   - Device IDs that don't exist or are offline
//   - HTTP error status codes from Join API (non-2xx responses)
//
// # Security Considerations
//
// - Store API keys securely - avoid hardcoding them in source code or configuration files
// - API keys provide access to send notifications to your devices - treat them as secrets
// - Consider using separate API keys for different purposes to limit exposure
// - Device IDs are specific to each device and should be kept private
// - Be cautious about what data you send in notifications - they may be visible on lock screens
// - Review Join's data handling and privacy policies
// - Consider implementing rate limiting on your side to avoid triggering any limits
// - Message content should not contain sensitive information if notifications are visible on lock screens
//
// # Limitations and Behaviors
//
// - At least one device ID must be specified
// - API key is required and must be a valid Join API key
// - Multiple devices can be specified as comma-separated values; notification will be sent to all
// - The HTTP client timeout is the default Go HTTP client timeout
// - Join API returns HTTP 200 on successful notification sends
// - Failed requests (non-2xx responses) will return an error
// - Title parameter creates a notification with a title (without it, just the message is sent)
// - Icon parameter expects a valid URL to an image file
//
// # Join API Details
//
// The notification is sent as a POST request to Join's messaging API:
//
//	https://joinjoaomgcd.appspot.com/_ah/api/messaging/v1/sendPush
//
// The request includes the following parameters:
//
//   - apikey: Your Join API key
//   - deviceIds: Comma-separated list of device IDs
//   - text: The notification message body
//   - title: Optional notification title
//   - icon: Optional icon URL
//
// For more information about the Join API and available parameters, visit
// the Join API documentation at https://joinjoaomgcd.appspot.com/
package join
