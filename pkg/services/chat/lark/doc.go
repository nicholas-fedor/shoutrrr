// Package lark provides a service for sending notifications to Lark (Feishu) via webhooks.
//
// The Lark service supports webhook integration for sending text and rich post messages
// to Lark or Feishu platforms using their Bot API.
//
// # URL Format
//
// The service URL follows the format:
//
//	lark://host/token?secret=SECRET&title=TITLE
//
// Where:
//   - host: The API host, either "open.larksuite.com" (Lark) or "open.feishu.cn" (Feishu)
//   - token: The webhook token from the bot configuration
//   - secret: Optional signing secret for request verification
//   - title: Optional message title for rich post messages
//
// # Basic Usage
//
//	import "github.com/nicholas-fedor/shoutrrr/pkg/services/chat/lark"
//
//	service := &lark.Service{}
//	err := service.Initialize(url, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Hello, Lark!", nil)
//
// # Configuration Options
//
// The service supports configuration via URL components:
//
//   - Host: The API host (default: "open.larksuite.com", or "open.feishu.cn")
//   - Path: The webhook token (required)
//   - Secret: Optional signing secret for request verification (query parameter)
//   - Title: Optional message title for rich post messages (query parameter)
//   - Link: Optional URL link to include in post messages (query parameter)
//
// # Message Types
//
// The service supports two message types:
//
//   - Text: Simple text messages sent when no title is provided
//   - Post: Rich formatted messages with title and optional link when title is provided
//
// # Obtaining Webhook Credentials
//
// To obtain the webhook URL and credentials:
//
//  1. Open your Lark/Feishu group chat
//  2. Click "Settings" → "Group Settings" → "Bots"
//  3. Add a custom bot
//  4. Copy the webhook URL and optionally enable signature verification
//  5. Extract the token from the webhook URL path
//
// # Important Notes
//
//   - The service requires a valid host and token for authentication
//   - Messages have a maximum length of 4096 bytes
//   - Signature verification requires the secret to be configured
//   - Supports both Lark (open.larksuite.com) and Feishu (open.feishu.cn) platforms
//   - Rich post messages require a title to be specified
//   - Link parameter is only used when title is provided for post messages
package lark
