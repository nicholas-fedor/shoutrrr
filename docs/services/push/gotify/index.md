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
    gotify://gotify.example.com/message?token=AzyoeNS.D4iJLVa&extras=%7B%22action%22%3A%22view%22%2C%22url%22%3A%22https%3A%2F%2Fexample.com%22%7D
    ```

    This sends a notification with an action button that opens `https://example.com` when clicked.

!!! Example "With extras for images"

    ```uri
    gotify://gotify.example.com/message?token=AzyoeNS.D4iJLVa&extras=%7B%22image%22%3A%22https%3A%2F%2Fexample.com%2Fimage.png%22%7D
    ```

    This includes an image URL in the notification.

### Disable TLS

The `disabletls` parameter can be set to `yes` to disable TLS entirely, forcing the use of HTTP instead of HTTPS. This is useful for:

- Self-signed certificates in development environments
- Internal Gotify servers with custom CA certificates
- Testing with local Gotify instances

!!! danger "Security Risk"
    Disabling TLS makes all communication unencrypted and vulnerable to interception. Only use this option in trusted, isolated networks.

### Skip TLS Certificate Verification

The `insecureskipverify` parameter can be set to `yes` to skip TLS certificate verification while still using HTTPS. This maintains encrypted communication but bypasses certificate validation. This is useful for:

- Self-signed certificates in development environments
- Internal Gotify servers with custom CA certificates
- Testing with certificates that don't match the hostname

!!! danger "Security Risk"
    Skipping certificate verification makes connections vulnerable to man-in-the-middle attacks. Only use this option when you trust the network path to the server.

### Priority Levels

Gotify supports priority levels to control how notifications are displayed and handled:

- **-2**: Very low priority (may be hidden in some clients)
- **-1**: Low priority
- **0**: Normal priority (default)
- **1**: High priority
- **2 to 10**: Very high priority (may trigger special handling like persistent notifications)

Higher priority notifications typically appear more prominently and may bypass quiet hours or notification filters. The priority can be set to a value between -2 and 10, where -2 is the lowest priority and 10 is the highest. Negative values have special meanings in some clients.

### Custom Title

The `title` parameter allows you to set a custom title for your notification. If not specified, Shoutrrr uses the default title "Shoutrrr notification". Titles should be concise and descriptive to help users quickly identify the notification's purpose.

### Custom Date

The `date` parameter allows you to set a custom timestamp for the notification in ISO 8601 format. If not specified, Gotify will use the current server time when the notification is received. This is useful for:

- Backdating notifications for historical events
- Ensuring consistent timestamps across multiple notifications
- Testing notification ordering

!!! Example "With custom date"

    ```uri
    gotify://gotify.example.com/message?token=AzyoeNS.D4iJLVa&date=2023-12-25T10:00:00Z
    ```

    This sets the notification timestamp to Christmas morning 2023.

### Use Header Authentication

By default, the Gotify token is sent as a query parameter in the URL (`?token=...`). The `useheader` parameter set to `yes` removes the token from the URL entirely and sends it in the `X-Gotify-Key` HTTP header instead, which provides better security since:

- Headers are less likely to be logged in server access logs
- Query parameters may appear in browser history or be exposed in URLs
- Headers are encrypted in HTTPS requests

!!! Example "Using header authentication"

    ```uri
    gotify://gotify.example.com/message?useheader=yes
    ```

    This sends the token in the `X-Gotify-Key` header, with no token appearing in the URL.

## Examples

!!! Example "Common usage"

    ```uri
    gotify://gotify.example.com:443/message?token=AzyoeNS.D4iJLVa&title=Great+News&priority=1
    ```

!!! Example "With subpath"
    ```uri
    gotify://example.com:443/path/to/gotify/message?token=AzyoeNS.D4iJLVa&title=Great+News&priority=1
    ```

!!! Example "With all parameters"
    ```uri
    gotify://gotify.example.com/message?title=System+Alert&priority=5&disabletls=yes&useheader=yes&date=2023-12-25T10%3A00%3A00Z&extras=%7B%22action%22%3A%22view%22%2C%22url%22%3A%22https%3A%2F%2Fexample.com%2Falert%22%2C%22image%22%3A%22https%3A%2F%2Fexample.com%2Falert.png%22%7D
    ```

!!! Example "Minimal configuration"
    ```uri
    gotify://gotify.example.com/message?token=AzyoeNS.D4iJLVa
    ```

!!! Example "With custom title and low priority"
    ```uri
    gotify://gotify.example.com/message?token=AzyoeNS.D4iJLVa&title=Info&priority=-1
    ```

!!! Example "With custom date"
    ```uri
    gotify://gotify.example.com/message?token=AzyoeNS.D4iJLVa&date=2023-12-25T10%3A00%3A00Z&title=Scheduled+Event
    ```

!!! Example "With TLS disabled for self-signed certificates"
    ```uri
    gotify://gotify.example.com:8080/message?token=AzyoeNS.D4iJLVa&disabletls=yes
    ```

!!! Example "With TLS certificate verification skipped"
    ```uri
    gotify://gotify.example.com/message?token=AzyoeNS.D4iJLVa&insecureskipverify=yes
    ```

## Security Considerations

### Authentication Security

- **Use header authentication** (`useheader=yes`) whenever possible to avoid exposing tokens in URLs
- Store URLs containing tokens securely and avoid sharing them in logs or version control
- Rotate tokens regularly through the Gotify web interface

### Network Security

- Always use HTTPS (default) for production deployments
- Only disable TLS (`disabletls=yes`) or skip certificate verification (`insecureskipverify=yes`) in trusted development environments
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

- For self-signed certificates, use `insecureskipverify=yes` to skip verification while maintaining HTTPS
- For development/testing with HTTP, use `disabletls=yes` to disable TLS entirely
- Install proper certificates on the Gotify server
- Verify the certificate is valid and not expired

### Testing Your Configuration

You can test your Gotify configuration using the Shoutrrr CLI:

```bash
shoutrrr verify --url "gotify://your-server/message?token=token"
```

This validates the URL format and service configuration. To send a test notification, use:

```bash
shoutrrr send --url "gotify://your-server/message?token=token" --message "Test notification"
```

### Gotify Server Logs

Check the Gotify server logs for additional error details. Common log locations:

- Docker: `docker logs gotify_container`
- Systemd: `journalctl -u gotify`
- Binary: Check the log file specified in your Gotify configuration

### Getting Help

- Check the [Gotify documentation](https://gotify.net/docs/) for server setup and configuration
- Report issues on the [Shoutrrr GitHub repository](https://github.com/nicholas-fedor/shoutrrr)
