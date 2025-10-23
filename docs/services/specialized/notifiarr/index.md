# Notifiarr

The Notifiarr service enables sending notifications through the Notifiarr API, which can forward notifications to various platforms including Discord.

Notifiarr acts as a notification relay service, allowing you to send notifications to multiple destinations through a single API endpoint.
It supports routing notifications to Discord channels and other configured services.

## Configuration Options

| Parameter   | Description                                   | Required | Default  |
|-------------|-----------------------------------------------|----------|----------|
| `apikey`    | Your Notifiarr API key for authentication     | Yes      | -        |
| `name`      | Name of the app/script for notifications      | No       | Shoutrrr |
| `channel`   | Discord channel ID for Discord notifications  | No       | -        |
| `thumbnail` | Thumbnail image URL for Discord notifications | No       | -        |
| `image`     | Image URL for Discord notifications           | No       | -        |
| `color`     | Color for Discord embed (6-digit hex)         | No       | -        |

## URL Format

!!! info ""
    notifiarr://**`apikey`**?name=**`name`**&channel=**`channel`**&thumbnail=**`thumbnail`**&image=**`image`**&color=**`color`**

### Complete URL Format

The full URL format supports all configuration parameters:

```uri
notifiarr://[apikey]?name=[app_name]&channel=[channel_id]&thumbnail=[thumbnail_url]&image=[image_url]&color=[color_value]
```

**Parameters:**

