# Matrix

!!! Note "Usage of the `title` parameter"
    Use a custom message format to specify the intended title as part of the message.
    Matrix will discard any information put in the `title` parameter as the service has no analog to a title.

## URL Format

*matrix://__`user`__:__`password`__@__`host`__:__`port`__/[?rooms=__`!roomID1`__[,__`roomAlias2`__]][&disableTLS=yes]*

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

## Rooms

If `rooms` are *not* specified, the service will send the message to all the rooms that the user has currently joined.

Otherwise, the service will only send the message to the specified rooms.
If the user is *not* in any of those rooms, but have been invited to it, it will automatically accept that invite.

__Note__: The service will __not__ join any rooms unless they are explicitly specified in `rooms`.
If you need the user to join those rooms, you can send a notification with `rooms` explicitly set once.

### Room Lookup

Rooms specified in `rooms` will be treated as room IDs if they start with a `!` and used directly to identify rooms.
If they have no such prefix (or use a *correctly escaped* `#`) they will instead be treated as aliases, and a directory lookup will be used to resolve their corresponding IDs.

__Auto-prepending__: For convenience, rooms that don't start with `#` or `!` will automatically have `#` prepended.
For example, `rooms=general` becomes `rooms=#general`.
This allows you to use simple channel names without worrying about the prefix.

You can use either `rooms` (for multiple rooms) or `room` (for a single room) - both parameters work identically.

__Note__: Don't use unescaped `#` for the channel aliases as that will be treated as the `fragment` part of the URL.
Either omit them or URL encode them, I.E. `rooms=%23alias:server` or `rooms=alias:server`

### TLS

If you do not have TLS enabled on the server you can disable it by providing `disableTLS=yes`.
This will effectively use `http` intead of `https` for the API calls.
