# Matrix

Matrix is an open protocol for real-time communication. It is designed to allow users with accounts at one communications service provider to communicate with users of a different service provider via online chat. Upstream docs: <https://matrix.org/>

## Features

- __Matrix API v3 Compliance__: Implements the Matrix Client-Server API v3 specification
- __Token-based Authentication__: Use pre-generated access tokens or password-based login
- __Room Management__: Send messages to specific rooms or all joined rooms
- __Auto-Join__: Automatically accepts room invites when rooms are explicitly specified in the configuration
- __TLS Support__: Secure connections with optional TLS disabling
- __Idempotent Sending__: Uses transaction IDs to prevent duplicate messages

## URL Format

!!! Info ""
    matrix://[__`user`__:__`password`__@]__`host`__[:__`port`__]/[?rooms=__`!roomID1`__[,__`roomAlias2`__]][&disableTLS=yes]

If the port is omitted, the default port 443 (HTTPS) is used.

--8<-- "docs/services/chat/matrix/config.md"

## Authentication

The Matrix service supports two authentication methods:

### Token-based Login (Default)

If no `user` is specified, the `password` is treated as an authentication token.
This allows you to use a pre-generated access token from your Matrix server, which is useful for CI/CD pipelines or when you don't want to store your password.
Simply omit the `user` parameter and provide your access token as the `password`.

### Password Login

If a `user` and `password` are both supplied, the service will attempt to authenticate using the `m.login.password` flow (if supported by your server).

## Matrix API v3 Compliance

The Matrix service implements the Matrix Client-Server API v3 specification.
All API calls use v3 endpoints:

```text
/_matrix/client/v3/...
```

### HTTP Method

Messages are sent using the __PUT__ method to `/send/{roomId}/{txnId}`. This follows the v3 specification for idempotent message sending, ensuring that retrying a request due to network issues won't result in duplicate messages.

### Transaction IDs

The service automatically generates and includes a transaction ID (`txnId`) with each message send request.

This provides:

- __Deduplication__: If a request is retried, the server recognizes the same transaction ID and doesn't create duplicate messages
- __Reliability__: Safe to retry failed requests without worrying about message duplication

### Authorization Header

The service passes the access token via the `Authorization: Bearer <token>` HTTP header, which is the recommended method per the v3 specification.

!!! Note "Backward Compatibility"
    For backward compatibility with older configurations, the access token is also set as a URL query parameter (`access_token=...`) when using direct token authentication (i.e., when no `user` is provided). However, all API requests use the Authorization header as the primary authentication method.

!!! Example "Authorization Header"
    ```bash
    Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
    ```

## Title Parameter

The Matrix service now supports the `title` parameter. When provided, the title is prepended to the message body.

!!! Example "With Title"

    ```bash
    shoutrrr send --title "Notification Title" --message "This is the message body" matrix://...
    ```

    Output in Matrix:

    ```text
    Notification Title

    This is the message body
    ```

## Rooms

If `rooms` are *not* specified, the service will send the message to all the rooms that the user has currently joined.

Otherwise, the service will only send the message to the specified rooms.
If the user is *not* in any of those rooms, but have been invited to it, it will automatically accept that invite.

!!! Warning "Room Joining"
    The service will __not__ join any rooms unless they are explicitly specified in `rooms`.
    If you need the user to join those rooms, you can send a notification with `rooms` explicitly set once.

### Room Lookup

Rooms specified in `rooms` will be treated as room IDs if they start with a `!` and used directly to identify rooms.
If they have no such prefix (or use a *correctly escaped* `#`) they will instead be treated as aliases, and a directory lookup will be used to resolve their corresponding IDs.

__Auto-prepending__: For convenience, rooms that don't start with `#` or `!` will automatically have `#` prepended.
For example, `rooms=general` becomes `rooms=#general`.
This allows you to use simple channel names without worrying about the prefix.

You can use either `rooms` (for multiple rooms) or `room` (for a single room) - both parameters work identically.

!!! Note "URL Encoding"
    Don't use unescaped `#` for the channel aliases as that will be treated as the `fragment` part of the URL.
    Either omit them or URL encode them, I.E. `rooms=%23alias:server` or `rooms=alias:server`

## TLS

If you do not have TLS enabled on the server you can disable it by providing `disableTLS=yes`.
This will effectively use `http` instead of `https` for the API calls.

!!! Warning "Security Risk"
    Disabling TLS exposes your credentials and messages in plain text over the network.
    Only use this option in trusted local networks or testing environments.

## Examples

!!! Example "Basic Notification"
    ```uri
    matrix://user:token@matrix.example.com
    ```

!!! Example "To Specific Room"
    ```uri
    matrix://user:token@matrix.example.com?rooms=!roomID:matrix.example.com
    ```

!!! Example "With Room Alias"
    ```uri
    matrix://user:token@matrix.example.com?rooms=#general:matrix.example.com
    ```

!!! Example "Multiple Rooms"
    ```uri
    matrix://user:token@matrix.example.com?rooms=!room1:matrix.example.com,#room2:matrix.example.com
    ```

!!! Example "With Custom Port"
    ```uri
    matrix://user:token@matrix.example.com:8448?rooms=!roomID:matrix.example.com
    ```

!!! Example "Without TLS (Not Recommended)"
    ```uri
    matrix://user:token@matrix.example.com?disableTLS=yes&rooms=!roomID:matrix.example.com
    ```
