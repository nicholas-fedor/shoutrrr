// Package ifttt provides a notification service for sending notifications via IFTTT webhooks.
//
// The IFTTT service allows sending notifications by triggering IFTTT (If This Then That) webhook
// events. When an event is triggered, IFTTT can execute any of its thousands of applets to send
// notifications via email, SMS, push notifications, or many other channels. This integration
// provides a flexible way to connect Shoutrrr with the extensive IFTTT ecosystem.
//
// For more information about IFTTT, visit https://ifttt.com/
//
// For more information about IFTTT Webhooks, visit https://ifttt.com/maker_webhooks
//
// # URL Format
//
// The service URL follows the format:
//
//	ifttt://webhookID[?query]
//
// Where:
//   - webhookID: IFTTT webhook key (required) - found at https://ifttt.com/maker_webhooks
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - events: comma-separated list of IFTTT event names to trigger (required, at least one)
//   - value1: custom value for the IFTTT payload (optional)
//   - value2: custom value for the IFTTT payload (optional)
//   - value3: custom value for the IFTTT payload (optional)
//   - messagevalue: which value field (1-3) to use for the notification message (default: 2)
//   - titlevalue: which value field (1-3) to use for the notification title, or 0 to disable (default: 0)
//   - title: notification title to set (optional)
//
// # Templates
//
// IFTTT does not use templates in the Shoutrrr sense, but supports three value fields (value1,
// value2, value3) in its webhook payload. The notification message and title can be mapped to
// any of these fields using the messagevalue and titlevalue parameters respectively.
//
// # Usage Examples
//
// ## Basic notification
//
//	url := "ifttt://my-webhook-key?events=test-event"
//	err := shoutrrr.Send(url, "Hello from Shoutrrr!")
//
// ## Notification with custom values
//
//	url := "ifttt://my-webhook-key?events=alert&value1=Server&value2=Down&value3=Critical"
//	err := shoutrrr.Send(url, "Production server is down!")
//
// ## Notification with message in value1
//
//	url := "ifttt://my-webhook-key?events=notification&messagevalue=1"
//	err := shoutrrr.Send(url, "This message goes to Value1")
//
// ## Notification with title in value1 and message in value2
//
//	url := "ifttt://my-webhook-key?events=alert&titlevalue=1&messagevalue=2"
//	err := shoutrrr.Send(url, "Alert message here")
//
// ## Triggering multiple events
//
//	url := "ifttt://my-webhook-key?events=email-alert,push-notification,sms-alert"
//	err := shoutrrr.Send(url, "Critical system alert!")
//
// ## Notification with title parameter
//
//	url := "ifttt://my-webhook-key?events=notification&title=System%20Alert"
//	err := shoutrrr.Send(url, "Something needs your attention")
//
// # Common Use Cases
//
// ## System Monitoring Alerts
//
// Send alerts from monitoring systems like Nagios, Prometheus, or Zabbix to multiple notification
// channels through IFTTT applets:
//
//	url := "ifttt://my-webhook-key?events=system-alert&value1=Disk%20Space&value2=95%25&value3=CRITICAL"
//	err := shoutrrr.Send(url, "Disk space at 95% - immediate action required")
//
// ## CI/CD Pipeline Notifications
//
// Notify about build status from Jenkins, GitHub Actions, or GitLab CI:
//
//	url := "ifttt://my-webhook-key?events=build-failed&value1=Build%20%23123&value2=main&value3=2m"
//	err := shoutrrr.Send(url, "Build failed after running tests")
//
// ## Home Automation
//
// Integrate with home automation systems via IFTTT to send notifications to your phone,
// smart speaker, or other connected devices:
//
//	url := "ifttt://my-webhook-key?events=home-automation&value1=Front%20Door&value2=Motion%20Detected"
//	err := shoutrrr.Send(url, "Motion detected at front door")
//
// ## IoT Device Notifications
//
// Receive alerts from IoT devices and sensors:
//
//	url := "ifttt://my-webhook-key?events=sensor-alert&value1=Temperature&value2=32C&value3=Living%20Room"
//	err := shoutrrr.Send(url, "Temperature above threshold in Living Room")
//
// ## Calendar Reminders
//
// Trigger IFTTT applets for calendar events:
//
//	url := "ifttt://my-webhook-key?events=meeting-reminder&value1=Team%20Standup&value2=10:00%20AM"
//	err := shoutrrr.Send(url, "Reminder: Team standup in 15 minutes")
//
// ## Incident Response
//
// Send urgent notifications that trigger multiple IFTTT actions:
//
//	url := "ifttt://my-webhook-key?events=incident,pagerduty,slack-alert&value1=Database&value2=Connection%20Lost&value3=P1"
//	err := shoutrrr.Send(url, "Database connection lost - immediate attention required")
//
// # Error Handling
//
// The service returns errors for various failure scenarios. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send IFTTT notification: %v", err)
//	}
//
// Common error scenarios:
//   - Missing webhook ID (required)
//   - Missing events (required)
//   - Invalid messagevalue (must be 1-3)
//   - Invalid titlevalue (must be 0 or 1-3)
//   - titlevalue and messagevalue cannot be the same
//   - Network connectivity issues
//   - HTTP error status codes from IFTTT (non-2xx responses)
//   - Invalid URL query parameters
//
// # Security Considerations
//
// - Store webhook keys securely - avoid hardcoding them in source code or configuration files
// - Webhook keys provide access to trigger your IFTTT applets - treat them as secrets
// - Consider using separate webhook keys for different purposes to limit exposure
// - IFTTT webhook URLs are person-specific - ensure proper access controls
// - Be cautious about what data you send in value fields - they may be logged by IFTTT
// - Review IFTTT's data handling and privacy policies
// - Consider implementing rate limiting on your side to avoid triggering IFTTT limits
// - Message content should not contain sensitive information if not using private applets
//
// # Limitations and Behaviors
//
// - At least one event name must be specified
// - Webhook ID is required and must be a valid IFTTT webhook key
// - messagevalue must be between 1 and 3 (default: 2, which maps to Value2)
// - titlevalue must be between 0 and 3 (default: 0, disabled)
// - titlevalue and messagevalue cannot use the same value field
// - Multiple events can be specified as comma-separated values; all will be triggered
// - The HTTP client timeout is the default Go HTTP client timeout
// - IFTTT returns HTTP 200 on successful event triggers
// - Failed events (non-2xx responses) will return an error with the event name
// - When multiple events are specified, the service stops at the first failure
// - Value fields accept any string content; IFTTT applets determine how to use them
//
// # IFTTT Webhook Payload
//
// The notification is sent as a JSON POST request to IFTTT's webhook endpoint:
//
//	https://maker.ifttt.com/trigger/{event}/with/key/{webhookID}
//
// The payload contains up to three values:
//
//	{
//	  "value1": "...",
//	  "value2": "...",
//	  "value3": "..."
//	}
//
// Your IFTTT applets can then use these values in any way - sending emails, SMS messages,
// push notifications, or triggering other actions. For more information about IFTTT webhook
// payloads, see the IFTTT documentation.
package ifttt
