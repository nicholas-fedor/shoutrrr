// Package generic provides a generic notification service for custom webhooks.
//
// The generic service allows sending notifications to any webhook endpoint that accepts
// HTTP POST requests, making it suitable for targets not explicitly supported by Shoutrrr.
// It supports flexible payload formatting through templates, custom headers, extra data,
// and various configuration options.
//
// # URL Format
//
// The service URL follows the format:
//
//	generic://host[:port]/path[?query]
//
// Where:
//   - host: the webhook endpoint hostname or IP address
//   - port: optional port number (defaults to 443 for HTTPS, 80 for HTTP)
//   - path: the webhook path
//   - query: configuration parameters
//
// # Configuration Options
//
// The following query parameters can be used to configure the service:
//
//   - template: payload template ("json" for JSON format, or custom template name)
//   - contenttype: HTTP Content-Type header (default: "application/json")
//   - method: HTTP method (default: "POST")
//   - disabletls: set to "yes" to use HTTP instead of HTTPS
//   - titlekey: JSON key for title field (default: "title")
//   - messagekey: JSON key for message field (default: "message")
//   - @HeaderName: custom HTTP headers (e.g., @Authorization=Bearer token)
//   - $ExtraKey: extra data to include in JSON payload (e.g., $context=value)
//
// # Templates
//
// ## JSON Template
//
// When template=json, the payload is a JSON object containing the message and optional title:
//
//	{
//	  "title": "Notification Title",
//	  "message": "Notification message"
//	}
//
// Custom keys can be specified with titlekey and messagekey parameters.
//
// ## Custom Templates
//
// Custom templates can be registered using the service's template system.
// Templates use Go template syntax and receive the notification parameters as context.
//
// # Shortcut URLs
//
// For convenience, you can prefix any webhook URL with "generic+" to use the generic service:
//
//	https://example.com/webhook → generic+https://example.com/webhook
//
// Note: Query parameters cannot be used with the shortcut format.
//
// # Examples
//
// ## Basic webhook notification
//
//	url := "generic://api.example.com/webhook"
//	err := shoutrrr.Send(url, "Hello, webhook!")
//
// ## JSON payload with custom headers
//
//	url := "generic://api.example.com/webhook?template=json&@Authorization=Bearer token123"
//	err := shoutrrr.Send(url, "Alert message", map[string]string{"title": "System Alert"})
//
// ## HTTP webhook with extra data
//
//	url := "generic://192.168.1.100:8123/api/webhook?template=json&disabletls=yes&$context=home-assistant"
//	err := shoutrrr.Send(url, "Motion detected")
//
// ## Custom template
//
//	service := &generic.Service{}
//	service.SetTemplateString("custom", `{"alert": "{{.message}}", "level": "info"}`)
//	url := "generic://api.example.com/webhook?template=custom"
//	err := service.Send("Custom alert message", nil)
//
// # Common Use Cases
//
// ## Home Assistant Integration
//
// Send notifications to Home Assistant webhooks:
//
//	url := "generic://home-assistant.local:8123/api/webhook/webhook-id?template=json&disabletls=yes"
//	err := shoutrrr.Send(url, "Front door opened")
//
// In Home Assistant automations, access the message with: {{ trigger.json.message }}
//
// ## Generic Webhook Service Integration
//
// Send notifications to any webhook service that accepts JSON payloads:
//
//	url := "generic://your-service.com/api/webhook?template=json&@Authorization=Bearer YOUR_TOKEN"
//	err := shoutrrr.Send(url, "Your notification message", map[string]string{"title": "Alert"})
//
// To adapt this template for your specific webhook service:
//
// 1. Replace "your-service.com" with your webhook endpoint hostname or IP address
// 2. Update the path "/api/webhook" to match your service's webhook path
// 3. Set the appropriate authorization header (e.g., Bearer token, API key, or custom header)
// 4. Adjust query parameters as needed for your service's requirements
// 5. Customize the title and message keys if your service expects different field names
// 6. Use disabletls=yes for HTTP endpoints or omit for HTTPS
//
// ## Custom API Endpoints
//
// Integrate with any REST API that accepts POST requests:
//
//	url := "generic://api.service.com/notifications?@X-API-Key=your-key&template=json"
//	err := shoutrrr.Send(url, "New order received", map[string]string{"order_id": "12345"})
//
// ## Monitoring Systems
//
// Send alerts to monitoring dashboards:
//
//	url := "generic://monitoring.company.com/alerts?template=json&$source=shoutrrr"
//	err := shoutrrr.Send(url, "Server CPU usage high", map[string]string{"severity": "warning"})
//
// # Error Handling
//
// The service returns errors for network failures, HTTP error status codes (4xx/5xx),
// and configuration issues. Always check the returned error:
//
//	err := shoutrrr.Send(url, message)
//	if err != nil {
//	    log.Printf("Failed to send notification: %v", err)
//	}
//
// # Security Considerations
//
// - Use HTTPS whenever possible (avoid disabletls=yes in production)
// - Store API keys and tokens securely, not in URLs
// - Validate webhook endpoints before deployment
// - Consider rate limiting and authentication requirements of target services
package generic
