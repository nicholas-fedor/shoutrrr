// Package teams provides a notification service for sending messages to Microsoft Teams via webhooks.
//
// This package is part of the shoutrrr notification library and implements the Service interface
// for delivering notifications to Microsoft Teams using Office 365 connector webhooks.
//
// # Service Configuration
//
// The Service struct provides the main functionality for sending notifications to Microsoft Teams.
// It embeds standard.Standard for common service features and supports both URL-based configuration
// and direct webhook URL parsing.
//
// # Configuration Fields
//
// The Config struct supports the following settings:
//
//   - Group: The Office 365 group name (optional)
//   - Tenant: The Office 365 tenant ID (optional)
//   - AltID: The first part of the webhook URL path (optional)
//   - GroupOwner: The second part of the webhook URL path (optional)
//   - ExtraID: The third part of the webhook URL path (optional)
//   - Title: The notification title displayed in Teams (optional)
//   - Color: The message accent color in hex format (optional)
//   - Host: The webhook host, typically organization.webhook.office.com (required)
//
// # MessageCard Format
//
// Notifications are sent using the Microsoft Teams MessageCard format, which supports:
//   - Markdown content
//   - Themed colors for visual distinction
//   - Section-based layout
//   - Summary text for notification previews
//
// # Usage Example
//
//	service := &teams.Service{}
//	err := service.Initialize(configURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// # Webhook URL Parsing
//
// The package supports parsing full Microsoft Teams webhook URLs for easier configuration:
//
//	teams+https://organization.webhook.office.com/webhookb2/...
//
// For more information about Teams connectors, visit:
// https://docs.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/what-are-webhooks-and-connectors
package teams
