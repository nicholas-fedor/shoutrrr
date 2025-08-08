# Send

## Overview

The `send` command delivers a notification using one or more specified service URLs.

## Usage

```bash title="Send Command Syntax"
shoutrrr send [FLAGS]
```

| Flag                    | Description                                                               |
|-------------------------|---------------------------------------------------------------------------|
| `-h, --help`            | Displays help for the `send` command.                                     |
| `-m, --message string`  | Specifies the message to send. Use `-` to read the message from stdin.    |
| `-t, --title string`    | Sets the title for services that support it (optional).                   |
| `-u, --url stringArray` | Specifies the notification service URL(s). Multiple URLs can be provided. |
| `-v, --verbose`         | Enables verbose output, logging URLs, message, and title to stderr.       |

!!! Note
    The `--url` and `--message` flags are required. Use `--message -` to read the message from stdin. Duplicate URLs are automatically removed.

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
    shoutrrr send --url "discord://abc123@123456789" --url "slack://token@team/channel" --message "Hello!" --verbose
    ```

    ```text title="Expected Output"
    URLs: discord://abc123@123456789
          slack://token@team/channel
    Message: Hello!
    Notification sent
    Notification sent
    ```
<!-- markdownlint-restore -->
