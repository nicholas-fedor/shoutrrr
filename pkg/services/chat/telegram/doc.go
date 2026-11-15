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
//   - Chats: List of chat IDs or channel names to send messages to (required).
//     A chat written as CHAT:THREAD posts in that forum topic, and a thread ID
//     that is not a positive integer fails with [ErrInvalidThreadID].
//   - Preview: Enable/disable web page preview for URLs (optional, default: true)
//   - Notification: Enable/disable notification sound (optional, default: true)
//   - ParseMode: How to parse the message text (None, Markdown, HTML, MarkdownV2) (optional, default: None)
//   - Title: Notification title sent in bold on a line before the message, in
//     every parse mode (optional)
//   - APIURL: Base URL of the Bot API server, such as a self-hosted one (optional, default: https://api.telegram.org)
//   - SkipTLSVerify: Skip verification of the Bot API server's TLS certificate (optional, default: false)
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
//	err := service.Initialize(serviceURL, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Your message here", nil)
//
// # Request Fields
//
// Sends use the sendMessage fields of Bot API 7.0 and later. A send with
// previews off carries link_preview_options with is_disabled, and a send
// carries no reply field.
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
