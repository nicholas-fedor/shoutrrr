// Package slack provides a notification service for sending messages to Slack channels or users via webhooks and Bot API.
//
// This package is part of the shoutrrr notification library and implements the Service interface
// for delivering notifications to Slack, supporting both incoming webhooks and the Slack Bot API.
//
// # Service Configuration
//
// The Service struct provides the main functionality for sending notifications to Slack.
// It embeds standard.Standard for common service features and supports dual-mode operation
// via webhooks or the Bot API depending on the token type provided.
//
// # Configuration Fields
//
// The Config struct supports the following settings:
//
//   - Token: API Bot token or webhook token (required, auto-detected type)
//   - BotName: Override the bot's display name (optional)
//   - Icon: Use an emoji or URL as the message icon (optional)
//   - Channel: Target channel ID in Cxxxxxxxxxx format for API mode (optional)
//   - Color: Message left-hand border color (optional)
//   - Title: Prepended text above the message (optional)
//   - ThreadTS: Thread timestamp for replying in threads (optional)
//
// # Token Types
//
// The Token type automatically detects and handles two formats:
//
//   - Webhook tokens: URLs beginning with https://hooks.slack.com/
//   - API tokens: Bot tokens beginning with xoxb- for Slack API access
//
// # Usage Example
//
//	service := &slack.Service{}
//	err := service.Initialize(configURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// # JSON Payload Creation
//
// The package provides CreateJSONPayload for building properly formatted Slack message
// payloads with support for text, attachments, colors, and thread replies.
//
// # URL Format
//
// The configuration URL format for API tokens is:
//
//	slack://xoxb-token@channel?botname=value&icon=value&color=value
//
// For webhooks:
//
//	slack://botname@https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX
//
// For more information about Slack apps, visit: https://api.slack.com/apps
package slack
