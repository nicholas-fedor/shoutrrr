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

Gotify 3.0 and later issue application tokens that start with `gtfya.` followed by at least 43 characters.
Tokens from earlier Gotify versions are exactly 15 characters long and start with the letter 'A'.
They can contain uppercase and lowercase letters, numbers, and the special characters '.', '-', and '_'.

!!! Warning "Token Validation"
    Shoutrrr rejects a URL whose token is missing or in neither format, with the error `invalid gotify token`. Always verify your token format matches Gotify's requirements.

### Extras Field Support

The `extras` parameter allows you to include additional metadata in the notification payload sent to Gotify. This is passed as a JSON object and can be used for features like actions, images, or custom data that Gotify supports.

!!! Example "With extras for actions"

    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa?extras=%7B%22action%22%3A%22view%22%2C%22url%22%3A%22https%3A%2F%2Fexample.com%22%7D
    ```

    This sends a notification with an action button that opens `https://example.com` when clicked.

!!! Example "With extras for images"

    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa?extras=%7B%22image%22%3A%22https%3A%2F%2Fexample.com%2Fimage.png%22%7D
    ```

    This includes an image URL in the notification.

### Disable TLS

The `disabletls` parameter can be set to `yes` to disable TLS entirely, forcing the use of HTTP instead of HTTPS. This is useful for:

- Self-signed certificates in development environments
- Internal Gotify servers with custom CA certificates
- Testing with local Gotify instances

Disabling TLS does not skip certificate verification. If the server redirects to HTTPS, its certificate is still verified unless `skiptlsverify` is also set. Shoutrrr logs a warning the first time it sends without TLS.

!!! danger "Security Risk"
    Disabling TLS makes all communication unencrypted and vulnerable to interception. Only use this option in trusted, isolated networks.

### Skip TLS Certificate Verification

The `skiptlsverify` parameter can be set to `yes`, in the URL or in the send params, to skip TLS certificate verification while still using HTTPS. This maintains encrypted communication but bypasses certificate validation. This is useful for:

- Self-signed certificates in development environments
- Internal Gotify servers with custom CA certificates
- Testing with certificates that don't match the hostname

Shoutrrr logs a warning the first time it sends without verifying the certificate. It logs no warning when the program using Shoutrrr supplies its own HTTP client, because that client's TLS settings apply.

!!! danger "Security Risk"
    Skipping certificate verification makes connections vulnerable to man-in-the-middle attacks. Only use this option when you trust the network path to the server.

!!! Warning "Deprecated"
    The `insecureskipverify` parameter is deprecated in favor of `skiptlsverify`.
    It still works and logs a warning the first time it is used, and `skiptlsverify` takes precedence when both are set. In Go, the `Config.InsecureSkipVerify` field is likewise deprecated in favor of `Config.SkipTLSVerify`.

### Priority Levels

Gotify supports priority levels to control how notifications are displayed and handled:

- **-2**: Very low priority (may be hidden in some clients)
- **-1**: Low priority
- **0**: Normal priority
- **1**: High priority
- **2 to 10**: Very high priority (may trigger special handling like persistent notifications)

Higher priority notifications typically appear more prominently and may bypass quiet hours or notification filters. The priority can be set to a value between -2 and 10, where -2 is the lowest priority and 10 is the highest. Negative values have special meanings in some clients.

When neither the URL nor the send params set a priority, Shoutrrr leaves it out of the message, and Gotify applies the application's default priority.
A priority set explicitly is always sent, including 0.

#### Message Level

A sender can set a message level with the `level` param, as `Params.SetLevel` does.
When neither the URL nor the send params set a priority, the level picks one from the bands of the Gotify Android app:

| Level   | Priority                | Android notification       |
|---------|-------------------------|----------------------------|
| Debug   | 2                       | Icon only                  |
| Info    | The application default |                            |
| Warning | 5                       | Icon and sound             |
| Error   | 8                       | Icon, sound, and vibration |

A priority set in the URL or the send params, including 0, takes precedence over the level.

### Custom Title

The `title` parameter allows you to set a custom title for your notification. If not specified, Shoutrrr uses the default title "Shoutrrr notification". Titles should be concise and descriptive to help users quickly identify the notification's purpose.

### Date

!!! Warning "Deprecated"
    The `date` parameter is deprecated and has no effect.
    Gotify stamps each message with the server time when it receives it, so Shoutrrr accepts `date` in URLs and params for compatibility but does not send it, and logs a warning the first time it is set.

### Use Header Authentication

By default, the Gotify token is sent as a query parameter in the URL (`?token=...`). The `useheader` parameter set to `yes` removes the token from the URL entirely and sends it in the `X-Gotify-Key` HTTP header instead, which provides better security since:

- Headers are less likely to be logged in server access logs
- Query parameters may appear in browser history or be exposed in URLs
- Headers are encrypted in HTTPS requests

!!! Example "Using header authentication"

    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa?useheader=yes
    ```

    This sends the token in the `X-Gotify-Key` header, with no token appearing in the URL.

