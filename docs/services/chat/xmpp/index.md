# XMPP

XMPP (Extensible Messaging and Presence Protocol) is an open protocol for near-real-time messaging.
Shoutrrr's XMPP service enables sending fire-and-forget chat and multi-user chat (MUC) notifications to XMPP servers, such as [ejabberd](https://ejabberd.im/) and [Prosody](https://prosody.im/).
Upstream docs: <https://xmpp.org/>

## URL Formats

The XMPP service supports two URL schemes for connection security:

- __`xmpp://`__: STARTTLS on port 5222 by default

!!! Info ""
    xmpp://__`user`__:__`password`__@__`host`__[:__`port`__]/?to=__`jid`__[&rooms=__`roomjid`__]

- __`xmpps://`__: implicit TLS on port 5223 by default

!!! Info ""
    xmpps://__`user`__:__`password`__@__`host`__[:__`port`__]/?to=__`jid`__[&rooms=__`roomjid`__]

--8<-- "docs/services/chat/xmpp/config.md"

At least one of `to` or `rooms` is required.

## Authentication JID

The URL userinfo is the login identity:

- A localpart (`alice`) becomes `alice@host`.
- A full JID (`alice%40jabber.example.com`) is used as-is, while `host` remains the TCP dial target.

```
xmpp://alice%40jabber.example.com:secret@xmpp.example.com/?to=bob@jabber.example.com
```

## TLS

- Use `xmpp://` for STARTTLS (default port 5222).
- Use `xmpps://` for implicit TLS (default port 5223).
- Use `disabletls=yes` only with `xmpp://` for plaintext on a trusted network. Combining it with `xmpps://` is rejected.
- Use `skiptlsverify=yes` to skip certificate verification (self-signed or internal TLS). TLS 1.2 remains the minimum.

Plaintext SASL is used only when `disabletls=yes` is set.
Encrypted connections prefer SCRAM-SHA-256, then SCRAM-SHA-1, then PLAIN.

## Recipients

- __`to`__: comma-separated JIDs sent as type=chat.
- __`rooms`__: comma-separated MUC JIDs. The service joins, sends type=groupchat, and leaves.
- __`nick`__: MUC nickname. Defaults to the auth JID localpart.
- __`roompassword`__: password applied to every room in the URL.

A `title` is prepended to the message body. The MUC subject is never changed.

## Examples

### Direct chat

```
xmpp://alice:secret@xmpp.example.com/?to=bob@example.com
```

### Multiple chat recipients and a room

```
xmpp://alice:secret@xmpp.example.com/?to=bob@example.com,carol@example.com&rooms=alerts@conference.example.com
```

### Implicit TLS with a self-signed certificate

```
xmpps://alice:secret@xmpp.example.com/?to=bob@example.com&skiptlsverify=yes
```

### Password-protected room

```
xmpp://alice:secret@xmpp.example.com/?rooms=secret@conference.example.com&nick=shoutrrr&roompassword=s3cret
```
