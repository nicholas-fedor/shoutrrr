# Verify

## Overview

The `verify` command checks the validity of a notification service URL.

## Usage

```bash title="Verify Command Syntax"
shoutrrr verify [FLAGS]
```

| Flag                | Description                                           |
|---------------------|-------------------------------------------------------|
| `-h, --help`        | Displays help for the `verify` command.                |
| `-u, --url string`  | Specifies the notification service URL to verify.      |

!!! Note
    The `--url` flag is required. The command validates the URL format and service configuration, reporting errors for issues like unknown services or invalid URL formats.

### URL

- Specifies the service URL to validate. The URL is parsed to identify the service, and its configuration is checked for correctness.

## Examples

<!-- markdownlint-disable -->
### Verify a Valid Discord URL

!!! Example
    ```bash title="Verify Discord URL"
    shoutrrr verify --url "discord://abc123@123456789"
    ```

    ```text title="Expected Output"
    Token: abc123
    WebhookID: 123456789
    ```

### Verify an Invalid URL

!!! Example
    ```bash title="Verify Invalid URL"
    shoutrrr verify --url "invalid://abc123"
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
