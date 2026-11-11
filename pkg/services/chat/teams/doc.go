// Package teams provides a notification service for sending messages to Microsoft Teams
// via Power Automate workflow incoming webhooks using the Adaptive Card format.
//
// # Service Configuration
//
// The Config struct supports the following settings:
//
//   - Host: The full Power Automate workflow webhook URL, required to send.
//     Example: https://{environment}.environment.api.powerplatform.com:443/powerautomate/automations/direct/workflows/{id}/triggers/manual/paths/invoke?api-version=1&sp=...&sv=1.0&sig=...
//   - Title: The notification title displayed as a bold header in the card (optional)
//   - Color: The title's text color (optional)
//
// A service URL whose host is not a signed Power Automate workflow webhook URL
// is rejected when it is read. That covers retired Office 365 connector URLs
// and workflow URLs on logic.azure.com, which stopped working on 2025-11-30.
// A query key that is not a Teams setting is rejected too, such as a parameter
// of a webhook URL that was not percent-encoded. A URL without a host is read,
// and a send fails unless a host param sets one.
//
// # Adaptive Card Format
//
// Notifications are sent as Adaptive Cards wrapped in the Power Automate
// webhook message envelope (type: "message" with attachments array).
// The message body is rendered as TextBlock elements within the card.
//
// # Usage Example
//
//	service := &teams.Service{}
//	err := service.Initialize(serviceURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// For more information about Power Automate workflow webhooks, visit:
// https://learn.microsoft.com/en-us/connectors/teams/#microsoft-teams-webhook
package teams