- `apikey` (required): Your Notifiarr API key
- `name` (optional): Name of the app/script for notifications (defaults to "Shoutrrr")
- `channel` (optional): Discord channel ID for routing notifications (numeric string)
- `thumbnail` (optional): Thumbnail image URL for Discord notifications
- `image` (optional): Image URL for Discord notifications
- `color` (optional): Color for Discord embed as 6-digit hex code (without # prefix) or color name

--8<-- "docs/services/specialized/notifiarr/config.md"

## Usage Examples

### Basic Notification

Send a simple notification using your Notifiarr API key:

!!! example
    ```bash title="Send Basic Notification"
    shoutrrr send --url "notifiarr://your-api-key-here" --message "Hello from Shoutrrr!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Notification with Discord Channel

Send a notification that will be forwarded to a specific Discord channel:

!!! example
    ```bash title="Send to Discord Channel"
    shoutrrr send --url "notifiarr://your-api-key-here?channel=123456789012345678" --message "Alert: System status changed"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Notification with Thumbnail and Color

Send a notification with custom thumbnail and color:

!!! example
    ```bash title="Send Notification with Thumbnail and Color"
    shoutrrr send --url "notifiarr://your-api-key-here?channel=123456789012345678&thumbnail=https://example.com/alert.png&color=FF0000" --title "Critical Alert" --message "Server is down!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Notification with Image

Send a notification with a custom image:

!!! example
    ```bash title="Send Notification with Image"
    shoutrrr send --url "notifiarr://your-api-key-here?channel=123456789012345678&image=https://example.com/chart.png" --title "Sales Report" --message "Monthly sales chart attached"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Notification with Thumbnail and Image

Send a notification with both thumbnail and image:

!!! example
    ```bash title="Send Notification with Thumbnail and Image"
    shoutrrr send --url "notifiarr://your-api-key-here?channel=123456789012345678&thumbnail=https://example.com/icon.png&image=https://example.com/full-image.png" --title "Product Update" --message "New product launch details"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Notification with User Mentions

Send a notification that mentions specific Discord users (parsed from message content):

!!! example
    ```bash title="Send Notification with User Mentions"
    shoutrrr send --url "notifiarr://your-api-key-here?channel=123456789012345678" --message "Hey <@123456789> and <@987654321>, the deployment is complete!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Notification with Custom App Name

Send a notification with a custom app name:

!!! example
    ```bash title="Send Notification with Custom App Name"
    shoutrrr send --url "notifiarr://your-api-key-here?name=MyApp" --message "Hello from MyApp!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Notification with Title

Send a notification with a custom title (appears in Discord embed):

!!! example
    ```bash title="Send Notification with Title"
    shoutrrr send --url "notifiarr://your-api-key-here?channel=123456789012345678" --title "System Alert" --message "Disk space running low on server"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

## API Documentation Reference

The Notifiarr service sends notifications to the Notifiarr API endpoint at `https://notifiarr.com/api/v1/notification/passthrough/{apikey}`.

### Request Format

The service sends a POST request with a JSON payload containing the notification data. The payload structure follows the official Notifiarr API specification:

```json
{
  "notification": {
    "name": "MyApp",
    "event": "Optional event ID",
    "update": null
  },
  "discord": {
    "ids": {
      "channel": 123456789012345678
    },
    "color": "Optional color (6-digit hex)",
    "images": {
      "thumbnail": "Optional thumbnail URL",
      "image": "Optional image URL"
    },
    "text": {
      "title": "Optional notification title",
      "icon": "Optional icon URL",
      "content": "Optional content text",
      "description": "Your notification message",
      "footer": "Optional footer text",
      "fields": [
        {
          "title": "Field title",
          "text": "Field content",
          "inline": false
        }
      ]
    },
    "ping": {
      "pingUser": [123456789, 987654321],
      "pingRole": [111111111, 222222222]
    }
  }
}
```

#### Payload Structure Details

- **`notification`** (required): Core notification metadata
  - `name`: Application identifier (configurable via `name` parameter, defaults to "Shoutrrr")
  - `event`: Optional unique event ID from the `id` parameter
  - `update`: Boolean flag to update existing messages with the same ID (optional, defaults to null)

- **`discord`** (optional): Discord-specific configuration (only included when channel is configured or other Discord fields are present)
  - `ids.channel`: Discord channel ID (required when discord object is present)
  - `color`: Optional embed color as 6-digit hex code (without # prefix)
  - `images.thumbnail`: Optional thumbnail image URL
  - `images.image`: Optional image URL
  - `text.title`: Optional notification title from the `title` parameter
  - `text.icon`: Optional icon URL for the embed
  - `text.content`: Optional content text above the main message
  - `text.description`: The notification message content
  - `text.footer`: Optional footer text for the embed
  - `text.fields`: Optional array of field objects with title, text, and inline properties
  - `ping.pingUser`: Array of Discord user IDs to mention (parsed from `<@123>` mentions in message)
  - `ping.pingRole`: Array of Discord role IDs to mention (parsed from `<@&123>` mentions in message)

### Response

Successful requests return a 2xx status code. The service logs the server response for debugging purposes.

### Error Handling

The service handles various error conditions:

- **Empty Message**: Returns an error if the message is empty
- **Invalid API Key**: Server returns authentication errors
- **Network Issues**: Connection timeouts or network failures
- **Server Errors**: Unexpected HTTP status codes from the Notifiarr API

## Troubleshooting

### Common Issues

#### Notifications not appearing in Discord

- **Check Channel ID**: Ensure the `channel` parameter contains a valid Discord channel ID (must be numeric)
- **Verify Permissions**: Confirm that Notifiarr has permission to post in the specified channel
- **API Key Validity**: Verify your Notifiarr API key is correct and active
- **Channel Configuration**: Ensure the channel parameter is included in the URL when Discord notifications are desired

#### Invalid color values

- **Color Format**: Colors must be provided as 6-digit hex codes (without # prefix) or valid color names
- **Example**: Use `color=FF0000` for red, not `color=#FF0000` or `color=red`

#### Thumbnail images not displaying

- **Valid URL**: Ensure the thumbnail URL is a valid, publicly accessible HTTPS URL
- **Image Format**: The URL should point directly to an image file (supported formats depend on Discord)

#### Image not displaying

- **Valid URL**: Ensure the image URL is a valid, publicly accessible HTTPS URL
- **Image Format**: The URL should point directly to an image file (supported formats depend on Discord)
- **Image Size**: Large images may be resized or may not display properly in Discord

### Debugging Tips

- Enable verbose logging with `--debug` flag to see detailed request/response information
- Check Notifiarr server logs for additional error details
- Test with minimal configuration first, then add parameters incrementally

For more information about [Notifiarr](https://notifiarr.com/) and its capabilities, visit the [Notifiarr Wiki](https://notifiarr.wiki/).
