# Send

## Overview

The `send` command delivers a notification using one or more specified service URLs.

## Usage

```bash title="Send Command Syntax"
shoutrrr send [FLAGS] [URL] [MESSAGE]
```

| Flag                    | Description                                                               |
|-------------------------|---------------------------------------------------------------------------|
| `-h, --help`            | Displays help for the `send` command.                                     |
| `-m, --message string`  | Specifies the message to send. Use `-` to read the message from stdin.    |
| `-t, --title string`    | Sets the title for services that support it (optional).                   |
| `-u, --url stringArray` | Specifies the notification service URL(s). Multiple URLs can be provided. |
| `-v, --verbose`         | Enables verbose output, logging URLs, message, and title to stderr.       |

!!! Note
    A URL and a message are required. They can be given with `--url` and `--message`, as positional arguments, or with the `SHOUTRRR_URL` and `SHOUTRRR_MESSAGE` environment variables. Positional arguments fill the URL and then the message, skipping any given as flags, so `send --url URL "Hello"` sends `Hello`. An argument with no flag left to fill is an error. Flags and arguments take precedence over environment variables. When the URL comes from `SHOUTRRR_URL` and no message is given, the message is read from stdin. Use `--message -` to read the message from stdin explicitly. Duplicate URLs are automatically removed. See the [CLI overview](../index.md#environment-variables) for all environment variables.

### URL

- Supports multiple service URLs, deduplicated before sending. URLs are parsed and services initialized accordingly.

### Message

- The message body. If set to `-`, reads from stdin and logs the byte count read.

### Title

- Optional title passed to services that support it.

### Verbose

- Enables detailed logging: lists URLs (with indentation for multiples), truncated message (up to 100 characters with ellipsis), title if provided, and "Notification sent" upon success.

## Examples

<!-- markdownlint-disable -->
### Send a Notification to a Single Service URL

!!! Example
    ```bash title="Send Command with Discord URL"
    shoutrrr send --url "discord://abc123@123456789" --message "Hello, Discord!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Send a Notification with a Title

!!! Example
    ```bash title="Send Command with Title"
    shoutrrr send --url "discord://abc123@123456789" --message "Hello, Discord!" --title "Test Notification"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Send a Notification with Verbose Output

!!! Example
    ```bash title="Send Command with Verbose Output"
    shoutrrr send --url "discord://abc123@123456789" --message "Hello, Discord!" --verbose
    ```

    ```text title="Expected Output"
    URLs: discord://abc123@123456789
    Message: Hello, Discord!
    Notification sent
    ```

### Send a Notification with Message from Stdin

!!! Example
    ```bash title="Send Command with Stdin Input"
    echo "Hello from stdin!" | shoutrrr send --url "discord://abc123@123456789" --message -
    ```

    ```text title="Expected Output"
    Reading from STDIN...
    Read 18 byte(s)
    Notification sent
    ```

### Send to Multiple URLs with Deduplication

!!! Example
    ```bash title="Send Command with Multiple URLs"
    shoutrrr send --url "discord://abc123@123456789" --url "discord://abc123@123456789" --message "Hello!"
    ```

    ```text title="Expected Output"
    Notification sent
    ```

### Send with Verbose and Multiple URLs

!!! Example
    ```bash title="Send Command with Verbose and Multiple URLs"
    shoutrrr send --url "discord://abc123@123456789" --url "slack://hook:T00000000-B00000000-XXXXXXXXXXXXXXXXXXXXXXXX@webhook" --message "Hello!" --verbose
    ```

    ```text title="Expected Output"
    URLs: discord://abc123@123456789
          slack://hook:T00000000-B00000000-XXXXXXXXXXXXXXXXXXXXXXXX@webhook
    Message: Hello!
    Notification sent
    Notification sent
    ```
<!-- markdownlint-restore -->
