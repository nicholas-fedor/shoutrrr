# Ntfy

Upstream docs: <https://docs.ntfy.sh/publish/>

## URL Format

!!! info ""
    ntfy://__`host`__/__`topic`__/?priority=__`priority`__&tags=__`tag1`__,__`tag2`__&title=__`title`__

--8<-- "docs/services/push/ntfy/config.md"

## Getting Started

Ntfy supports both the public [ntfy.sh](https://ntfy.sh) service and self-hosted instances. For the public service, simply use `ntfy.sh` as the host. For self-hosted instances, specify your server's hostname and port.

!!! example "Public ntfy.sh service"
    ```uri
    ntfy://ntfy.sh/mytopic
    ```

!!! example "Self-hosted instance"
    ```uri
    ntfy://my-ntfy-server.com:8080/mytopic
    ```

Topics are user-defined and can be any string. For authentication, include username and password in the URL if your ntfy server requires it.

## Features

Ntfy offers several advanced features that enhance notification capabilities:

- __Action Buttons__: Add interactive buttons to notifications that can trigger HTTP requests, open URLs, or broadcast intents
- __Attachments__: Send files and images as attachments via URL or direct upload
- __Delayed Delivery__: Schedule notifications for future delivery using timestamps or duration strings
- __Email Notifications__: Forward notifications to email addresses for additional reach
- __Priority Levels__: Set message priority from 1 (min) to 5 (max) to control notification behavior
- __Tags and Emojis__: Add tags that automatically map to emojis for visual enhancement
- __Click Actions__: Specify URLs that open when notifications are tapped
- __Message Caching__: Control whether messages are cached for offline delivery

For complete feature documentation, see the [ntfy publish documentation](https://docs.ntfy.sh/publish/).

## Optional Parameters

Commonly used parameters can be added as query parameters to customize notifications:

- __`priority`__: Message priority level (1-5, where 1=min, 3=default, 5=max)
- __`title`__: Custom title for the notification
- __`tags`__: Comma-separated list of tags (e.g., `warning,alert` maps to ⚠️🚨)
- __`actions`__: JSON array of action buttons (see [action buttons docs](https://docs.ntfy.sh/publish/#action-buttons))
- __`delay`__: Schedule delivery (e.g., `5m` for 5 minutes, `2023-12-31T23:59:59Z` for specific time)
- __`email`__: Email address to send notification to
- __`click`__: URL to open when notification is clicked

!!! example "High-priority notification with tags"
    ```uri
    ntfy://ntfy.sh/alerts/?priority=5&tags=warning,fire&title=System+Alert
    ```

!!! note
    Action buttons require JSON formatting. See the [upstream documentation](https://docs.ntfy.sh/publish/#action-buttons) for detailed syntax.

## TLS Configuration

Ntfy supports two TLS-related configuration options to handle different security scenarios:

- **`DisableTLSVerification`**: When set to `true`, disables TLS certificate verification. This allows connections to servers with self-signed or invalid certificates, but still uses the HTTPS protocol. Use this option when you trust the server but the certificate cannot be verified by standard means.

- **`DisableTLS`**: When set to `true`, disables TLS entirely and forces the use of the HTTP scheme instead of HTTPS. This makes the connection unencrypted and should only be used in secure, trusted environments or for testing purposes.

!!! warning
    Using either of these options reduces the security of your connection. `DisableTLS` is particularly insecure as it transmits data in plain text. Only use these options when necessary and in controlled environments.

## Examples

!!! example "Basic notification"
    ```uri
    ntfy://ntfy.sh/mytopic
    ```

!!! example "With authentication"
    ```uri
    ntfy://username:password@ntfy.sh/privatetopic
    ```

!!! example "With parameters"
    ```uri
    ntfy://ntfy.sh/updates/?priority=4&tags=info,computer&title=Server+Update
    ```

!!! example "Delayed notification"
    ```uri
    ntfy://ntfy.sh/reminders/?delay=1h&title=Meeting+in+1+hour
    ```

!!! example "With action button"
    ```uri
    ntfy://ntfy.sh/tasks/?actions=[{"action":"view","label":"Open Dashboard","url":"https://dashboard.example.com"}]&title=Task+Completed
    ```
