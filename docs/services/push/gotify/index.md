# Gotify

Shoutrrr integrates with Gotify to enable sending notifications from command-line tools, scripts, or applications.

Gotify is a self-hosted notification service that allows you to send push notifications to your devices. It provides a simple REST API for sending messages and supports various features like priorities, custom titles, and additional metadata through extras.

For more information about Gotify, visit the [official Gotify documentation](https://gotify.net/docs/).

## URL Format

--8<-- "docs/services/push/gotify/config.md"

## API Endpoint

Shoutrrr sends notifications to Gotify using the REST API endpoint `POST /message`.
The request includes the notification data as JSON in the request body and uses either URL query parameters or HTTP headers for authentication.

## Configuration Parameters

### Token Requirements

!!! Note
    Tokens are generated within the Gotify web interface when creating applications.

The Gotify application token must be exactly 15 characters long and must start with the letter 'A'.
It can contain uppercase and lowercase letters, numbers, and the special characters '.', '-', and '_'.

!!! Warning "Token Validation"
    Invalid tokens will result in notification delivery failures. Always verify your token format matches Gotify's requirements.

### Extras Field Support

The `extras` parameter allows you to include additional metadata in the notification payload sent to Gotify. This is passed as a JSON object and can be used for features like actions, images, or custom data that Gotify supports.

!!! Example "With extras for actions"

    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa/?extras=%7B%22action%22%3A%22view%22%2C%22url%22%3A%22https%3A%2F%2Fexample.com%22%7D
    ```

    This sends a notification with an action button that opens `https://example.com` when clicked.

!!! Example "With extras for images"

    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa/?extras=%7B%22image%22%3A%22https%3A%2F%2Fexample.com%2Fimage.png%22%7D
    ```

    This includes an image URL in the notification.

### Disable TLS

The `disabletls` parameter can be set to `yes` to disable TLS certificate verification. This is useful for:

- Self-signed certificates in development environments
- Internal Gotify servers with custom CA certificates

!!! danger "Security Risk"
    Disabling TLS verification makes connections vulnerable to man-in-the-middle attacks. Only use this option in trusted networks.

### Priority Levels

Gotify supports priority levels to control how notifications are displayed and handled:

- **-2**: Very low priority (may be hidden in some clients)
- **-1**: Low priority
- **0**: Normal priority (default)
- **1**: High priority
- **2 to 10**: Very high priority (may trigger special handling like persistent notifications)

Higher priority notifications typically appear more prominently and may bypass quiet hours or notification filters. Values outside the -2 to 10 range may be accepted but are typically clamped by the Gotify server.

### Custom Title

The `title` parameter allows you to set a custom title for your notification. If not specified, Shoutrrr uses the default title "Shoutrrr notification". Titles should be concise and descriptive to help users quickly identify the notification's purpose.

### Use Header Authentication

By default, the Gotify token is sent as a query parameter in the URL (`?token=...`). The `useheader` parameter allows you to send the token in the `X-Gotify-Key` HTTP header instead, which provides better security since:

- Headers are less likely to be logged in server access logs
- Query parameters may appear in browser history or be exposed in URLs
- Headers are encrypted in HTTPS requests

!!! Example "Using header authentication"

    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa/?useheader=yes
    ```

    This sends the token `AzyoeNS.D4iJLVa` in the `X-Gotify-Key` header instead of as `?token=AzyoeNS.D4iJLVa` in the URL.

## Examples

!!! Example "Common usage"

    ```uri
    gotify://gotify.example.com:443/AzyoeNS.D4iJLVa/?title=Great+News&priority=1
    ```

!!! Example "With subpath"
    ```uri
    gotify://example.com:443/path/to/gotify/AzyoeNS.D4iJLVa/?title=Great+News&priority=1
    ```

!!! Example "With all parameters"
    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa/?title=System+Alert&priority=5&disabletls=yes&useheader=yes&extras=%7B%22action%22%3A%22view%22%2C%22url%22%3A%22https%3A%2F%2Fexample.com%2Falert%22%2C%22image%22%3A%22https%3A%2F%2Fexample.com%2Falert.png%22%7D
    ```

!!! Example "Minimal configuration"
    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa/
    ```

!!! Example "With custom title and low priority"
    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa/?title=Info&priority=-1
    ```

!!! Example "With TLS disabled for self-signed certificates"
    ```uri
    gotify://gotify.example.com:8080/AzyoeNS.D4iJLVa/?disabletls=yes
    ```

## Security Considerations

### Authentication Security

- **Use header authentication** (`useheader=yes`) whenever possible to avoid exposing tokens in URLs
- Store URLs containing tokens securely and avoid sharing them in logs or version control
- Rotate tokens regularly through the Gotify web interface

### Network Security

- Always use HTTPS (default) for production deployments
- Only disable TLS verification (`disabletls=yes`) in trusted development environments
- Consider using Gotify behind a reverse proxy with additional authentication layers

### Token Management

- Tokens are application-specific in Gotify
- Each application can have its own token with specific permissions
- Delete unused applications and tokens immediately

## Troubleshooting

### Common Issues

#### Connection Failed

**Error**: `failed to send notification to Gotify`

**Possible causes:**

- Invalid or unreachable server URL
- Network connectivity issues
- Firewall blocking outbound connections

**Solutions:**

- Verify the Gotify server is running and accessible
- Check network connectivity and firewall rules
- Ensure the correct host and port are specified

#### Authentication Failed

**Error**: `invalid gotify token`

**Possible causes:**

- Incorrect token format (must be 15 chars starting with 'A')
- Token has been revoked or expired
- Wrong token for the application

**Solutions:**

- Verify token format: exactly 15 characters, starts with 'A'
- Regenerate token in Gotify web interface
- Ensure you're using the correct application token

#### TLS Certificate Issues

**Error**: TLS certificate verification failed

**Solutions:**

- For self-signed certificates, use `disabletls=yes`
- Install proper certificates on the Gotify server
- Verify the certificate is valid and not expired

### Testing Your Configuration

You can test your Gotify configuration using the Shoutrrr CLI:

```bash
shoutrrr verify --url "gotify://your-server/token"
```

This validates the URL format and service configuration. To send a test notification, use:

```bash
shoutrrr send --url "gotify://your-server/token" --message "Test notification"
```

### Gotify Server Logs

Check the Gotify server logs for additional error details. Common log locations:

- Docker: `docker logs gotify_container`
- Systemd: `journalctl -u gotify`
- Binary: Check the log file specified in your Gotify configuration

### Getting Help

- Check the [Gotify documentation](https://gotify.net/docs/) for server setup and configuration
- Report issues on the [Shoutrrr GitHub repository](https://github.com/nicholas-fedor/shoutrrr)
