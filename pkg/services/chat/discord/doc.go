// Package discord provides a service for sending notifications to Discord channels via webhooks.
//
// The Discord service supports comprehensive webhook integration with features including:
//
//   - Plain text messages using Discord's content field for clean formatting
//   - Rich embeds with customizable colors, authors, images, thumbnails, and fields
//   - Message levels with color-coded embeds (error, warning, info, debug)
//   - Thread support for sending messages to existing threads
//   - File attachments with multipart/form-data uploads
//   - Custom username and avatar overrides
//   - JSON mode for raw payload control
//   - Message chunking for long content
//   - Line splitting for multi-line messages
//   - Multiple embeds per message
//   - Timestamp support in embeds
//   - Rate limiting with exponential backoff and Retry-After header handling
//
// # URL Format
//
// The service URL follows the format:
//
//	discord://token@webhookid[?param=value&...]
//
// Where:
//   - token: The webhook token from Discord
//   - webhookid: The webhook ID from Discord
//   - Optional parameters: thread_id, username, avatar, etc.
//
// # Basic Usage
//
//	import "github.com/nicholas-fedor/shoutrrr/pkg/services/chat/discord"
//
//	service := &discord.Service{}
//	err := service.Initialize(url, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Hello, Discord!", nil)
//
// # Configuration Options
//
// The service supports various configuration options via URL parameters or Send parameters:
//
//   - avatar: Override webhook avatar URL
//   - username: Override webhook username
//   - thread_id: Send to specific thread
//   - color: Border color for plain messages (hex)
//   - color_error: Border color for error messages (hex)
//   - color_warn: Border color for warning messages (hex)
//   - color_info: Border color for info messages (hex)
//   - color_debug: Border color for debug messages (hex)
//   - json: Send raw JSON payload (true/false)
//   - split_lines: Send each line as separate embed (true/false)
//   - title: Message title
//
// # Embed Features
//
// Embeds can include:
//   - Author information (name, URL, icon)
//   - Images and thumbnails
//   - Custom fields as key-value pairs
//   - Color-coded borders based on message level
//   - Timestamps for event timing
//   - Multiple embeds per message
//
// Embed fields can be set using MessageItem.Fields with special keys:
//   - embed_author_name: Author name
//   - embed_author_url: Author URL
//   - embed_author_icon_url: Author icon URL
//   - embed_image_url: Main image URL
//   - embed_thumbnail_url: Thumbnail image URL
//   - Any other key becomes a custom embed field
//
// # File Attachments
//
// Files can be attached using MessageItem.File or via CLI --file option.
// Multiple files are supported per message.
//
// # Important Notes
//
//   - Webhook permissions are required for file uploads and thread posting
//   - Files must be accessible from the running system
//   - Thread IDs can be obtained via Discord's Developer Mode
//   - Message content is limited by Discord's API constraints (2000 chars per message)
//   - Rate limiting applies as per Discord's webhook limits with automatic retry handling
//   - The service implements exponential backoff and respects Retry-After headers
package discord
