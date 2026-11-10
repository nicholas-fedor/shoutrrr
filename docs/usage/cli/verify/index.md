# Verify

## Overview

The `verify` command checks the validity of a notification service URL.

## Usage

```bash title="Verify Command Syntax"
shoutrrr verify [FLAGS] [URL]
```

| Flag                     | Description                                                                                                                |
|--------------------------|----------------------------------------------------------------------------------------------------------------------------|
| `-h, --help`             | Displays help for the `verify` command.                                                                                    |
| `-u, --url stringArray`  | Specifies the notification service URL to verify.                                                                          |
| `-x, --show-sensitive`   | Displays sensitive values (e.g., tokens, passwords) in the configuration. By default, they are masked with `REDACTED`. |

!!! Note
    Exactly one URL is required. It can be given with `--url`, as a positional argument, or with the `SHOUTRRR_URL` environment variable. Giving it both with `--url` and as an argument is an error, and either takes precedence over the environment variable. The command validates the URL format and service configuration, reporting errors for issues like unknown services or invalid URL formats.

### URL

- Specifies the service URL to validate. The URL is parsed to identify the service, and its configuration is checked for correctness.

## Examples

Replace `TOKEN` and `WEBHOOK_ID` with the token and ID of a Discord webhook, which the [Discord](../../../services/chat/discord/index.md) page shows how to find.

<!-- markdownlint-disable -->
### Verify a Valid Discord URL

!!! Example
    ```bash title="Verify Discord URL"
    shoutrrr verify --url "discord://TOKEN@WEBHOOK_ID"
    ```

    ```text title="Expected Output"
    Token: REDACTED
    WebhookID: WEBHOOK_ID
    ```

    Add `--show-sensitive` to display the token.

### Verify an Invalid URL

!!! Example
    ```bash title="Verify Invalid URL"
    shoutrrr verify --url "invalid://TOKEN"
    ```

    ```text title="Expected Output"
    error verifying URL: service not recognized
    ```

### Verify a Malformed URL

!!! Example
    ```bash title="Verify Malformed URL"
    shoutrrr verify --url "discord://"
    ```

    ```text title="Expected Output"
    error verifying URL: invalid URL format
    ```
<!-- markdownlint-restore -->
