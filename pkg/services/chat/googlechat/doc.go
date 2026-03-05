// Package googlechat provides a service for sending notifications to Google Chat spaces via webhooks.
//
// The Google Chat service supports webhook integration for sending plain text messages
// to Google Chat spaces using the Google Chat API.
//
// # URL Format
//
// The service URL follows the format:
//
//	googlechat://chat.googleapis.com/v1/spaces/SPACE_ID/messages?key=KEY&token=TOKEN
//
// Where:
//   - SPACE_ID: The Google Chat space ID (included in the Path)
//   - KEY: The API key for authentication
//   - TOKEN: The webhook token for authentication
//
// # Basic Usage
//
//	import "github.com/nicholas-fedor/shoutrrr/pkg/services/chat/googlechat"
//
//	service := &googlechat.Service{}
//	err := service.Initialize(url, logger)
//	if err != nil {
//	    // handle error
//	}
//	err = service.Send("Hello, Google Chat!", nil)
//
// # Configuration Options
//
// The service supports configuration via URL components:
//
//   - Host: The API host (default: "chat.googleapis.com")
//   - Path: The webhook path including the space ID
//   - Key: The API key for authentication (query parameter)
//   - Token: The webhook token for authentication (query parameter)
//
// # Obtaining Webhook Credentials
//
// To obtain the webhook URL and credentials:
//
//  1. Open your Google Chat space
//  2. Click the space name and select "Apps & integrations"
//  3. Click "Add webhooks"
//  4. Configure the webhook name and avatar (optional)
//  5. Copy the webhook URL
//  6. Extract the key and token from the URL query parameters
//
// # Important Notes
//
//   - The service requires both key and token for authentication
//   - Messages are sent as plain text via the Google Chat API
//   - The API host defaults to "chat.googleapis.com" if not specified
//   - Webhook URLs can be obtained from Google Chat space settings
//   - The service returns ErrUnexpectedStatus for non-2xx HTTP responses
//   - Message formatting is limited to plain text (no rich formatting)
package googlechat
