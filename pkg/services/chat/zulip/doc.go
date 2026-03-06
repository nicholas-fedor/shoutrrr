// Package zulip provides a notification service for sending messages to Zulip streams or topics.
//
// This package is part of the shoutrrr notification library and implements the Service interface
// for delivering notifications to Zulip, an open-source team chat application with topic-based
// threading.
//
// # Service Configuration
//
// The Service struct provides the main functionality for sending notifications to Zulip.
// It embeds standard.Standard for common service features and uses form-encoded API requests
// for message delivery.
//
// # Configuration Fields
//
// The Config struct supports the following settings:
//
//   - BotMail: The bot's email address for authentication (required)
//   - BotKey: The bot's API key for authentication (required)
//   - Host: The Zulip server hostname (required)
//   - Stream: The target stream name (optional)
//   - Topic: The target topic within the stream (optional, defaults to empty)
//
// # Usage Example
//
//	service := &zulip.Service{}
//	err := service.Initialize(configURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// # Payload Creation
//
// The package provides CreatePayload for building properly formatted form-encoded requests
// for the Zulip messages API, supporting stream and topic targeting.
//
// # URL Format
//
// The configuration URL format is:
//
//	zulip://botmail:botkey@host?stream=general&topic=alerts
//
// # Validation
//
// The package includes validation for:
//   - Bot email address presence
//   - API key presence
//   - Host configuration
//   - Topic length limits (60 characters maximum)
//   - Message size limits (10,000 bytes maximum)
//
// For more information about Zulip bots, visit: https://zulip.com/help/bots-and-integrations
package zulip
