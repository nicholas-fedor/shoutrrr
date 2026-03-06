// Package mattermost provides a notification service for sending messages to Mattermost channels or users via webhooks.
//
// This package is part of the shoutrrr notification library and implements the Service interface
// for delivering notifications to Mattermost, an open-source team collaboration platform.
//
// # Service Configuration
//
// The Service struct provides the main functionality for sending notifications to Mattermost.
// It embeds standard.Standard for common service features and includes configuration and HTTP client management.
//
// # Configuration Fields
//
// The Config struct supports the following settings:
//
//   - Host: The Mattermost server hostname (required)
//   - Token: The webhook token for authentication (required)
//   - Channel: Override the default webhook channel (optional)
//   - UserName: Override the webhook username (optional)
//   - Icon: Use an emoji or URL as the message icon (optional)
//   - Title: Notification title (optional, not used in webhook mode)
//   - DisableTLS: Use plain HTTP instead of HTTPS (optional, default: false)
//
// # Usage Example
//
//	service := &mattermost.Service{}
//	err := service.Initialize(serviceURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// # JSON Payload Creation
//
// The package provides CreateJSONPayload for building properly formatted Mattermost webhook
// payloads with support for text, channel overrides, username customization, and icon settings.
//
// # URL Format
//
// The configuration URL format is:
//
//	mattermost://[username@]host/token[/channel]?icon=value&disabletls=No
//
// For more information about Mattermost webhooks, visit: https://docs.mattermost.com/developer/webhooks-incoming.html
package mattermost
