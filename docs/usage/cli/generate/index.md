# Generate

## Overview

The `generate` command creates a notification service URL by guiding the user through an interactive process or using provided properties. If no service is specified, the command displays the list of supported services and exits.

## Usage

```bash
shoutrrr generate [FLAGS] <SERVICE> [GENERATOR]
```

| Flag                         | Description                                                                                                                                                                              |
|------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `-g, --generator string`     | Specifies the generator to use (e.g., `basic`, `oauth2`, `telegram`). Defaults to a service-specific generator if available, or `basic` otherwise.                                       |
| `-p, --property stringArray` | Provides configuration properties in `key=value` format (e.g., `token=abc123`). Multiple properties can be specified by repeating the flag. Invalid properties are reported but ignored. |
| `-s, --service string`       | Specifies the notification service to generate a URL for (e.g., `discord`, `smtp`, `telegram`). Can also be provided as the first positional argument.                                   |
| `-x, --show-sensitive`       | Displays sensitive data (e.g., tokens, passwords) in the generated URL. By default, sensitive fields are masked with `REDACTED` for security.                                            |

!!! Note
    The `SERVICE` can be supplied as the first positional argument or using the `-s` flag. The `GENERATOR` can be supplied as the second positional argument or using the `-g` flag. If no generator is specified, a service-specific generator is used if available; otherwise, the `basic` generator is used.

### Generators

#### Basic

The default generator that dynamically prompts for service configuration fields.

- Inspects service struct tags (`key`, `desc`, `default`) to generate prompts.
- Handles required fields by reprompting if values are missing.
- Integrates with `-p` properties to skip prompts for prefilled fields.

!!! Example
    ```bash
    shoutrrr generate discord -g basic
    ```

#### OAuth2

Specialized generator for OAuth2 authentication in SMTP services.

- Supports JSON credential files (specified as a positional argument) or interactive prompts for details like Client ID, Client Secret, and Auth URL.
- Generates an authentication URL and exchanges verification codes for access tokens.
- Configures Gmail-specific defaults (port 587, STARTTLS, sender email as `FromAddress` and `ToAddresses`).

!!! Example
    ```bash
    shoutrrr generate smtp oauth2 -p provider=gmail credentials.json
    ```

#### Telegram

Interactive generator tailored for Telegram bot and chat configuration.

- Prompts for bot token (from `@BotFather`) and fetches bot info.
- Listens for real-time messages to collect chat IDs (PMs, groups, channels).
- Supports dynamic chat addition via user interaction.
- Generates URL with token and selected chat IDs.

!!! Example
    ```bash
    shoutrrr generate telegram -g telegram
    ```

### Properties

Properties prefill configuration fields, reducing or eliminating interactive prompts.
For example, `-p token=abc123` sets the token field without prompting.

!!! Example
    ```bash
    shoutrrr generate discord -p token=abc123
    ```

### Services

Services like `telegram` and `smtp` (with `oauth2`) use specialized generators for a tailored experience, while others use the `basic` generator.

!!! Example
    ```bash
    shoutrrr generate -s discord
    ```

### Show Sensitive

Use this flag to view the full URL, including sensitive fields like tokens or passwords, for debugging or verification.

!!! Example
    ```bash
    shoutrrr generate smtp oauth2 -x -p provider=gmail credentials.json
    ```

## Examples

<!-- markdownlint-disable -->
### Generate a Telegram URL with the Telegram Generator
!!! Example
    ```bash
    shoutrrr generate telegram -g telegram
    ```

    ```text
    To start we need your bot token. If you haven't created a bot yet, you can use this link:
      https://t.me/botfather?start

    Enter your bot token: 110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw
    Fetching bot info...

    Okay! @MyBot will listen for any messages in PMs and group chats it is invited to.
    Waiting for messages to arrive...
    Got Message 'Hello' from @User in private chat -100123456789
    Added new chat User!
    Got %0 chat ID(s) so far. Want to add some more? [Yes]: No
    ```

    ```text
    Cleaning up the bot session...
    Selected chats:
      -100123456789 (private) User

    URL: telegram://110201543:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw@telegram?chats=-100123456789
    ```

### Generate a Discord URL with the Basic Generator

!!! Example
    ```bash
    shoutrrr generate discord
    ```

    ```text
    Generating URL for discord using basic generator

    Token: abc123
    WebhookID: 123456789
    ```

    ```text
    URL: discord://abc123@123456789
    ```

### Generate an SMTP URL with the OAuth2 Generator for Gmail

!!! Example
    ```bash
    shoutrrr generate smtp oauth2 -p provider=gmail credentials.json
    ```

    ```text
    Generating URL for smtp using oauth2 generator

    Visit the following URL to authenticate:
    https://accounts.google.com/o/oauth2/auth?...
    Enter verification code: 4/0AX4Xf...
    Enter sender e-mail: user@example.com
    ```

    ```text
    URL: smtp://user@example.com:REDACTED@smtp.gmail.com:587/?auth=OAuth2&fromaddress=user@example.com&toaddresses=user@example.com&fromname=Shoutrrr&usehtml=true&usestarttls=true
    ```
<!-- markdownlint-restore -->
