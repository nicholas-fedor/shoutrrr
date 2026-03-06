// Package rocketchat provides a notification service for sending messages to Rocket.Chat channels or users.
//
// This package is part of the shoutrrr notification library and implements the Service interface
// for delivering notifications to Rocket.Chat, an open-source team collaboration platform.
//
// # Service Configuration
//
// The Service struct provides the main functionality for sending notifications to Rocket.Chat.
// It embeds standard.Standard for common service features and includes configuration and HTTP client management.
//
// # Configuration Fields
//
// The Config struct supports the following settings:
//
//   - Host: The Rocket.Chat server hostname (required)
//   - Port: The Rocket.Chat server port (optional)
//   - TokenA: The first part of the webhook token (required)
//   - TokenB: The second part of the webhook token (required)
//   - Channel: The target channel or user (optional, can include # for channels or @ for users)
//   - UserName: Override the webhook username (optional)
//
// # Usage Example
//
//	service := &rocketchat.Service{}
//	err := service.Initialize(configURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// # JSON Payload Creation
//
// The package provides CreateJSONPayload for building properly formatted Rocket.Chat webhook
// payloads with support for text, channel targeting, and username customization.
//
// # URL Format
//
// The configuration URL format is:
//
//	rocketchat://[username@]host/tokenA/tokenB[/channel]
//
// For more information about Rocket.Chat webhooks, visit: https://docs.rocket.chat/guides/administration/administration/integrations
package rocketchat