## Input Validation

Shoutrrr validates input parameters before sending notifications to Gotify to ensure data integrity and prevent common errors.

### Message Requirements

Messages must contain at least one character. Empty messages are not allowed.

!!! Failure "Validation Failure Example"
    ```bash
    shoutrrr send --url "gotify://example.com/AzyoeNS.D4iJLVa" --message ""
    ```
    **Error**: `message cannot be empty`

### Priority Ranges

Priority values must be integers between -2 and 10 inclusive. Values outside this range will be rejected.

!!! Failure "Validation Failure Example"
    ```bash
    shoutrrr send --url "gotify://example.com/AzyoeNS.D4iJLVa?priority=15" --message "test"
    ```
    **Error**: `priority must be between -2 and 10`

## Examples

!!! Example "Common usage"

    ```uri
    gotify://gotify.example.com:443/AzyoeNS.D4iJLVa?title=Great+News&priority=1
    ```

!!! Example "With subpath"
    ```uri
    gotify://example.com:443/path/to/gotify/AzyoeNS.D4iJLVa?title=Great+News&priority=1
    ```

!!! Example "With all parameters"
    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa?title=System+Alert&priority=5&disabletls=yes&useheader=yes&extras=%7B%22action%22%3A%22view%22%2C%22url%22%3A%22https%3A%2F%2Fexample.com%2Falert%22%2C%22image%22%3A%22https%3A%2F%2Fexample.com%2Falert.png%22%7D
    ```

!!! Example "Minimal configuration"
    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa
    ```

!!! Example "With custom title and low priority"
    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa?title=Info&priority=-1
    ```

!!! Example "With TLS disabled for self-signed certificates"
    ```uri
    gotify://gotify.example.com:8080/AzyoeNS.D4iJLVa?disabletls=yes
    ```

!!! Example "With TLS certificate verification skipped"
    ```uri
    gotify://gotify.example.com/AzyoeNS.D4iJLVa?skiptlsverify=yes
    ```

## Security Considerations

### Authentication Security

- **Use header authentication** (`useheader=yes`) whenever possible to avoid exposing tokens in URLs
- Store URLs containing tokens securely and avoid sharing them in logs or version control
- Rotate tokens regularly through the Gotify web interface

### Network Security

- Always use HTTPS (default) for production deployments
- Only disable TLS (`disabletls=yes`) or skip certificate verification (`skiptlsverify=yes`) in trusted development environments
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

- Incorrect token format (a `gtfya.` token from Gotify 3.0 or later, or 15 chars starting with 'A' from earlier versions)
- Token has been revoked or expired
- Wrong token for the application

**Solutions:**

- Verify the token format matches your Gotify version
- Regenerate token in Gotify web interface
- Ensure you're using the correct application token

#### TLS Certificate Issues

**Error**: TLS certificate verification failed

**Solutions:**

- For self-signed certificates, use `skiptlsverify=yes` to skip verification while maintaining HTTPS
- For development/testing with HTTP, use `disabletls=yes` to disable TLS entirely
- Install proper certificates on the Gotify server
- Verify the certificate is valid and not expired

#### Validation Errors

##### Empty Messages

**Error**: `message cannot be empty`

**Possible causes:**

- No message provided in the request
- Message parameter is an empty string

**Solutions:**

- Always include a non-empty message in your notification
- Check that your message parameter is properly set

##### Invalid Priorities

**Error**: `priority must be between -2 and 10`

**Possible causes:**

- Priority value is less than -2 or greater than 10
- Priority parameter contains non-numeric characters

**Solutions:**

- Use integer values between -2 and 10
- Verify the priority parameter is a valid number

##### Malformed Extras JSON

**Error**: `failed to unmarshal extras JSON` or `failed to parse extras JSON from URL query`

**Possible causes:**

- Invalid JSON syntax in the extras parameter
- Improper URL encoding of the JSON string
- Missing quotes or brackets in JSON structure

**Solutions:**

- Validate JSON syntax using a JSON validator
- Ensure proper URL encoding when passing JSON in URLs
- Test extras JSON separately before including in URLs

#### Error Handling Patterns

Shoutrrr employs several error handling strategies for malformed inputs:

- **Early Validation**: Parameters are validated before API requests to fail fast
- **Graceful Degradation**: Non-critical validation failures (like invalid extras in send params) allow notifications to proceed with defaults
- **Detailed Error Messages**: Specific error messages help identify the exact validation issue
- **Fallback Behavior**: When possible, invalid inputs fall back to sensible defaults rather than failing completely

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
