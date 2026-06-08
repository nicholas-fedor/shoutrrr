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
//   - Host: The Zulip server hostname (with optional port) (required)
//   - Type: Message type, "channel" or "direct" (optional, defaults to channel)
//   - Stream: The target stream (channel) name (optional)
//   - Topic: The target topic within the stream (optional)
//   - Title: Notification title prepended to message content (optional)
//   - To: Comma-separated recipients for direct messages (optional)
//   - ReadBySender: Whether to mark sent DM read by the bot (optional, default false)
//
// # Direct Messages
//
// Set Type to MessageTypeDirect (or "direct" in params/URL) and provide recipients via the To
// field or "to" param (comma separated emails/IDs). The "stream" field can be used as fallback
// recipient for direct if To is not set.
//
// # Server-Side Limits
//
// On first send, the service fetches max_message_length and max_topic_length from the
// Zulip /api/v1/register endpoint (realm section). Falls back to built-in defaults (10000 bytes
// content, 60 chars topic) if the call fails or returns zero/ non-success.
//
// # Usage Example
//
//	service := &zulip.Service{}
//	err := service.Initialize(serviceURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// # Payload Creation
//
// The package provides CreatePayload for building properly formatted form-encoded requests
// for the Zulip messages API, supporting channel/direct types, JSON-encoded recipient lists
// for DMs, topics, and read_by_sender.
//
// # URL Format
//
// The configuration URL format is:
//
//	zulip://botmail:botkey@host?stream=general&topic=alerts
//
// Or for direct:
//
//	zulip://botmail:botkey@host?type=direct&to=user1@example.com,user2@example.com
//
// # Validation
//
// The package includes validation for:
//   - Bot email address presence
//   - API key presence
//   - Host configuration (including optional :port)
//   - Message type (channel/direct/empty)
//   - Recipient presence per message type
//   - Topic length (server or default 60 chars)
//   - Message size (server or default 10000 bytes)
//
// For more information about Zulip bots, visit: https://zulip.com/help/bots-and-integrations
package zulip
