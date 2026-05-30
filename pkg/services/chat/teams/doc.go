// Package teams provides a notification service for sending messages to Microsoft Teams
// via Power Automate workflow incoming webhooks using the Adaptive Card format.
//
// # Service Configuration
//
// The Config struct supports the following settings:
//
//   - Host: The full Power Automate workflow webhook URL (required)
//     Example: https://prod-00.westus.logic.azure.com:443/workflows/{id}/triggers/manual/paths/invoke?api-version=...
//   - Title: The notification title displayed as a bold header in the card (optional)
//   - Color: Reserved for future use (optional)
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
