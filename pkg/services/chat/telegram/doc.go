// Package telegram provides a notification service for sending messages to Telegram chats.
//
// This package is part of the shoutrrr notification library and implements the Service interface
// for delivering notifications to Telegram chats using the Telegram Bot API.
//
// # Service Configuration
//
// The Service struct provides the main functionality for sending notifications to Telegram.
// It embeds standard.Standard for common service features and supports sending messages
// to multiple chat IDs with customizable formatting options.
//
// # Configuration Fields
//
// The Config struct supports the following settings:
//
//   - Token: The Telegram bot token in the format "botID:token" (required)
//   - Chats: List of chat IDs or channel names to send messages to (required)
//   - Preview: Enable/disable web page preview for URLs (optional, default: true)
//   - Notification: Enable/disable notification sound (optional, default: true)
//   - ParseMode: How to parse the message text (None, Markdown, HTML, MarkdownV2) (optional, default: None)
//   - Title: Notification title prepended to messages (optional)
//
// # Parse Modes
//
// The package supports multiple text parsing modes:
//
//   - None: Plain text without formatting
//   - Markdown: Classic Markdown formatting
//   - HTML: HTML tag-based formatting
//   - MarkdownV2: Extended Markdown formatting
//
// # Usage Example
//
//	service := &telegram.Service{}
//	err := service.Initialize(configURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// # JSON Payload Creation
//
// The package provides createSendMessagePayload for building properly formatted Telegram
// Bot API payloads with support for message text, parse modes, notifications, and previews.
//
// # URL Format
//
// The configuration URL format is:
//
//	telegram://botID:token@telegram/?chats=@channel1,@channel2&parsemode=HTML
//
// # Interactive Setup
//
// The package includes a generator for interactive configuration setup, allowing users
// to configure the service through a guided process.
//
// For more information about Telegram bots, visit: https://core.telegram.org/bots
package telegram
